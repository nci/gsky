package processor

//#include "ogr_srs_api.h"
//#cgo pkg-config: gdal
import "C"

import (
	"fmt"
	"log"
	"math"
	"math/rand"
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

func (gi *GeoRasterGRPC) Run(varList []string, verbose bool) {
	if verbose {
		defer log.Printf("tile grpc done")
	}
	defer close(gi.Out)

	var g0 *GeoTileGranule
	var grans []*GeoTileGranule
	var nullGrans []*GeoTileGranule
	availNamespaces := make(map[string]bool)
	dedupGrans := make(map[string]bool)
	var connPool []*grpc.ClientConn
	var projWKT string
	var cLimiter *ConcLimiter

	accumMetrics := &pb.WorkerMetrics{}

	var outRasters []*FlexRaster
	var outMetrics []*pb.WorkerMetrics

	t0 := time.Now()

	iGran := 0
	var wgRpc sync.WaitGroup
	for inGran := range gi.In {
		if inGran.Path == "NULL" {
			if len(nullGrans) == 0 {
				nullGrans = append(nullGrans, inGran)
			}
			continue
		}

		granKey := fmt.Sprintf("%s_%d", inGran.Path, inGran.BandIdx)
		if _, hasGran := dedupGrans[granKey]; hasGran {
			continue
		}
		dedupGrans[granKey] = true
		grans = append(grans, inGran)

		if _, found := availNamespaces[inGran.VarNameSpace]; !found {
			availNamespaces[inGran.VarNameSpace] = true
		}

		if iGran == 0 {
			g0 = grans[0]
			defer func() {
				if g0.MetricsCollector != nil {
					g0.MetricsCollector.Info.RPC.BytesRead += accumMetrics.BytesRead
					g0.MetricsCollector.Info.RPC.UserTime += accumMetrics.UserTime
					g0.MetricsCollector.Info.RPC.SysTime += accumMetrics.SysTime
				}
			}()

			opts := []grpc.DialOption{
				grpc.WithInsecure(),
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(gi.MaxGrpcRecvMsgSize)),
			}

			clientIdx := make([]int, len(gi.Clients))
			for ic := range clientIdx {
				clientIdx[ic] = ic
			}
			rand.Shuffle(len(clientIdx), func(i, j int) { clientIdx[i], clientIdx[j] = clientIdx[j], clientIdx[i] })

			effectivePoolSize := len(gi.Clients)
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
			crsC := C.CString(g0.CRS)
			C.OSRSetFromUserInput(hSRS, crsC)
			var projWKTC *C.char
			C.OSRExportToWkt(hSRS, &projWKTC)
			projWKT = C.GoString(projWKTC)

			C.free(unsafe.Pointer(projWKTC))
			C.free(unsafe.Pointer(crsC))
			C.OSRDestroySpatialReference(hSRS)

			g0.DstGeoTransform = BBox2Geot(g0.Width, g0.Height, g0.BBox)

			cLimiter = NewConcLimiter(g0.GrpcConcLimit * len(connPool))
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
			if maxXTileSize <= 0 {
				maxXTileSize = g0.Width
			}

			maxYTileSize := g0.Height
			if g0.GrpcTileYSize <= 0.0 {
				maxYTileSize = g0.Height
			} else if g0.GrpcTileYSize <= 1.0 {
				maxYTileSize = int(float64(g0.Height) * g0.GrpcTileYSize)
			} else if g0.GrpcTileYSize < float64(g0.Height) {
				maxYTileSize = int(g0.GrpcTileYSize)
			}
			if maxYTileSize <= 0 {
				maxYTileSize = g0.Height
			}

			var tmpGrans []*GeoTileGranule
			for _, g := range grans {
				tmpGrans = append(tmpGrans, g)
			}

			if verbose && iGran == 0 {
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

						tileBBox := []float64{xMin, yMin, xMax, yMax}
						tileGeot := BBox2Geot(tileXSize, tileYSize, tileBBox)
						tileGran := &GeoTileGranule{ConfigPayLoad: g.ConfigPayLoad, RawPath: g.RawPath, Path: g.Path, NameSpace: g.NameSpace, VarNameSpace: g.VarNameSpace, RasterType: g.RasterType, TimeStamp: g.TimeStamp, BandIdx: g.BandIdx, Polygon: g.Polygon, BBox: tileBBox, Height: tileYSize, Width: tileXSize, RawHeight: g.Height, RawWidth: g.Width, OffX: x, OffY: g.Height - y - tileYSize, CRS: g.CRS, SrcSRS: g.SrcSRS, SrcGeoTransform: g.SrcGeoTransform, DstGeoTransform: tileGeot, GeoLocation: g.GeoLocation}
						grans = append(grans, tileGran)
					}
				}

			}
		}

		for ig := range grans {
			gran := grans[ig]
			outRasters = append(outRasters, nil)
			outMetrics = append(outMetrics, nil)
			select {
			case <-gi.Context.Done():
				gi.sendError(fmt.Errorf("tile grpc: context has been cancel: %v", gi.Context.Err()))
				return
			case err := <-gi.Error:
				gi.sendError(err)
				return
			default:
				if gran.Path == "NULL" {
					outRasters[iGran] = &FlexRaster{ConfigPayLoad: gran.ConfigPayLoad, Data: make([]uint8, gran.Width*gran.Height), Height: gran.Height, Width: gran.Width, OffX: gran.OffX, OffY: gran.OffY, Type: gran.RasterType, NoData: 0.0, NameSpace: gran.NameSpace, TimeStamp: gran.TimeStamp, Polygon: gran.Polygon}
					continue
				}

				wgRpc.Add(1)
				cLimiter.Increase()
				go func(g *GeoTileGranule, idx int) {
					defer wgRpc.Done()
					defer cLimiter.Decrease()
					var geot []float64
					if len(g.DstGeoTransform) > 0 {
						geot = g.DstGeoTransform
					} else {
						geot = g0.DstGeoTransform
					}
					r, err := getRPCRaster(gi.Context, g, projWKT, geot, connPool[idx%len(connPool)])
					if err != nil {
						gi.sendError(err)
						r = &pb.Result{Raster: &pb.Raster{Data: make([]uint8, g.Width*g.Height), RasterType: "Byte", NoData: -1.}}
					}
					outMetrics[idx] = r.Metrics
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
				}(gran, iGran)
			}
			iGran++
		}
		grans = nil
	}
	wgRpc.Wait()

	if g0 != nil && g0.MetricsCollector != nil {
		g0.MetricsCollector.Info.RPC.Duration += time.Since(t0)
	}

	if iGran == 0 {
		if len(nullGrans) > 0 {
			gran := nullGrans[0]
			gi.Out <- []*FlexRaster{&FlexRaster{ConfigPayLoad: gran.ConfigPayLoad, Data: make([]uint8, gran.Width*gran.Height), Height: gran.Height, Width: gran.Width, OffX: gran.OffX, OffY: gran.OffY, Type: gran.RasterType, NoData: 0.0, NameSpace: gran.NameSpace, TimeStamp: gran.TimeStamp, Polygon: gran.Polygon}}
		}
		return
	}

	for i := 0; i < len(outMetrics); i++ {
		if outMetrics[i] != nil {
			accumMetrics.BytesRead += outMetrics[i].BytesRead
			accumMetrics.UserTime += outMetrics[i].UserTime
			accumMetrics.SysTime += outMetrics[i].SysTime
		}
	}

	if g0 != nil && g0.MetricsCollector != nil {
		g0.MetricsCollector.Info.RPC.NumTiledGranules += iGran
	}

	if verbose {
		log.Printf("tile grpc: %v effective granules", iGran)
	}

	for _, v := range varList {
		if _, found := availNamespaces[v]; !found {
			gi.sendError(fmt.Errorf("band '%v' not found", v))
			return
		}
	}

	if gi.checkCancellation() {
		return
	}

	gi.Out <- outRasters
}

func (gi *GeoRasterGRPC) sendError(err error) {
	select {
	case gi.Error <- err:
	default:
	}
}

func (gi *GeoRasterGRPC) checkCancellation() bool {
	select {
	case <-gi.Context.Done():
		gi.sendError(fmt.Errorf("tile grpc: context has been cancel: %v", gi.Context.Err()))
		return true
	case err := <-gi.Error:
		gi.sendError(err)
		return true
	default:
		return false
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

func getRPCRaster(ctx context.Context, g *GeoTileGranule, projWKT string, geot []float64, conn *grpc.ClientConn) (*pb.Result, error) {
	c := pb.NewGDALClient(conn)
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

	if g.SRSCf > 0 {
		granule.SRSCf = int32(g.SRSCf)
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
