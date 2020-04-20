package processor

type TimeSplitter struct {
	In       chan *GeoDrillRequest
	Out      chan *GeoDrillRequest
	Error    chan error
	YearStep int
}

func NewTimeSplitter(yearStep int, errChan chan error) *TimeSplitter {
	return &TimeSplitter{
		In:       make(chan *GeoDrillRequest, 100),
		Out:      make(chan *GeoDrillRequest, 100),
		Error:    errChan,
		YearStep: yearStep,
	}
}

func (ts *TimeSplitter) Run() {
	defer close(ts.Out)
	for geoReq := range ts.In {
		if ts.YearStep > 0 {
			for t := geoReq.StartTime; t.Before(geoReq.EndTime); t = t.AddDate(ts.YearStep, 0, 0) {
				ts.Out <- &GeoDrillRequest{geoReq.Geometry, geoReq.CRS, geoReq.Collection, geoReq.NameSpaces, geoReq.BandExpr, geoReq.Mask, "", t, t.AddDate(ts.YearStep, 0, 0), geoReq.ClipUpper, geoReq.ClipLower, geoReq.MetricsCollector}
			}
		} else {
			ts.Out <- geoReq
		}
	}
}
