package processor

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"

	"github.com/nci/gsky/utils"
	pb "github.com/nci/gsky/worker/gdalservice"
)

type DrillMerger struct {
	Context context.Context
	In      chan *DrillResult
	Out     chan string
	Error   chan error
}

func NewDrillMerger(ctx context.Context, errChan chan error) *DrillMerger {
	return &DrillMerger{
		Context: ctx,
		In:      make(chan *DrillResult, 100),
		Out:     make(chan string),
		Error:   errChan,
	}
}

func (dm *DrillMerger) Run(suffix string, namespaces []string, templateFileName string, bandExpr *utils.BandExpressions, decileCount int, verbose bool) {
	if verbose {
		defer log.Printf("Drill Merger done")
	}
	defer close(dm.Out)
	results := make(map[string]map[string][]*pb.TimeSeries)

	var drillResult *DrillResult
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

		if drillResult == nil {
			drillResult = drillRes
		}
	}
	if len(results) == 0 {
		return
	}

	var dates []string
	for ts := range results[namespaces[0]] {
		dates = append(dates, ts)
	}
	sort.Strings(dates)

	var csv strings.Builder
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

		fmt.Fprintf(&csv, "%s", key)

		if len(bandExpr.Expressions) == 0 {
			for _, ns := range namespaces {
				fmt.Fprint(&csv, ",")
				if val, ok := values[ns]; ok {
					fmt.Fprintf(&csv, "%f", val)
				}
			}

			fmt.Fprint(&csv, "\\n")
			continue
		}

		nCols := 1 + decileCount
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

				fmt.Fprint(&csv, ",")

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
					dm.sendError(fmt.Errorf("WPS: Eval '%v' error: %v", bandExpr.ExprText[ix], err))
					return
				}

				val, ok := result.(float32)
				if !ok {
					dm.sendError(fmt.Errorf("WPS: Failed to cast eval results '%v' to float32, %v", val, bandExpr.ExprText[ix]))
					return
				}

				if drillResult != nil && float32(drillResult.NoData) != val {
					fmt.Fprintf(&csv, "%f", float64(val))
				}
			}
		}

		fmt.Fprint(&csv, "\\n")

	}

	var out strings.Builder
	err := utils.ExecuteWriteTemplateFile(&out, csv.String(), templateFileName)
	if err != nil {
		dm.sendError(fmt.Errorf("WPS: output template error: %v", err))
		return
	}

	if dm.checkCancellation() {
		return
	}
	dm.Out <- fmt.Sprintf(out.String(), suffix)
}

func (dm *DrillMerger) sendError(err error) {
	select {
	case dm.Error <- err:
	default:
	}
}

func (dm *DrillMerger) checkCancellation() bool {
	select {
	case <-dm.Context.Done():
		dm.sendError(fmt.Errorf("Drill Merger: context has been cancel: %v", dm.Context.Err()))
		return true
	case err := <-dm.Error:
		dm.sendError(err)
		return true
	default:
		return false
	}
}
