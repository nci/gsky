package processor

import (
	"context"

	"github.com/nci/gsky/utils"
)

type TilePipeline struct {
	Context            context.Context
	Error              chan error
	RPCAddress         []string
	MaxGrpcRecvMsgSize int
	MASAddress         string
}

func InitTilePipeline(ctx context.Context, masAddr string, rpcAddr []string, maxGrpcRecvMsgSize int, errChan chan error) *TilePipeline {
	return &TilePipeline{
		Context:            ctx,
		Error:              errChan,
		RPCAddress:         rpcAddr,
		MaxGrpcRecvMsgSize: maxGrpcRecvMsgSize,
		MASAddress:         masAddr,
	}
}

func (dp *TilePipeline) Process(geoReq *GeoTileRequest) chan []utils.Raster {
	// We can actually simplify this piece of code as GeoProcessor only takes a single request right now.
	// but we will leave it for now in case we need to handle multile time points for WCS.
	// In the case of WCS with multiple time points, we will create one request per time point
	// which will require looping through the input channel as the GeoProcessor currently does.
	/*
		p := NewGeoProcessor(dp.Context, dp.MASAddress, dp.RPCAddress, dp.MaxGrpcRecvMsgSize, dp.Error)

		p.In <- geoReq
		close(p.In)

		go p.Run()
		return p.Out
	*/

	grpcTiler := NewRasterGRPC(dp.Context, dp.RPCAddress, dp.MaxGrpcRecvMsgSize, dp.Error)

	i := NewTileIndexer(dp.Context, dp.MASAddress, dp.Error)
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
