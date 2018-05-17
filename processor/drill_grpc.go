package processor

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	pb "github.com/nci/gsky/grpc_server/gdalservice"
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

func (gi *GeoDrillGRPC) Run() {
	defer close(gi.Out)
	//start := time.Now()
	//i := 0

	conns := make([]*grpc.ClientConn, len(gi.Clients))
	for i, client := range gi.Clients {
		conn, err := grpc.Dial(client, grpc.WithInsecure())
		if err != nil {
			log.Fatalf("gRPC connection problem: %v", err)
		}
		defer conn.Close()
		conns[i] = conn
	}

	cLimiter := NewConcLimiter(16)
	start := time.Now()
	i := 0
	for gran := range gi.In {
		i++
		select {
		case <-gi.Context.Done():
			gi.Error <- fmt.Errorf("Drill gRPC context has been cancel: %v", gi.Context.Err())
			return
		default:
			cLimiter.Increase()
			//i += 1
			go func(g *GeoDrillGranule, conc *ConcLimiter) {
				defer conc.Decrease()
				c := pb.NewGDALClient(conns[rand.Intn(len(conns))])
				bands, err := getBands(g.TimeStamps)
				epsg, err := extractEPSGCode(g.CRS)
				granule := &pb.GeoRPCGranule{Path: g.Path, EPSG: int32(epsg), Geometry: g.Geometry, Bands: bands}
				r, err := c.Process(gi.Context, granule)
				if err != nil {
					gi.Error <- err
					r = &pb.Result{}
					return
				}
				gi.Out <- &DrillResult{NameSpace: g.NameSpace, Data: r.TimeSeries, Dates: g.TimeStamps}
			}(gran, cLimiter)
		}
	}
	cLimiter.Wait()
	fmt.Println("gRPC Time", time.Since(start), "Processed:", i)
}

func getBands(times []time.Time) ([]int32, error) {
	out := make([]int32, len(times))

	for i := range times {
		out[i] = int32(i + 1)
	}
	return out, nil
}
