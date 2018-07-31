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
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
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

func serveWCS(ctx context.Context, params utils.WCSParams, conf *utils.Config, reqURL string, w http.ResponseWriter) {
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

		xRes := (params.BBox[2] - params.BBox[0]) / float64(*params.Width)
		if conf.Layers[idx].ZoomLimit != 0.0 && xRes > conf.Layers[idx].ZoomLimit {
			http.Error(w, err.Error(), 417)
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
			PolygonSegments: conf.Layers[idx].WcsPolygonSegments,
			Timeout:         conf.Layers[idx].WcsTimeout,
			GrpcConcLimit:   conf.Layers[idx].GrpcWcsConcPerNode,
		},
			Collection: conf.Layers[idx].DataSource,
			CRS:        *params.CRS,
			BBox:       params.BBox,
			Height:     *params.Height,
			Width:      *params.Width,
			StartTime:  params.Time,
			EndTime:    endTime,
		}

		geot := utils.BBox2Geot(geoReq.Width, geoReq.Height, geoReq.BBox)
		// TODO do this conversion here and pass the int to the pipeline
		epsg, err := utils.ExtractEPSGCode(geoReq.CRS)
		if err != nil {
			http.Error(w, fmt.Sprintf("Request %s should contain valid 'width' and 'height' parameters.", reqURL), 400)
			return
		}

		ctx, ctxCancel := context.WithCancel(ctx)
		defer ctxCancel()
		errChan := make(chan error)
		tp := proc.InitTilePipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, conf.Layers[idx].MaxGrpcRecvMsgSize, conf.Layers[idx].WcsPolygonShardConcLimit, errChan)

		select {
		case res := <-tp.Process(geoReq):
			out, err := utils.EncodeGdal(*params.Format, res, geot, epsg)
			if err != nil {
				Info.Printf("Error in the utils.EncodeGdal: %v\n", err)
				http.Error(w, err.Error(), 500)
				return
			}

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
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(out)))

			w.Write(out)
		case err := <-errChan:
			Info.Printf("Error in the pipeline: %v\n", err)
			http.Error(w, err.Error(), 500)
		case <-ctx.Done():
			Error.Printf("Context cancelled with message: %v\n", ctx.Err())
			http.Error(w, ctx.Err().Error(), 500)
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
			err := utils.ExecuteWriteTemplateFile(w, "Request doesn't contain any Feature.",
				utils.DataDir+"/templates/WPS_Exception.tpl")
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
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
			proc := dp.Process(geoReq, suffix, dataSource.MetadataURL, dataSource.BandEval, dataSource.BandStrides)

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
		serveWCS(ctx, params, conf, r.URL.String(), w)
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
