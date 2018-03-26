package processor

import (
	"context"
)

type TilePipeline struct {
	Context    context.Context
	Error      chan error
	RPCAddress string
	MASAddress string
}

func InitTilePipeline(ctx context.Context, masAddr string, rpcAddr string, errChan chan error) *TilePipeline {
	return &TilePipeline{
		Context:    ctx,
		Error:      errChan,
		RPCAddress: rpcAddr,
		MASAddress: masAddr,
	}
}

func (dp *TilePipeline) Process(geoReq *GeoTileRequest) chan []byte {

	s := NewTileSplitter(dp.Context, dp.Error)
	go func() {
		s.In <- geoReq
		close(s.In)
	}()
	p := NewGeoProcessor(dp.Context, dp.MASAddress, dp.RPCAddress, dp.Error)
	enc := NewPNGEncoder(dp.Error)

	p.In = s.Out
	enc.In = p.Out

	go s.Run()
	go p.Run()
	go enc.Run()

	return enc.Out
}
