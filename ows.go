package main

/* ows is a web server implementing the WMS, WCS and WPS protocols
   to serve geospatial data. This server is intended to be
   consumed directly by users and exposes a series of
   functionalities through the GetCapabilities.xml document.
   Configuration of the server is specified in the config.json
   file where features such as layers or color scales can be
   defined.
   This server depends on two other services to operate: the
   index server which registers the files involved in one operation
   and the warp server which performs the actual rendering of
   a tile. */

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	proc "github.com/nci/gsky/processor"
	"github.com/nci/gsky/utils"

	_ "net/http/pprof"

	geo "github.com/nci/geometry"
)

// Global variable to hold the values specified
// on the config.json document.
var configMap map[string]*utils.Config

var (
	port           = flag.Int("p", 8080, "Server listening port.")
	validateConfig = flag.Bool("check_conf", false, "Validate server config files.")
	verbose        = flag.Bool("v", false, "Verbose mode for more server outputs.")
)

var reWMSMap map[string]*regexp.Regexp
var reWCSMap map[string]*regexp.Regexp
var reWPSMap map[string]*regexp.Regexp

var (
	Error *log.Logger
	Info  *log.Logger
)

// init initialises the Error logger, checks
// required files are in place  and sets Config struct.
// This is the first function to be called in main.
func init() {
	Error = log.New(os.Stderr, "OWS: ", log.Ldate|log.Ltime|log.Lshortfile)
	Info = log.New(os.Stdout, "OWS: ", log.Ldate|log.Ltime|log.Lshortfile)

	flag.Parse()

	filePaths := []string{
		utils.DataDir + "/static/index.html",
		utils.DataDir + "/templates/WMS_GetCapabilities.tpl",
		utils.DataDir + "/templates/WMS_DescribeLayer.tpl",
		utils.DataDir + "/templates/WMS_ServiceException.tpl",
		utils.DataDir + "/templates/WPS_DescribeProcess.tpl",
		utils.DataDir + "/templates/WPS_Execute.tpl",
		utils.DataDir + "/templates/WPS_GetCapabilities.tpl",
		utils.DataDir + "/templates/WCS_GetCapabilities.tpl",
		utils.DataDir + "/templates/WCS_DescribeCoverage.tpl"}

	for _, filePath := range filePaths {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			panic(err)
		}
	}

	confMap, err := utils.LoadAllConfigFiles(utils.EtcDir)
	if err != nil {
		Error.Printf("Error in loading config files: %v\n", err)
		panic(err)
	}

	if *validateConfig {
		os.Exit(0)
	}

	configMap = confMap

	utils.WatchConfig(Info, Error, &configMap)

	reWMSMap = utils.CompileWMSRegexMap()
	reWCSMap = utils.CompileWCSRegexMap()
	reWPSMap = utils.CompileWPSRegexMap()

}

