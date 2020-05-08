package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"

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
	PoolSize int
	Pool     *pp.ProcessPool
}

func (s *server) Process(ctx context.Context, in *pb.GeoRPCGranule) (*pb.Result, error) {
	if in.Operation == "worker_info" {
		return &pb.Result{WorkerInfo: &pb.WorkerInfo{PoolSize: int32(s.PoolSize)}}, nil
	}

	rChan := make(chan *pb.Result, 1)
	defer close(rChan)
	errChan := make(chan error, 1)
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
	maxTaskProcessed := flag.Int("max_tasks", 20000, "Maximum number of tasks processed before starting gsky-gdal-process.")
	oomThreshold := flag.Int("oom_threshold", int(1.5*1024*1024), "MemAvailable lower than the threshold (KB) triggers an OOM of the worker process")
	debug := flag.Bool("debug", false, "verbose logging")
	flag.Parse()

	procPool, err := pp.CreateProcessPool(*poolSize, *executable, *port, *maxTaskProcessed, *debug)
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

	go func() {
		parts := strings.Split(*executable, "/")
		fileName := parts[len(parts)-1]
		execMatch := fileName

		// The maximum length of the name field under /proc/<pid>/status is 16 bytes
		maxLen := 15
		if len(fileName) > maxLen {
			execMatch = fileName[:maxLen]
		}
		mon := pp.NewOOMMonitor(execMatch, *oomThreshold, true)
		mon.StartMonitorLoop()
	}()

	s := grpc.NewServer()
	pb.RegisterGDALServer(s, &server{Pool: procPool, PoolSize: *poolSize})

	lis, err := reuseport.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
