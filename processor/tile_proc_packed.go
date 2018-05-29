package processor

import (
	"github.com/nci/gsky/utils"
	"fmt"
	"golang.org/x/net/context"
)

type GeoProcessor struct {
	Context    context.Context
	In         chan *GeoTileRequest
	Out        chan []utils.Raster
	Error      chan error
	APIAddress string
	RPCAddress string
}

func NewGeoProcessor(ctx context.Context, apiAddr, serverAddr string, errChan chan error) *GeoProcessor {
	return &GeoProcessor{
		Context:    ctx,
		In:         make(chan *GeoTileRequest, 100),
		Out:        make(chan []utils.Raster, 100),
		Error:      errChan,
		APIAddress: apiAddr,
		RPCAddress: serverAddr,
	}
}

func (gp *GeoProcessor) Run() {
	defer close(gp.Out)

	cLimiter := NewConcLimiter(4)
	for gran := range gp.In {
		select {
		case <-gp.Context.Done():
			gp.Error <- fmt.Errorf("Tile gRPC context has been cancel: %v", gp.Context.Err())
			return
		default:
			cLimiter.Increase()
			go func(g *GeoTileRequest, conc *ConcLimiter) {
				defer conc.Decrease()
				p := NewTileInternalPipeline(gp.Context, gp.APIAddress, gp.RPCAddress, gp.Error)
				for rast := range p.Process(g) {
					gp.Out <- rast
				}
			}(gran, cLimiter)
		}
	}
	cLimiter.Wait()
}
