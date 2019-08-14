package processor

//#include "ogr_srs_api.h"
//#cgo pkg-config: gdal
import "C"

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
	"unsafe"

	pb "github.com/nci/gsky/worker/gdalservice"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type GeoRasterGRPC struct {
	Context               context.Context
	In                    chan *GeoTileGranule
	Out                   chan []*FlexRaster
	Error                 chan error
	Clients               []string
	MaxGrpcRecvMsgSize    int
	PolygonShardConcLimit int
	MaxGrpcBufferSize     int
}

func NewRasterGRPC(ctx context.Context, serverAddress []string, maxGrpcRecvMsgSize int, polygonShardConcLimit int, maxGrpcBufferSize int, errChan chan error) *GeoRasterGRPC {
	return &GeoRasterGRPC{
		Context:               ctx,
		In:                    make(chan *GeoTileGranule, 100),
		Out:                   make(chan []*FlexRaster, 100),
		Error:                 errChan,
		Clients:               serverAddress,
		MaxGrpcRecvMsgSize:    maxGrpcRecvMsgSize,
		PolygonShardConcLimit: polygonShardConcLimit,
		MaxGrpcBufferSize:     maxGrpcBufferSize,
	}
}

func (gi *GeoRasterGRPC) Run(polyLimiter *ConcLimiter, varList []string, verbose bool) {
	if verbose {
		defer log.Printf("tile grpc done")
	}
	defer close(gi.Out)

	t0 := time.Now()

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

			if _, found := availNamespaces[gran.VarNameSpace]; !found {
				availNamespaces[gran.VarNameSpace] = true
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

	dataSize, err := getDataSize(g0.RasterType)
	if err != nil {
		gi.Error <- err
		return
	}

	if g0.MetricsCollector != nil {
		defer func() { g0.MetricsCollector.Info.RPC.Duration += time.Since(t0) }()
	}

	if g0.GrpcTileXSize > 0.0 || g0.GrpcTileYSize > 0.0 {
		maxXTileSize := g0.Width
		if g0.GrpcTileXSize <= 0.0 {
			maxXTileSize = g0.Width
		} else if g0.GrpcTileXSize <= 1.0 {
			maxXTileSize = int(float64(g0.Width) * g0.GrpcTileXSize)
		} else if g0.GrpcTileXSize < float64(g0.Width) {
			maxXTileSize = int(g0.GrpcTileXSize)
		}

		maxYTileSize := g0.Height
		if g0.GrpcTileYSize <= 0.0 {
			maxYTileSize = g0.Height
		} else if g0.GrpcTileYSize <= 1.0 {
			maxYTileSize = int(float64(g0.Height) * g0.GrpcTileYSize)
		} else if g0.GrpcTileYSize < float64(g0.Height) {
			maxYTileSize = int(g0.GrpcTileYSize)
		}

		var tmpGrans []*GeoTileGranule
		for _, g := range grans {
			tmpGrans = append(tmpGrans, g)
		}

		if verbose {
			log.Printf("tile grpc maxTileSize: %v, %v", maxXTileSize, maxYTileSize)
		}

		grans = nil
		for _, g := range tmpGrans {
			xRes := (g.BBox[2] - g.BBox[0]) / float64(g.Width)
			yRes := (g.BBox[3] - g.BBox[1]) / float64(g.Height)

			for y := 0; y < g.Height; y += maxYTileSize {
				for x := 0; x < g.Width; x += maxXTileSize {
					yMin := g.BBox[1] + float64(y)*yRes
					yMax := math.Min(g.BBox[1]+float64(y+maxYTileSize)*yRes, g.BBox[3])
					xMin := g.BBox[0] + float64(x)*xRes
					xMax := math.Min(g.BBox[0]+float64(x+maxXTileSize)*xRes, g.BBox[2])

					tileXSize := int(.5 + (xMax-xMin)/xRes)
					tileYSize := int(.5 + (yMax-yMin)/yRes)

					tileGran := &GeoTileGranule{ConfigPayLoad: g.ConfigPayLoad, RawPath: g.RawPath, Path: g.Path, NameSpace: g.NameSpace, VarNameSpace: g.VarNameSpace, RasterType: g.RasterType, TimeStamp: g.TimeStamp, BandIdx: g.BandIdx, Polygon: g.Polygon, BBox: []float64{xMin, yMin, xMax, yMax}, Height: tileYSize, Width: tileXSize, RawHeight: g.Height, RawWidth: g.Width, OffX: x, OffY: g.Height - y - tileYSize, CRS: g.CRS, SrcSRS: g.SrcSRS, SrcGeoTransform: g.SrcGeoTransform, GeoLocation: g.GeoLocation}
					grans = append(grans, tileGran)
				}
			}

		}

		if verbose {
			log.Printf("tile grpc: %v tiled granules", len(grans))
		}
	}

	if g0.MetricsCollector != nil {
		g0.MetricsCollector.Info.RPC.NumTiledGranules += len(grans)
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
		if accumLen >= g0.GrpcConcLimit*effectivePoolSize {
			accumLen = 0
			iShard++
		}

	}

	if gi.MaxGrpcBufferSize > 0 {
		// We figure the sizes of each shard and then sort them in descending order.
		// We then compute the total size of the top PolygonShardConcLimit shards.
		// If the total size of the top shards is below memory threshold, we are good to go.
		shardSizes := make([]int, len(gransByShard))
		iShard = 0
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

		if requestedSize > gi.MaxGrpcBufferSize {
			log.Printf("requested size greater than MaxGrpcBufferSize, requested:%v, MaxGrpcBufferSize:%v", requestedSize, gi.MaxGrpcBufferSize)
			gi.sendError(fmt.Errorf("Server resources exhausted"))
			return
		}
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

	hSRS := C.OSRNewSpatialReference(nil)
	defer C.OSRDestroySpatialReference(hSRS)
	crsC := C.CString(g0.CRS)
	defer C.free(unsafe.Pointer(crsC))
	C.OSRSetFromUserInput(hSRS, crsC)
	var projWKTC *C.char
	defer C.free(unsafe.Pointer(projWKTC))
	C.OSRExportToWkt(hSRS, &projWKTC)
	projWKT := C.GoString(projWKTC)

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
						r, err := getRPCRaster(gi.Context, g, projWKT, connPool[gCnt%len(connPool)])
						if err != nil {
							gi.sendError(err)
							r = &pb.Result{Raster: &pb.Raster{Data: make([]uint8, g.Width*g.Height), RasterType: "Byte", NoData: -1.}}
						}
						if len(r.Raster.Bbox) == 0 {
							r.Raster.Bbox = []int32{0, 0, int32(g.Width), int32(g.Height)}
						}
						rOffX := g.OffX + int(r.Raster.Bbox[0])
						rOffY := g.OffY + int(r.Raster.Bbox[1])
						rWidth := int(r.Raster.Bbox[2])
						rHeight := int(r.Raster.Bbox[3])
						rawHeight := g.Height
						rawWidth := g.Width
						if g.RawHeight > 0 && g.RawWidth > 0 {
							rawHeight = g.RawHeight
							rawWidth = g.RawWidth
						}
						outRasters[idx] = &FlexRaster{ConfigPayLoad: g.ConfigPayLoad, Data: r.Raster.Data, Height: rawHeight, Width: rawWidth, DataHeight: rHeight, DataWidth: rWidth, OffX: rOffX, OffY: rOffY, Type: r.Raster.RasterType, NoData: r.Raster.NoData, NameSpace: g.NameSpace, TimeStamp: g.TimeStamp, Polygon: g.Polygon}
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
	case "Byte", "SignedByte":
		return 1, nil
	case "Int16":
		return 2, nil
	case "UInt16":
		return 2, nil
	case "Float32":
		return 4, nil

	// grpc workers convert any real data types other than the above to float32
	case "Float64":
		return 4, nil
	case "Int32":
		return 4, nil
	case "UInt32":
		return 4, nil
	default:
		return -1, fmt.Errorf("Unsupported raster type %s", dataType)
	}
}

func getRPCRaster(ctx context.Context, g *GeoTileGranule, projWKT string, conn *grpc.ClientConn) (*pb.Result, error) {
	c := pb.NewGDALClient(conn)
	geot := BBox2Geot(g.Width, g.Height, g.BBox)
	granule := &pb.GeoRPCGranule{Operation: "warp", Height: int32(g.Height), Width: int32(g.Width), Path: g.Path, DstSRS: projWKT, DstGeot: geot, Bands: []int32{int32(g.BandIdx)}}
	if g.GeoLocation != nil {
		granule.GeoLocOpts = []string{
			fmt.Sprintf("X_DATASET=%s", g.GeoLocation.XDSName),
			fmt.Sprintf("Y_DATASET=%s", g.GeoLocation.YDSName),

			fmt.Sprintf("X_BAND=%d", g.GeoLocation.XBand),
			fmt.Sprintf("Y_BAND=%d", g.GeoLocation.YBand),

			fmt.Sprintf("LINE_OFFSET=%d", g.GeoLocation.LineOffset),
			fmt.Sprintf("PIXEL_OFFSET=%d", g.GeoLocation.PixelOffset),
			fmt.Sprintf("LINE_STEP=%d", g.GeoLocation.LineStep),
			fmt.Sprintf("PIXEL_STEP=%d", g.GeoLocation.PixelStep),
		}

	}

	if g.UserSrcSRS > 0 {
		granule.SrcSRS = g.SrcSRS
	}

	if g.UserSrcGeoTransform > 0 {
		granule.SrcGeot = g.SrcGeoTransform
	}

	if g.NoReprojection {
		granule.DstSRS = ""
	}

	r, err := c.Process(ctx, granule)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// BBox2Geot return the geotransform from the
// parameters received in a WMS GetMap request
func BBox2Geot(width, height int, bbox []float64) []float64 {
	return []float64{bbox[0], (bbox[2] - bbox[0]) / float64(width), 0, bbox[3], 0, (bbox[1] - bbox[3]) / float64(height)}
}