func serveWMS(ctx context.Context, params utils.WMSParams, conf *utils.Config, reqURL string, w http.ResponseWriter) {

	if params.Request == nil {
		http.Error(w, "Malformed WMS, a Request field needs to be specified", 400)
		return
	}

	switch *params.Request {
	case "GetCapabilities":
		if params.Version != nil && !utils.CheckWMSVersion(*params.Version) {
			http.Error(w, fmt.Sprintf("This server can only accept WMS requests compliant with version 1.1.1 and 1.3.0: %s", reqURL), 400)
			return
		}

		err := utils.ExecuteWriteTemplateFile(w, conf,
			utils.DataDir+"/templates/WMS_GetCapabilities.tpl")
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	case "GetFeatureInfo":
		x, y, err := utils.GetCoordinates(params)
		if err != nil {
			Error.Printf("%s\n", err)
			http.Error(w, fmt.Sprintf("Malformed WMS GetFeatureInfo request: %v", err), 400)
			return
		}
		resp := fmt.Sprintf(`{"type":"FeatureCollection","totalFeatures":"unknown","features":[{"type":"Feature","id":"","geometry":null,"properties":{"x":%f, "y":%f}}],"crs":null}`, x, y)
		w.Write([]byte(resp))

	case "DescribeLayer":
		idx, err := utils.GetLayerIndex(params, conf)
		if err != nil {
			Error.Printf("%s\n", err)
			http.Error(w, fmt.Sprintf("Malformed WMS DescribeLayer request: %v", err), 400)
			return
		}

		err = utils.ExecuteWriteTemplateFile(w, conf.Layers[idx],
			utils.DataDir+"/templates/WMS_DescribeLayer.tpl")
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

	case "GetMap":
		if params.Version == nil || !utils.CheckWMSVersion(*params.Version) {
			http.Error(w, fmt.Sprintf("This server can only accept WMS requests compliant with version 1.1.1 and 1.3.0: %s", reqURL), 400)
			return
		}

		idx, err := utils.GetLayerIndex(params, conf)
		if err != nil {
			Error.Printf("%s\n", err)
			http.Error(w, fmt.Sprintf("Malformed WMS GetMap request: %v", err), 400)
			return
		}
		if params.Time == nil {
			currentTime, err := time.Parse(utils.ISOFormat, conf.Layers[idx].Dates[len(conf.Layers[idx].Dates)-1])
			if err != nil {
				http.Error(w, fmt.Sprintf("Cannot find a valid date to proceed with the request: %s", reqURL), 400)
				return
			}
			params.Time = &currentTime
		}
		if params.CRS == nil {
			http.Error(w, fmt.Sprintf("Request %s should contain a valid ISO 'crs/srs' parameter.", reqURL), 400)
			return
		}
		if len(params.BBox) != 4 {
			http.Error(w, fmt.Sprintf("Request %s should contain a valid 'bbox' parameter.", reqURL), 400)
			return
		}
		if params.Height == nil || params.Width == nil {
			http.Error(w, fmt.Sprintf("Request %s should contain valid 'width' and 'height' parameters.", reqURL), 400)
			return
		}

		if strings.ToUpper(*params.CRS) == "EPSG:4326" && *params.Version == "1.3.0" {
			params.BBox = []float64{params.BBox[1], params.BBox[0], params.BBox[3], params.BBox[2]}
		}

		if strings.ToUpper(*params.CRS) == "CRS:84" && *params.Version == "1.3.0" {
			*params.CRS = "EPSG:4326"
		}

		var endTime *time.Time
		if conf.Layers[idx].Accum == true {
			step := time.Minute * time.Duration(60*24*conf.Layers[idx].StepDays+60*conf.Layers[idx].StepHours+conf.Layers[idx].StepMinutes)
			eT := params.Time.Add(step)
			endTime = &eT
		}

		xRes := (params.BBox[2] - params.BBox[0]) / float64(*params.Width)
		if conf.Layers[idx].ZoomLimit != 0.0 && xRes > conf.Layers[idx].ZoomLimit {
			out, err := utils.GetEmptyTile(utils.DataDir+"/zoom.png", *params.Height, *params.Width)
			if err != nil {
				Info.Printf("Error in the utils.GetEmptyTile(zoom.png): %v\n", err)
				http.Error(w, err.Error(), 500)
				return
			}
			w.Write(out)
			return
		}

		geoReq := &proc.GeoTileRequest{ConfigPayLoad: proc.ConfigPayLoad{NameSpaces: conf.Layers[idx].RGBProducts,
			Mask:    conf.Layers[idx].Mask,
			Palette: conf.Layers[idx].Palette,
			ScaleParams: proc.ScaleParams{Offset: conf.Layers[idx].OffsetValue,
				Scale: conf.Layers[idx].ScaleValue,
				Clip:  conf.Layers[idx].ClipValue,
			},
			ZoomLimit:       conf.Layers[idx].ZoomLimit,
			PolygonSegments: conf.Layers[idx].WmsPolygonSegments,
			Timeout:         conf.Layers[idx].WmsTimeout,
			GrpcConcLimit:   conf.Layers[idx].GrpcWmsConcPerNode,
		},
			Collection: conf.Layers[idx].DataSource,
			CRS:        *params.CRS,
			BBox:       params.BBox,
			Height:     *params.Height,
			Width:      *params.Width,
			StartTime:  params.Time,
			EndTime:    endTime,
		}

		ctx, ctxCancel := context.WithCancel(ctx)
		defer ctxCancel()
		errChan := make(chan error)
		tp := proc.InitTilePipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, conf.Layers[idx].MaxGrpcRecvMsgSize, conf.Layers[idx].WmsPolygonShardConcLimit, errChan)
		select {
		case res := <-tp.Process(geoReq):
			scaleParams := utils.ScaleParams{Offset: geoReq.ScaleParams.Offset,
				Scale: geoReq.ScaleParams.Scale,
				Clip:  geoReq.ScaleParams.Clip,
			}

			norm, err := utils.Scale(res, scaleParams)
			if err != nil {
				Info.Printf("Error in the utils.Scale: %v\n", err)
				http.Error(w, err.Error(), 500)
				return
			}

			if norm[0].Width == 0 || norm[0].Height == 0 {
				out, err := utils.GetEmptyTile(utils.DataDir+"/data_unavailable.png", *params.Height, *params.Width)
				if err != nil {
					Info.Printf("Error in the utils.GetEmptyTile(data_unavailable.png): %v\n", err)
					http.Error(w, err.Error(), 500)
				} else {
					w.Write(out)
				}
				return
			}

			out, err := utils.EncodePNG(norm, conf.Layers[idx].Palette)
			if err != nil {
				Info.Printf("Error in the utils.EncodePNG: %v\n", err)
				http.Error(w, err.Error(), 500)
				return
			}
			w.Write(out)
		case err := <-errChan:
			Info.Printf("Error in the pipeline: %v\n", err)
			http.Error(w, err.Error(), 500)
		case <-ctx.Done():
			Error.Printf("Context cancelled with message: %v\n", ctx.Err())
			http.Error(w, ctx.Err().Error(), 500)
		}
		return

	case "GetLegendGraphic":
		idx, err := utils.GetLayerIndex(params, conf)
		if err != nil {
			Error.Printf("%s\n", err)
			if len(params.Layers) > 0 {
				utils.ExecuteWriteTemplateFile(w, params.Layers[0],
					utils.DataDir+"/templates/WMS_ServiceException.tpl")
			} else {
				http.Error(w, err.Error(), 400)
			}
			return
		}

		b, err := ioutil.ReadFile(conf.Layers[idx].LegendPath)
		if err != nil {
			Error.Printf("Error reading legend image: %v, %v\n", conf.Layers[idx].LegendPath, err)
			http.Error(w, "Legend graphics not found", 500)
			return
		}
		w.Write(b)

	default:
		http.Error(w, fmt.Sprintf("%s not recognised.", *params.Request), 400)
	}

}

