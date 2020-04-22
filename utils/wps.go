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
	"unsafe"

	geo "github.com/nci/geometry"
)

type Data struct {
	ComplexData string
	LiteralData string
}

type Input struct {
	Identifier string
	Data       Data
}

type DataInputs struct {
	Input []Input
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
	var exec Execute
	err := xml.Unmarshal(buf.Bytes(), &exec)
	if err != nil {
		return map[string][]string{}, err
	}

	parsedBody := map[string][]string{"status": []string{"true"},
		"service":    []string{exec.Service},
		"request":    []string{"Execute"},
		"version":    []string{exec.Version},
		"identifier": []string{exec.Identifier}}

	for _, input := range exec.DataInputs.Input {
		inputID := strings.ToLower(strings.TrimSpace(input.Identifier))
		if inputID == "start_datetime" {
			parsedBody["start_datetime"] = []string{input.Data.ComplexData}
		} else if inputID == "end_datetime" {
			parsedBody["end_datetime"] = []string{input.Data.ComplexData}
		} else if inputID == "geometry" {
			parsedBody["geometry"] = []string{fmt.Sprintf(`geometry=%s`, input.Data.ComplexData)}
		} else if inputID == "geometry_id" {
			parsedBody["geometry_id"] = []string{input.Data.LiteralData}
		} else if strings.Index(inputID, "_clip_lower") >= 0 || strings.Index(inputID, "_clip_upper") >= 0 {
			parsedBody[inputID] = []string{input.Data.LiteralData}
		}
	}

	return parsedBody, nil
}

// WPSParams contains the serialised version
// of the parameters contained in a WPS request.
type WPSParams struct {
	Service       *string               `json:"service"`
	Request       *string               `json:"request"`
	Identifier    *string               `json:"identifier"`
	StartDateTime *string               `json:"start_datetime"`
	EndDateTime   *string               `json:"end_datetime"`
	Product       *string               `json:"product"`
	FeatCol       geo.FeatureCollection `json:"feature_collection"`
	GeometryId    *string               `json:"geometry_id"`
	ClipUppers    map[string]float32    `json:"clip_uppers"`
	ClipLowers    map[string]float32    `json:"clip_lowers"`
}

// WPSRegexpMap maps WPS request parameters to
// regular expressions for doing validation
// when parsing.
// --- These regexp do not avoid every case of
// --- invalid code but filter most of the malformed
// --- cases. Error free JSON deserialisation into types
// --- also validates correct values.
var WPSRegexpMap = map[string]string{"service": `^WPS$`,
	"request": `^GetCapabilities$|^DescribeProcess$|^Execute$`,
	"time":    `^\d{4}-(?:1[0-2]|0[1-9])-(?:3[01]|0[1-9]|[12][0-9])T[0-2]\d:[0-5]\d$`}

func CompileWPSRegexMap() map[string]*regexp.Regexp {
	REMap := make(map[string]*regexp.Regexp)
	for key, re := range WPSRegexpMap {
		REMap[key] = regexp.MustCompile(re)
	}

	return REMap
}

