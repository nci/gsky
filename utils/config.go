package utils

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var LibexecDir = "/usr/local/libexec"
var EtcDir = "/usr/local/etc"
var DataDir = "/usr/local/share"

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
	ID    string `json:"id"`
	Value string `json:"value"`
}

type Palette struct {
	Interpolate bool         `json:"interpolate"`
	Colours     []color.RGBA `json:"colours"`
}

// Layer contains all the details that a layer needs
// to be published and rendered
type Layer struct {
	OWSHostname string `json:"ows_hostname"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Abstract    string `json:"abstract"`
	MetadataURL string `json:"metadata_url"`
	DataURL     string `json:"data_url"`
	//CacheLevels  []CacheLevel `json:"cache_levels"`
	DataSource   string   `json:"data_source"`
	StartISODate string   `json:"start_isodate"`
	EndISODate   string   `json:"end_isodate"`
	StepDays     int      `json:"step_days"`
	StepHours    int      `json:"step_hours"`
	StepMinutes  int      `json:"step_minutes"`
	Accum        bool     `json:"accum"`
	TimeGen      string   `json:"time_generator"`
	ResFilter    *int     `json:"resolution_filter"`
	Dates        []string `json:"dates"`
	RGBProducts  []string `json:"rgb_products"`
	Mask         *Mask    `json:"mask"`
	OffsetValue  float64  `json:"offset_value"`
	ClipValue    float64  `json:"clip_value"`
	ScaleValue   float64  `json:"scale_value"`
	Palette      *Palette `json:"palette"`
	LegendPath   string   `json:"legend_path"`
	ZoomLimit    float64  `json:"zoom_limit"`
}

// Process contains all the details that a WPS needs
// to be published and processed
type Process struct {
	Paths       []string   `json:"paths"`
	Identifier  string     `json:"identifier"`
	Title       string     `json:"title"`
	Abstract    string     `json:"abstract"`
	MaxArea     float64    `json:"max_area"`
	LiteralData []LitData  `json:"literal_data"`
	ComplexData []CompData `json:"complex_data"`
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

func GenerateDates(name string, start, end time.Time, stepMins time.Duration) []string {
	dateGen := make(map[string]func(time.Time, time.Time, time.Duration) []string)
	dateGen["aux"] = GenerateDatesAux
	dateGen["mcd43"] = GenerateDatesMCD43A4
	dateGen["chirps20"] = GenerateDatesChirps20
	dateGen["regular"] = GenerateDatesRegular
	dateGen["monthly"] = GenerateMonthlyDates
	dateGen["yearly"] = GenerateYearlyDates

	return dateGen[name](start, end, stepMins)
}

// LoadConfigFile marshall the config.json document returning an
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
		start, _ := time.Parse(ISOFormat, layer.StartISODate)
		end, _ := time.Parse(ISOFormat, layer.EndISODate)
		step := time.Minute * time.Duration(60*24*layer.StepDays+60*layer.StepHours+layer.StepMinutes)
		config.Layers[i].Dates = GenerateDates(layer.TimeGen, start, end, step)
		config.Layers[i].OWSHostname = config.ServiceConfig.OWSHostname

		if layer.Palette != nil && layer.Palette.Colours != nil && len(layer.Palette.Colours) < 3 {
			return fmt.Errorf("The colour palette must contain at least 2 colours.")
		}
		/*
			if _, err := os.Stat(layer.DataSource); err != nil {
				return fmt.Errorf("Config layer %s DataSource defined a not reachable location.", layer.Name)
			}
		*/
	}
	return nil
}

func (config *Config) Watch(infoLog, errLog *log.Logger) {
	// Catch SIGHUP to automatically reload cache
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		select {
		case <-sighup:
			infoLog.Println("Caught SIGHUP, reloading config...")
			err := config.LoadConfigFile(EtcDir + "/config.json")
			if err != nil {
				errLog.Printf("%v\n", err)
				panic(err)
			}
		}
	}()
}
