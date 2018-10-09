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
	"net/url"
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
	port            = flag.Int("p", 8080, "Server listening port.")
	serverDataDir   = flag.String("data_dir", utils.DataDir, "Server data directory.")
	serverConfigDir = flag.String("conf_dir", utils.EtcDir, "Server config directory.")
	validateConfig  = flag.Bool("check_conf", false, "Validate server config files.")
	verbose         = flag.Bool("v", false, "Verbose mode for more server outputs.")
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
	rand.Seed(time.Now().UnixNano())

	Error = log.New(os.Stderr, "OWS: ", log.Ldate|log.Ltime|log.Lshortfile)
	Info = log.New(os.Stdout, "OWS: ", log.Ldate|log.Ltime|log.Lshortfile)

	flag.Parse()

	utils.DataDir = *serverDataDir
	utils.EtcDir = *serverConfigDir

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

	confMap, err := utils.LoadAllConfigFiles(utils.EtcDir, *verbose)
	if err != nil {
		Error.Printf("Error in loading config files: %v\n", err)
		panic(err)
	}

	if *validateConfig {
		os.Exit(0)
	}

	configMap = confMap

	utils.WatchConfig(Info, Error, &configMap, *verbose)

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

		for iLayer := range conf.Layers {
			if conf.Layers[iLayer].AutoRefreshTimestamps {
				conf.GetLayerDates(iLayer)
			}
		}

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")
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
			currentTime, err := utils.GetCurrentTimeStamp(conf.Layers[idx].Dates)
			if err != nil {
				http.Error(w, fmt.Sprintf("%v: %s", err, reqURL), 400)
				return
			}
			params.Time = currentTime
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

		if *params.Height > conf.Layers[idx].WmsMaxHeight || *params.Width > conf.Layers[idx].WmsMaxWidth {
			http.Error(w, fmt.Sprintf("Requested width/height is too large, max width:%d, height:%d", conf.Layers[idx].WmsMaxWidth, conf.Layers[idx].WmsMaxHeight), 400)
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
			QueryLimit:      -1,
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

		xRes := (params.BBox[2] - params.BBox[0]) / float64(*params.Width)
		yRes := (params.BBox[3] - params.BBox[1]) / float64(*params.Height)
		reqRes := xRes
		if yRes > reqRes {
			reqRes = yRes
		}

		if conf.Layers[idx].ZoomLimit != 0.0 && reqRes > conf.Layers[idx].ZoomLimit {
			indexer := proc.NewTileIndexer(ctx, conf.ServiceConfig.MASAddress, errChan)
			go func() {
				geoReq.Mask = nil
				geoReq.QueryLimit = 1
				indexer.In <- geoReq
				close(indexer.In)
			}()

			go indexer.Run()

			hasData := false
			for geo := range indexer.Out {
				select {
				case <-errChan:
					break
				case <-ctx.Done():
					break
				default:
					if geo.NameSpace != "EmptyTile" {
						hasData = true
						break
					}
				}

				if hasData {
					break
				}
			}

			if hasData {
				out, err := utils.GetEmptyTile(utils.DataDir+"/zoom.png", *params.Height, *params.Width)
				if err != nil {
					Info.Printf("Error in the utils.GetEmptyTile(zoom.png): %v\n", err)
					http.Error(w, err.Error(), 500)
					return
				}
				w.Write(out)
			} else {
				emptyTile := &utils.ByteRaster{Height: *params.Height, Width: *params.Width}
				out, err := utils.EncodePNG([]*utils.ByteRaster{emptyTile}, nil)
				if err != nil {
					Info.Printf("Error in the utils.EncodePNG: %v\n", err)
					http.Error(w, err.Error(), 500)
					return
				}
				w.Write(out)
			}

			return
		}

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

		newConf := *conf
		newConf.Layers = make([]utils.Layer, len(newConf.Layers))
		for i, layer := range conf.Layers {
			if layer.AutoRefreshTimestamps {
				conf.GetLayerDates(i)
			}
			newConf.Layers[i] = layer
			newConf.Layers[i].Dates = []string{newConf.Layers[i].Dates[0], newConf.Layers[i].Dates[len(newConf.Layers[i].Dates)-1]}
		}

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")
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
			currentTime, err := utils.GetCurrentTimeStamp(conf.Layers[idx].Dates)
			if err != nil {
				http.Error(w, fmt.Sprintf("%v: %s", err, reqURL), 400)
				return
			}
			params.Time = currentTime
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

		var wcsWorkerNodes []string
		workerTileRequests := [][]*proc.GeoTileRequest{}

		_, isWorker := query["wbbox"]

		getGeoTileRequest := func(width int, height int, bbox []float64, offX int, offY int) *proc.GeoTileRequest {
			geoReq := &proc.GeoTileRequest{ConfigPayLoad: proc.ConfigPayLoad{NameSpaces: conf.Layers[idx].RGBProducts,
				Mask:    conf.Layers[idx].Mask,
				Palette: conf.Layers[idx].Palette,
				ScaleParams: proc.ScaleParams{Offset: conf.Layers[idx].OffsetValue,
					Scale: conf.Layers[idx].ScaleValue,
					Clip:  conf.Layers[idx].ClipValue,
				},
				ZoomLimit:       0.0,
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

		ctx, ctxCancel := context.WithCancel(ctx)
		defer ctxCancel()
		errChan := make(chan error)

		epsg, err := utils.ExtractEPSGCode(*params.CRS)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid CRS code %s", *params.CRS), 400)
			return
		}

		if *params.Width <= 0 || *params.Height <= 0 {
			if isWorker {
				msg := "WCS: worker width or height negative"
				Info.Printf(msg)
				http.Error(w, msg, 500)
				return
			}

			geoReq := getGeoTileRequest(0, 0, params.BBox, 0, 0)
			maxWidth, maxHeight, err := proc.ComputeReprojectionExtent(ctx, geoReq, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, epsg, params.BBox, *verbose)
			Info.Printf("WCS: Output image size: width=%v, height=%v", maxWidth, maxHeight)
			if maxWidth > 0 && maxHeight > 0 {
				*params.Width = maxWidth
				*params.Height = maxHeight

				rex := regexp.MustCompile(`(?i)&width\s*=\s*[-+]?[0-9]+`)
				reqURL = rex.ReplaceAllString(reqURL, ``)

				rex = regexp.MustCompile(`(?i)&height\s*=\s*[-+]?[0-9]+`)
				reqURL = rex.ReplaceAllString(reqURL, ``)

				reqURL += fmt.Sprintf("&width=%d&height=%d", maxWidth, maxHeight)
			} else {
				errMsg := "WCS: failed to compute output extent"
				Info.Printf(errMsg, err)
				http.Error(w, errMsg, 500)
				return
			}

		}

		if *params.Height > conf.Layers[idx].WcsMaxHeight || *params.Width > conf.Layers[idx].WcsMaxWidth {
			http.Error(w, fmt.Sprintf("Requested width/height is too large, max width:%d, height:%d", conf.Layers[idx].WcsMaxWidth, conf.Layers[idx].WcsMaxHeight), 400)
			return
		}

		if !isWorker {
			if *params.Width > maxXTileSize || *params.Height > maxYTileSize {
				tmpTileRequests := []*proc.GeoTileRequest{}
				xRes := (params.BBox[2] - params.BBox[0]) / float64(*params.Width)
				yRes := (params.BBox[3] - params.BBox[1]) / float64(*params.Height)

				for y := 0; y < *params.Height; y += maxYTileSize {
					for x := 0; x < *params.Width; x += maxXTileSize {
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

				for iw, worker := range conf.ServiceConfig.OWSClusterNodes {
					parsedURL, err := url.Parse(worker)
					if err != nil {
						if *verbose {
							Info.Printf("WCS: invalid worker hostname %v, (%v of %v)\n", worker, iw, len(conf.ServiceConfig.OWSClusterNodes))
						}
						continue
					}

					if parsedURL.Host == conf.ServiceConfig.OWSHostname {
						if *verbose {
							Info.Printf("WCS: skipping worker whose hostname == OWSHostName %v, (%v of %v)\n", worker, iw, len(conf.ServiceConfig.OWSClusterNodes))
						}
						continue
					}
					wcsWorkerNodes = append(wcsWorkerNodes, worker)
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

		hDstDS := utils.GetDummyGDALDatasetH()
		var masterTempFile string

		tempFileGeoReq := make(map[string][]*proc.GeoTileRequest)

		workerErrChan := make(chan error)
		workerDoneChan := make(chan string, len(workerTileRequests)-1)

		if !isWorker && len(workerTileRequests) > 1 {
			for iw := 1; iw < len(workerTileRequests); iw++ {
				workerHostName := wcsWorkerNodes[iw-1]
				queryURL := workerHostName + reqURL
				for _, geoReq := range workerTileRequests[iw] {
					paramStr := fmt.Sprintf("&wbbox=%f,%f,%f,%f&wwidth=%d&wheight=%d&woffx=%d&woffy=%d",
						geoReq.BBox[0], geoReq.BBox[1], geoReq.BBox[2], geoReq.BBox[3], geoReq.Width, geoReq.Height, geoReq.OffX, geoReq.OffY)

					queryURL += paramStr
				}

				if *verbose {
					Info.Printf("WCS worker (%v of %v): %v\n", iw, len(workerTileRequests)-1, queryURL)
				}

				trans := &http.Transport{}
				req, err := http.NewRequest("GET", queryURL, nil)
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
						tempFileHandle.Close()
						workerErrChan <- fmt.Errorf("WCS: worker error in io.Copy(): %v", err)
						return
					}

					workerDoneChan <- tempFileName
				}(req, trans, tempFileHandle.Name())
			}
		}

		geot := utils.BBox2Geot(*params.Width, *params.Height, params.BBox)

		driverFormat := *params.Format
		if isWorker {
			driverFormat = "geotiff"
		}

		isInit := false

		tp := proc.InitTilePipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, conf.Layers[idx].MaxGrpcRecvMsgSize, conf.Layers[idx].WcsPolygonShardConcLimit, errChan)
		for ir, geoReq := range workerTileRequests[0] {
			if *verbose {
				Info.Printf("WCS: processing tile (%d of %d): xOff:%v, yOff:%v, width:%v, height:%v", ir+1, len(workerTileRequests[0]), geoReq.OffX, geoReq.OffY, geoReq.Width, geoReq.Height)
			}

			select {
			case res := <-tp.Process(geoReq):
				if !isInit {
					hDstDS, masterTempFile, err = utils.EncodeGdalOpen(conf.ServiceConfig.TempDir, 1024, 256, driverFormat, geot, epsg, res, *params.Width, *params.Height, len(conf.Layers[idx].RGBProducts))
					defer os.Remove(masterTempFile)
					if err != nil {
						errMsg := fmt.Sprintf("EncodeGdalOpen() failed: %v", err)
						Info.Printf(errMsg)
						http.Error(w, errMsg, 500)
						return
					}
					isInit = true
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
				utils.EncodeGdalFlush(hDstDS)
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

					var t0 time.Time
					if *verbose {
						t0 = time.Now()
					}
					err := utils.EncodeGdalMerge(ctx, hDstDS, "geotiff", workerTempFileName, width, height, offX, offY)
					if err != nil {
						utils.EncodeGdalClose(hDstDS)
						Info.Printf("%v\n", err)
						http.Error(w, err.Error(), 500)
						return
					}
					os.Remove(workerTempFileName)
					nWorkerDone++

					if *verbose {
						t1 := time.Since(t0)
						Info.Printf("WCS: merge %v to %v done (%v of %v), time: %v", workerTempFileName, masterTempFile, nWorkerDone, len(workerTileRequests)-1, t1)
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

		fileHandle, err := os.Open(masterTempFile)
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
		err := utils.ExecuteWriteTemplateFile(w, conf,
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
		idx, err := utils.GetProcessIndex(params, conf)
		if err != nil {
			Error.Printf("Requested process not found: %v, %v\n", err, reqURL)
			http.Error(w, fmt.Sprintf("%v: %s", err, reqURL), 400)
			return
		}
		process := conf.Processes[idx]
		if len(process.DataSources) == 0 {
			Error.Printf("No data source specified")
			http.Error(w, "No data source specified", 500)
			return
		}

		if len(params.FeatCol.Features) == 0 {
			Info.Printf("The request does not contain the 'feature' property.\n")
			http.Error(w, "The request does not contain the 'feature' property", 400)
			return
		}

		var feat []byte
		geom := params.FeatCol.Features[0].Geometry
		switch geom := geom.(type) {

		case *geo.Point:
			feat, _ = json.Marshal(&geo.Feature{Type: "Feature", Geometry: geom})

		case *geo.Polygon, *geo.MultiPolygon:
			area := utils.GetArea(geom)
			log.Println("Requested polygon has an area of", area)
			if area == 0.0 || area > process.MaxArea {
				Info.Printf("The requested area %.02f, is too large.\n", area)
				http.Error(w, "The requested area is too large. Please try with a smaller one.", 400)
				return
			}
			feat, _ = json.Marshal(&geo.Feature{Type: "Feature", Geometry: geom})

		default:
			http.Error(w, "Geometry not supported. Only Features containing Polygon or MultiPolygon are available..", 400)
			return
		}

		var result string
		ctx, ctxCancel := context.WithCancel(ctx)
		defer ctxCancel()
		errChan := make(chan error)
		suffix := fmt.Sprintf("_%04d", rand.Intn(1000))

		for ids, dataSource := range process.DataSources {
			log.Printf("WPS: Processing '%v' (%d of %d)", dataSource.DataSource, ids+1, len(process.DataSources))

			startDateTime := time.Time{}
			stStartInput, errStartInput := time.Parse(utils.ISOFormat, *params.StartDateTime)
			if errStartInput != nil {
				if len(*params.StartDateTime) > 0 {
					log.Printf("WPS: invalid input start date '%v' with error '%v'", *params.StartDateTime, errStartInput)
				}
				startDateTimeStr := strings.TrimSpace(dataSource.StartISODate)
				if len(startDateTimeStr) > 0 {
					st, errStart := time.Parse(utils.ISOFormat, startDateTimeStr)
					if errStart != nil {
						log.Printf("WPS: Failed to parse start date '%v' into ISO format with error: %v, defaulting to no start date", startDateTimeStr, errStart)
					} else {
						startDateTime = st
					}
				}
			} else {
				startDateTime = stStartInput
			}

			endDateTime := time.Now().UTC()
			stEndInput, errEndInput := time.Parse(utils.ISOFormat, *params.EndDateTime)
			if errEndInput != nil {
				if len(*params.EndDateTime) > 0 {
					log.Printf("WPS: invalid input end date '%v' with error '%v'", *params.EndDateTime, errEndInput)
				}
				endDateTimeStr := strings.TrimSpace(dataSource.EndISODate)
				if len(endDateTimeStr) > 0 && strings.ToLower(endDateTimeStr) != "now" {
					dt, errEnd := time.Parse(utils.ISOFormat, endDateTimeStr)
					if errEnd != nil {
						log.Printf("WPS: Failed to parse end date '%s' into ISO format with error: %v, defaulting to now()", endDateTimeStr, errEnd)
					} else {
						endDateTime = dt
					}
				}
			} else {
				if !time.Time.IsZero(stEndInput) {
					endDateTime = stEndInput
				}
			}

			geoReq := proc.GeoDrillRequest{Geometry: string(feat),
				CRS:        "EPSG:4326",
				Collection: dataSource.DataSource,
				NameSpaces: dataSource.RGBProducts,
				StartTime:  startDateTime,
				EndTime:    endDateTime,
			}

			dp := proc.InitDrillPipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, process.IdentityTol, process.DpTol, errChan)

			if dataSource.BandStrides <= 0 {
				dataSource.BandStrides = 1
			}
			proc := dp.Process(geoReq, suffix, dataSource.MetadataURL, dataSource.BandEval, dataSource.BandStrides, *process.Approx)

			select {
			case res := <-proc:
				result += res
			case err := <-errChan:
				Info.Printf("Error in the pipeline: %v\n", err)
				http.Error(w, err.Error(), 500)
				return
			case <-ctx.Done():
				Error.Printf("Context cancelled with message: %v\n", ctx.Err())
				http.Error(w, ctx.Err().Error(), 500)
				return
			}
		}

		err = utils.ExecuteWriteTemplateFile(w, result, utils.DataDir+"/templates/WPS_Execute.tpl")
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
	config.ServiceConfig.NameSpace = namespace
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
