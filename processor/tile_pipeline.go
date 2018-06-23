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
