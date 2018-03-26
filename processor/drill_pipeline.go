package processor

import (
	"context"
	"fmt"
)

type DrillPipeline struct {
	Context  context.Context
	Error    chan error
	RPCAddrs []string
	APIAddr  string
}

func InitDrillPipeline(ctx context.Context, apiAddr string, rpcAddrs []string, errChan chan error) *DrillPipeline {
	return &DrillPipeline{
		Context:  ctx,
		Error:    errChan,
		RPCAddrs: rpcAddrs,
		APIAddr:  apiAddr,
	}
}

func (dp *DrillPipeline) Process(geoReq GeoDrillRequest) chan string {
	grpcDriller := NewDrillGRPC(dp.Context, dp.RPCAddrs, dp.Error)
	if grpcDriller == nil {
		dp.Error <- fmt.Errorf("Couldn't instantiate RPCDriller %s/n", dp.RPCAddrs)
	}

	splt := NewTimeSplitter(-1, dp.Error)
	go func() {
		splt.In <- &geoReq
		close(splt.In)
	}()
	i := NewDrillIndexer(dp.Context, dp.APIAddr, dp.Error)
	dm := NewDrillMerger(dp.Error)

	i.In = splt.Out
	grpcDriller.In = i.Out
	dm.In = grpcDriller.Out

	go splt.Run()
	go i.Run()
	go grpcDriller.Run()
	go dm.Run()

	return dm.Out
}