func serveWCS(ctx context.Context, params utils.WCSParams, conf *utils.Config, reqURL string, w http.ResponseWriter, query map[string][]string) {
	if params.Request == nil {
		http.Error(w, "Malformed WCS, a Request field needs to be specified", 400)
	}

	switch *params.Request {
	case "GetCapabilities":
		if params.Version != nil && !utils.CheckWCSVersion(*params.Version) {
			http.Error(w, fmt.Sprintf("This server can only accept WCS requests compliant with version 1.0.0: %s", reqURL), 400)
			return
		}

		// TODO this might be solved copying the Layer slice
		newConf := *conf
		newConf.Layers = make([]utils.Layer, len(newConf.Layers))
		for i, layer := range conf.Layers {
			newConf.Layers[i] = layer
			newConf.Layers[i].Dates = []string{newConf.Layers[i].Dates[0], newConf.Layers[i].Dates[len(newConf.Layers[i].Dates)-1]}
		}

		err := utils.ExecuteWriteTemplateFile(w, &newConf, utils.DataDir+"/templates/WCS_GetCapabilities.tpl")
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

	case "DescribeCoverage":
		idx, err := utils.GetCoverageIndex(params, conf)
		if err != nil {
			Info.Printf("Error in the pipeline: %v\n", err)
			http.Error(w, fmt.Sprintf("Malformed WMS DescribeCoverage request: %v", err), 400)
			return
		}

		err = utils.ExecuteWriteTemplateFile(w, conf.Layers[idx], utils.DataDir+"/templates/WCS_DescribeCoverage.tpl")
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

	case "GetCoverage":
		if params.Version == nil || !utils.CheckWCSVersion(*params.Version) {
			http.Error(w, fmt.Sprintf("This server can only accept WCS requests compliant with version 1.0.0: %s", reqURL), 400)
			return
		}

		idx, err := utils.GetCoverageIndex(params, conf)
		if err != nil {
			http.Error(w, fmt.Sprintf("%v: %s", err, reqURL), 400)
			return
		}

		if params.Time == nil {
			currentTime, err := time.Parse(utils.ISOFormat, conf.Layers[idx].Dates[len(conf.Layers[idx].Dates)-1])
			if err != nil {
				http.Error(w, fmt.Sprintf("Cannot find a valid date to proceed with the request: %s", reqURL), 400)
				return
			}
			params.Time = &currentTime
		}
		if params.CRS == nil {
			http.Error(w, fmt.Sprintf("Request %s should contain a valid ISO 'crs/srs' parameter.", reqURL), 400)
			return
		}
		if len(params.BBox) != 4 {
			http.Error(w, fmt.Sprintf("Request %s should contain a valid 'bbox' parameter.", reqURL), 400)
			return
		}
		if params.Height == nil || params.Width == nil {
			http.Error(w, fmt.Sprintf("Request %s should contain valid 'width' and 'height' parameters.", reqURL), 400)
			return
		}
		if params.Format == nil {
			http.Error(w, fmt.Sprintf("Unsupported encoding format"), 400)
			return
		}

		var endTime *time.Time
		if conf.Layers[idx].Accum == true {
			step := time.Minute * time.Duration(60*24*conf.Layers[idx].StepDays+60*conf.Layers[idx].StepHours+conf.Layers[idx].StepMinutes)
			eT := params.Time.Add(step)
			endTime = &eT
		}

		maxXTileSize := 1024
		maxYTileSize := 1024
		checkpointThreshold := 300
		minTilesPerWorker := 5

		getGeoTileRequest := func(width int, height int, bbox []float64, offX int, offY int) *proc.GeoTileRequest {
			geoReq := &proc.GeoTileRequest{ConfigPayLoad: proc.ConfigPayLoad{NameSpaces: conf.Layers[idx].RGBProducts,
				Mask:    conf.Layers[idx].Mask,
				Palette: conf.Layers[idx].Palette,
				ScaleParams: proc.ScaleParams{Offset: conf.Layers[idx].OffsetValue,
					Scale: conf.Layers[idx].ScaleValue,
					Clip:  conf.Layers[idx].ClipValue,
				},
				ZoomLimit:       conf.Layers[idx].ZoomLimit,
				PolygonSegments: conf.Layers[idx].WcsPolygonSegments,
				Timeout:         conf.Layers[idx].WcsTimeout,
				GrpcConcLimit:   conf.Layers[idx].GrpcWcsConcPerNode,
			},
				Collection: conf.Layers[idx].DataSource,
				CRS:        *params.CRS,
				BBox:       bbox,
				Height:     height,
				Width:      width,
				StartTime:  params.Time,
				EndTime:    endTime,
				OffX:       offX,
				OffY:       offY,
			}

			return geoReq
		}

		wcsWorkerNodes := conf.ServiceConfig.OWSClusterNodes
		workerTileRequests := [][]*proc.GeoTileRequest{}

		_, isWorker := query["wbbox"]
		if !isWorker {
			if *params.Width > maxXTileSize || *params.Height > maxYTileSize {
				tmpTileRequests := []*proc.GeoTileRequest{}
				xRes := (params.BBox[2] - params.BBox[0]) / float64(*params.Width)
				yRes := (params.BBox[3] - params.BBox[1]) / float64(*params.Height)

				for x := 0; x < *params.Width; x += maxXTileSize {
					for y := 0; y < *params.Height; y += maxYTileSize {
						yMin := params.BBox[1] + float64(y)*yRes
						yMax := math.Min(params.BBox[1]+float64(y+maxYTileSize)*yRes, params.BBox[3])
						xMin := params.BBox[0] + float64(x)*xRes
						xMax := math.Min(params.BBox[0]+float64(x+maxXTileSize)*xRes, params.BBox[2])

						tileXSize := int(.5 + (xMax-xMin)/xRes)
						tileYSize := int(.5 + (yMax-yMin)/yRes)

						geoReq := getGeoTileRequest(tileXSize, tileYSize, []float64{xMin, yMin, xMax, yMax}, x, *params.Height-y-tileYSize)
						tmpTileRequests = append(tmpTileRequests, geoReq)
					}
				}

				nWorkers := len(wcsWorkerNodes) + 1
				tilesPerWorker := int(math.Round(float64(len(tmpTileRequests)) / float64(nWorkers)))
				if tilesPerWorker < minTilesPerWorker {
					tilesPerWorker = minTilesPerWorker
				}

				isLastWorker := false
				for i := 0; i < nWorkers; i++ {
					iBgn := i * tilesPerWorker
					iEnd := iBgn + tilesPerWorker
					if iEnd > len(tmpTileRequests) {
						iEnd = len(tmpTileRequests)
						isLastWorker = true
					}

					workerTileRequests = append(workerTileRequests, tmpTileRequests[iBgn:iEnd])
					if isLastWorker {
						break
					}
				}

			} else {
				geoReq := getGeoTileRequest(*params.Width, *params.Height, params.BBox, 0, 0)
				workerTileRequests = append(workerTileRequests, []*proc.GeoTileRequest{geoReq})
			}
		} else {
			for _, qParams := range []string{"wwidth", "wheight", "woffx", "woffy"} {
				if len(query[qParams]) != len(query["wbbox"]) {
					http.Error(w, fmt.Sprintf("worker parameter %v has different length from wbbox: %v", qParams, reqURL), 400)
					return
				}
			}

			workerBbox := query["wbbox"]
			workerWidth := query["wwidth"]
			workerHeight := query["wheight"]
			workerOffX := query["woffx"]
			workerOffY := query["woffy"]

			wParams := make(map[string][]string)
			wParams["bbox"] = []string{""}
			wParams["width"] = []string{""}
			wParams["height"] = []string{""}
			wParams["x"] = []string{""}
			wParams["y"] = []string{""}

			tmpTileRequests := []*proc.GeoTileRequest{}
			for iw, bbox := range workerBbox {
				wParams["bbox"][0] = bbox
				wParams["width"][0] = workerWidth[iw]
				wParams["height"][0] = workerHeight[iw]
				wParams["x"][0] = workerOffX[iw]
				wParams["y"][0] = workerOffY[iw]

				workerParams, err := utils.WMSParamsChecker(wParams, reWMSMap)
				if err != nil {
					http.Error(w, fmt.Sprintf("worker parameter error: %v", err), 400)
					return
				}

				geoReq := getGeoTileRequest(*workerParams.Width, *workerParams.Height, workerParams.BBox, *workerParams.X, *workerParams.Y)
				tmpTileRequests = append(tmpTileRequests, geoReq)
			}

			workerTileRequests = append(workerTileRequests, tmpTileRequests)
		}

		geot := utils.BBox2Geot(*params.Width, *params.Height, params.BBox)
		// TODO do this conversion here and pass the int to the pipeline
		epsg, err := utils.ExtractEPSGCode(workerTileRequests[0][0].CRS)
		if err != nil {
			http.Error(w, fmt.Sprintf("Request %s should contain valid 'width' and 'height' parameters.", reqURL), 400)
			return
		}

		hDstDS := utils.GetDummyGDALDatasetH()
		var tempFile string
		isInit := true

		ctx, ctxCancel := context.WithCancel(ctx)
		defer ctxCancel()
		errChan := make(chan error)

		tempFileGeoReq := make(map[string][]*proc.GeoTileRequest)

		workerErrChan := make(chan error)
		workerDoneChan := make(chan string)

		if !isWorker && len(workerTileRequests) > 1 {
			for iw := 1; iw < len(workerTileRequests); iw++ {
				queryUrl := wcsWorkerNodes[iw-1] + reqURL
				for _, geoReq := range workerTileRequests[iw] {
					paramStr := fmt.Sprintf("&wbbox=%f,%f,%f,%f&wwidth=%d&wheight=%d&woffx=%d&woffy=%d",
						geoReq.BBox[0], geoReq.BBox[1], geoReq.BBox[2], geoReq.BBox[3], geoReq.Width, geoReq.Height, geoReq.OffX, geoReq.OffY)

					queryUrl += paramStr
				}

				if *verbose {
					Info.Printf("WCS worker (%v of %v): %v\n", iw, len(workerTileRequests), queryUrl)
				}

				trans := &http.Transport{}
				req, err := http.NewRequest("GET", queryUrl, nil)
				if err != nil {
					errMsg := fmt.Sprintf("WCS: worker NewRequest error: %v", err)
					Info.Printf(errMsg)
					http.Error(w, errMsg, 500)
					return
				}
				defer trans.CancelRequest(req)

				tempFileHandle, err := ioutil.TempFile(conf.ServiceConfig.TempDir, "worker_raster_")
				if err != nil {
					errMsg := fmt.Sprintf("WCS: failed to create raster temp file for WCS worker: %v", err)
					Info.Printf(errMsg)
					http.Error(w, errMsg, 500)
					return
				}
				tempFileHandle.Close()
				defer os.Remove(tempFileHandle.Name())
				tempFileGeoReq[tempFileHandle.Name()] = workerTileRequests[iw]

				go func(req *http.Request, transport *http.Transport, tempFileName string) {
					client := &http.Client{Transport: transport}

					resp, err := client.Do(req)
					if err != nil {
						workerErrChan <- fmt.Errorf("WCS: worker error: %v", err)
						return
					}
					defer resp.Body.Close()

					tempFileHandle, err := os.Create(tempFileName)
					if err != nil {
						workerErrChan <- fmt.Errorf("failed to open raster temp file for WCS worker: %v\n", err)
						return
					}
					defer tempFileHandle.Close()

					_, err = io.Copy(tempFileHandle, resp.Body)
					if err != nil {
						workerErrChan <- fmt.Errorf("WCS: worker error in io.Copy(): %v", err)
						return
					}

					workerDoneChan <- tempFileName
				}(req, trans, tempFileHandle.Name())

			}
		}

		driverFormat := *params.Format
		if isWorker {
			driverFormat = "geotiff"
		}

		tp := proc.InitTilePipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, conf.Layers[idx].MaxGrpcRecvMsgSize, conf.Layers[idx].WcsPolygonShardConcLimit, errChan)
		for ir, geoReq := range workerTileRequests[0] {
			if *verbose {
				Info.Printf("WCS: processing tile (%d of %d): xOff:%v, yOff:%v, width:%v, height:%v", ir+1, len(workerTileRequests[0]), geoReq.OffX, geoReq.OffY, geoReq.Width, geoReq.Height)
			}

			select {
			case res := <-tp.Process(geoReq):
				if isInit {
					hDstDS, tempFile, err = utils.EncodeGdalOpen(conf.ServiceConfig.TempDir, driverFormat, geot, epsg, res, *params.Width, *params.Height, len(conf.Layers[idx].RGBProducts))
					defer os.Remove(tempFile)
					if err != nil {
						errMsg := fmt.Sprintf("EncodeGdalOpen() failed: %v", err)
						Info.Printf(errMsg)
						http.Error(w, errMsg, 500)
						return
					}
					isInit = false
				}

				err := utils.EncodeGdal(hDstDS, res, geoReq.OffX, geoReq.OffY)
				if err != nil {
					Info.Printf("Error in the utils.EncodeGdal: %v\n", err)
					http.Error(w, err.Error(), 500)
					return
				}

			case err := <-errChan:
				Info.Printf("WCS: error in the pipeline: %v\n", err)
				http.Error(w, err.Error(), 500)
				return
			case err := <-workerErrChan:
				Info.Printf("WCS worker error: %v\n", err)
				http.Error(w, err.Error(), 500)
				return
			case <-ctx.Done():
				Error.Printf("Context cancelled with message: %v\n", ctx.Err())
				http.Error(w, ctx.Err().Error(), 500)
				return
			}

			if (ir+1)%checkpointThreshold == 0 {
				hDstDS, err = utils.EncodeGdalFlush(hDstDS, tempFile, driverFormat)
				if err != nil {
					Info.Printf("Error in the pipeline: %v\n", err)
					http.Error(w, err.Error(), 500)
				}
				runtime.GC()
			}
		}

		if !isWorker && len(workerTileRequests) > 1 {
			nWorkerDone := 0
			allWorkerDone := false
			for {
				select {
				case workerTempFileName := <-workerDoneChan:
					offX := make([]int, len(tempFileGeoReq[workerTempFileName]))
					offY := make([]int, len(offX))
					width := make([]int, len(offX))
					height := make([]int, len(offX))

					for ig, geoReq := range tempFileGeoReq[workerTempFileName] {
						offX[ig] = geoReq.OffX
						offY[ig] = geoReq.OffY
						width[ig] = geoReq.Width
						height[ig] = geoReq.Height
					}

					err := utils.EncodeGdalMerge(hDstDS, "geotiff", workerTempFileName, width, height, offX, offY)
					if err != nil {
						utils.EncodeGdalClose(hDstDS)
						Info.Printf("%v\n", err)
						http.Error(w, err.Error(), 500)
						return
					}
					os.Remove(workerTempFileName)
					nWorkerDone++

					if *verbose {
						Info.Printf("WCS: worker done (%v of %v)", nWorkerDone, len(workerTileRequests)-1)
					}

					if nWorkerDone == len(workerTileRequests)-1 {
						allWorkerDone = true
					}
				case err := <-workerErrChan:
					utils.EncodeGdalClose(hDstDS)
					Info.Printf("%v\n", err)
					http.Error(w, err.Error(), 500)
					return
				case <-ctx.Done():
					utils.EncodeGdalClose(hDstDS)
					Error.Printf("Context cancelled with message: %v\n", ctx.Err())
					http.Error(w, ctx.Err().Error(), 500)
					return
				}

				if allWorkerDone {
					break
				}
			}
		}

		utils.EncodeGdalClose(hDstDS)

		fileExt := "wcs"
		contentType := "application/wcs"
		switch strings.ToLower(*params.Format) {
		case "geotiff":
			fileExt = "tiff"
			contentType = "application/geotiff"
		case "netcdf":
			fileExt = "nc"
			contentType = "application/netcdf"
		}
		ISOFormat := "2006-01-02T15:04:05.000Z"
		fileNameDateTime := params.Time.Format(ISOFormat)

		var re = regexp.MustCompile(`[^a-zA-Z0-9\-_\s]`)
		fileNameCoverages := re.ReplaceAllString(params.Coverages[0], `-`)

		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.%s.%s", fileNameCoverages, fileNameDateTime, fileExt))
		w.Header().Set("Content-Type", contentType)

		fileHandle, err := os.Open(tempFile)
		if err != nil {
			errMsg := fmt.Sprintf("Error opening raster file: %v", err)
			Info.Printf(errMsg)
			http.Error(w, errMsg, 500)
		}
		defer fileHandle.Close()

		fileInfo, err := fileHandle.Stat()
		if err != nil {
			errMsg := fmt.Sprintf("file stat() failed: %v", err)
			Info.Printf(errMsg)
			http.Error(w, errMsg, 500)
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		bytesSent, err := io.Copy(w, fileHandle)
		if err != nil {
			errMsg := fmt.Sprintf("SendFile failed: %v", err)
			Info.Printf(errMsg)
			http.Error(w, errMsg, 500)
		}

		if *verbose {
			Info.Printf("WCS: file_size:%v, bytes_sent:%v\n", fileInfo.Size(), bytesSent)
		}

		return

	default:
		http.Error(w, fmt.Sprintf("%s not recognised.", *params.Request), 400)
	}
}

