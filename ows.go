package main

/* wms is a web server implementing the WMS protocol
   to serve maps. This server is intended to be consumed
   directly by users and exposes a series of functionalities
   through the GetCapabilities.xml document.
   Configuration of the server is specified at the config.json
   file where features such as layers or color scales can be
   defined.
   This server depends on two other services to operate: the
   index server which registers the files involved in one operation
   and the warp server which performs the actual rendering of
   a tile.
   Most of the functionality for this service is contained in the
   utils/wms.go file */

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

	geo "bitbucket.org/monkeyforecaster/geometry"
	_ "net/http/pprof"
)

// Global variable to hold the values specified
// on the config.json document.
var config *utils.Config

var (
	port = flag.Int("p", 8080, "Server listening port.")
	//conc = flag.Int("c", 100, "Maximum number of concurrent requests.")
)

var reWMSMap map[string]*regexp.Regexp
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

	filePaths := [] string {
		utils.EtcDir + "/config.json",
		utils.DataDir + "/templates/WMS_GetCapabilities.tpl",
		utils.DataDir + "/templates/WMS_DescribeLayer.tpl",
		utils.DataDir + "/templates/WMS_ServiceException.tpl",
		utils.DataDir + "/templates/WPS_DescribeProcess.tpl",
		utils.DataDir + "/templates/WPS_Execute.tpl",
		utils.DataDir + "/templates/WPS_GetCapabilities.tpl" }

	for _, filePath := range filePaths {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			panic(err)
		}
	}

	config = &utils.Config{}
	err := config.LoadConfigFile(utils.EtcDir + "/config.json")
	if err != nil {
		Error.Printf("%v\n", err)
		panic(err)
	}
	config.Watch(Info, Error)

	reWMSMap = utils.CompileWMSRegexMap()
	reWPSMap = utils.CompileWPSRegexMap()

}

// TODO This is a mocked version of the real load balancer sitting in front to the service
func LoadBalance(servers []string) string {
	return servers[rand.Intn(len(servers))]
}

func serveWMS(ctx context.Context, params utils.WMSParams, conf *utils.Config, reqURL string, w http.ResponseWriter) {

	if params.Request == nil {
		http.Error(w, "Malformed WMS, a Request field needs to be specified", 400)
		return
	}

	switch *params.Request {
	case "GetCapabilities":
		if params.Version == nil {
			http.Error(w, fmt.Sprintf("This server can only accept WMS requests compliant with version 1.1.1 and 1.3.0: %s", reqURL), 400)
			return
		}

		err := utils.ExecuteWriteTemplateFile(w, conf,
			utils.DataDir + "/templates/WMS_GetCapabilities.tpl")
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
			utils.DataDir + "/templates/WMS_DescribeLayer.tpl")
		if err != nil {
			http.Error(w, err.Error(), 500)
		}

	case "GetMap":
		if params.Version == nil {
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
			firstTime, err := time.Parse(utils.ISOFormat, conf.Layers[idx].Dates[0])
			if err != nil {
				http.Error(w, fmt.Sprintf("Cannot find a valid date to proceed with the request: %s", reqURL), 400)
				return
			}
			params.Time = &firstTime
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

		geoReq := &proc.GeoTileRequest{ConfigPayLoad: proc.ConfigPayLoad{NameSpaces: conf.Layers[idx].RGBProducts,
			Mask:    conf.Layers[idx].Mask,
			Palette: conf.Layers[idx].Palette,
			ScaleParams: proc.ScaleParams{Offset: conf.Layers[idx].OffsetValue,
				Scale: conf.Layers[idx].ScaleValue,
				Clip:  conf.Layers[idx].ClipValue,
			},
			ZoomLimit: conf.Layers[idx].ZoomLimit,
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
		errChan := make(chan error)
		//start := time.Now()
		tp := proc.InitTilePipeline(ctx, config.ServiceConfig.MASAddress, LoadBalance(config.ServiceConfig.WorkerNodes), errChan)
		//log.Println("Pipeline Init Time", time.Since(start))
		select {
		case res := <-tp.Process(geoReq):
			w.Write(res)
			//log.Println("Pipeline Total Time", time.Since(start))
		case err := <-errChan:
			ctxCancel()
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
			utils.ExecuteWriteTemplateFile(w, params.Layers[0],
				utils.DataDir + "/templates/WMS_ServiceException.tpl")
			return
		}

		b, err := ioutil.ReadFile(conf.Layers[idx].LegendPath)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Write(b)

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
			utils.DataDir + "/templates/WPS_GetCapabilities.tpl")
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
	case "DescribeProcess":
		for _, process := range conf.Processes {
			if process.Identifier == *params.Identifier {
				err := utils.ExecuteWriteTemplateFile(w, process,
					utils.DataDir + "/templates/WPS_DescribeProcess.tpl")
				if err != nil {
					http.Error(w, err.Error(), 500)
				}
				break
			}
		}
	case "Execute":
		if len(params.FeatCol.Features) == 0 {
			err := utils.ExecuteWriteTemplateFile(w, "Request doesn't contain any Feature.",
				utils.DataDir + "/templates/WPS_Exception.tpl")
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
			return

		}

		if len(conf.Processes) != 1 || len(conf.Processes[0].Paths) != 2 {
			http.Error(w, `Request cannot be processed. Error detected in the contents of the "processes" object on config file.`, 400)
			return
		}

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

		geoReq1 := proc.GeoDrillRequest{Geometry: string(feat),
			CRS:        "EPSG:4326",
			Collection: conf.Processes[0].Paths[0],
			NameSpaces: []string{"phot_veg", "nphot_veg", "bare_soil"},
			StartTime:  time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:    time.Date(2017, 6, 1, 0, 0, 0, 0, time.UTC),
		}
		geoReq2 := proc.GeoDrillRequest{Geometry: string(feat),
			CRS:        "EPSG:4326",
			Collection: conf.Processes[0].Paths[1],
			NameSpaces: []string{""},
			StartTime:  time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			EndTime:    time.Date(2017, 6, 1, 0, 0, 0, 0, time.UTC),
		}

		fmt.Println(geoReq1, geoReq2)

		// This is not concurrent as in the previous version
		var result string
		ctx, ctxCancel := context.WithCancel(ctx)
		errChan := make(chan error)
		//start := time.Now()
		dp1 := proc.InitDrillPipeline(ctx, config.ServiceConfig.MASAddress, config.ServiceConfig.WorkerNodes, errChan)
		proc1 := dp1.Process(geoReq1)
		dp2 := proc.InitDrillPipeline(ctx, config.ServiceConfig.MASAddress, config.ServiceConfig.WorkerNodes, errChan)
		proc2 := dp2.Process(geoReq2)

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
				http.Error(w, ctx.Err().Error(), 500)
				return
			}
		}

		err := utils.ExecuteWriteTemplateFile(w, result,
			utils.DataDir + "/templates/WPS_Execute.tpl")
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
	generalHandler(config, w, r)
}

func main() {
	flag.Parse()

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)
	http.HandleFunc("/ows", owsHandler)
	Info.Printf("GSKY is ready")
	http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", *port), nil)
}
