package processor

import (
	"context"
	"fmt"
	"github.com/nci/gsky/utils"
)

type TileInternalPipeline struct {
	Context    context.Context
	Error      chan error
	RPCAddress string
	APIAddress string
}

func NewTileInternalPipeline(ctx context.Context, apiAddr string, rpcAddr string, errChan chan error) *TileInternalPipeline {
	return &TileInternalPipeline{
		Context:    ctx,
		Error:      errChan,
		RPCAddress: rpcAddr,
		APIAddress: apiAddr,
	}
}

func (dp *TileInternalPipeline) Process(geoReq *GeoTileRequest) chan []utils.Raster {
	grpcTiler := NewRasterGRPC(dp.Context, dp.RPCAddress, dp.Error)
	if grpcTiler == nil {
		dp.Error <- fmt.Errorf("Couldn't instantiate RPCTiler %s/n", dp.RPCAddress)
		return nil
	}

	i := NewTileIndexer(dp.Context, dp.APIAddress, dp.Error)
	go func() {
		i.In <- geoReq
		close(i.In)
	}()

	m := NewRasterMerger(dp.Error)

	grpcTiler.In = i.Out
	m.In = grpcTiler.Out

	go i.Run()
	go grpcTiler.Run()
	go m.Run()

	return m.Out
}
