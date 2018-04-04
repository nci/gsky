package processor

import (
	pb "github.com/nci/gsky/grpc_server/gdalservice"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"strconv"
	"time"
)

type GeoRasterGRPC struct {
	Context context.Context
	In      chan *GeoTileGranule
	Out     chan *FlexRaster
	Error   chan error
	Client  string
}

func NewRasterGRPC(ctx context.Context, serverAddress string, errChan chan error) *GeoRasterGRPC {
	return &GeoRasterGRPC{
		Context: ctx,
		In:      make(chan *GeoTileGranule, 100),
		Out:     make(chan *FlexRaster, 100),
		Error:   errChan,
		Client:  serverAddress,
	}
}

func (gi *GeoRasterGRPC) Run() {
	defer close(gi.Out)
	//start := time.Now()
	//i := 0

	conn, err := grpc.Dial(gi.Client, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("gRPC connection problem: %v", err)
	}
	defer conn.Close()
	cLimiter := NewConcLimiter(16)
	for gran := range gi.In {
		select {
		case <-gi.Context.Done():
			gi.Error <- fmt.Errorf("Tile gRPC context has been cancel: %v", gi.Context.Err())
			return
		default:
			if gran.Path == "NULL" {
				gi.Out <- &FlexRaster{ConfigPayLoad: gran.ConfigPayLoad, Data: make([]uint8, gran.Width*gran.Height), Height: gran.Height, Width: gran.Width, OffX: gran.OffX, OffY: gran.OffY, Type: gran.RasterType, NoData: 0.0, NameSpace: gran.NameSpace, TimeStamp: gran.TimeStamp, Polygon: gran.Polygon}
				continue
			}

			cLimiter.Increase()
			//i += 1
			go func(g *GeoTileGranule, conc *ConcLimiter) {
				defer conc.Decrease()
				c := pb.NewGDALClient(conn)
				band, err := getBand(g.TimeStamps, g.TimeStamp)
				epsg, err := extractEPSGCode(g.CRS)
				geot := BBox2Geot(g.Width, g.Height, g.BBox)
				granule := &pb.GeoRPCGranule{Height: int32(g.Height), Width: int32(g.Width), Path: g.Path, EPSG: int32(epsg), Geot: geot, Bands: []int32{band}}
				r, err := c.Process(gi.Context, granule)
				if err != nil {
					gi.Error <- err
					r = &pb.Result{Raster: &pb.Raster{Data: make([]uint8, g.Width*g.Height), RasterType: "Byte", NoData: -1.}}
				}
				gi.Out <- &FlexRaster{ConfigPayLoad: g.ConfigPayLoad, Data: r.Raster.Data, Height: g.Height, Width: g.Width, OffX: g.OffX, OffY: g.OffY, Type: r.Raster.RasterType, NoData: r.Raster.NoData, NameSpace: g.NameSpace, TimeStamp: g.TimeStamp, Polygon: g.Polygon}
			}(gran, cLimiter)
		}
	}
	cLimiter.Wait()
	//log.Println("gRPC Time", time.Since(start), "Processed:", i)

}

func getBand(times []time.Time, rasterTime time.Time) (int32, error) {
	if len(times) == 1 {
		return 1, nil
	}
	for i, t := range times {
		if t.Equal(rasterTime) {
			return int32(i + 1), nil
		}
	}
	return -1, fmt.Errorf("%s dataset does not contain Unix date: %d", "Handler", rasterTime)
}

// ExtractEPSGCode parses an SRS string and gets
// the EPSG code
func extractEPSGCode(srs string) (int, error) {
	return strconv.Atoi(srs[5:])
}

// BBox2Geot return the geotransform from the
// parameters received in a WMS GetMap request
func BBox2Geot(width, height int, bbox []float64) []float64 {
	return []float64{bbox[0], (bbox[2] - bbox[0]) / float64(width), 0, bbox[3], 0, (bbox[1] - bbox[3]) / float64(height)}
}
