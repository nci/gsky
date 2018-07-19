package processor

import (
	"fmt"
	"math"
	"sort"
	"bytes"

	"github.com/nci/gsky/utils"
	pb "github.com/nci/gsky/worker/gdalservice"
	goeval "github.com/Knetic/govaluate"
)

type DrillMerger struct {
	In    chan *DrillResult
	Out   chan string
	Error chan error
}

func NewDrillMerger(errChan chan error) *DrillMerger {
	return &DrillMerger{
		In:    make(chan *DrillResult, 100),
		Out:   make(chan string),
		Error: errChan,
	}
}

func (dm *DrillMerger) Run(suffix string, templateFileName string, bandEval []string) {
	defer close(dm.Out)
	results := make(map[string]map[string][]*pb.TimeSeries)
	namespaces := []string{}

	for drillRes := range dm.In {
		if _, ok := results[drillRes.NameSpace]; !ok {
			results[drillRes.NameSpace] = make(map[string][]*pb.TimeSeries)
			namespaces = append(namespaces, drillRes.NameSpace)
		}

		for i, date := range drillRes.Dates {
			isoDate := date.Format(ISOFormat)
			if _, ok := results[drillRes.NameSpace][isoDate]; !ok {
				results[drillRes.NameSpace][isoDate] = []*pb.TimeSeries{drillRes.Data[i]}
			} else {
				results[drillRes.NameSpace][isoDate] = append(results[drillRes.NameSpace][isoDate], drillRes.Data[i])
			}
		}
	}
	if len(results) == 0 {
		dm.Error <- fmt.Errorf("Merger hasn't received any result")
		return
	}

	var dates []string
	for ts := range results[namespaces[0]] {
		dates = append(dates, ts)
	}
	sort.Strings(dates)

	varRefs := [][]string{}
	expressions := make([]*goeval.EvaluableExpression, len(bandEval))

	for iExpr := range bandEval {
		expr, expErr := goeval.NewEvaluableExpression(bandEval[iExpr])
		if expErr != nil {
			dm.Error <- fmt.Errorf("eval experession error: %v", expErr)
			return
		}

		expressions[iExpr] = expr

		variables := []string{}
		for iToken, token := range expr.Tokens() {
			if token.Kind == goeval.VARIABLE {
				varName, ok := token.Value.(string)
				if !ok {
					dm.Error <- fmt.Errorf("eval parsing error at expression: %d", iToken)
					return
				}	
				variables = append(variables, varName)
			}
		}

		varRefs = append(varRefs, variables)
	}

	csv := bytes.NewBufferString("") 	
	for _, key := range dates {
		values := map[string]float64{}
		for _, ns := range namespaces {
			total := 0.0
			count := 0
			for _, data := range results[ns][key] {
				if !math.IsNaN(data.Value) {
					total += data.Value * float64(data.Count)
					count += int(data.Count)
				}
			}
			if !math.IsNaN(total) && count > 0 {
				values[ns] = total / float64(count)
			}
		}

		fmt.Fprintf(csv, "%s", key)

		for _, ns := range namespaces {
			fmt.Fprint(csv, ",")
			if val, ok := values[ns]; ok {
				fmt.Fprintf(csv, "%f", val)
			}
		}

		for iv, variables := range varRefs {
			noData := false
			for _, variable := range variables {
				if _, ok := values[variable]; !ok {
					noData = true
					break
				}
			}

			fmt.Fprint(csv, ",")

			if noData {
				continue
			}

			parameters := make(map[string]interface{}, len(variables))	
			for _, variable := range variables {
				parameters[variable] = values[variable]
			}

			result, err := expressions[iv].Evaluate(parameters)
			if err != nil {
				dm.Error <- fmt.Errorf("eval '%v' error: %v", bandEval[iv], err)
				return
			}

			val, ok := result.(float64)
			if !ok {
				dm.Error <- fmt.Errorf("Failed to cast eval results '%v' to float64, %v", val, bandEval[iv])
				return
			}

			fmt.Fprintf(csv, "%f", val)
		}

		fmt.Fprint(csv, "\\n")

	}

	out := bytes.NewBufferString("") 
	err := utils.ExecuteWriteTemplateFile(out, csv.String(), templateFileName)
	if err != nil {
		dm.Error <- fmt.Errorf("drill merger error: %v", err)
		return
	}
	dm.Out <- fmt.Sprintf(out.String(), suffix)
	/*
	out := `<wps:Output>`

	switch len(namespaces) {
	case 1:
		out += `<ows:Identifier>precipitation</ows:Identifier>
<ows:Title>Accumulated Precipitation</ows:Title>
<ows:Abstract>Time series data for location.</ows:Abstract>
<wps:Data>
<wps:ComplexData mimeType="application/vnd.terriajs.catalog-member+json" schema="https://tools.ietf.org/html/rfc7159">
<![CDATA[{ "data": "date,Prec\n`

		for _, key := range dates {
			values := map[string]float64{}
			for _, ns := range namespaces {
				total := 0.0
				count := 0
				for _, data := range results[ns][key] {
					if !math.IsNaN(data.Value) {
						total += data.Value * float64(data.Count)
						count += int(data.Count)
					}
				}
				if !math.IsNaN(total) && count > 0 {
					values[ns] = total / float64(count)
				}
			}
			out += fmt.Sprintf("%s,", key)
			if val, ok := values[""]; ok {
				out += fmt.Sprintf("%f", val)
			}
			out += ",\\n"
		}
		out += fmt.Sprintf(`", "isEnabled": true, "type": "csv", "name": "Precipitation%s", "tableStyle": { "columns": { "Prec": { "units": "mm", "chartLineColor": "#72ecfa", "yAxisMin": 0, "active": true } } } }]]>`, suffix)
	case 3:
		out += `<ows:Identifier>veg_cover</ows:Identifier>
<ows:Title>Vegetation Cover</ows:Title>
<ows:Abstract>Time series data for location.</ows:Abstract>
<wps:Data>
<wps:ComplexData mimeType="application/vnd.terriajs.catalog-member+json" schema="https://tools.ietf.org/html/rfc7159">
<![CDATA[{ "data": "date,PV,NPV,BS,Total\n`
		for _, key := range dates {
			values := map[string]float64{}
			for _, ns := range namespaces {
				total := 0.0
				count := 0
				for _, data := range results[ns][key] {
					if !math.IsNaN(data.Value) {
						total += data.Value * float64(data.Count)
						count += int(data.Count)
					}
				}
				if !math.IsNaN(total) && count > 0 {
					values[ns] = total / float64(count)
				}
			}
			out += fmt.Sprintf("%s,", key)
			tc := 0.0
			if val, ok := values["phot_veg"]; ok {
				out += fmt.Sprintf("%f", val)
				tc += val
			}
			out += ","
			if val, ok := values["nphot_veg"]; ok {
				out += fmt.Sprintf("%f", val)
				tc += val
			}
			out += ","
			if val, ok := values["bare_soil"]; ok {
				out += fmt.Sprintf("%f", val)
			}
			out += ","
			if tc != 0.0 {
				out += fmt.Sprintf("%f", tc)
			}
			out += "\\n"
		}
		out += fmt.Sprintf(`", "isEnabled": true, "type": "csv", "name": "Veg. Frac.%s", "tableStyle": { "columns": { "NPV": { "units": "%%", "chartLineColor": "#0070c0", "yAxisMin": 0, "yAxisMax": 100, "active": true }, "PV": { "units": "%%", "chartLineColor": "#00b050", "yAxisMin": 0, "yAxisMax": 100, "active": true }, "BS": { "units": "%%", "chartLineColor": "#FF0000", "yAxisMin": 0, "yAxisMax": 100,  "active": true }, "Total": { "units": "%%", "chartLineColor": "#FFFFFF", "yAxisMin": 0, "yAxisMax": 100,  "active": true } } } }]]>`, suffix)
	}
	out += `</wps:ComplexData>
</wps:Data>
</wps:Output>`

	dm.Out <- out
	*/
}
