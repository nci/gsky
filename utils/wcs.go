package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// WCSParams contains the serialised version
// of the parameters contained in a WCS request.
type WCSParams struct {
	Service        *string      `json:"service,omitempty"`
	Version        *string      `json:"version,omitempty"`
	Request        *string      `json:"request,omitempty"`
	Coverages      []string     `json:"coverage,omitempty"`
	CRS            *string      `json:"crs,omitempty"`
	ReqCRS         *string      `json:"req_crs,omitempty"`
	BBox           []float64    `json:"bbox,omitempty"`
	Time           *time.Time   `json:"time,omitempty"`
	Height         *int         `json:"height,omitempty"`
	Width          *int         `json:"width,omitempty"`
	Format         *string      `json:"format,omitempty"`
	Styles         []string     `json:"styles,omitempty"`
	Axes           []*AxisParam `json:"axes,omitempty"`
	BandExpr       *BandExpressions
	NoReprojection bool
	AxisMapping    int
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
	"axis":     `^[A-Za-z_][A-Za-z0-9_]*$`,
	"format":   `^(?i)(GeoTIFF|NetCDF|DAP4)$`}

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

	var wcsParams WCSParams

	axesInfo := []string{}
	for key, val := range params {
		if strings.HasPrefix(key, "dim_") {
			if len(key) <= len("dim_") {
				continue
			}

			axisName := key[len("dim_"):]
			axisName = strings.TrimSpace(axisName)

			if !compREMap["axis"].MatchString(axisName) {
				return wcsParams, fmt.Errorf("invalid axis name: %v", key)
			}

			valFloat64, err := strconv.ParseFloat(val[0], 64)
			if err != nil {
				return wcsParams, fmt.Errorf("the value '%v' for dimension '%v' is not float64", key, val)
			}

			axisVal := valFloat64

			axesInfo = append(axesInfo, fmt.Sprintf(`{"name":"%s", "start":%f, "order":1, "aggregate": 1}`, axisName, axisVal))
		}
	}

	jsonFields = append(jsonFields, fmt.Sprintf(`"axes":[%s]`, strings.Join(axesInfo, ",")))
	jsonParams := fmt.Sprintf("{%s}", strings.Join(jsonFields, ","))

	err := json.Unmarshal([]byte(jsonParams), &wcsParams)
	if err != nil {
		return wcsParams, err
	}

	/**** An example subset query
	//params["subset"] = []string{"time(2013-03-19T00:00:00.000Z, 2013-03-21T00:00:00.000Z)order=desc; aa1_ (12,12.5) order = desc, agg= (union ),  ;  ;a2 ((22)); a3(*)", "b((12.333, ));", " c (*,*); d (*,111)"}
	*/

	if subsets, subsetsOK := params["subset"]; subsetsOK {
		sub := strings.Join(subsets, ";")
		axes, err := parseSubsetClause(sub, compREMap)
		if err != nil {
			return wcsParams, err
		}

		for _, axis := range axes {
			wcsParams.Axes = append(wcsParams.Axes, axis)
		}
	}

	foundTime := false
	axesMap := make(map[string]*AxisParam)
	for _, axis := range wcsParams.Axes {
		axesMap[axis.Name] = axis

		if axis.Name == "time" {
			foundTime = true
		}
	}

	if !foundTime {
		wcsParams.Axes = append(wcsParams.Axes, &AxisParam{Name: "time", Aggregate: 1})
	}

	codeFormats, codeFormatOK := params["code_format"]
	var codeFormat string
	if codeFormatOK {
		codeFormat = strings.ToLower(strings.TrimSpace(codeFormats[0]))
		if codeFormat != "plain" && codeFormat != "base64" {
			return wcsParams, fmt.Errorf("code_format must be either plain or base64")
		}
	}

	if code, codeOK := params["code"]; codeOK {
		params["rangesubset"] = code
	}

	if rangeSubsets, rangeSubsetsOK := params["rangesubset"]; rangeSubsetsOK {
		if codeFormatOK && codeFormat == "base64" {
			for ir, s := range rangeSubsets {
				data, err := base64.StdEncoding.DecodeString(s)
				if err != nil {
					return wcsParams, err
				}
				rangeSubsets[ir] = string(data)
			}
		}

		sub := strings.Join(rangeSubsets, ";")
		parts := strings.Split(sub, ";")

		var rangeSubs []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if len(p) < 1 {
				continue
			}

			rangeSubs = append(rangeSubs, p)
		}

		bandExpr, err := ParseBandExpressions(rangeSubs)
		if err != nil {
			return wcsParams, fmt.Errorf("parsing error in band expressions: %v", err)
		}

		wcsParams.BandExpr = bandExpr
	}

	return wcsParams, err
}

