package processor

import (
	"bytes"
	"fmt"
	"math"
	"sort"

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

func (dm *DrillMerger) Run(suffix string, namespaces []string, templateFileName string, bandExpr *utils.BandExpressions) {
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

		if len(bandExpr.Expressions) == 0 {
			for _, ns := range namespaces {
				fmt.Fprint(csv, ",")
				if val, ok := values[ns]; ok {
					fmt.Fprintf(csv, "%f", val)
				}
			}

			fmt.Fprint(csv, "\\n")
			continue
		}

		nCols := 1 + DecileCount
		for ix, expr := range bandExpr.Expressions {
			for ic := 0; ic < nCols; ic++ {
				noData := false
				for _, variable := range bandExpr.ExprVarRef[ix] {
					varCol := variable
					if ic > 0 {
						varCol = variable + fmt.Sprintf(DecileNamespace, ic)
					}
					if _, ok := values[varCol]; !ok {
						noData = true
						break
					}
				}

				fmt.Fprint(csv, ",")

				if noData {
					continue
				}

				parameters := make(map[string]interface{}, len(bandExpr.ExprVarRef[ix]))
				for _, variable := range bandExpr.ExprVarRef[ix] {
					varCol := variable
					if ic > 0 {
						varCol = variable + fmt.Sprintf(DecileNamespace, ic)
					}
					parameters[variable] = values[varCol]
				}

				result, err := expr.Evaluate(parameters)
				if err != nil {
					dm.Error <- fmt.Errorf("WPS: Eval '%v' error: %v", bandExpr.ExprText[ix], err)
					return
				}

				val, ok := result.(float32)
				if !ok {
					dm.Error <- fmt.Errorf("WPS: Failed to cast eval results '%v' to float32, %v", val, bandExpr.ExprText[ix])
					return
				}

				fmt.Fprintf(csv, "%f", float64(val))
			}
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
