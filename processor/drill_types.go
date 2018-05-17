package processor

import (
	pb "github.com/nci/gsky/grpc_server/gdalservice"
	"image"
	"time"
)

type GeoDrillRequest struct {
	Geometry   string
	CRS        string
	Collection string
	NameSpaces []string
	StartTime  time.Time
	EndTime    time.Time
}

type GeoDrillGranule struct {
	Path       string
	NameSpace  string
	RasterType string
	TimeStamps []time.Time
	Geometry   string
	CRS        string
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
