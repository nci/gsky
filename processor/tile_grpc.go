package processor

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/nci/go.procmeminfo"
	pb "github.com/nci/gsky/worker/gdalservice"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const ReservedMemorySize = 1.5 * 1024 * 1024 * 1024

type GeoRasterGRPC struct {
	Context               context.Context
	In                    chan *GeoTileGranule
	Out                   chan []*FlexRaster
	Error                 chan error
	Clients               []string
	MaxGrpcRecvMsgSize    int
	PolygonShardConcLimit int
}

func NewRasterGRPC(ctx context.Context, serverAddress []string, maxGrpcRecvMsgSize int, polygonShardConcLimit int, errChan chan error) *GeoRasterGRPC {
	return &GeoRasterGRPC{
		Context:               ctx,
		In:                    make(chan *GeoTileGranule, 100),
		Out:                   make(chan []*FlexRaster, 100),
		Error:                 errChan,
		Clients:               serverAddress,
		MaxGrpcRecvMsgSize:    maxGrpcRecvMsgSize,
		PolygonShardConcLimit: polygonShardConcLimit,
	}
}

func (gi *GeoRasterGRPC) Run(polyLimiter *ConcLimiter) {
	defer close(gi.Out)

	var grans []*GeoTileGranule
	i := 0
	imageSize := 0
	for gran := range gi.In {
		if gran.Path == "NULL" {
			polyLimiter.Increase()
			gi.Out <- []*FlexRaster{&FlexRaster{ConfigPayLoad: gran.ConfigPayLoad, Data: make([]uint8, gran.Width*gran.Height), Height: gran.Height, Width: gran.Width, OffX: gran.OffX, OffY: gran.OffY, Type: gran.RasterType, NoData: 0.0, NameSpace: gran.NameSpace, TimeStamp: gran.TimeStamp, Polygon: gran.Polygon}}
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

	g0 := grans[0]
	effectivePoolSize := int(math.Ceil(float64(len(grans)) / float64(g0.GrpcConcLimit)))
	if effectivePoolSize < 1 {
		effectivePoolSize = 1
	} else if effectivePoolSize > len(gi.Clients) {
		effectivePoolSize = len(gi.Clients)
	}

	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(gi.MaxGrpcRecvMsgSize)),
	}

	var connPool []*grpc.ClientConn
	for i := 0; i < effectivePoolSize; i++ {
		conn, err := grpc.Dial(gi.Clients[i], opts...)
		if err != nil {
			log.Printf("gRPC connection problem: %v", err)
			continue
		}
		defer conn.Close()

		connPool = append(connPool, conn)
	}

	if len(connPool) == 0 {
		gi.Error <- fmt.Errorf("All gRPC servers offline")
		return
	}

	// We have to do this because there is no way to figure out
	// the data type of the raster but returned from the grpc worker
	// We need the data type to dertermine the byte size for memory
	// bound calcuations
	r0, err := getRpcRaster(gi.Context, g0, connPool[0])
	if err != nil {
		polyLimiter.Increase()
		gi.Error <- err
		r0 = &pb.Result{Raster: &pb.Raster{Data: make([]uint8, g0.Width*g0.Height), RasterType: "Byte", NoData: -1.}}
		gi.Out <- []*FlexRaster{&FlexRaster{ConfigPayLoad: g0.ConfigPayLoad, Data: r0.Raster.Data, Height: g0.Height, Width: g0.Width, OffX: g0.OffX, OffY: g0.OffY, Type: r0.Raster.RasterType, NoData: r0.Raster.NoData, NameSpace: g0.NameSpace, TimeStamp: g0.TimeStamp, Polygon: g0.Polygon}}
		return
	}

	dataSize, err := getDataSize(r0.Raster.RasterType)
	if err != nil {
		gi.Error <- err
		return
	}

	// We divide the GeoTileGranules (i.e. inputs) into shards whose key is the
	// corresponding polygon string.
	// Once we obtain all the gPRC output results for a particular shard, this shard
	// will be sent asynchronously to the merger algorithm and then become ready for GC.
	// Comments:
	// 1) This algorithm is a streaming processing model that allows us to process the
	// volume of data beyond the size of physical server memory.
	// We also allow processing shards concurrently so that the theoretical performance
	// of our streaming processing model is at least no worse batch processing model.
	// In practice, we often observe better performance with streaming processing model
	// for two reasons: a) the concurrency among polygon shards b) interleave merger
	// computation with gRPC IO.
	// 3) The concurrency of shards is controled by PolygonShardConcLimit
	// A typical range of value between 5 to 10 scales well for
	// both small and large requests.
	// By varing this shard concurrency value, we can trade off space and time.
	gransByPolygon := make(map[string][]*GeoTileGranule)
	for i := 1; i < len(grans); i++ {
		gran := grans[i]
		gransByPolygon[gran.Polygon] = append(gransByPolygon[gran.Polygon], gran)
	}

	meminfo := procmeminfo.MemInfo{}
	err = meminfo.Update()

	if err == nil {
		// We figure the sizes of each shard and then sort them in descending order.
		// We then compute the total size of the top DefaultPolyShardConcLimit shards.
		// If the total size of the top shards is below memory threshold, we are good to go.
		shardSizes := make([]int, len(gransByPolygon))
		iShard := 0
		for _, polyGran := range gransByPolygon {
			shardSizes[iShard] = imageSize * dataSize * len(polyGran)
			iShard += 1
		}

		sort.Slice(shardSizes, func(i, j int) bool { return shardSizes[i] > shardSizes[j] })

		effectiveLen := gi.PolygonShardConcLimit
		if len(shardSizes) < effectiveLen {
			effectiveLen = len(shardSizes)
		}

		requestedSize := 0
		for i := 0; i < effectiveLen; i++ {
			requestedSize += shardSizes[i]
		}

		freeMem := int(meminfo.Available())
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		availableMem := freeMem + int(mem.HeapIdle)

		if availableMem-requestedSize <= ReservedMemorySize {
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

	polyLimiter.Increase()
	// We first send out the raster we used to figure out data type
	gi.Out <- []*FlexRaster{&FlexRaster{ConfigPayLoad: g0.ConfigPayLoad, Data: r0.Raster.Data, Height: g0.Height, Width: g0.Width, OffX: g0.OffX, OffY: g0.OffY, Type: r0.Raster.RasterType, NoData: r0.Raster.NoData, NameSpace: g0.NameSpace, TimeStamp: g0.TimeStamp, Polygon: g0.Polygon}}

	timeoutCtx, cancel := context.WithTimeout(gi.Context, time.Duration(g0.Timeout)*time.Second)
	defer cancel()
	cLimiter := NewConcLimiter(g0.GrpcConcLimit * len(connPool))

	var wg sync.WaitGroup
	wg.Add(len(gransByPolygon))

	granCounter := 0
	for _, polyGrans := range gransByPolygon {
		polyLimiter.Increase()
		go func(polyGrans []*GeoTileGranule, granCounter int) {
			defer wg.Done()
			outChan := make(chan *FlexRaster, len(polyGrans))
			defer close(outChan)

			for iGran := range polyGrans {
				gran := polyGrans[iGran]
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
						outChan <- &FlexRaster{ConfigPayLoad: gran.ConfigPayLoad, Data: make([]uint8, gran.Width*gran.Height), Height: gran.Height, Width: gran.Width, OffX: gran.OffX, OffY: gran.OffY, Type: gran.RasterType, NoData: 0.0, NameSpace: gran.NameSpace, TimeStamp: gran.TimeStamp, Polygon: gran.Polygon}
						continue
					}

					cLimiter.Increase()
					go func(g *GeoTileGranule, iTile int, gCnt int) {
						defer cLimiter.Decrease()
						r, err := getRpcRaster(gi.Context, g, connPool[gCnt%len(connPool)])
						if err != nil {
							gi.Error <- err
							r = &pb.Result{Raster: &pb.Raster{Data: make([]uint8, g.Width*g.Height), RasterType: "Byte", NoData: -1.}}
						}
						outChan <- &FlexRaster{ConfigPayLoad: g.ConfigPayLoad, Data: r.Raster.Data, Height: g.Height, Width: g.Width, OffX: g.OffX, OffY: g.OffY, Type: r.Raster.RasterType, NoData: r.Raster.NoData, NameSpace: g.NameSpace, TimeStamp: g.TimeStamp, Polygon: g.Polygon}
					}(gran, iGran, granCounter+iGran)
				}

			}

			outRasters := make([]*FlexRaster, len(polyGrans))
			iOut := 0
			for o := range outChan {
				outRasters[iOut] = o
				iOut += 1

				if iOut == len(polyGrans) {
					gi.Out <- outRasters
					break
				}
			}

		}(polyGrans, granCounter)

		granCounter += len(polyGrans)

	}

	wg.Wait()
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
