package utils

// #include "gdal.h"
// #include "ogr_api.h"
// #include "ogr_srs_api.h"
// #cgo pkg-config: gdal
import "C"

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"

	geo "github.com/nci/geometry"
)

type Data struct {
	ComplexData string
}

type Input struct {
	Identifier string
	Data       Data
}

type DataInputs struct {
	Input Input
}

type Execute struct {
	Version    string `xml:"version,attr"`
	Service    string `xml:"service,attr"`
	Identifier string
	DataInputs DataInputs
}

func ParsePost(rc io.ReadCloser) (map[string][]string, error) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(rc)
	rc.Close()
	var e Execute
	err := xml.Unmarshal(buf.Bytes(), &e)
	if err != nil {
		return map[string][]string{}, err
	}

	return map[string][]string{"datainputs": []string{fmt.Sprintf(`geometry=%s`, e.DataInputs.Input.Data.ComplexData)}, "status": []string{"true"}, "service": []string{e.Service}, "request": []string{"Execute"}, "version": []string{e.Version}, "identifier": []string{e.Identifier}}, nil
}

// WPSParams contains the serialised version
// of the parameters contained in a WPS request.
type WPSParams struct {
	Service    *string               `json:"service"`
	Request    *string               `json:"request"`
	Identifier *string               `json:"identifier"`
	Product    *string               `json:"product"`
	FeatCol    geo.FeatureCollection `json:"feature_collection"`
}

// WPSRegexMap maps WPS request parameters to
// regular expressions for doing validation
// when parsing.
// --- These regexp do not avoid every case of
// --- invalid code but filter most of the malformed
// --- cases. Error free JSON deserialisation into types
// --- also validates correct values.
var WPSRegexpMap = map[string]string{"service": `^WPS$`,
	"request": `^GetCapabilities$|^DescribeProcess$|^Execute$`,
	"time":    `^\d{4}-(?:1[0-2]|0[1-9])-(?:3[01]|0[1-9]|[12][0-9])T[0-2]\d:[0-5]\d:[0-5]\d\.\d+Z$`}

func CompileWPSRegexMap() map[string]*regexp.Regexp {
	REMap := make(map[string]*regexp.Regexp)
	for key, re := range WPSRegexpMap {
		REMap[key] = regexp.MustCompile(re)
	}

	return REMap
}

// WPSParamsChecker checks and marshals the content
// of the parameters of a WPS request into a
// WPSParams struct.
func WPSParamsChecker(params map[string][]string, compREMap map[string]*regexp.Regexp) (WPSParams, error) {

	jsonFields := []string{}

	if service, serviceOK := params["service"]; serviceOK {
		if compREMap["service"].MatchString(service[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"service":"%s"`, service[0]))
		}
	}

	if request, requestOK := params["request"]; requestOK {
		if compREMap["request"].MatchString(request[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"request":"%s"`, request[0]))
		} else {
			return WPSParams{}, fmt.Errorf("%s is not a valid WPS request", request[0])
		}
	}

	if id, idOK := params["identifier"]; idOK {
		jsonFields = append(jsonFields, fmt.Sprintf(`"identifier":"%s"`, id[0]))
	}

	if inputs, inputsOK := params["datainputs"]; inputsOK {
		rawInputs := strings.Split(inputs[0], ";")
		if len(rawInputs) > 1 {
			prod := strings.Split(rawInputs[0], "=")
			jsonFields = append(jsonFields, fmt.Sprintf(`"product":"%s"`, prod[1]))
			featCol := strings.Split(rawInputs[1], "=")
			jsonFields = append(jsonFields, fmt.Sprintf(`"feature_collection":%s`, featCol[1]))
		} else {
			featCol := strings.Split(rawInputs[0], "=")
			jsonFields = append(jsonFields, fmt.Sprintf(`"feature_collection":%s`, featCol[1]))
		}

	}

	jsonParams := fmt.Sprintf("{%s}", strings.Join(jsonFields, ","))
	var wpsParamms WPSParams
	err := json.Unmarshal([]byte(jsonParams), &wpsParamms)
	return wpsParamms, err
}

const WGS84WKT = `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.01745329251994328,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]]`

func GetArea(wgs84Poly geo.Geometry) float64 {
	geomJSON, _ := json.Marshal(wgs84Poly)
	hPt := C.OGR_G_CreateGeometryFromJson(C.CString(string(geomJSON)))
	selSRS := C.OSRNewSpatialReference(C.CString(WGS84WKT))
	C.OGR_G_AssignSpatialReference(hPt, selSRS)
	return float64(C.OGR_G_Area(hPt))
}

func GetProcessIndex(params WPSParams, config *Config) (int, error) {
	if params.Identifier != nil {
		for i := range config.Processes {
			if config.Processes[i].Identifier == *params.Identifier {
				return i, nil
			}
		}
		return -1, fmt.Errorf("%s not found in config processes", *params.Identifier)
	}
	return -1, fmt.Errorf("WPS request doesn't specify a process")
}

