package main

import (
	"flag"
	"fmt"
	pb "github.com/nci/gsky/grpc_server/gdalservice"
	"log"
	"net"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	_ "net/http/pprof"
	//"time"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type server struct {
	Pool *pb.ProcessPool
}

func (s *server) Process(ctx context.Context, in *pb.GeoRPCGranule) (*pb.Result, error) {
	rChan := make(chan *pb.Result)
	defer close(rChan)
	errChan := make(chan error)
	defer close(errChan)

	s.Pool.AddQueue(&pb.Task{Payload: in, Resp: rChan, Error: errChan})

	select {
	case out, ok := <-rChan:
		if !ok {
			return &pb.Result{}, fmt.Errorf("task response channel has been closed")
		}
		if out.Error != "OK" {
			return &pb.Result{}, fmt.Errorf("%s", out.Error)
		}
		return out, nil
	case err := <-errChan:
		return &pb.Result{}, fmt.Errorf("Error in ops: %v", err)
	}
}

func main() {
	port := flag.Int("p", 6000, "gRPC server listening port.")
	poolSize := flag.Int("n", 8, "Maximum number of requests handled concurrently.")
	debug := flag.Bool("debug", false, "verbose logging")
	flag.Parse()

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	p := pb.CreateProcessPool(*poolSize, *debug)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for {
			select {
			case <-signals:
				p.DeleteProcessPool()
				time.Sleep(1 * time.Second)
				os.Exit(1)
			}
		}
	}()

	s := grpc.NewServer()
	pb.RegisterGDALServer(s, &server{Pool: p})

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
