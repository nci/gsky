package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strings"
	"text/template"
	"time"
)

const ISOZeroTime = "0001-01-01T00:00:00.000Z"

// WMSParams contains the serialised version
// of the parameters contained in a WMS request.
type WMSParams struct {
	Service *string    `json:"service,omitempty"`
	Request *string    `json:"request,omitempty"`
	CRS     *string    `json:"crs,omitempty"`
	BBox    []float64  `json:"bbox,omitempty"`
	Format  *string    `json:"format,omitempty"`
	X       *int       `json:"x,omitempty"`
	Y       *int       `json:"y,omitempty"`
	Height  *int       `json:"height,omitempty"`
	Width   *int       `json:"width,omitempty"`
	Time    *time.Time `json:"time,omitempty"`
	Layers  []string   `json:"layers,omitempty"`
	Styles  []string   `json:"styles,omitempty"`
	Version *string    `json:"version,omitempty"`
}

// WMSRegexpMap maps WMS request parameters to
// regular expressions for doing validation
// when parsing.
// --- These regexp do not avoid every case of
// --- invalid code but filter most of the malformed
// --- cases. Error free JSON deserialisation into types
// --- also validates correct values.
var WMSRegexpMap = map[string]string{"service": `^WMS$`,
	"request": `^GetCapabilities$|^GetFeatureInfo$|^DescribeLayer$|^GetMap$|^GetLegendGraphic$`,
	"crs":     `^(?i)(?:[A-Z]+):(?:[0-9]+)$`,
	"bbox":    `^[-+]?[0-9]*\.?[0-9]*([eE][-+]?[0-9]+)?(,[-+]?[0-9]*\.?[0-9]*([eE][-+]?[0-9]+)?){3}$`,
	"x":       `^[0-9]+$`,
	"y":       `^[0-9]+$`,
	"width":   `^[0-9]+$`,
	"height":  `^[0-9]+$`,
	"time":    `^\d{4}-(?:1[0-2]|0[1-9])-(?:3[01]|0[1-9]|[12][0-9])T[0-2]\d:[0-5]\d:[0-5]\d\.\d+Z$`}

// BBox2Geot return the geotransform from the
// parameters received in a WMS GetMap request
func BBox2Geot(width, height int, bbox []float64) []float64 {
	return []float64{bbox[0], (bbox[2] - bbox[0]) / float64(width), 0, bbox[3], 0, (bbox[1] - bbox[3]) / float64(height)}
}

func CompileWMSRegexMap() map[string]*regexp.Regexp {
	REMap := make(map[string]*regexp.Regexp)
	for key, re := range WMSRegexpMap {
		REMap[key] = regexp.MustCompile(re)
	}

	return REMap
}

func NormaliseKeys(params map[string][]string) map[string][]string {
	// As in WMS 1.1.1 spec: http://cite.opengeospatial.org/OGCTestData/wms/1.1.1/spec/wms1.1.1.html#basic_elements.param_rules.order_and_case
	// Parameter names shall not be case sensitive, but parameter values shall be case sensitive."
	for key, value := range params {
		if key != strings.ToLower(key) {
			params[strings.ToLower(key)] = value
			delete(params, key)
		}
	}
	return params
}

func CheckWMSVersion(version string) bool {
	return version == "1.3.0" || version == "1.1.1"
}

