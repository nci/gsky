package processor

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/nci/gsky/utils"
	pb "github.com/nci/gsky/worker/gdalservice"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const ReservedMemorySize = 1.5 * 1024 * 1024 * 1024

type GeoRasterGRPC struct {
	Context            context.Context
	In                 chan *GeoTileGranule
	Out                chan *FlexRaster
	Error              chan error
	Client             string
	MaxGrpcRecvMsgSize int
}

func NewRasterGRPC(ctx context.Context, serverAddress string, maxGrpcRecvMsgSize int, errChan chan error) *GeoRasterGRPC {
	return &GeoRasterGRPC{
		Context:            ctx,
		In:                 make(chan *GeoTileGranule, 100),
		Out:                make(chan *FlexRaster, 100),
		Error:              errChan,
		Client:             serverAddress,
		MaxGrpcRecvMsgSize: maxGrpcRecvMsgSize,
	}
}

func (gi *GeoRasterGRPC) Run() {
	defer close(gi.Out)

	var grans []*GeoTileGranule
	i := 0
	imageSize := 0
	for gran := range gi.In {
		if gran.Path == "NULL" {
			gi.Out <- &FlexRaster{ConfigPayLoad: gran.ConfigPayLoad, Data: make([]uint8, gran.Width*gran.Height), Height: gran.Height, Width: gran.Width, OffX: gran.OffX, OffY: gran.OffY, Type: gran.RasterType, NoData: 0.0, NameSpace: gran.NameSpace, TimeStamp: gran.TimeStamp, Polygon: gran.Polygon}
			continue
		} else {
			grans = append(grans, gran)
			if i == 0 {
				imageSize = gran.Height * gran.Width
			}

			i += 1
		}
	}

	if len(grans) == 0 {
		return
	}

	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(gi.MaxGrpcRecvMsgSize)),
	}
	conn, err := grpc.Dial(gi.Client, opts...)
	if err != nil {
		log.Fatalf("gRPC connection problem: %v", err)
	}
	defer conn.Close()

	// We have to do this because there is no way to figure out
	// the data type of the raster but returned from the grpc worker
	// We need the data type to dertermine the byte size for memory
	// bound calcuations
	g0 := grans[0]
	r0, err := getRpcRaster(gi.Context, g0, conn)
	if err != nil {
		gi.Error <- err
		return
	}

	dataSize, err := getDataSize(r0.Raster.RasterType)
	if err != nil {
		gi.Error <- err
		return
	}
	requestedSize := imageSize * dataSize * len(grans)

	meminfo := utils.MemInfo{}
	err = meminfo.Update()

	if err == nil {
		freeMem := int(meminfo.Available())
		log.Printf("freeMem:%v, requested:%v, diff:%v", freeMem, requestedSize, freeMem-requestedSize)
		if freeMem-requestedSize <= ReservedMemorySize {
			log.Printf("Out of memory, freeMem:%v, requested:%v", freeMem, requestedSize)
			gi.Error <- fmt.Errorf("Server resources exhausted")
			return
		}

	} else {
		// If we have error obtaining meminfo,
		// we assume that we have enough memory.
		// This can happen if the OS doesn't support
		// /proc/meminfo
		log.Printf("meminfo error: %v", err)
	}

	// We first send out the raster we used to figure out data type
	gi.Out <- &FlexRaster{ConfigPayLoad: g0.ConfigPayLoad, Data: r0.Raster.Data, Height: g0.Height, Width: g0.Width, OffX: g0.OffX, OffY: g0.OffY, Type: r0.Raster.RasterType, NoData: r0.Raster.NoData, NameSpace: g0.NameSpace, TimeStamp: g0.TimeStamp, Polygon: g0.Polygon}

	timeoutCtx, cancel := context.WithTimeout(gi.Context, time.Duration(g0.Timeout)*time.Second)
	defer cancel()
	cLimiter := NewConcLimiter(16)
	for i := 1; i < len(grans); i++ {
		gran := grans[i]
		select {
		case <-gi.Context.Done():
			gi.Error <- fmt.Errorf("Tile gRPC context has been cancel: %v", gi.Context.Err())
			return
		case <-timeoutCtx.Done():
			log.Printf("tile grpc timed out, threshold:%v seconds", g0.Timeout)
			gi.Error <- fmt.Errorf("Processing timed out")
			return
		default:
			if gran.Path == "NULL" {
				gi.Out <- &FlexRaster{ConfigPayLoad: gran.ConfigPayLoad, Data: make([]uint8, gran.Width*gran.Height), Height: gran.Height, Width: gran.Width, OffX: gran.OffX, OffY: gran.OffY, Type: gran.RasterType, NoData: 0.0, NameSpace: gran.NameSpace, TimeStamp: gran.TimeStamp, Polygon: gran.Polygon}
				continue
			}

			cLimiter.Increase()
			go func(g *GeoTileGranule, conc *ConcLimiter) {
				defer conc.Decrease()
				r, err := getRpcRaster(gi.Context, g, conn)
				if err != nil {
					gi.Error <- err
					r = &pb.Result{Raster: &pb.Raster{Data: make([]uint8, g.Width*g.Height), RasterType: "Byte", NoData: -1.}}
				}
				gi.Out <- &FlexRaster{ConfigPayLoad: g.ConfigPayLoad, Data: r.Raster.Data, Height: g.Height, Width: g.Width, OffX: g.OffX, OffY: g.OffY, Type: r.Raster.RasterType, NoData: r.Raster.NoData, NameSpace: g.NameSpace, TimeStamp: g.TimeStamp, Polygon: g.Polygon}
			}(gran, cLimiter)
		}
	}
	cLimiter.Wait()
}

func getDataSize(dataType string) (int, error) {
	switch dataType {
	case "Byte":
		return 1, nil
	case "Int16":
		return 2, nil
	case "UInt16":
		return 2, nil
	case "Float32":
		return 4, nil
	default:
		return -1, fmt.Errorf("Unsupported raster type %s", dataType)

	}
}

func getRpcRaster(ctx context.Context, g *GeoTileGranule, conn *grpc.ClientConn) (*pb.Result, error) {
	c := pb.NewGDALClient(conn)
	band, err := getBand(g.TimeStamps, g.TimeStamp)
	epsg, err := extractEPSGCode(g.CRS)
	geot := BBox2Geot(g.Width, g.Height, g.BBox)
	granule := &pb.GeoRPCGranule{Height: int32(g.Height), Width: int32(g.Width), Path: g.Path, EPSG: int32(epsg), Geot: geot, Bands: []int32{band}}
	r, err := c.Process(ctx, granule)
	if err != nil {
		return nil, err
	}

	return r, nil
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
	return -1, fmt.Errorf("%s dataset does not contain Unix date: %d", "Handler", rasterTime.Unix())
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
