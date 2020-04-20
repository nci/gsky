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
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/nci/gsky/metrics"
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
	serverLogDir    = flag.String("log_dir", "", "Server log directory.")
	validateConfig  = flag.Bool("check_conf", false, "Validate server config files.")
	dumpConfig      = flag.Bool("dump_conf", false, "Dump server config files.")
	verbose         = flag.Bool("v", false, "Verbose mode for more server outputs.")
)

var reWMSMap map[string]*regexp.Regexp
var reWCSMap map[string]*regexp.Regexp
var reWPSMap map[string]*regexp.Regexp

var (
	Error *log.Logger
	Info  *log.Logger
)

var metricsLogger metrics.Logger

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

	if *dumpConfig {
		configJson, err := utils.DumpConfig(confMap)
		if err != nil {
			Error.Printf("Error in dumping configs: %v\n", err)
		} else {
			log.Print(configJson)
		}
		os.Exit(0)
	}

	configMap = confMap

	utils.WatchConfig(Info, Error, &configMap, *verbose)

	reWMSMap = utils.CompileWMSRegexMap()
	reWCSMap = utils.CompileWCSRegexMap()
	reWPSMap = utils.CompileWPSRegexMap()

	utils.InitGdal()

	if len(*serverLogDir) > 0 {
		if *serverLogDir == "-" {
			metricsLogger = metrics.NewStdoutLogger()
		} else {
			maxLogFileSize := int64(0)
			if val, ok := os.LookupEnv("GSKY_MAX_LOG_FILE_SIZE"); ok {
				valInt, e := strconv.ParseInt(val, 10, 64)
				if e == nil {
					maxLogFileSize = valInt
				} else {
					Error.Printf("invalid GSKY_MAX_LOG_FILE_SIZE: %v", e)
				}
			}

			maxLogFiles := -1
			if val, ok := os.LookupEnv("GSKY_MAX_LOG_FILES"); ok {
				valInt, e := strconv.ParseInt(val, 10, 32)
				if e == nil {
					maxLogFiles = int(valInt)
				} else {
					Error.Printf("invalid GSKY_MAX_LOG_FILES: %v", e)
				}
			}

			metricsLogger = metrics.NewFileLogger(*serverLogDir, maxLogFileSize, maxLogFiles, *verbose)
		}
	}
}

