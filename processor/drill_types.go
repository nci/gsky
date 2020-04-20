package processor

import (
	"image"
	"time"

	"github.com/nci/gsky/metrics"
	"github.com/nci/gsky/utils"
	pb "github.com/nci/gsky/worker/gdalservice"
)

type GeoDrillRequest struct {
	Geometry         string
	CRS              string
	Collection       string
	NameSpaces       []string
	BandExpr         *utils.BandExpressions
	Mask             *utils.Mask
	VRTURL           string
	StartTime        time.Time
	EndTime          time.Time
	ClipUpper        float32
	ClipLower        float32
	MetricsCollector *metrics.MetricsCollector
}

type GeoDrillGranule struct {
	Path             string
	NameSpace        string
	RasterType       string
	TimeStamps       []time.Time
	Geometry         string
	CRS              string
	VRT              string
	Means            []float64
	SampleCounts     []int
	NoData           float64
	Approx           bool
	ClipUpper        float32
	ClipLower        float32
	MetricsCollector *metrics.MetricsCollector
}

type DrillResult struct {
	NameSpace string
	Dates     []time.Time
	Data      []*pb.TimeSeries
}

type DrillFileDescriptor struct {
	OffX, OffY     int
	CountX, CountY int
	NoData         float64
	Mask           *image.Gray
}
