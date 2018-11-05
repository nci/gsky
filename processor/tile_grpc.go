package processor

import (
	"fmt"
	"log"
	"math"
	"math/rand"
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

func (gi *GeoRasterGRPC) Run(polyLimiter *ConcLimiter, varList []string, verbose bool) {
	if verbose {
		defer log.Printf("tile grpc done")
	}
	defer close(gi.Out)

	var grans []*GeoTileGranule
	availNamespaces := make(map[string]bool)
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

			if _, found := availNamespaces[gran.NameSpace]; !found {
				availNamespaces[gran.NameSpace] = true
			}

			i++
		}
	}

	if len(grans) == 0 {
		return
	}

	for _, v := range varList {
		if _, found := availNamespaces[v]; !found {
			gi.sendError(fmt.Errorf("band '%v' not found", v))
			return
		}
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

	clientIdx := make([]int, len(gi.Clients))
	for ic := range clientIdx {
		clientIdx[ic] = ic
	}
	rand.Shuffle(len(clientIdx), func(i, j int) { clientIdx[i], clientIdx[j] = clientIdx[j], clientIdx[i] })

	var connPool []*grpc.ClientConn
	for i := 0; i < effectivePoolSize; i++ {
		conn, err := grpc.Dial(gi.Clients[clientIdx[i]], opts...)
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

	dataSize, err := getDataSize(g0.RasterType)
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
	// of our streaming processing model is at least no worse than batch processing model.
	// In practice, we often observe better performance with the streaming processing
	// model for two reasons: a) the concurrency among polygon shards b) interleave merger
	// computation with gRPC IO.
	// 2) The concurrency of shards is controled by PolygonShardConcLimit
	// A typical value of 2 scales well for both small and large requests.
	// By varying this shard concurrency value, we can trade off space and time.
	gransByPolygon := make(map[string][]*GeoTileGranule)
	for i := 0; i < len(grans); i++ {
		gran := grans[i]
		gransByPolygon[gran.Polygon] = append(gransByPolygon[gran.Polygon], gran)
	}

	// We consolidate the shards such that each shard is not too small to spread across
	// the number of gRPC workers
	gransByShard := make([][]*GeoTileGranule, 0)

	accumLen := 0
	iShard := 0
	for _, polyGran := range gransByPolygon {
		if accumLen == 0 {
			gransByShard = append(gransByShard, make([]*GeoTileGranule, 0))
		}

		for _, gran := range polyGran {
			gransByShard[iShard] = append(gransByShard[iShard], gran)
		}

		accumLen += len(polyGran)
		if accumLen >= g0.GrpcConcLimit*len(connPool) {
			accumLen = 0
			iShard++
		}

	}

	meminfo := procmeminfo.MemInfo{}
	err = meminfo.Update()

	if err == nil {
		// We figure the sizes of each shard and then sort them in descending order.
		// We then compute the total size of the top PolygonShardConcLimit shards.
		// If the total size of the top shards is below memory threshold, we are good to go.
		shardSizes := make([]int, len(gransByShard))
		iShard := 0
		for _, polyGran := range gransByShard {
			shardSizes[iShard] = imageSize * dataSize * len(polyGran)
			iShard++
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
			gi.sendError(fmt.Errorf("Server resources exhausted"))
			return
		}

	} else {
		// If we have error obtaining meminfo,
		// we assume that we have enough memory.
		// This can happen if the OS doesn't support
		// /proc/meminfo
		log.Printf("meminfo error: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(len(gransByShard))

	cLimiter := NewConcLimiter(g0.GrpcConcLimit * len(connPool))
	granCounter := 0
	for _, polyGrans := range gransByShard {
		polyLimiter.Increase()
		go func(polyGrans []*GeoTileGranule, granCounter int) {
			defer wg.Done()
			outRasters := make([]*FlexRaster, len(polyGrans))

			var wgRpc sync.WaitGroup
			for iGran := range polyGrans {
				gran := polyGrans[iGran]
				select {
				case <-gi.Context.Done():
					polyLimiter.Decrease()
					return
				default:
					if gran.Path == "NULL" {
						outRasters[iGran] = &FlexRaster{ConfigPayLoad: gran.ConfigPayLoad, Data: make([]uint8, gran.Width*gran.Height), Height: gran.Height, Width: gran.Width, OffX: gran.OffX, OffY: gran.OffY, Type: gran.RasterType, NoData: 0.0, NameSpace: gran.NameSpace, TimeStamp: gran.TimeStamp, Polygon: gran.Polygon}
						continue
					}

					wgRpc.Add(1)
					cLimiter.Increase()
					go func(g *GeoTileGranule, gCnt int, idx int) {
						defer wgRpc.Done()
						defer cLimiter.Decrease()
						r, err := getRPCRaster(gi.Context, g, connPool[gCnt%len(connPool)])
						if err != nil {
							gi.sendError(err)
							r = &pb.Result{Raster: &pb.Raster{Data: make([]uint8, g.Width*g.Height), RasterType: "Byte", NoData: -1.}}
						}
						outRasters[idx] = &FlexRaster{ConfigPayLoad: g.ConfigPayLoad, Data: r.Raster.Data, Height: g.Height, Width: g.Width, OffX: g.OffX, OffY: g.OffY, Type: r.Raster.RasterType, NoData: r.Raster.NoData, NameSpace: g.NameSpace, TimeStamp: g.TimeStamp, Polygon: g.Polygon}
					}(gran, granCounter+iGran, iGran)
				}

			}
			wgRpc.Wait()

			select {
			case <-gi.Context.Done():
				return
			default:
			}

			gi.Out <- outRasters

		}(polyGrans, granCounter)

		granCounter += len(polyGrans)
	}

	wg.Wait()
}
func (gi *GeoRasterGRPC) sendError(err error) {
	select {
	case gi.Error <- err:
	default:
	}
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

func getRPCRaster(ctx context.Context, g *GeoTileGranule, conn *grpc.ClientConn) (*pb.Result, error) {
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
