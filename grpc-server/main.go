package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	pb "github.com/nci/gsky/worker/gdalservice"

	_ "net/http/pprof"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"os"
	"os/signal"
	"syscall"
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
	executable := flag.String("exec", "", "Executable filepath")
	debug := flag.Bool("debug", false, "verbose logging")
	flag.Parse()

	p, err := pb.CreateProcessPool(*poolSize, *executable, *debug)
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
				for _, proc := range p.Pool {
					os.Remove(proc.TempFile)
					os.Remove(proc.Address)
				}

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
