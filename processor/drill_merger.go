package processor

import (
	"bytes"
	"fmt"
	"math"
	"sort"

	goeval "github.com/edisonguo/govaluate"
	"github.com/nci/gsky/utils"
	pb "github.com/nci/gsky/worker/gdalservice"
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

func (dm *DrillMerger) Run(suffix string, namespaces []string, templateFileName string, bandEval []string) {
	defer close(dm.Out)
	results := make(map[string]map[string][]*pb.TimeSeries)

	for drillRes := range dm.In {
		if _, ok := results[drillRes.NameSpace]; !ok {
			results[drillRes.NameSpace] = make(map[string][]*pb.TimeSeries)
			nsFound := false
			for _, ns := range namespaces {
				if ns == drillRes.NameSpace {
					nsFound = true
					break
				}
			}
			if !nsFound {
				namespaces = append(namespaces, drillRes.NameSpace)
			}
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
		dm.Error <- fmt.Errorf("WPS: Merger hasn't received any result")
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
			dm.Error <- fmt.Errorf("WPS: Experession parsing error: %v", expErr)
			return
		}

		expressions[iExpr] = expr

		variables := []string{}
		for iToken, token := range expr.Tokens() {
			if token.Kind == goeval.VARIABLE {
				varName, ok := token.Value.(string)
				if !ok {
					dm.Error <- fmt.Errorf("WPS: Expression token name failed to cast to string %d", iToken)
					return
				}

				_, varFound := results[varName]
				if !varFound {
					dm.Error <- fmt.Errorf("WPS: undefined variable '%s' in '%s'. All variables: %v", varName, bandEval[iExpr], namespaces)
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
				dm.Error <- fmt.Errorf("WPS: Eval '%v' error: %v", bandEval[iv], err)
				return
			}

			val, ok := result.(float32)
			if !ok {
				dm.Error <- fmt.Errorf("WPS: Failed to cast eval results '%v' to float323232, %v", val, bandEval[iv])
				return
			}

			fmt.Fprintf(csv, "%f", float64(val))
		}

		fmt.Fprint(csv, "\\n")

	}

	out := bytes.NewBufferString("")
	err := utils.ExecuteWriteTemplateFile(out, csv, templateFileName)
	if err != nil {
		dm.Error <- fmt.Errorf("WPS: output template error: %v", err)
		return
	}
	dm.Out <- fmt.Sprintf(out.String(), suffix)
}
