package utils

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var LibexecDir = "."
var EtcDir = "."
var DataDir = "."

const ReservedMemorySize = 1.5 * 1024 * 1024 * 1024

type ServiceConfig struct {
	OWSHostname string   `json:"ows_hostname"`
	MASAddress  string   `json:"mas_address"`
	WorkerNodes []string `json:"worker_nodes"`
}

// CacheLevel contains the source files of one layer as well as the
// max and min resolutions it represents.  Its fields encode the path
// to the collection and the maximum and minimum resolution that can
// be served.  This enables the definition of different versions of
// the same dataset with different resolutions to speedup response
// times.
type CacheLevel struct {
	Path   string  `json:"path"`
	MinRes float64 `json:"min_res"`
	MaxRes float64 `json:"max_res"`
}

type Mask struct {
	ID         string   `json:"id"`
	Value      string   `json:"value"`
	DataSource string   `json:"data_source"`
	Inclusive  bool     `json:"inclusive"`
	BitTests   []string `json:"bit_tests"`
}

type Palette struct {
	Interpolate bool         `json:"interpolate"`
	Colours     []color.RGBA `json:"colours"`
}

// Layer contains all the details that a layer needs
// to be published and rendered
type Layer struct {
	OWSHostname string `json:"ows_hostname"`
	NameSpace   string
	Name        string `json:"name"`
	Title       string `json:"title"`
	Abstract    string `json:"abstract"`
	MetadataURL string `json:"metadata_url"`
	DataURL     string `json:"data_url"`
	//CacheLevels  []CacheLevel `json:"cache_levels"`
	DataSource               string   `json:"data_source"`
	StartISODate             string   `json:"start_isodate"`
	EndISODate               string   `json:"end_isodate"`
	StepDays                 int      `json:"step_days"`
	StepHours                int      `json:"step_hours"`
	StepMinutes              int      `json:"step_minutes"`
	Accum                    bool     `json:"accum"`
	TimeGen                  string   `json:"time_generator"`
	ResFilter                *int     `json:"resolution_filter"`
	Dates                    []string `json:"dates"`
	RGBProducts              []string `json:"rgb_products"`
	Mask                     *Mask    `json:"mask"`
	OffsetValue              float64  `json:"offset_value"`
	ClipValue                float64  `json:"clip_value"`
	ScaleValue               float64  `json:"scale_value"`
	Palette                  *Palette `json:"palette"`
	LegendPath               string   `json:"legend_path"`
	ZoomLimit                float64  `json:"zoom_limit"`
	MaxGrpcRecvMsgSize       int      `json:"max_grpc_recv_msg_size"`
	WmsPolygonSegments       int      `json:"wms_polygon_segments"`
	WcsPolygonSegments       int      `json:"wcs_polygon_segments"`
	WmsTimeout               int      `json:"wms_timeout"`
	WcsTimeout               int      `json:"wcs_timeout"`
	GrpcWmsConcPerNode       int      `json:"grpc_wms_conc_per_node"`
	GrpcWcsConcPerNode       int      `json:"grpc_wcs_conc_per_node"`
	WmsPolygonShardConcLimit int      `json:"wms_polygon_shard_conc_limit"`
	WcsPolygonShardConcLimit int      `json:"wcs_polygon_shard_conc_limit"`
	BandEval                 []string `json:"band_eval"`
	BandStrides              int      `json:"band_strides"`
}

// Process contains all the details that a WPS needs
// to be published and processed
type Process struct {
	DataSources []Layer    `json:"data_sources"`
	Identifier  string     `json:"identifier"`
	Title       string     `json:"title"`
	Abstract    string     `json:"abstract"`
	MaxArea     float64    `json:"max_area"`
	LiteralData []LitData  `json:"literal_data"`
	ComplexData []CompData `json:"complex_data"`
	IdentityTol float64    `json:"identity_tol"`
	DpTol       float64    `json:"dp_tol"`
}

// LitData contains the description of a variable used to compute a
// WPS operation
type LitData struct {
	Identifier    string   `json:"identifier"`
	Title         string   `json:"title"`
	Abstract      string   `json:"abstract"`
	DataType      string   `json:"data_type"`
	DataTypeRef   string   `json:"data_type_ref"`
	AllowedValues []string `json:"allowed_values"`
}

// CompData contains the description of a variable used to compute a
// WPS operation
type CompData struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Abstract   string `json:"abstract"`
	MimeType   string `json:"mime_type"`
	Encoding   string `json:"encoding"`
	Schema     string `json:"schema"`
}

// Config is the struct representing the configuration
// of a WMS server. It contains information about the
// file index API as well as the list of WMS layers that
// can be served.
type Config struct {
	ServiceConfig ServiceConfig `json:"service_config"`
	Layers        []Layer       `json:"layers"`
	Processes     []Process     `json:"processes"`
}

// string used to format Go ISO times
const ISOFormat = "2006-01-02T15:04:05.000Z"

