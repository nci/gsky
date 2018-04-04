package processor

import (
	pb "github.com/nci/gsky/grpc_server/gdalservice"
	"fmt"
	"math"
	"sort"
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

func (dm *DrillMerger) Run() {
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
	/*nameMap := map[string]string{
	  "": "Prec",
	  "phot_veg": "PV",
	  "nphot_veg": "NPV",
	  "bare_soil": "BS",}*/
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
		out += `", "isEnabled": true, "type": "csv", "name": "Precipitation", "tableStyle": { "columns": { "Prec": { "units": "mm", "chartLineColor": "#72ecfa", "yAxisMin": 0, "active": true } } } }]]>`
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
		out += `", "isEnabled": true, "type": "csv", "name": "Veg. Frac.", "tableStyle": { "columns": { "NPV": { "units": "%", "chartLineColor": "#0070c0", "yAxisMin": 0, "yAxisMax": 100, "active": true }, "PV": { "units": "%", "chartLineColor": "#00b050", "yAxisMin": 0, "yAxisMax": 100, "active": true }, "BS": { "units": "%", "chartLineColor": "#FF0000", "yAxisMin": 0, "yAxisMax": 100,  "active": true }, "Total": { "units": "%", "chartLineColor": "#FFFFFF", "yAxisMin": 0, "yAxisMax": 100,  "active": true } } } }]]>`
	}
	out += `</wps:ComplexData>
</wps:Data>
</wps:Output>`

	dm.Out <- out
}