func serveWMS(ctx context.Context, params utils.WMSParams, conf *utils.Config, r *http.Request, w http.ResponseWriter, metricsCollector *metrics.MetricsCollector) {

	if params.Request == nil {
		metricsCollector.Info.HTTPStatus = 400
		http.Error(w, "Malformed WMS, a Request field needs to be specified", 400)
		return
	}

	reqURL := r.URL.String()

	switch *params.Request {
	case "GetCapabilities":
		if params.Version != nil && !utils.CheckWMSVersion(*params.Version) {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("This server can only accept WMS requests compliant with version 1.1.1 and 1.3.0: %s", reqURL), 400)
			return
		}

		conf = conf.Copy(r)
		for iLayer := range conf.Layers {
			conf.GetLayerDates(iLayer, *verbose)
		}

		err := utils.ExecuteWriteTemplateFile(w, conf,
			utils.DataDir+"/templates/WMS_GetCapabilities.tpl")
		if err != nil {
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, err.Error(), 500)
		}
	case "GetFeatureInfo":
		x, y, err := utils.GetCoordinates(params)
		if err != nil {
			Error.Printf("%s\n", err)
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Malformed WMS GetFeatureInfo request: %v", err), 400)
			return
		}

		if params.Time == nil {
			idx, err := utils.GetLayerIndex(params, conf)
			if err != nil {
				metricsCollector.Info.HTTPStatus = 400
				http.Error(w, fmt.Sprintf("Malformed getFeatureInfo request: %s", reqURL), 400)
				return
			}

			currentTime, err := utils.GetCurrentTimeStamp(conf.Layers[idx].Dates)
			if err != nil {
				metricsCollector.Info.HTTPStatus = 400
				http.Error(w, fmt.Sprintf("%v: %s", err, reqURL), 400)
				return
			}
			params.Time = currentTime
		}

		var times []string
		for _, axis := range params.Axes {
			if axis.Name == utils.WeightedTimeAxis {
				for _, val := range axis.InValues {
					t := time.Unix(int64(val), 0).UTC().Format(utils.ISOFormat)
					times = append(times, fmt.Sprintf(`"%s"`, t))
				}
			}
		}

		var timeStr string
		if len(times) > 0 {
			timeStr = fmt.Sprintf(`"times": [%s]`, strings.Join(times, ","))
		} else {
			timeStr = fmt.Sprintf(`"time": "%s"`, (*params.Time).Format(utils.ISOFormat))
		}

		feat_info, err := proc.GetFeatureInfo(ctx, params, conf, configMap, *verbose, metricsCollector)
		if err != nil {
			feat_info = fmt.Sprintf(`"error": "%v"`, err)
			Error.Printf("%v\n", err)
		}

		resp := fmt.Sprintf(`{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"x":%f, "y":%f, %s, %s}}]}`, x, y, timeStr, feat_info)
		w.Write([]byte(resp))

	case "DescribeLayer":
		idx, err := utils.GetLayerIndex(params, conf)
		if err != nil {
			Error.Printf("%s\n", err)
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Malformed WMS DescribeLayer request: %v", err), 400)
			return
		}

		err = utils.ExecuteWriteTemplateFile(w, conf.Layers[idx],
			utils.DataDir+"/templates/WMS_DescribeLayer.tpl")
		if err != nil {
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, err.Error(), 500)
		}

	case "GetMap":
		if params.Version == nil || !utils.CheckWMSVersion(*params.Version) {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("This server can only accept WMS requests compliant with version 1.1.1 and 1.3.0: %s", reqURL), 400)
			return
		}

		idx, err := utils.GetLayerIndex(params, conf)
		if err != nil {
			Error.Printf("%s\n", err)
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Malformed WMS GetMap request: %v", err), 400)
			return
		}
		if params.Time == nil {
			currentTime, err := utils.GetCurrentTimeStamp(conf.Layers[idx].Dates)
			if err != nil {
				metricsCollector.Info.HTTPStatus = 400
				http.Error(w, fmt.Sprintf("%v: %s", err, reqURL), 400)
				return
			}
			params.Time = currentTime
		}
		if params.CRS == nil {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Request %s should contain a valid ISO 'crs/srs' parameter.", reqURL), 400)
			return
		}
		if len(params.BBox) != 4 {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Request %s should contain a valid 'bbox' parameter.", reqURL), 400)
			return
		}
		if params.Height == nil || params.Width == nil {
			metricsCollector.Info.HTTPStatus = 400
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

		styleIdx, err := utils.GetLayerStyleIndex(params, conf, idx)
		if err != nil {
			Error.Printf("%s\n", err)
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Malformed WMS GetMap request: %v", err), 400)
			return
		}

		styleLayer := &conf.Layers[idx]
		if styleIdx >= 0 {
			styleLayer = &conf.Layers[idx].Styles[styleIdx]
		}

		if utils.CheckDisableServices(styleLayer, "wms") {
			Error.Printf("WMS GetMap is disabled for this layer")
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, "WMS GetMap is disabled for this layer", 400)
			return
		}

		offset := styleLayer.OffsetValue
		scale := styleLayer.ScaleValue
		clip := styleLayer.ClipValue
		if params.Offset != nil && params.Clip != nil {
			offset = *params.Offset
			clip = *params.Clip
			scale = 0.0
		}

		palette := styleLayer.Palette
		if params.Palette != nil {
			for _, p := range styleLayer.Palettes {
				if strings.ToLower(p.Name) == strings.ToLower(*params.Palette) {
					palette = p
					break
				}
			}
		}

		colourScale := styleLayer.ColourScale
		if params.ColourScale != nil {
			colourScale = *params.ColourScale
		}

		bbox, err := utils.GetCanonicalBbox(*params.CRS, params.BBox)
		if err != nil {
			bbox = params.BBox
		}
		reqRes := utils.GetPixelResolution(bbox, *params.Width, *params.Height)

		geoReq := &proc.GeoTileRequest{ConfigPayLoad: proc.ConfigPayLoad{NameSpaces: styleLayer.RGBExpressions.VarList,
			BandExpr: styleLayer.RGBExpressions,
			Mask:     styleLayer.Mask,
			Palette:  palette,
			ScaleParams: proc.ScaleParams{Offset: offset,
				Scale:       scale,
				Clip:        clip,
				ColourScale: colourScale,
			},
			ZoomLimit:           conf.Layers[idx].ZoomLimit,
			PolygonSegments:     conf.Layers[idx].WmsPolygonSegments,
			GrpcConcLimit:       conf.Layers[idx].GrpcWmsConcPerNode,
			QueryLimit:          -1,
			UserSrcSRS:          conf.Layers[idx].UserSrcSRS,
			UserSrcGeoTransform: conf.Layers[idx].UserSrcGeoTransform,
			AxisMapping:         conf.Layers[idx].WmsAxisMapping,
			GrpcTileXSize:       conf.Layers[idx].GrpcTileXSize,
			GrpcTileYSize:       conf.Layers[idx].GrpcTileYSize,
			IndexTileXSize:      conf.Layers[idx].IndexTileXSize,
			IndexTileYSize:      conf.Layers[idx].IndexTileYSize,
			SpatialExtent:       conf.Layers[idx].SpatialExtent,
			IndexResLimit:       conf.Layers[idx].IndexResLimit,
			MasQueryHint:        conf.Layers[idx].MasQueryHint,
			ReqRes:              reqRes,
			SRSCf:               conf.Layers[idx].SRSCf,
			MetricsCollector:    metricsCollector,
		},
			Collection: styleLayer.DataSource,
			CRS:        *params.CRS,
			BBox:       params.BBox,
			OrigBBox:   params.BBox,
			Height:     *params.Height,
			Width:      *params.Width,
			StartTime:  params.Time,
			EndTime:    endTime,
		}

		if len(params.Axes) > 0 {
			geoReq.Axes = make(map[string]*proc.GeoTileAxis)
			for _, axis := range params.Axes {
				geoReq.Axes[axis.Name] = &proc.GeoTileAxis{Start: axis.Start, End: axis.End, InValues: axis.InValues, Order: axis.Order, Aggregate: axis.Aggregate}
			}
		}

		if params.BandExpr != nil {
			geoReq.ConfigPayLoad.NameSpaces = params.BandExpr.VarList
			geoReq.ConfigPayLoad.BandExpr = params.BandExpr
		}

		masAddress := styleLayer.MASAddress
		hasOverview := len(styleLayer.Overviews) > 0
		if hasOverview {
			iOvr := utils.FindLayerBestOverview(styleLayer, reqRes, true)
			if iOvr >= 0 {
				ovr := styleLayer.Overviews[iOvr]
				geoReq.Collection = ovr.DataSource
				masAddress = ovr.MASAddress
			}
		}

		ctx, ctxCancel := context.WithCancel(ctx)
		defer ctxCancel()
		errChan := make(chan error, 100)

		timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Duration(conf.Layers[idx].WmsTimeout)*time.Second)
		defer timeoutCancel()

		tp := proc.InitTilePipeline(ctx, masAddress, conf.ServiceConfig.WorkerNodes, conf.Layers[idx].MaxGrpcRecvMsgSize, conf.Layers[idx].WmsPolygonShardConcLimit, conf.ServiceConfig.MaxGrpcBufferSize, errChan)
		tp.CurrentLayer = styleLayer
		tp.DataSources = configMap

		if !hasOverview && styleLayer.ZoomLimit != 0.0 && reqRes > styleLayer.ZoomLimit {
			geoReq.Mask = nil
			geoReq.QueryLimit = 1

			hasData := false
			indexerOut, err := tp.GetFileList(geoReq, *verbose)
			if err == nil {
				for _, geo := range indexerOut {
					if geo.NameSpace != utils.EmptyTileNS {
						hasData = true
						break
					}
				}
			}

			if hasData {
				out, err := utils.GetEmptyTile(utils.DataDir+"/zoom.png", *params.Height, *params.Width)
				if err != nil {
					Info.Printf("Error in the utils.GetEmptyTile(zoom.png): %v\n", err)
					metricsCollector.Info.HTTPStatus = 500
					http.Error(w, err.Error(), 500)
					return
				}
				w.Write(out)
			} else {
				out, err := utils.GetEmptyTile("", *params.Height, *params.Width)
				if err != nil {
					Info.Printf("Error in the utils.GetEmptyTile(): %v\n", err)
					metricsCollector.Info.HTTPStatus = 500
					http.Error(w, err.Error(), 500)
				} else {
					w.Write(out)
				}
			}

			return
		}

		select {
		case res := <-tp.Process(geoReq, *verbose):
			scaleParams := utils.ScaleParams{Offset: geoReq.ScaleParams.Offset,
				Scale:       geoReq.ScaleParams.Scale,
				Clip:        geoReq.ScaleParams.Clip,
				ColourScale: geoReq.ScaleParams.ColourScale,
			}

			norm, err := utils.Scale(res, scaleParams)
			if err != nil {
				Info.Printf("Error in the utils.Scale: %v\n", err)
				metricsCollector.Info.HTTPStatus = 500
				http.Error(w, err.Error(), 500)
				return
			}

			if len(norm) == 0 || norm[0].Width == 0 || norm[0].Height == 0 {
				out, err := utils.GetEmptyTile(conf.Layers[idx].NoDataLegendPath, *params.Height, *params.Width)
				if err != nil {
					Info.Printf("Error in the utils.GetEmptyTile(): %v\n", err)
					metricsCollector.Info.HTTPStatus = 500
					http.Error(w, err.Error(), 500)
				} else {
					w.Write(out)
				}
				return
			}

			out, err := utils.EncodePNG(norm, palette)
			if err != nil {
				Info.Printf("Error in the utils.EncodePNG: %v\n", err)
				metricsCollector.Info.HTTPStatus = 500
				http.Error(w, err.Error(), 500)
				return
			}
			w.Write(out)
		case err := <-errChan:
			Info.Printf("Error in the pipeline: %v\n", err)
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, err.Error(), 500)
		case <-ctx.Done():
			Error.Printf("Context cancelled with message: %v\n", ctx.Err())
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, ctx.Err().Error(), 500)
		case <-timeoutCtx.Done():
			Error.Printf("WMS pipeline timed out, threshold:%v seconds", conf.Layers[idx].WmsTimeout)
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, "WMS request timed out", 500)
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
				metricsCollector.Info.HTTPStatus = 400
				http.Error(w, err.Error(), 400)
			}
			return
		}
		styleIdx, err := utils.GetLayerStyleIndex(params, conf, idx)
		if err != nil {
			Error.Printf("%s\n", err)
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Malformed WMS GetMap request: %v", err), 400)
			return
		}

		styleLayer := &conf.Layers[idx]
		if styleIdx >= 0 {
			styleLayer = &conf.Layers[idx].Styles[styleIdx]
		}

		b, err := ioutil.ReadFile(styleLayer.LegendPath)
		if err != nil {
			Error.Printf("Error reading legend image: %v, %v\n", styleLayer.LegendPath, err)
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, "Legend graphics not found", 500)
			return
		}
		w.Write(b)

	default:
		metricsCollector.Info.HTTPStatus = 400
		http.Error(w, fmt.Sprintf("%s not recognised.", *params.Request), 400)
	}

}

