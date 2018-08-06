package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	pp "github.com/nci/gsky/worker/gdalprocess"
	pb "github.com/nci/gsky/worker/gdalservice"

	_ "net/http/pprof"

	"os"
	"os/signal"
	"syscall"

	reuseport "github.com/kavu/go_reuseport"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type server struct {
	Pool *pp.ProcessPool
}

func (s *server) Process(ctx context.Context, in *pb.GeoRPCGranule) (*pb.Result, error) {
	rChan := make(chan *pb.Result)
	defer close(rChan)
	errChan := make(chan error)
	defer close(errChan)

	s.Pool.AddQueue(&pp.Task{Payload: in, Resp: rChan, Error: errChan})

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
	poolSize := flag.Int("n", runtime.NumCPU(), "Maximum number of requests handled concurrently.")
	executable := flag.String("exec", filepath.Dir(os.Args[0])+"/gsky-gdal-process", "Executable filepath")
	debug := flag.Bool("debug", false, "verbose logging")
	flag.Parse()

	procPool, err := pp.CreateProcessPool(*poolSize, *executable, *port, *debug)
	if err != nil {
		log.Printf("Failed to create process pool: %v", err)
		os.Exit(2)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for {
			select {
			case <-signals:
				for _, proc := range procPool.Pool {
					proc.RemoveTempFiles()
				}

				os.Exit(1)
			}
		}
	}()

	s := grpc.NewServer()
	pb.RegisterGDALServer(s, &server{Pool: procPool})

	lis, err := reuseport.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
