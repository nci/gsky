package processor

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"
)

const (
	maxXTileSize = 256
	maxYTileSize = 256
)

type TileSplitter struct {
	Context context.Context
	In      chan *GeoTileRequest
	Out     chan *GeoTileRequest
	Error   chan error
}

func NewTileSplitter(ctx context.Context, errChan chan error) *TileSplitter {
	return &TileSplitter{
		Context: ctx,
		In:      make(chan *GeoTileRequest, 100),
		Out:     make(chan *GeoTileRequest, 100),
		Error:   errChan,
	}
}

func (s *TileSplitter) Run() {
	defer close(s.Out)
	start := time.Now()
	for geoReq := range s.In {
		select {
		case <-s.Context.Done():
			s.Error <- fmt.Errorf("Tile splitter context has been cancel: %v", s.Context.Err())
			return
		default:
			if geoReq.Width <= maxXTileSize || geoReq.Height <= maxYTileSize {
				s.Out <- geoReq
				return
			}
			geoReqs := splitGeoTileRequest(geoReq)
			for _, gReq := range geoReqs {
				s.Out <- gReq
			}
		}
	}
	log.Println("Splitter Time", time.Since(start))
}

func splitGeoTileRequest(request *GeoTileRequest) []*GeoTileRequest {
	yRes := (request.BBox[3] - request.BBox[1]) / float64(request.Height)
	xRes := (request.BBox[2] - request.BBox[0]) / float64(request.Width)

	out := []*GeoTileRequest{}

	for x := 0; x < request.Width; x += maxXTileSize {
		for y := 0; y < request.Height; y += maxYTileSize {
			yMin := request.BBox[1] + float64(y)*yRes
			yMax := math.Min(request.BBox[1]+float64(y+maxYTileSize)*yRes, request.BBox[3])
			xMin := request.BBox[0] + float64(x)*xRes
			xMax := math.Min(request.BBox[0]+float64(x+maxXTileSize)*xRes, request.BBox[2])

			tileXSize := int(.5 + (xMax-xMin)/xRes)
			tileYSize := int(.5 + (yMax-yMin)/yRes)

			out = append(out, &GeoTileRequest{ConfigPayLoad: request.ConfigPayLoad, Collection: request.Collection,
				CRS: request.CRS, BBox: []float64{xMin, yMin, xMax, yMax},
				Width: tileXSize, Height: tileYSize, OffX: x, OffY: y,
				StartTime: request.StartTime, EndTime: request.EndTime})
		}
	}
	return out
}