func serveWCS(ctx context.Context, params utils.WCSParams, conf *utils.Config, r *http.Request, w http.ResponseWriter, query map[string][]string, metricsCollector *metrics.MetricsCollector) {
	if params.Request == nil {
		metricsCollector.Info.HTTPStatus = 400
		http.Error(w, "Malformed WCS, a Request field needs to be specified", 400)
	}

	reqURL := r.URL.String()

	switch *params.Request {
	case "GetCapabilities":
		if params.Version != nil && !utils.CheckWCSVersion(*params.Version) {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("This server can only accept WCS requests compliant with version 1.0.0: %s", reqURL), 400)
			return
		}

		newConf := conf.Copy(r)
		for i := range newConf.Layers {
			conf.GetLayerDates(i, *verbose)
			if len(newConf.Layers[i].Dates) > 0 {
				newConf.Layers[i].Dates = []string{newConf.Layers[i].Dates[0], newConf.Layers[i].Dates[len(newConf.Layers[i].Dates)-1]}
			}
		}

		err := utils.ExecuteWriteTemplateFile(w, &newConf, utils.DataDir+"/templates/WCS_GetCapabilities.tpl")
		if err != nil {
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, err.Error(), 500)
		}

	case "DescribeCoverage":
		idx, err := utils.GetCoverageIndex(params, conf)
		if err != nil {
			Info.Printf("Error in the pipeline: %v\n", err)
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Malformed WMS DescribeCoverage request: %v", err), 400)
			return
		}

		err = utils.ExecuteWriteTemplateFile(w, conf.Layers[idx], utils.DataDir+"/templates/WCS_DescribeCoverage.tpl")
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

	case "GetCoverage":
		if params.Version == nil || !utils.CheckWCSVersion(*params.Version) {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("This server can only accept WCS requests compliant with version 1.0.0: %s", reqURL), 400)
			return
		}

		idx, err := utils.GetCoverageIndex(params, conf)
		if err != nil {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("%v: %s", err, reqURL), 400)
			return
		}

		if params.Time == nil {
			currentTime, err := utils.GetCurrentTimeStamp(conf.Layers[idx].Dates)
			if err != nil {
				metricsCollector.Info.HTTPStatus = 400
				http.Error(w, fmt.Sprintf("%v: %s", err, reqURL), 400)
				return
			}
			params.Time = currentTime
		}
		if params.CRS == nil {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Request %s should contain a valid ISO 'crs/srs' parameter.", reqURL), 400)
			return
		}
		if len(params.BBox) != 4 {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Request %s should contain a valid 'bbox' parameter.", reqURL), 400)
			return
		}
		if params.Height == nil || params.Width == nil {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Request %s should contain valid 'width' and 'height' parameters.", reqURL), 400)
			return
		}
		if params.Format == nil {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Unsupported encoding format"), 400)
			return
		}

		var endTime *time.Time
		if conf.Layers[idx].Accum == true {
			step := time.Minute * time.Duration(60*24*conf.Layers[idx].StepDays+60*conf.Layers[idx].StepHours+conf.Layers[idx].StepMinutes)
			eT := params.Time.Add(step)
			endTime = &eT
		}

		styleIdx, err := utils.GetCoverageStyleIndex(params, conf, idx)
		if err != nil {
			Error.Printf("%s\n", err)
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Malformed WCS GetCoverage request: %v", err), 400)
			return
		} else if styleIdx < 0 {
			styleCount := len(conf.Layers[idx].Styles)
			if styleCount > 1 && params.BandExpr == nil {
				Error.Printf("WCS style not specified")
				metricsCollector.Info.HTTPStatus = 400
				http.Error(w, "WCS style not specified", 400)
				return
			} else if styleCount == 1 {
				styleIdx = 0
			}
		}

		styleLayer := &conf.Layers[idx]
		if styleIdx >= 0 {
			styleLayer = &conf.Layers[idx].Styles[styleIdx]
		}

		if utils.CheckDisableServices(styleLayer, "wcs") {
			Error.Printf("WCS GetCoverage is disabled for this layer")
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, "WCS GetCoverage is disabled for this layer", 400)
			return
		}

		maxXTileSize := conf.Layers[idx].WcsMaxTileWidth
		maxYTileSize := conf.Layers[idx].WcsMaxTileHeight
		checkpointThreshold := 300
		minTilesPerWorker := 5

		var wcsWorkerNodes []string
		workerTileRequests := [][]*proc.GeoTileRequest{}

		_, isWorker := query["wbbox"]

		getGeoTileRequest := func(width int, height int, bbox []float64, offX int, offY int) *proc.GeoTileRequest {
			geoReq := &proc.GeoTileRequest{ConfigPayLoad: proc.ConfigPayLoad{NameSpaces: styleLayer.RGBExpressions.VarList,
				BandExpr: styleLayer.RGBExpressions,
				Mask:     styleLayer.Mask,
				Palette:  styleLayer.Palette,
				ScaleParams: proc.ScaleParams{Offset: styleLayer.OffsetValue,
					Scale: styleLayer.ScaleValue,
					Clip:  styleLayer.ClipValue,
				},
				ZoomLimit:           0.0,
				PolygonSegments:     conf.Layers[idx].WcsPolygonSegments,
				GrpcConcLimit:       conf.Layers[idx].GrpcWcsConcPerNode,
				QueryLimit:          -1,
				UserSrcSRS:          conf.Layers[idx].UserSrcSRS,
				UserSrcGeoTransform: conf.Layers[idx].UserSrcGeoTransform,
				NoReprojection:      params.NoReprojection,
				AxisMapping:         params.AxisMapping,
				GrpcTileXSize:       conf.Layers[idx].GrpcTileXSize,
				GrpcTileYSize:       conf.Layers[idx].GrpcTileYSize,
				IndexTileXSize:      conf.Layers[idx].IndexTileXSize,
				IndexTileYSize:      conf.Layers[idx].IndexTileYSize,
				SpatialExtent:       conf.Layers[idx].SpatialExtent,
				IndexResLimit:       conf.Layers[idx].IndexResLimit,
				MasQueryHint:        conf.Layers[idx].MasQueryHint,
				SRSCf:               conf.Layers[idx].SRSCf,
				FusionUnscale:       1,
				MetricsCollector:    metricsCollector,
			},
				Collection: styleLayer.DataSource,
				CRS:        *params.CRS,
				BBox:       bbox,
				OrigBBox:   params.BBox,
				Height:     height,
				Width:      width,
				StartTime:  params.Time,
				EndTime:    endTime,
				OffX:       offX,
				OffY:       offY,
			}

			if len(params.Axes) > 0 {
				geoReq.Axes = make(map[string]*proc.GeoTileAxis)
				for _, axis := range params.Axes {
					geoReq.Axes[axis.Name] = &proc.GeoTileAxis{Start: axis.Start, End: axis.End, InValues: axis.InValues, Order: axis.Order, Aggregate: axis.Aggregate}
					for _, sel := range axis.IdxSelectors {
						tileIdxSel := &proc.GeoTileIdxSelector{Start: sel.Start, End: sel.End, Step: sel.Step, IsRange: sel.IsRange, IsAll: sel.IsAll}
						geoReq.Axes[axis.Name].IdxSelectors = append(geoReq.Axes[axis.Name].IdxSelectors, tileIdxSel)
					}
				}
			}

			if params.BandExpr != nil {
				geoReq.ConfigPayLoad.NameSpaces = params.BandExpr.VarList
				geoReq.ConfigPayLoad.BandExpr = params.BandExpr
			}

			return geoReq
		}

		ctx, ctxCancel := context.WithCancel(ctx)
		defer ctxCancel()
		errChan := make(chan error, 100)

		epsg, err := utils.ExtractEPSGCode(*params.CRS)
		if err != nil {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Invalid CRS code %s", *params.CRS), 400)
			return
		}

		if *params.Width <= 0 || *params.Height <= 0 {
			if isWorker {
				msg := "WCS: worker width or height negative"
				Info.Printf(msg)
				metricsCollector.Info.HTTPStatus = 500
				http.Error(w, msg, 500)
				return
			}

			geoReq := getGeoTileRequest(0, 0, params.BBox, 0, 0)
			maxWidth, maxHeight, err := proc.ComputeReprojectionExtent(ctx, geoReq, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, epsg, params.BBox, *verbose)
			if *verbose {
				Info.Printf("WCS: Output image size: width=%v, height=%v", maxWidth, maxHeight)
			}
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
				metricsCollector.Info.HTTPStatus = 500
				http.Error(w, errMsg, 500)
				return
			}

		}

		if *params.Height > conf.Layers[idx].WcsMaxHeight || *params.Width > conf.Layers[idx].WcsMaxWidth {
			metricsCollector.Info.HTTPStatus = 400
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
					metricsCollector.Info.HTTPStatus = 400
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
					metricsCollector.Info.HTTPStatus = 400
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

		workerErrChan := make(chan error, 100)
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
					metricsCollector.Info.HTTPStatus = 500
					http.Error(w, errMsg, 500)
					return
				}
				defer trans.CancelRequest(req)

				tempFileHandle, err := ioutil.TempFile(conf.ServiceConfig.TempDir, "worker_raster_")
				if err != nil {
					errMsg := fmt.Sprintf("WCS: failed to create raster temp file for WCS worker: %v", err)
					Info.Printf(errMsg)
					metricsCollector.Info.HTTPStatus = 500
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
		if isWorker || driverFormat == "dap4" {
			driverFormat = "geotiff"
		}

		timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Duration(conf.Layers[idx].WcsTimeout)*time.Second)
		defer timeoutCancel()

		isInit := false
		var bandNames []string

		tp := proc.InitTilePipeline(ctx, styleLayer.MASAddress, conf.ServiceConfig.WorkerNodes, conf.Layers[idx].MaxGrpcRecvMsgSize, conf.Layers[idx].WcsPolygonShardConcLimit, conf.ServiceConfig.MaxGrpcBufferSize, errChan)
		for ir, geoReq := range workerTileRequests[0] {
			if *verbose {
				Info.Printf("WCS: processing tile (%d of %d): xOff:%v, yOff:%v, width:%v, height:%v", ir+1, len(workerTileRequests[0]), geoReq.OffX, geoReq.OffY, geoReq.Width, geoReq.Height)
			}

			hasOverview := len(styleLayer.Overviews) > 0
			if hasOverview {
				bbox, err := utils.GetCanonicalBbox(geoReq.CRS, geoReq.BBox)
				if err == nil {
					reqRes := utils.GetPixelResolution(bbox, geoReq.Width, geoReq.Height)
					iOvr := utils.FindLayerBestOverview(styleLayer, reqRes, false)
					if iOvr >= 0 {
						ovr := styleLayer.Overviews[iOvr]
						geoReq.Collection = ovr.DataSource
						tp.MASAddress = ovr.MASAddress
					}
				} else if *verbose {
					Info.Printf("WCS: processing tile (%d of %d): %v", ir+1, len(workerTileRequests[0]), err)
				}
			}

			select {
			case res := <-tp.Process(geoReq, *verbose):
				if !isInit {
					if ir < len(workerTileRequests[0])-1 {
						isEmptyTile, _ := utils.CheckEmptyTile(res)
						if isEmptyTile {
							continue
						}
					}

					hDstDS, masterTempFile, err = utils.EncodeGdalOpen(conf.ServiceConfig.TempDir, 1024, 256, driverFormat, geot, epsg, res, *params.Width, *params.Height, len(res))
					if err != nil {
						utils.RemoveGdalTempFile(masterTempFile)
						errMsg := fmt.Sprintf("EncodeGdalOpen() failed: %v", err)
						Info.Printf(errMsg)
						metricsCollector.Info.HTTPStatus = 500
						http.Error(w, errMsg, 500)
						return
					}
					defer utils.EncodeGdalClose(&hDstDS)
					defer utils.RemoveGdalTempFile(masterTempFile)

					isInit = true
				}

				bn, err := utils.EncodeGdal(hDstDS, res, geoReq.OffX, geoReq.OffY)
				if err != nil {
					Info.Printf("Error in the utils.EncodeGdal: %v\n", err)
					metricsCollector.Info.HTTPStatus = 500
					http.Error(w, err.Error(), 500)
					return
				}
				bandNames = bn

			case err := <-errChan:
				Info.Printf("WCS: error in the pipeline: %v\n", err)
				metricsCollector.Info.HTTPStatus = 500
				http.Error(w, err.Error(), 500)
				return
			case err := <-workerErrChan:
				Info.Printf("WCS worker error: %v\n", err)
				metricsCollector.Info.HTTPStatus = 500
				http.Error(w, err.Error(), 500)
				return
			case <-ctx.Done():
				Error.Printf("Context cancelled with message: %v\n", ctx.Err())
				metricsCollector.Info.HTTPStatus = 500
				http.Error(w, ctx.Err().Error(), 500)
				return
			case <-timeoutCtx.Done():
				Error.Printf("WCS pipeline timed out, threshold:%v seconds", conf.Layers[idx].WcsTimeout)
				metricsCollector.Info.HTTPStatus = 500
				http.Error(w, "WCS pipeline timed out", 500)
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
						Info.Printf("%v\n", err)
						metricsCollector.Info.HTTPStatus = 500
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
					Info.Printf("%v\n", err)
					metricsCollector.Info.HTTPStatus = 500
					http.Error(w, err.Error(), 500)
					return
				case <-ctx.Done():
					Error.Printf("Context cancelled with message: %v\n", ctx.Err())
					metricsCollector.Info.HTTPStatus = 500
					http.Error(w, ctx.Err().Error(), 500)
					return
				}

				if allWorkerDone {
					break
				}
			}
		}

		utils.EncodeGdalClose(&hDstDS)
		hDstDS = nil

		if *params.Format == "dap4" {
			err := utils.EncodeDap4(w, masterTempFile, bandNames, *verbose)
			if err != nil {
				errMsg := fmt.Sprintf("DAP: error: %v", err)
				Info.Printf(errMsg)
				metricsCollector.Info.HTTPStatus = 500
				http.Error(w, errMsg, 500)
			}
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

		fileHandle, err := os.Open(masterTempFile)
		if err != nil {
			errMsg := fmt.Sprintf("Error opening raster file: %v", err)
			Info.Printf(errMsg)
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, errMsg, 500)
		}
		defer fileHandle.Close()

		fileInfo, err := fileHandle.Stat()
		if err != nil {
			errMsg := fmt.Sprintf("file stat() failed: %v", err)
			Info.Printf(errMsg)
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, errMsg, 500)
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		bytesSent, err := io.Copy(w, fileHandle)
		if err != nil {
			errMsg := fmt.Sprintf("SendFile failed: %v", err)
			Info.Printf(errMsg)
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, errMsg, 500)
		}

		if *verbose {
			Info.Printf("WCS: file_size:%v, bytes_sent:%v\n", fileInfo.Size(), bytesSent)
		}

		return

	default:
		metricsCollector.Info.HTTPStatus = 400
		http.Error(w, fmt.Sprintf("%s not recognised.", *params.Request), 400)
	}
}