func parseSubsetClause(sub string, compREMap map[string]*regexp.Regexp) (map[string]*AxisParam, error) {
	axesMap := make(map[string]*AxisParam)

	subList := strings.Split(sub, ";")
	for _, sb := range subList {
		sb = strings.TrimSpace(sb)
		if len(sb) < 1 {
			continue
		}

		var axisName string
		pos := 0
		for ; pos < len(sb); pos++ {
			if sb[pos] == '(' {
				axisName = strings.TrimSpace(sb[:pos])
				break
			}
		}

		if len(axisName) < 1 {
			return axesMap, fmt.Errorf("invalid subset syntax: %v", sb)
		}

		if !compREMap["axis"].MatchString(axisName) {
			return axesMap, fmt.Errorf("invalid axis name '%v' in subset: %v", axisName, sb)
		}

		if _, found := axesMap[axisName]; found {
			return axesMap, fmt.Errorf("Subseting axis '%v' already existed in %v", axisName, sb)
		}

		pos++

		//Order: 1 means sorting axis in ascending order
		//Aggregate: 0 means no aggregation
		axesMap[axisName] = &AxisParam{Name: axisName, Order: 1, Aggregate: 0}

		indexTupleBgn := -1
		indexTupleEnd := -1
		for ip := pos; ip < len(sb); ip++ {
			if indexTupleBgn < 0 && sb[ip] != ' ' {
				if sb[ip] != '(' {
					break
				}

				indexTupleBgn = ip + 1
				continue
			}

			if indexTupleBgn >= 0 && indexTupleEnd < 0 && sb[ip] == ')' {
				indexTupleEnd = ip
				break
			}
		}

		if indexTupleBgn >= 0 && indexTupleEnd <= indexTupleBgn {
			return axesMap, fmt.Errorf("empty index tuple in subset: %v", sb)
		}

		orderAggStart := -1

		if indexTupleBgn >= 0 {
			iClosingBracket := -1
			for ip := indexTupleEnd + 1; ip < len(sb); ip++ {
				if sb[ip] == ' ' {
					continue
				}

				if sb[ip] == ')' {
					iClosingBracket = ip
				}
				break
			}

			if iClosingBracket < 0 {
				return axesMap, fmt.Errorf("missing closing bracket: %v", sb)
			}
			orderAggStart = iClosingBracket + 1

			indexTuple := sb[indexTupleBgn:indexTupleEnd]
			selectors := strings.Split(indexTuple, ",")
			for _, sel := range selectors {
				sel = strings.TrimSpace(sel)
				if len(sel) < 1 {
					continue
				}
				selFloat, err := strconv.ParseFloat(sel, 64)
				if err != nil {
					return axesMap, fmt.Errorf("invalid element '%v', index tuple must contain float only: %v", sel, indexTuple)
				}
				axesMap[axisName].InValues = append(axesMap[axisName].InValues, selFloat)
			}

		} else {
			rangeBgn := pos
			rangeEnd := -1

			for ip := pos; ip < len(sb); ip++ {
				if sb[ip] == ')' {
					rangeEnd = ip
					break
				}
			}

			if rangeEnd < 0 {
				return axesMap, fmt.Errorf("missing close bracket: %v", sb)
			}
			orderAggStart = rangeEnd + 1

			if rangeEnd <= rangeBgn {
				return axesMap, fmt.Errorf("empty range index: %v", sb)
			}

			indexRange := sb[rangeBgn:rangeEnd]
			parts := strings.Split(indexRange, ",")
			var endpoints []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if len(p) < 1 {
					continue
				}
				endpoints = append(endpoints, p)
			}

			if len(endpoints) > 2 || len(endpoints) == 0 {
				return axesMap, fmt.Errorf("only maximum two end points are supported for range selection: %v", sb)
			}

			var lowerVal, upperVal float64
			lower := strings.TrimSpace(endpoints[0])
			if lower == "*" {
				lowerVal = math.SmallestNonzeroFloat64
			} else {
				val, err := strconv.ParseFloat(lower, 64)
				if err != nil {
					lowerTime, errTime := time.Parse(ISOFormat, lower)
					if errTime != nil {
						return axesMap, fmt.Errorf("invalid lower endpoint: %v", sb)
					}

					val = float64(lowerTime.Unix())
				}
				lowerVal = val
			}
			axesMap[axisName].Start = &lowerVal
			if len(endpoints) == 1 {
				if lower == "*" {
					upperVal = math.MaxFloat64
					axesMap[axisName].End = &upperVal
				}
				continue
			}

			upper := strings.TrimSpace(endpoints[1])
			if upper == "*" {
				upperVal = math.MaxFloat64
			} else {
				val, err := strconv.ParseFloat(upper, 64)
				if err != nil {
					upperTime, errTime := time.Parse(ISOFormat, upper)
					if errTime != nil {
						return axesMap, fmt.Errorf("invalid upper endpoint: %v", sb)
					}

					val = float64(upperTime.Unix())
				}
				upperVal = val
			}

			if upperVal <= lowerVal {
				return axesMap, fmt.Errorf("upper endpoint must be greater than lower endpoint: %v", sb)
			}

			axesMap[axisName].End = &upperVal
		}

		if orderAggStart >= len(sb) {
			continue
		}

		subClauseBgn := orderAggStart
		subClauseEnd := -1
		foundAssgn := false
		foundOrder := false
		foundAgg := false

		for ip := orderAggStart; ip < len(sb); ip++ {
			if !foundAssgn && sb[ip] == '=' {
				foundAssgn = true
				continue
			}

			if foundAssgn {
				if sb[ip] == '(' {
					iClosingBracket := -1
					for ib := ip + 1; ib < len(sb); ib++ {
						if sb[ib] == ')' {
							iClosingBracket = ib
							break
						}
					}

					if iClosingBracket < 0 {
						return axesMap, fmt.Errorf("missing closing bracket in order/agg subclause: %v", sb)
					}

					iComma := iClosingBracket + 1
					for ib := iClosingBracket + 1; ib < len(sb); ib++ {
						if sb[ib] == ',' {
							iComma = ib
							break
						} else if sb[ib] != ' ' {
							return axesMap, fmt.Errorf("invalid order/agg subclause: %v", sb)
						}
					}

					subClauseEnd = iComma
				} else if sb[ip] == ',' {
					subClauseEnd = ip
				} else if ip == len(sb)-1 {
					subClauseEnd = ip + 1
				}
			}

			if subClauseEnd > 0 {
				subClause := sb[subClauseBgn:subClauseEnd]
				parts := strings.Split(subClause, "=")
				if len(parts) != 2 {
					return axesMap, fmt.Errorf("invalid order/agg subclause: %v", sb)
				}

				op := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				if len(value) > 1 && value[0] == '(' && value[len(value)-1] == ')' {
					value = strings.TrimSpace(value[1 : len(value)-1])
				}

				if op == "order" {
					if foundOrder {
						return axesMap, fmt.Errorf("multiple order subclause: %v", sb)
					}

					if value == "asc" {
						axesMap[axisName].Order = 1
					} else if value == "desc" {
						axesMap[axisName].Order = 0
					} else {
						return axesMap, fmt.Errorf("unknown parameter '%v' for order subclause", value)
					}
					foundOrder = true
				} else if op == "agg" {
					if foundAgg {
						return axesMap, fmt.Errorf("multiple aggregate subclause: %v", sb)
					}

					if value == "union" {
						axesMap[axisName].Aggregate = 1
					} else {
						return axesMap, fmt.Errorf("unknown parameter '%v' for aggregate subclause", value)
					}
					foundAgg = true
				} else {
					return axesMap, fmt.Errorf("unknown op '%v' in order/agg subclause: %v", op, sb)
				}

				ip = subClauseEnd
				subClauseBgn = subClauseEnd + 1
				foundAssgn = false
				subClauseEnd = -1
			}
		}

		if subClauseBgn < len(sb) {
			return axesMap, fmt.Errorf("invalid order/agg subclause: %v", sb)
		}
	}

	return axesMap, nil

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