func GenerateDatesAux(start, end time.Time, stepMins time.Duration) []string {
	dates := []string{}
	for start.Before(end) {
		dates = append(dates, start.Format(ISOFormat))
		start = time.Date(start.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return dates
}

// GenerateDatesMCD43A4 function is used to generate the list of ISO
// dates from its especification in the Config.Layer struct.
func GenerateDatesMCD43A4(start, end time.Time, stepMins time.Duration) []string {
	dates := []string{}
	year := start.Year()
	for start.Before(end) {
		for start.Year() == year && start.Before(end) {
			dates = append(dates, start.Format(ISOFormat))
			start = start.Add(stepMins)
		}
		if !start.Before(end) {
			break
		}
		year = start.Year()
		start = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return dates
}

func GenerateDatesGeoglam(start, end time.Time, stepMins time.Duration) []string {
	dates := []string{}
	year := start.Year()
	for start.Before(end) {
		for start.Year() == year && start.Before(end) {
			dates = append(dates, start.Format(ISOFormat))
			nextDate := start.AddDate(0, 0, 4)
			if start.Month() == nextDate.Month() {
				start = start.Add(stepMins)
			} else {
				start = nextDate
			}

		}
		if !start.Before(end) {
			break
		}
		year = start.Year()
		start = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return dates
}

func GenerateDatesChirps20(start, end time.Time, stepMins time.Duration) []string {
	dates := []string{}
	for start.Before(end) {
		dates = append(dates, time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC).Format(ISOFormat))
		dates = append(dates, time.Date(start.Year(), start.Month(), 11, 0, 0, 0, 0, time.UTC).Format(ISOFormat))
		dates = append(dates, time.Date(start.Year(), start.Month(), 21, 0, 0, 0, 0, time.UTC).Format(ISOFormat))
		start = start.AddDate(0, 1, 0)
	}
	return dates
}

func GenerateMonthlyDates(start, end time.Time, stepMins time.Duration) []string {
	dates := []string{}
	for start.Before(end) {
		start = start.AddDate(0, 1, 0)
		dates = append(dates, time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC).Format(ISOFormat))
	}
	return dates
}

func GenerateYearlyDates(start, end time.Time, stepMins time.Duration) []string {
	dates := []string{}
	for start.Before(end) {
		start = start.AddDate(1, 0, 0)
		dates = append(dates, time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC).Format(ISOFormat))
	}
	return dates
}

func GenerateDatesRegular(start, end time.Time, stepMins time.Duration) []string {
	dates := []string{}
	for start.Before(end) {
		dates = append(dates, start.Format(ISOFormat))
		start = start.Add(stepMins)
	}
	return dates
}

func GenerateDatesMas(start, end string, masAddress string, collection string, namespaces []string) []string {
	emptyDates := []string{}

	start = strings.TrimSpace(start)
	if len(start) > 0 {
		_, err := time.Parse(ISOFormat, start)
		if err != nil {
			log.Printf("start date parsing error: %v", err)
			return emptyDates
		}
	}

	end = strings.TrimSpace(end)
	if len(end) > 0 {
		_, err := time.Parse(ISOFormat, end)
		if err != nil {
			log.Printf("end date parsing error: %v", err)
			return emptyDates
		}
	}

	ns := strings.Join(namespaces, ",")
	url := strings.Replace(fmt.Sprintf("http://%s%s?timestamps&time=%s&until=%s&namespace=%s", masAddress, collection, start, end, ns), " ", "%20", -1)
	log.Printf("config querying MAS for timestamps: %v", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("MAS http error: %v,%v", url, err)
		return emptyDates
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("MAS http error: %v,%v", url, err)
		return emptyDates
	}

	type MasTimestamps struct {
		Timestamps []string `json:"timestamps"`
	}

	var timestamps MasTimestamps
	err = json.Unmarshal(body, &timestamps)
	if err != nil {
		log.Printf("MAS json response error: %v", err)
		return emptyDates
	}

	return timestamps.Timestamps
}

func GenerateDates(name string, start, end time.Time, stepMins time.Duration) []string {
	dateGen := make(map[string]func(time.Time, time.Time, time.Duration) []string)
	dateGen["aux"] = GenerateDatesAux
	dateGen["mcd43"] = GenerateDatesMCD43A4
	dateGen["geoglam"] = GenerateDatesGeoglam
	dateGen["chirps20"] = GenerateDatesChirps20
	dateGen["regular"] = GenerateDatesRegular
	dateGen["monthly"] = GenerateMonthlyDates
	dateGen["yearly"] = GenerateYearlyDates

	return dateGen[name](start, end, stepMins)
}

func LoadAllConfigFiles(rootDir string) (map[string]*Config, error) {
	configMap := make(map[string]*Config)
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.Name() == "config.json" {
			relPath, _ := filepath.Rel(rootDir, filepath.Dir(path))
			log.Printf("Loading config file: %s under namespace: %s\n", path, relPath)

			config := &Config{}
			e := config.LoadConfigFile(path)
			if e != nil {
				return e
			}

			configMap[relPath] = config

			for i := range config.Layers {
				ns := relPath
				if relPath == "." {
					ns = ""
				}
				config.Layers[i].NameSpace = ns
			}
		}
		return nil
	})

	if err == nil && len(configMap) == 0 {
		err = fmt.Errorf("No config file found")
	}

	return configMap, err
}