func serveWPS(ctx context.Context, params utils.WPSParams, conf *utils.Config, r *http.Request, w http.ResponseWriter, metricsCollector *metrics.MetricsCollector) {
	if params.Request == nil {
		metricsCollector.Info.HTTPStatus = 400
		http.Error(w, "Malformed WPS, a Request field needs to be specified", 400)
		return
	}

	reqURL := r.URL.String()

	switch *params.Request {
	case "GetCapabilities":
		newConf := conf.Copy(r)
		err := utils.ExecuteWriteTemplateFile(w, newConf,
			utils.DataDir+"/templates/WPS_GetCapabilities.tpl")
		if err != nil {
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, err.Error(), 500)
		}
	case "DescribeProcess":
		idx, err := utils.GetProcessIndex(params, conf)
		if err != nil {
			Error.Printf("Requested process not found: %v, %v\n", err, reqURL)
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("%v: %s", err, reqURL), 400)
			return
		}
		process := conf.Processes[idx]
		err = utils.ExecuteWriteTemplateFile(w, process,
			utils.DataDir+"/templates/WPS_DescribeProcess.tpl")
		if err != nil {
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, err.Error(), 500)
		}
	case "Execute":
		idx, err := utils.GetProcessIndex(params, conf)
		if err != nil {
			Error.Printf("Requested process not found: %v, %v\n", err, reqURL)
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("%v: %s", err, reqURL), 400)
			return
		}
		process := conf.Processes[idx]
		if len(process.DataSources) == 0 {
			Error.Printf("No data source specified")
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, "No data source specified", 500)
			return
		}

		if len(params.FeatCol.Features) == 0 {
			Info.Printf("The request does not contain the 'feature' property.\n")
			metricsCollector.Info.HTTPStatus = 400
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
			metricsCollector.Info.Indexer.GeometryArea = area
			if *verbose {
				log.Println("Requested polygon has an area of", area)
			}
			if area == 0.0 || area > process.MaxArea {
				Info.Printf("The requested area %.02f, is too large.\n", area)
				metricsCollector.Info.HTTPStatus = 400
				http.Error(w, "The requested area is too large. Please try with a smaller one.", 400)
				return
			}
			feat, _ = json.Marshal(&geo.Feature{Type: "Feature", Geometry: geom})

		default:
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, "Geometry not supported. Only Features containing Polygon or MultiPolygon are available..", 400)
			return
		}

		var result string
		ctx, ctxCancel := context.WithCancel(ctx)
		defer ctxCancel()
		errChan := make(chan error, 100)

		var suffix string
		if params.GeometryId != nil {
			geoId := strings.TrimSpace(*params.GeometryId)
			if len(geoId) > 0 {
				suffix = fmt.Sprintf("%s", geoId)
			}
		}
		if len(suffix) < 2 {
			suffix = fmt.Sprintf("%04d", rand.Intn(1000))
		}

		for ids, dataSource := range process.DataSources {
			if *verbose {
				log.Printf("WPS: Processing '%v' (%d of %d)", dataSource.DataSource, ids+1, len(process.DataSources))
			}

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
						if *verbose {
							log.Printf("WPS: Failed to parse start date '%v' into ISO format with error: %v, defaulting to no start date", startDateTimeStr, errStart)
						}
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
					if *verbose {
						log.Printf("WPS: invalid input end date '%v' with error '%v'", *params.EndDateTime, errEndInput)
					}
				}
				endDateTimeStr := strings.TrimSpace(dataSource.EndISODate)
				if len(endDateTimeStr) > 0 && strings.ToLower(endDateTimeStr) != "now" {
					dt, errEnd := time.Parse(utils.ISOFormat, endDateTimeStr)
					if errEnd != nil {
						if *verbose {
							log.Printf("WPS: Failed to parse end date '%s' into ISO format with error: %v, defaulting to now()", endDateTimeStr, errEnd)
						}
					} else {
						endDateTime = dt
					}
				}
			} else {
				if !time.Time.IsZero(stEndInput) {
					endDateTime = stEndInput
				}
			}

			clipUpper := float32(math.MaxFloat32)
			if cu, cuOk := params.ClipUppers[fmt.Sprintf("%s_clip_upper", dataSource.Name)]; cuOk {
				clipUpper = cu
			}

			clipLower := float32(-math.MaxFloat32)
			if cl, clOk := params.ClipLowers[fmt.Sprintf("%s_clip_lower", dataSource.Name)]; clOk {
				clipLower = cl
			}

			if clipLower > clipUpper {
				metricsCollector.Info.HTTPStatus = 400
				http.Error(w, "clipLower greater than clipUpper", 400)
				return
			}

			geoReq := proc.GeoDrillRequest{Geometry: string(feat),
				CRS:              "EPSG:4326",
				Collection:       dataSource.DataSource,
				NameSpaces:       dataSource.RGBExpressions.VarList,
				BandExpr:         dataSource.RGBExpressions,
				Mask:             dataSource.Mask,
				VRTURL:           dataSource.VRTURL,
				StartTime:        startDateTime,
				EndTime:          endDateTime,
				ClipUpper:        clipUpper,
				ClipLower:        clipLower,
				MetricsCollector: metricsCollector,
			}

			dp := proc.InitDrillPipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, process.IdentityTol, process.DpTol, errChan)

			if dataSource.BandStrides <= 0 {
				dataSource.BandStrides = 1
			}
			proc := dp.Process(geoReq, suffix, dataSource.MetadataURL, dataSource.BandStrides, *process.Approx, process.DrillAlgorithm, *verbose)

			select {
			case res := <-proc:
				result += res
			case err := <-errChan:
				Info.Printf("Error in the pipeline: %v\n", err)
				metricsCollector.Info.HTTPStatus = 500
				http.Error(w, err.Error(), 500)
				return
			case <-ctx.Done():
				Error.Printf("Context cancelled with message: %v\n", ctx.Err())
				metricsCollector.Info.HTTPStatus = 500
				http.Error(w, ctx.Err().Error(), 500)
				return
			}
		}

		err = utils.ExecuteWriteTemplateFile(w, result, utils.DataDir+"/templates/WPS_Execute.tpl")
		if err != nil {
			metricsCollector.Info.HTTPStatus = 500
			http.Error(w, err.Error(), 500)
		}

	default:
		metricsCollector.Info.HTTPStatus = 400
		http.Error(w, fmt.Sprintf("%s not recognised.", *params.Request), 400)
	}
}

