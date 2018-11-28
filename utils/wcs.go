package utils

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// WCSParams contains the serialised version
// of the parameters contained in a WCS request.
type WCSParams struct {
	Service   *string    `json:"service,omitempty"`
	Version   *string    `json:"version,omitempty"`
	Request   *string    `json:"request,omitempty"`
	Coverages []string   `json:"coverage,omitempty"`
	CRS       *string    `json:"crs,omitempty"`
	ReqCRS    *string    `json:"req_crs,omitempty"`
	BBox      []float64  `json:"bbox,omitempty"`
	Time      *time.Time `json:"time,omitempty"`
	Height    *int       `json:"height,omitempty"`
	Width     *int       `json:"width,omitempty"`
	Format    *string    `json:"format,omitempty"`
	Styles    []string   `json:"styles,omitempty"`
}

// WCSRegexpMap maps WCS request parameters to
// regular expressions for doing validation
// when parsing.
// --- These regexp do not avoid every case of
// --- invalid code but filter most of the malformed
// --- cases. Error free JSON deserialisation into types
// --- also validates correct values.
var WCSRegexpMap = map[string]string{"service": `^WCS$`,
	"request":  `^GetCapabilities$|^DescribeCoverage$|^GetCoverage$`,
	"coverage": `^[A-Za-z.:0-9\s_-]+$`,
	"crs":      `^(?i)(?:[A-Z]+):(?:[0-9]+)$`,
	"bbox":     `^[-+]?[0-9]*\.?[0-9]*([eE][-+]?[0-9]+)?(,[-+]?[0-9]*\.?[0-9]*([eE][-+]?[0-9]+)?){3}$`,
	"time":     `^\d{4}-(?:1[0-2]|0[1-9])-(?:3[01]|0[1-9]|[12][0-9])T[0-2]\d:[0-5]\d:[0-5]\d(\.\d+)?Z$`,
	"width":    `^[-+]?[0-9]+$`,
	"height":   `^[-+]?[0-9]+$`,
	"format":   `^(?i)(GeoTIFF|NetCDF)$`}

func CompileWCSRegexMap() map[string]*regexp.Regexp {
	REMap := make(map[string]*regexp.Regexp)
	for key, re := range WCSRegexpMap {
		REMap[key] = regexp.MustCompile(re)
	}

	return REMap
}

// CheckWCSVersion checks if the requested
// version of WCS is supported by the server
func CheckWCSVersion(version string) bool {
	return version == "1.0.0"
}

// WCSParamsChecker checks and marshals the content
// of the parameters of a WCS request into a
// WCSParams struct.
func WCSParamsChecker(params map[string][]string, compREMap map[string]*regexp.Regexp) (WCSParams, error) {

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
		if compREMap["request"].MatchString(request[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"request":"%s"`, request[0]))
		}
	}

	if coverage, coverageOK := params["coverage"]; coverageOK {
		if compREMap["coverage"].MatchString(coverage[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"coverage":["%s"]`, coverage[0]))
		}
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

	if format, formatOK := params["format"]; formatOK {
		if compREMap["format"].MatchString(format[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"format":"%s"`, format[0]))
		}
	}

	if styles, stylesOK := params["styles"]; stylesOK {
		if !strings.Contains(styles[0], "\"") {
			jsonFields = append(jsonFields, fmt.Sprintf(`"styles":["%s"]`, strings.Replace(styles[0], ",", "\",\"", -1)))
		}
	}

	jsonParams := fmt.Sprintf("{%s}", strings.Join(jsonFields, ","))

	var wcsParams WCSParams
	err := json.Unmarshal([]byte(jsonParams), &wcsParams)
	return wcsParams, err
}

// GetCoverageIndex returns the index of the
// specified layer inside the Config.Layers
// field.
func GetCoverageIndex(params WCSParams, config *Config) (int, error) {
	if params.Coverages != nil {
		product := params.Coverages[0]
		for i := range config.Layers {
			if config.Layers[i].Name == product {
				return i, nil
			}
		}
		return -1, fmt.Errorf("%s not found in config Layers", product)
	}
	return -1, fmt.Errorf("WCS request doesn't specify a product")
}

// GetCoverageStyleIndex returns the index of the
// specified style inside a coverage
func GetCoverageStyleIndex(params WCSParams, config *Config, covIdx int) (int, error) {
	if params.Styles != nil {
		style := strings.TrimSpace(params.Styles[0])
		if len(style) == 0 {
			return -1, nil
		}
		for i := range config.Layers[covIdx].Styles {
			if config.Layers[covIdx].Styles[i].Name == style {
				return i, nil
			}
		}
		return -1, fmt.Errorf("style %s not found in this coverage", style)
	}
	return -1, nil
}