const DefaultRecvMsgSize = 10 * 1024 * 1024

const DefaultWmsPolygonSegments = 2
const DefaultWcsPolygonSegments = 10

const DefaultWmsTimeout = 20
const DefaultWcsTimeout = 30

const DefaultGrpcWmsConcPerNode = 16
const DefaultGrpcWcsConcPerNode = 16

const DefaultWmsPolygonShardConcLimit = 5
const DefaultWcsPolygonShardConcLimit = 10

// LoadConfigFile marshalls the config.json document returning an
// instance of a Config variable containing all the values
func (config *Config) LoadConfigFile(configFile string) error {
	*config = Config{}
	cfg, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("Error while reading config file: %s. Error: %v", configFile, err)
	}

	err = json.Unmarshal(cfg, config)
	if err != nil {
		return fmt.Errorf("Error at JSON parsing config document: %s. Error: %v", configFile, err)
	}
	for i, layer := range config.Layers {
		if strings.TrimSpace(strings.ToLower(layer.TimeGen)) == "mas" {
			config.Layers[i].Dates = GenerateDatesMas(layer.StartISODate, layer.EndISODate, config.ServiceConfig.MASAddress, config.Layers[i].DataSource, config.Layers[i].RGBProducts)
		} else {
			start, errStart := time.Parse(ISOFormat, layer.StartISODate)
			if errStart != nil {
				log.Printf("start date parsing error: %v", errStart)
			}

			end, errEnd := time.Parse(ISOFormat, layer.EndISODate)
			if errEnd != nil {
				log.Printf("end date parsing error: %v", errEnd)
			}

			step := time.Minute * time.Duration(60*24*layer.StepDays+60*layer.StepHours+layer.StepMinutes)
			config.Layers[i].Dates = GenerateDates(layer.TimeGen, start, end, step)
		}
		config.Layers[i].OWSHostname = config.ServiceConfig.OWSHostname

		if config.Layers[i].MaxGrpcRecvMsgSize <= DefaultRecvMsgSize {
			config.Layers[i].MaxGrpcRecvMsgSize = DefaultRecvMsgSize
		}

		if config.Layers[i].WmsPolygonSegments <= DefaultWmsPolygonSegments {
			config.Layers[i].WmsPolygonSegments = DefaultWmsPolygonSegments
		}

		if config.Layers[i].WcsPolygonSegments <= DefaultWcsPolygonSegments {
			config.Layers[i].WcsPolygonSegments = DefaultWcsPolygonSegments
		}

		if config.Layers[i].WmsTimeout <= 0 {
			config.Layers[i].WmsTimeout = DefaultWmsTimeout
		}

		if config.Layers[i].WcsTimeout <= 0 {
			config.Layers[i].WcsTimeout = DefaultWcsTimeout
		}

		if config.Layers[i].GrpcWmsConcPerNode <= 0 {
			config.Layers[i].GrpcWmsConcPerNode = DefaultGrpcWmsConcPerNode
		}

		if config.Layers[i].GrpcWcsConcPerNode <= 0 {
			config.Layers[i].GrpcWcsConcPerNode = DefaultGrpcWcsConcPerNode
		}

		if config.Layers[i].WmsPolygonShardConcLimit <= 0 {
			config.Layers[i].WmsPolygonShardConcLimit = DefaultWmsPolygonShardConcLimit
		}

		if config.Layers[i].WcsPolygonShardConcLimit <= 0 {
			config.Layers[i].WcsPolygonShardConcLimit = DefaultWcsPolygonShardConcLimit
		}

		if layer.Palette != nil && layer.Palette.Colours != nil && len(layer.Palette.Colours) < 3 {
			return fmt.Errorf("The colour palette must contain at least 2 colours.")
		}
	}

	for i, proc := range config.Processes {
		if proc.IdentityTol <= 0 {
			config.Processes[i].IdentityTol = -1.0
		}

		if proc.DpTol <= 0 {
			config.Processes[i].DpTol = -1.0
		}
	}
	return nil
}

func WatchConfig(infoLog, errLog *log.Logger, configMap *map[string]*Config) {
	// Catch SIGHUP to automatically reload cache
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-sighup:
				infoLog.Println("Caught SIGHUP, reloading config...")
				confMap, err := LoadAllConfigFiles(EtcDir)
				if err != nil {
					errLog.Printf("Error in loading config files: %v\n", err)
					return
				}

				for k := range *configMap {
					delete(*configMap, k)
				}

				for k := range confMap {
					(*configMap)[k] = confMap[k]
				}
			}
		}
	}()
}
