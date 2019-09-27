package processor

import (
	"context"
	"fmt"
	"strings"
)

const DecileNamespace = "_d%d"

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

func (dp *DrillPipeline) Process(geoReq GeoDrillRequest, suffix string, templateFileName string, bandStrides int, approx bool, drillAlgorithm string, verbose bool) chan string {
	const DefaultDecileAnchorPoints = 9
	decileCount := 0
	pixelCount := 0
	if len(drillAlgorithm) > 0 {
		drillAlgos := strings.Split(drillAlgorithm, ",")
		for _, algo := range drillAlgos {
			algo = strings.ToLower(strings.TrimSpace(algo))
			if len(algo) == 0 {
				continue
			}

			if algo == "deciles" {
				decileCount = DefaultDecileAnchorPoints
			}

			if algo == "pixel_count" {
				pixelCount = 1
			}
		}

	}
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

	go i.Run(verbose)
	go grpcDriller.Run(bandStrides, decileCount, pixelCount, verbose)

	nCols := decileCount + 1
	var namespaces []string
	for _, ns := range geoReq.NameSpaces {
		for i := 0; i < nCols; i++ {
			newNs := ns
			if i > 0 {
				newNs = ns + fmt.Sprintf(DecileNamespace, i)
			}
			namespaces = append(namespaces, newNs)
		}
	}
	go dm.Run(suffix, namespaces, templateFileName, geoReq.BandExpr, decileCount)

	return dm.Out
}
