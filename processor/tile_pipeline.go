package processor

import (
	"encoding/json"
	"context"
	"github.com/nci/gsky/utils"
	"fmt"
)
func P(text string) {
    fmt.Printf("%+v\n", text)
}
func Ptu(item FlexRaster) {
	out, err := json.Marshal(item)
	if err != nil {
		panic (err)
	}
	P(string(out))
}

type TilePipeline struct {
	Context               context.Context
	Error                 chan error
	RPCAddress            []string
	MaxGrpcRecvMsgSize    int
	PolygonShardConcLimit int
	MASAddress            string
	MaxGrpcBufferSize     int
}
func Pu0(item *ConcLimiter) {
	out, err := json.Marshal(item)
	if err != nil {
		panic (err)
	}
	P(string(out))
}

func InitTilePipeline(ctx context.Context, masAddr string, rpcAddr []string, maxGrpcRecvMsgSize int, polygonShardConcLimit int, maxGrpcBufferSize int, errChan chan error) *TilePipeline {
	return &TilePipeline{
		Context:               ctx,
		Error:                 errChan,
		RPCAddress:            rpcAddr,
		MaxGrpcRecvMsgSize:    maxGrpcRecvMsgSize,
		PolygonShardConcLimit: polygonShardConcLimit,
		MASAddress:            masAddr,
		MaxGrpcBufferSize:     maxGrpcBufferSize,
	}
}

func (dp *TilePipeline) Process(geoReq *GeoTileRequest, verbose bool) chan []utils.Raster {
	grpcTiler := NewRasterGRPC(dp.Context, dp.RPCAddress, dp.MaxGrpcRecvMsgSize, dp.PolygonShardConcLimit, dp.MaxGrpcBufferSize, dp.Error)
	i := NewTileIndexer(dp.Context, dp.MASAddress, dp.Error)
	go func() {
		i.In <- geoReq
		close(i.In)
	}()

	m := NewRasterMerger(dp.Context, dp.Error)
//Ptu(grpcTiler.Out)	

	grpcTiler.In = i.Out
	m.In = grpcTiler.Out
//fmt.Println(dp)
	polyLimiter := NewConcLimiter(dp.PolygonShardConcLimit)
//fmt.Printf("\n--------geoReq.BandExpr: %v\n", geoReq.BandExpr)
//fmt.Println(geoReq.BandExpr)
	
	go i.Run(verbose)
	go grpcTiler.Run(polyLimiter, geoReq.BandExpr.VarList, verbose)
	go m.Run(polyLimiter, geoReq.BandExpr, verbose)
//fmt.Println("------------------")
//fmt.Println(m.Out)
	return m.Out

}