// WMSParamsChecker checks and marshals the content
// of the parameters of a WMS request into a
// WMSParams struct.
func WMSParamsChecker(params map[string][]string, compREMap map[string]*regexp.Regexp) (WMSParams, error) {

	jsonFields := []string{}

	if service, serviceOK := params["service"]; serviceOK {
		if compREMap["service"].MatchString(service[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"service":"%s"`, service[0]))
		}
	}

	if version, versionOK := params["version"]; versionOK {
		jsonFields = append(jsonFields, fmt.Sprintf(`"version":"%s"`, version[0]))
	}

	if request, requestOK := params["request"]; requestOK {
		jsonFields = append(jsonFields, fmt.Sprintf(`"request":"%s"`, request[0]))
	}

	// WMS specifies that coordinate reference systems can be designed by either: ["srs", "crs"]
	if value, srsOK := params["srs"]; srsOK {
		params["crs"] = value
		delete(params, "srs")
	}

	if crs, crsOK := params["crs"]; crsOK {
		if compREMap["crs"].MatchString(crs[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"crs":"%s"`, crs[0]))
		}
	}

	if bbox, bboxOK := params["bbox"]; bboxOK {
		if compREMap["bbox"].MatchString(bbox[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"bbox":[%s]`, bbox[0]))
		}
	}

	if i, iOK := params["i"]; iOK {
		params["x"] = i
	}

	if x, xOK := params["x"]; xOK {
		if compREMap["x"].MatchString(x[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"x":%s`, x[0]))
		}
	}

	if j, jOK := params["j"]; jOK {
		params["y"] = j
	}

	if y, yOK := params["y"]; yOK {
		if compREMap["y"].MatchString(y[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"y":%s`, y[0]))
		}
	}

	if width, widthOK := params["width"]; widthOK {
		if compREMap["width"].MatchString(width[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"width":%s`, width[0]))
		}
	}

	if height, heightOK := params["height"]; heightOK {
		if compREMap["height"].MatchString(height[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"height":%s`, height[0]))
		}
	}

	if time, timeOK := params["time"]; timeOK {
		if compREMap["time"].MatchString(time[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"time":"%s"`, time[0]))
		}
	}

	if layers, layersOK := params["layers"]; layersOK {
		if !strings.Contains(layers[0], "\"") {
			jsonFields = append(jsonFields, fmt.Sprintf(`"layers":["%s"]`, strings.Replace(layers[0], ",", "\",\"", -1)))
		}
	}

	if styles, stylesOK := params["styles"]; stylesOK {
		if !strings.Contains(styles[0], "\"") {
			jsonFields = append(jsonFields, fmt.Sprintf(`"styles":["%s"]`, strings.Replace(styles[0], ",", "\",\"", -1)))
		}
	}

	jsonParams := fmt.Sprintf("{%s}", strings.Join(jsonFields, ","))

	var wmsParams WMSParams
	err := json.Unmarshal([]byte(jsonParams), &wmsParams)
	return wmsParams, err
}

// GetCoordinates returns the x and y
// coordinates in the original projection
// from the tile relative WMS parameters.
func GetCoordinates(params WMSParams) (float64, float64, error) {
	if len(params.BBox) != 4 {
		return 0, 0, fmt.Errorf("No BBox parameter has been specified")
	}
	if params.Width == nil || params.Height == nil {
		return 0, 0, fmt.Errorf("Width and Height have to be bigger than 0")
	}

	return params.BBox[0] + (params.BBox[2]-params.BBox[0])*float64(*params.X)/float64(*params.Width), params.BBox[3] + (params.BBox[1]-params.BBox[3])*float64(*params.Y)/float64(*params.Height), nil
}

// GetLayerIndex returns the index of the
// specified layer inside the Config.Layers
// field.
func GetLayerIndex(params WMSParams, config *Config) (int, error) {
	if params.Layers != nil {
		product := params.Layers[0]
		for i := range config.Layers {
			if config.Layers[i].Name == product {
				return i, nil
			}
		}
		return -1, fmt.Errorf("%s not found in config Layers", product)
	}
	return -1, fmt.Errorf("WMS request doesn't specify a product")
}

func ExecuteWriteTemplateFile(w io.Writer, data interface{}, filePath string) error {
	// General template compilation, execution and writting in to
	// a stream.
	tplStr, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("Error trying to read %s file: %v", filePath, err)
	}
	tpl, err := template.New("template").Parse(string(tplStr))
	if err != nil {
		return fmt.Errorf("Error trying to parse template document: %v", err)
	}
	err = tpl.Execute(w, data)
	if err != nil {
		return fmt.Errorf("Error executing template: %v\n", err)
	}

	return nil
}

// Get current timestamp if time is not specified in the HTTP request
func GetCurrentTimeStamp(timestamps []string) (*time.Time, error) {
	var currentTime time.Time

	// Empty timestamps often indicate something wrong with user data, GSKY config files,
	// or both. We simply fill Now() to prevent the out-of-range index error for the Dates
	// array. The implification of this is that users will get a blank image in the HTTP
	// response instead of the 500 internal server error.
	if len(timestamps) == 0 {
		currentTime = time.Now().UTC()
	} else {
		tmpTime, err := time.Parse(ISOFormat, timestamps[len(timestamps)-1])
		if err != nil {
			return nil, fmt.Errorf("Cannot find a valid date to proceed with the request")
		}
		currentTime = tmpTime
	}

	return &currentTime, nil
}
