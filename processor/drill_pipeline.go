package processor

import (
	"context"
	"fmt"
)

type DrillPipeline struct {
	Context     context.Context
	Error       chan error
	RPCAddrs    []string
	APIAddr     string
	IdentityTol float64
	DpTol       float64
}

func InitDrillPipeline(ctx context.Context, apiAddr string, rpcAddrs []string, identityTol float64, dpTol float64, errChan chan error) *DrillPipeline {
	return &DrillPipeline{
		Context:     ctx,
		Error:       errChan,
		RPCAddrs:    rpcAddrs,
		APIAddr:     apiAddr,
		IdentityTol: identityTol,
		DpTol:       dpTol,
	}
}

func (dp *DrillPipeline) Process(geoReq GeoDrillRequest, suffix string, templateFileName string, bandStrides int, approx bool) chan string {
	grpcDriller := NewDrillGRPC(dp.Context, dp.RPCAddrs, dp.Error)
	if grpcDriller == nil {
		dp.Error <- fmt.Errorf("Couldn't instantiate RPCDriller %s/n", dp.RPCAddrs)
	}

	i := NewDrillIndexer(dp.Context, dp.APIAddr, dp.IdentityTol, dp.DpTol, approx, dp.Error)
	go func() {
		i.In <- &geoReq
		close(i.In)
	}()

	dm := NewDrillMerger(dp.Error)

	grpcDriller.In = i.Out
	dm.In = grpcDriller.Out

	go i.Run()
	go grpcDriller.Run(bandStrides)
	go dm.Run(suffix, geoReq.NameSpaces, templateFileName, geoReq.BandExpr)

	return dm.Out
}