func serveWPS(ctx context.Context, params utils.WPSParams, conf *utils.Config, reqURL string, w http.ResponseWriter) {

	if params.Request == nil {
		http.Error(w, "Malformed WPS, a Request field needs to be specified", 400)
		return
	}

	switch *params.Request {
	case "GetCapabilities":
		err := utils.ExecuteWriteTemplateFile(w, conf.Processes,
			utils.DataDir+"/templates/WPS_GetCapabilities.tpl")
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	case "DescribeProcess":
		for _, process := range conf.Processes {
			if process.Identifier == *params.Identifier {
				err := utils.ExecuteWriteTemplateFile(w, process,
					utils.DataDir+"/templates/WPS_DescribeProcess.tpl")
				if err != nil {
					http.Error(w, err.Error(), 500)
				}
				break
			}
		}
	case "Execute":
		if len(params.FeatCol.Features) == 0 {
			err := utils.ExecuteWriteTemplateFile(w, "Request doesn't contain any Feature.",
				utils.DataDir+"/templates/WPS_Exception.tpl")
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return

		}

		if len(conf.Processes) != 1 || len(conf.Processes[0].Paths) != 2 {
			http.Error(w, `Request cannot be processed. Error detected in the contents of the "processes" object on config file.`, 400)
			return
		}

		process := conf.Processes[0]

		var feat []byte
		geom := params.FeatCol.Features[0].Geometry
		switch geom := geom.(type) {

		case *geo.Point:
			feat, _ = json.Marshal(&geo.Feature{Type: "Feature", Geometry: geom})

		case *geo.Polygon, *geo.MultiPolygon:
			area := utils.GetArea(geom)
			log.Println("yes, it's a polygon with area", area)
			if area == 0.0 || area > conf.Processes[0].MaxArea {
				Info.Printf("The requested area %.02f, is too large.\n", area)
				http.Error(w, "The requested area is too large. Please try with a smaller one.", 400)
				return
			}
			feat, _ = json.Marshal(&geo.Feature{Type: "Feature", Geometry: geom})

		default:
			http.Error(w, "Geometry not supported. Only Features containing Polygon or MultiPolygon are available..", 400)
			return
		}

		year, month, day := time.Now().Date()
		geoReq1 := proc.GeoDrillRequest{Geometry: string(feat),
			CRS:        "EPSG:4326",
			Collection: process.Paths[0],
			NameSpaces: []string{"phot_veg", "nphot_veg", "bare_soil"},
			StartTime:  time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:    time.Date(year, month, day, 0, 0, 0, 0, time.UTC),
		}
		geoReq2 := proc.GeoDrillRequest{Geometry: string(feat),
			CRS:        "EPSG:4326",
			Collection: process.Paths[1],
			NameSpaces: []string{""},
			StartTime:  time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:    time.Date(year, month, day, 0, 0, 0, 0, time.UTC),
		}

		fmt.Println(geoReq1, geoReq2)

		// This is not concurrent as in the previous version
		var result string
		ctx, ctxCancel := context.WithCancel(ctx)
		defer ctxCancel()
		errChan := make(chan error)

		suffix := fmt.Sprintf("_%04d", rand.Intn(1000))
		dp1 := proc.InitDrillPipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, process.IdentityTol, process.DpTol, errChan)
		proc1 := dp1.Process(geoReq1, suffix)
		dp2 := proc.InitDrillPipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, process.IdentityTol, process.DpTol, errChan)
		proc2 := dp2.Process(geoReq2, suffix)

		for _, proc := range []chan string{proc1, proc2} {
			select {
			case res := <-proc:
				result += res
			case err := <-errChan:
				Info.Printf("Error in the pipeline: %v\n", err)
				ctxCancel()
				http.Error(w, err.Error(), 500)
				return
			case <-ctx.Done():
				Error.Printf("Context cancelled with message: %v\n", ctx.Err())
				ctxCancel()
				http.Error(w, ctx.Err().Error(), 500)
				return
			}
		}
		ctxCancel()

		err := utils.ExecuteWriteTemplateFile(w, result,
			utils.DataDir+"/templates/WPS_Execute.tpl")
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

	default:
		http.Error(w, fmt.Sprintf("%s not recognised.", *params.Request), 400)
	}
}