// owsHandler handles every request received on /ows
func generalHandler(conf *utils.Config, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")
	if *verbose {
		Info.Printf("%s\n", r.URL.String())
	}
	ctx := r.Context()

	metricsCollector := metrics.NewMetricsCollector(metricsLogger)
	defer metricsCollector.Log()

	t0 := time.Now()
	metricsCollector.Info.ReqTime = t0.Format(utils.ISOFormat)
	defer func() { metricsCollector.Info.ReqDuration = time.Since(t0) }()

	reqUrl, e := url.QueryUnescape(r.URL.String())
	if e == nil {
		metricsCollector.Info.URL.RawURL = reqUrl
	} else {
		metricsCollector.Info.URL.RawURL = r.URL.String()
	}

	remoteAddr := utils.ParseRemoteAddr(r)
	metricsCollector.Info.RemoteAddr = remoteAddr
	metricsCollector.Info.HTTPStatus = 200

	var query map[string][]string
	var err error
	switch r.Method {
	case "POST":
		query, err = utils.ParsePost(r.Body)
		if err != nil {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Error parsing WPS POST payload: %s", err), 400)
			return
		}

	case "GET":
		query, err = utils.ParseQuery(r.URL.RawQuery)
		if err != nil {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Failed to parse query: %v", err), 400)
			return
		}
	}

	if _, fOK := query["dap4.ce"]; fOK {
		if len(query["dap4.ce"]) == 0 {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, "Failed to parse dap4.ce", 400)
			return
		}
		serveDap(ctx, conf, r, w, query, metricsCollector)
		return
	}

	if _, fOK := query["service"]; !fOK {
		canInferService := false
		if request, hasReq := query["request"]; hasReq {
			reqService := map[string]string{
				"GetFeatureInfo":   "WMS",
				"GetMap":           "WMS",
				"DescribeLayer":    "WMS",
				"GetLegendGraphic": "WMS",
				"DescribeCoverage": "WCS",
				"GetCoverage":      "WCS",
				"DescribeProcess":  "WPS",
				"Execute":          "WPS",
			}
			if service, found := reqService[request[0]]; found {
				query["service"] = []string{service}
				canInferService = true
			}
		}

		if !canInferService {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Not a OWS request. Request does not contain a 'service' parameter."), 400)
			return
		}
	}

	switch query["service"][0] {
	case "WMS":
		params, err := utils.WMSParamsChecker(query, reWMSMap)
		if err != nil {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Wrong WMS parameters on URL: %s", err), 400)
			return
		}
		serveWMS(ctx, params, conf, r, w, metricsCollector)
	case "WCS":
		params, err := utils.WCSParamsChecker(query, reWCSMap)
		if err != nil {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Wrong WCS parameters on URL: %s", err), 400)
			return
		}
		serveWCS(ctx, params, conf, r, w, query, metricsCollector)
	case "WPS":
		params, err := utils.WPSParamsChecker(query, reWPSMap)
		if _, hasId := query["identifier"]; hasId && r.Method == "POST" {
			if params.Identifier != nil {
				url := metricsCollector.Info.URL.RawURL
				var sep string
				if len(query) > 0 {
					sep = "&"
				} else {
					sep = "?"
				}
				metricsCollector.Info.URL.RawURL = fmt.Sprintf("%s%sidentifier=%s", url, sep, *params.Identifier)
			}
		}
		if err != nil {
			metricsCollector.Info.HTTPStatus = 400
			http.Error(w, fmt.Sprintf("Wrong WPS parameters on URL: %s", err), 400)
			return
		}
		serveWPS(ctx, params, conf, r, w, metricsCollector)
	default:
		metricsCollector.Info.HTTPStatus = 400
		http.Error(w, fmt.Sprintf("Not a valid OWS request. URL %s does not contain a valid 'request' parameter.", r.URL.String()), 400)
		return
	}
}

func owsHandler(w http.ResponseWriter, r *http.Request) {
	namespace := "."
	if len(r.URL.Path) > len("/ows/") {
		namespace = r.URL.Path[len("/ows/"):]
		dapExt := ".dap"
		if len(namespace) >= len(dapExt) && namespace[len(namespace)-len(dapExt):] == dapExt {
			namespace = namespace[:len(namespace)-len(dapExt)]
		}
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

func fileHandler(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.URL.Path = upath
	}
	upath = path.Clean(upath)
	upath = filepath.Join(utils.DataDir+"/static", upath)

	if *verbose {
		Info.Printf("%s -> %s\n", r.URL.String(), upath)
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")
	http.ServeFile(w, r, upath)
}

func main() {
	http.HandleFunc("/", fileHandler)
	http.HandleFunc("/ows", owsHandler)
	http.HandleFunc("/ows/", owsHandler)

	Info.Printf("GSKY is ready")
	log.Fatal(http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", *port), nil))
}
