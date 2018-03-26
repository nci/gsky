package processor

import (
	"context"
	"fmt"
	"io"
	"regexp"
)

type InfoPipeline struct {
	Context context.Context
	Error   chan error
	RPCAdds []string
}

func InitInfoPipeline(ctx context.Context, rpcAddrs []string, errChan chan error) *InfoPipeline {
	return &InfoPipeline{
		Context: ctx,
		Error:   errChan,
		RPCAdds: rpcAddrs,
	}
}

func (dp *InfoPipeline) Process(rootPath string, contains *regexp.Regexp, file io.Writer) chan struct{} {
	i := NewFileCrawler(rootPath, contains, dp.Error)
	go func() {
		i.In <- rootPath
		close(i.In)
	}()

	e := NewJSONEncoder(dp.Error)
	p := NewJSONPrinter(file, dp.Error)

	grpcInfo := NewInfoGRPC(dp.Context, dp.RPCAdds, dp.Error)
	if grpcInfo == nil {
		dp.Error <- fmt.Errorf("Couldn't instantiate RPCTiler %s/n", dp.RPCAdds)
		close(p.Out)
		return p.Out
	}

	grpcInfo.In = i.Out
	e.In = grpcInfo.Out
	p.In = e.Out

	go i.Run()
	go grpcInfo.Run()
	go e.Run()
	go p.Run()

	return p.Out
}