// owsHandler handles every request received on /ows
func generalHandler(conf *utils.Config, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	Info.Printf("%s\n", r.URL.String())
	ctx := r.Context()

	var query map[string][]string
	var err error
	switch r.Method {
	case "POST":
		query, err = utils.ParsePost(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error parsing WPS POST payload: %s", err), 400)
			return
		}

	case "GET":
		query = utils.NormaliseKeys(r.URL.Query())
	}

	if _, fOK := query["service"]; !fOK {
		http.Error(w, fmt.Sprintf("Not a OWS request. Request does not contain a 'service' parameter."), 400)
		return
	}

	switch query["service"][0] {
	case "WMS":
		params, err := utils.WMSParamsChecker(query, reWMSMap)
		if err != nil {
			http.Error(w, fmt.Sprintf("Wrong WMS parameters on URL: %s", err), 400)
			return
		}
		serveWMS(ctx, params, conf, r.URL.String(), w)
	case "WCS":
		params, err := utils.WCSParamsChecker(query, reWCSMap)
		if err != nil {
			http.Error(w, fmt.Sprintf("Wrong WCS parameters on URL: %s", err), 400)
			return
		}
		serveWCS(ctx, params, conf, r.URL.String(), w, query)
	case "WPS":
		params, err := utils.WPSParamsChecker(query, reWPSMap)
		if err != nil {
			http.Error(w, fmt.Sprintf("Wrong WPS parameters on URL: %s", err), 400)
			return
		}
		serveWPS(ctx, params, conf, r.URL.String(), w)
	default:
		http.Error(w, fmt.Sprintf("Not a valid OWS request. URL %s does not contain a valid 'request' parameter.", r.URL.String()), 400)
		return
	}
}

func owsHandler(w http.ResponseWriter, r *http.Request) {
	namespace := "."
	if len(r.URL.Path) > len("/ows/") {
		namespace = r.URL.Path[len("/ows/"):]
	}
	config, ok := configMap[namespace]
	if !ok {
		Info.Printf("Invalid dataset namespace: %v for url: %v\n", namespace, r.URL.Path)
		http.Error(w, fmt.Sprintf("Invalid dataset namespace: %v\n", namespace), 404)
		return
	}
	generalHandler(config, w, r)
}

func main() {
	fs := http.FileServer(http.Dir(utils.DataDir + "/static"))
	http.Handle("/", fs)
	http.HandleFunc("/ows", owsHandler)
	http.HandleFunc("/ows/", owsHandler)
	Info.Printf("GSKY is ready")
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", *port), nil))
}