func extractDateTime(timeObjStr string) (string, error) {
	var timeObj interface{}
	err := json.Unmarshal([]byte(timeObjStr), &timeObj)
	if err != nil {
		return "", err
	}
	prop, propOk := timeObj.(map[string]interface{})["properties"]
	if !propOk {
		return "", fmt.Errorf("'properties' attribute not found")
	}
	timestamp, tsOk := prop.(map[string]interface{})["timestamp"]
	if !tsOk {
		return "", fmt.Errorf("'timestamp' attribute not found")
	}
	datetime, dtOk := timestamp.(map[string]interface{})["date-time"]
	if !dtOk {
		return "", fmt.Errorf("'date-time' attribute not found")
	}
	datetimeStr, dtOk := datetime.(string)
	if !dtOk {
		return "", fmt.Errorf("failed to cast date-time into string")
	}

	return strings.TrimSpace(datetimeStr), nil
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
	} else {
		jsonFields = append(jsonFields, fmt.Sprintf(`"service":""`))
	}

	if request, requestOK := params["request"]; requestOK {
		if compREMap["request"].MatchString(request[0]) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"request":"%s"`, request[0]))
		} else {
			return WPSParams{}, fmt.Errorf("%s is not a valid WPS request", request[0])
		}
	} else {
		return WPSParams{}, fmt.Errorf("WPS 'request' not found")
	}

	if id, idOK := params["identifier"]; idOK {
		jsonFields = append(jsonFields, fmt.Sprintf(`"identifier":"%s"`, id[0]))
	} else {
		jsonFields = append(jsonFields, fmt.Sprintf(`"identifier":""`))
	}

	if startTime, startTimeOK := params["start_datetime"]; startTimeOK {
		timeStr, err := extractDateTime(startTime[0])
		if err != nil {
			return WPSParams{}, fmt.Errorf("Invalid start datetime: %v", startTime[0])
		}

		if compREMap["time"].MatchString(timeStr) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"start_datetime":"%s"`, timeStr+":00.000Z"))
		} else {
			return WPSParams{}, fmt.Errorf("Invalid start datetime format: %v", timeStr)
		}
	} else {
		jsonFields = append(jsonFields, fmt.Sprintf(`"start_datetime":"%s"`, ""))
	}

	if endTime, endTimeOK := params["end_datetime"]; endTimeOK {
		timeStr, err := extractDateTime(endTime[0])
		if err != nil {
			return WPSParams{}, fmt.Errorf("Invalid end datetime: %v", endTime[0])
		}

		if compREMap["time"].MatchString(timeStr) {
			jsonFields = append(jsonFields, fmt.Sprintf(`"end_datetime":"%s"`, timeStr+":00.000Z"))
		} else {
			return WPSParams{}, fmt.Errorf("Invalid end datetime format: %v", timeStr)
		}
	} else {
		jsonFields = append(jsonFields, fmt.Sprintf(`"end_datetime":""`))
	}

	if inputs, inputsOK := params["geometry"]; inputsOK {
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

	if geometryId, geometryIdOk := params["geometry_id"]; geometryIdOk {
		jsonFields = append(jsonFields, fmt.Sprintf(`"geometry_id":"%s"`, geometryId[0]))
	}

	var clipLowerFields []string
	for k, p := range params {
		if strings.Index(k, "_clip_lower") >= 0 {
			clipLowerFields = append(clipLowerFields, fmt.Sprintf(`"%s":%s`, k, p[0]))
		}
	}
	if len(clipLowerFields) > 0 {
		clipLower := strings.Join(clipLowerFields, ",")
		jsonFields = append(jsonFields, fmt.Sprintf(`"clip_lowers":{%s}`, clipLower))
	}

	var clipUpperFields []string
	for k, p := range params {
		if strings.Index(k, "_clip_upper") >= 0 {
			clipUpperFields = append(clipUpperFields, fmt.Sprintf(`"%s":%s`, k, p[0]))
		}
	}
	if len(clipUpperFields) > 0 {
		clipUpper := strings.Join(clipUpperFields, ",")
		jsonFields = append(jsonFields, fmt.Sprintf(`"clip_uppers":{%s}`, clipUpper))
	}

	jsonParams := fmt.Sprintf("{%s}", strings.Join(jsonFields, ","))
	var wpsParamms WPSParams
	err := json.Unmarshal([]byte(jsonParams), &wpsParamms)
	return wpsParamms, err
}

const WGS84WKT = `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.01745329251994328,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]]`

func GetArea(wgs84Poly geo.Geometry) float64 {
	geomJSON, _ := json.Marshal(wgs84Poly)
	geomJSONC := C.CString(string(geomJSON))
	hPt := C.OGR_G_CreateGeometryFromJson(geomJSONC)
	C.free(unsafe.Pointer(geomJSONC))

	wktC := C.CString(WGS84WKT)
	selSRS := C.OSRNewSpatialReference(wktC)
	C.free(unsafe.Pointer(wktC))
	C.OGR_G_AssignSpatialReference(hPt, selSRS)

	area := float64(C.OGR_G_Area(hPt))
	C.OGR_G_DestroyGeometry(hPt)
	C.OSRDestroySpatialReference(selSRS)
	return area
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
