package processor

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	pb "github.com/nci/gsky/worker/gdalservice"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type GeoDrillGRPC struct {
	Context context.Context
	In      chan *GeoDrillGranule
	Out     chan *DrillResult
	Error   chan error
	Clients []string
}

func NewDrillGRPC(ctx context.Context, serverAddress []string, errChan chan error) *GeoDrillGRPC {
	return &GeoDrillGRPC{
		Context: ctx,
		In:      make(chan *GeoDrillGranule, 100),
		Out:     make(chan *DrillResult, 100),
		Error:   errChan,
		Clients: serverAddress,
	}
}

func (gi *GeoDrillGRPC) Run(bandStrides int, decileCount int, pixelCount int, verbose bool) {
	if verbose {
		defer log.Printf("Drill gRPC done")
	}
	defer close(gi.Out)
	start := time.Now()

	const DefaultWpsRecvMsgSize = 100 * 1024 * 1024
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(DefaultWpsRecvMsgSize)),
	}

	conns := make([]*grpc.ClientConn, len(gi.Clients))
	for i, client := range gi.Clients {
		conn, err := grpc.Dial(client, opts...)
		if err != nil {
			log.Fatalf("gRPC connection problem: %v", err)
		}
		defer conn.Close()
		conns[i] = conn
	}

	var metrics []*pb.WorkerMetrics
	var geoReq *GeoDrillGranule
	var cLimiter *ConcLimiter

	workerStart := rand.Intn(len(conns))
	i := 0
	for gran := range gi.In {
		if gran.Path == "NULL" {
			continue
		}

		if geoReq == nil {
			geoReq = gran
		}

		if geoReq.Approx {
			needsRecompute := len(gran.Means) == 0 || len(gran.TimeStamps) != len(gran.Means) || len(gran.SampleCounts) != len(gran.Means)
			if !needsRecompute {
				hasStats := true
				ts := make([]*pb.TimeSeries, len(gran.TimeStamps))
				for it := range gran.TimeStamps {
					if gran.SampleCounts[it] < 0 {
						hasStats = false
						break
					}

					ts[it] = &pb.TimeSeries{Value: 0.0, Count: 0}
					if gran.Means[it] != gran.NoData {
						ts[it].Value = gran.Means[it]
						ts[it].Count = int32(gran.SampleCounts[it])
					}
				}

				if hasStats {
					gi.Out <- &DrillResult{NameSpace: gran.NameSpace, Data: ts, Dates: gran.TimeStamps}
					continue
				}
			}
		}

		if geoReq.MetricsCollector != nil {
			if i == 0 {
				defer func(t0 time.Time) { geoReq.MetricsCollector.Info.RPC.Duration += time.Since(t0) }(start)
				defer func() {
					for i := 0; i < len(metrics); i++ {
						if metrics[i] != nil {
							geoReq.MetricsCollector.Info.RPC.BytesRead += metrics[i].BytesRead
							geoReq.MetricsCollector.Info.RPC.UserTime += metrics[i].UserTime
							geoReq.MetricsCollector.Info.RPC.SysTime += metrics[i].SysTime
						}
					}
				}()

			}
			geoReq.MetricsCollector.Info.RPC.NumTiledGranules++
			metrics = append(metrics, &pb.WorkerMetrics{})
		}

		if cLimiter == nil {
			cLimiter = NewConcLimiter(geoReq.GrpcConcLimit * len(conns))
		}

		i++
		select {
		case <-gi.Context.Done():
			gi.sendError(fmt.Errorf("Drill gRPC: context has been cancel: %v", gi.Context.Err()))
			return
		case err := <-gi.Error:
			gi.sendError(err)
			return
		default:
			cLimiter.Increase()
			go func(g *GeoDrillGranule, conc *ConcLimiter, iTile int) {
				defer conc.Decrease()
				c := pb.NewGDALClient(conns[(iTile+workerStart)%len(conns)])
				bands, err := getBands(g.TimeStamps)

				granule := &pb.GeoRPCGranule{Operation: "drill", Path: g.Path, Geometry: g.Geometry, Bands: bands, Height: float32(gran.RasterYSize), Width: float32(gran.RasterXSize), BandStrides: int32(bandStrides), DrillDecileCount: int32(decileCount), ClipUpper: gran.ClipUpper, ClipLower: gran.ClipLower, PixelCount: int32(pixelCount), VRT: g.VRT}
				r, err := c.Process(gi.Context, granule)
				if err != nil {
					gi.sendError(fmt.Errorf("Drill gRPC: %v", err))
					r = &pb.Result{}
					return
				}

				nCols := int(r.Shape[1])
				nRows := int(r.Shape[0])
				for i := 0; i < nCols; i++ {
					ns := g.NameSpace
					if i > 0 {
						ns = g.NameSpace + fmt.Sprintf(DecileNamespace, i)
					}
					tsRow := make([]*pb.TimeSeries, nRows)
					for ir := 0; ir < nRows; ir++ {
						tsRow[ir] = r.TimeSeries[ir*nCols+i]
					}
					if gi.checkCancellation() {
						return
					}
					gi.Out <- &DrillResult{NameSpace: ns, Data: tsRow, NoData: r.Raster.NoData, Dates: g.TimeStamps}
				}

				if geoReq.MetricsCollector != nil {
					metrics[iTile-1] = r.Metrics
				}
			}(gran, cLimiter, i)
		}
	}

	if cLimiter != nil {
		cLimiter.Wait()
	}

	if verbose {
		log.Println("Drill gRPC Time", time.Since(start), "Processed:", i)
	}
}

func (gi *GeoDrillGRPC) sendError(err error) {
	select {
	case gi.Error <- err:
	default:
	}
}

func (gi *GeoDrillGRPC) checkCancellation() bool {
	select {
	case <-gi.Context.Done():
		gi.sendError(fmt.Errorf("Drill gRPC: context has been cancel: %v", gi.Context.Err()))
		return true
	case err := <-gi.Error:
		gi.sendError(err)
		return true
	default:
		return false
	}
}

func getBands(times []time.Time) ([]int32, error) {
	out := make([]int32, len(times))

	for i := range times {
		out[i] = int32(i + 1)
	}
	return out, nil
}
