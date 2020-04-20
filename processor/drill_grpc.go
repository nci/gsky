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
	defer close(gi.Out)
	start := time.Now()

	var inputs []*GeoDrillGranule
	for gran := range gi.In {
		if gran.Path == "NULL" {
			continue
		}
		inputs = append(inputs, gran)
	}

	if len(inputs) == 0 {
		return
	}

	geoReq := inputs[0]
	if geoReq.MetricsCollector != nil {
		defer func() { geoReq.MetricsCollector.Info.RPC.Duration += time.Since(start) }()
		geoReq.MetricsCollector.Info.RPC.NumTiledGranules += len(inputs)
	}

	var inputsRecompute []*GeoDrillGranule
	if geoReq.Approx {
		for _, gran := range inputs {
			if len(gran.Means) == 0 || len(gran.TimeStamps) != len(gran.Means) || len(gran.SampleCounts) != len(gran.Means) {
				inputsRecompute = append(inputsRecompute, gran)
				continue
			}

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
			} else {
				inputsRecompute = append(inputsRecompute, gran)
			}
		}
	} else {
		inputsRecompute = inputs
	}

	if len(inputsRecompute) == 0 {
		if verbose {
			fmt.Println("gRPC Time", time.Since(start), "Processed:", len(inputs))
		}
		return
	}

	const DefaultWpsRecvMsgSize = 100 * 1024 * 1024
	const DefaultWpsConcLimit = 16

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

	metrics := make([]*pb.WorkerMetrics, len(inputsRecompute))
	for i := 0; i < len(metrics); i++ {
		metrics[i] = &pb.WorkerMetrics{}
	}
	defer func() {
		if geoReq.MetricsCollector != nil {
			for i := 0; i < len(metrics); i++ {
				if metrics[i] != nil {
					geoReq.MetricsCollector.Info.RPC.BytesRead += metrics[i].BytesRead
					geoReq.MetricsCollector.Info.RPC.UserTime += metrics[i].UserTime
					geoReq.MetricsCollector.Info.RPC.SysTime += metrics[i].SysTime
				}
			}
		}
	}()

	cLimiter := NewConcLimiter(DefaultWpsConcLimit * len(conns))
	workerStart := rand.Intn(len(conns))
	i := 0
	for _, gran := range inputsRecompute {
		i++
		select {
		case <-gi.Context.Done():
			gi.Error <- fmt.Errorf("Drill gRPC context has been cancel: %v", gi.Context.Err())
			return
		default:
			cLimiter.Increase()
			go func(g *GeoDrillGranule, conc *ConcLimiter, iTile int) {
				defer conc.Decrease()
				c := pb.NewGDALClient(conns[(iTile+workerStart)%len(conns)])
				bands, err := getBands(g.TimeStamps)

				granule := &pb.GeoRPCGranule{Operation: "drill", Path: g.Path, Geometry: g.Geometry, Bands: bands, BandStrides: int32(bandStrides), DrillDecileCount: int32(decileCount), ClipUpper: gran.ClipUpper, ClipLower: gran.ClipLower, PixelCount: int32(pixelCount), VRT: g.VRT}
				r, err := c.Process(gi.Context, granule)
				if err != nil {
					gi.Error <- err
					r = &pb.Result{}
					return
				}
				metrics[iTile-1] = r.Metrics

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

					gi.Out <- &DrillResult{NameSpace: ns, Data: tsRow, Dates: g.TimeStamps}
				}
			}(gran, cLimiter, i)
		}
	}
	cLimiter.Wait()
	if verbose {
		fmt.Println("gRPC Time", time.Since(start), "Processed:", i)
	}
}

func getBands(times []time.Time) ([]int32, error) {
	out := make([]int32, len(times))

	for i := range times {
		out[i] = int32(i + 1)
	}
	return out, nil
}
