package processor

import (
	"context"
	"github.com/nci/gsky/utils"
)

type TilePipeline struct {
	Context               context.Context
	Error                 chan error
	RPCAddress            []string
	MaxGrpcRecvMsgSize    int
	PolygonShardConcLimit int
	MASAddress            string
}

func InitTilePipeline(ctx context.Context, masAddr string, rpcAddr []string, maxGrpcRecvMsgSize int, polygonShardConcLimit int, errChan chan error) *TilePipeline {
	return &TilePipeline{
		Context:               ctx,
		Error:                 errChan,
		RPCAddress:            rpcAddr,
		MaxGrpcRecvMsgSize:    maxGrpcRecvMsgSize,
		PolygonShardConcLimit: polygonShardConcLimit,
		MASAddress:            masAddr,
	}
}

func (dp *TilePipeline) Process(geoReq *GeoTileRequest, verbose bool) chan []utils.Raster {
	grpcTiler := NewRasterGRPC(dp.Context, dp.RPCAddress, dp.MaxGrpcRecvMsgSize, dp.PolygonShardConcLimit, dp.Error)

	i := NewTileIndexer(dp.Context, dp.MASAddress, dp.Error)
	go func() {
		i.In <- geoReq
		close(i.In)
	}()

	m := NewRasterMerger(dp.Context, dp.Error)

	grpcTiler.In = i.Out
	m.In = grpcTiler.Out

	polyLimiter := NewConcLimiter(dp.PolygonShardConcLimit)
	go i.Run(verbose)
	go grpcTiler.Run(polyLimiter, verbose)
	go m.Run(polyLimiter, verbose)

	return m.Out

}
