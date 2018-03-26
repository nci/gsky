package processor

import (
	"context"
	"fmt"
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

func (dp *TileInternalPipeline) Process(geoReq *GeoTileRequest) chan *ByteRaster {
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
	sc := NewRasterScaler(dp.Error)

	grpcTiler.In = i.Out
	m.In = grpcTiler.Out
	sc.In = m.Out

	go i.Run()
	go grpcTiler.Run()
	go m.Run()
	go sc.Run()

	return sc.Out
}
