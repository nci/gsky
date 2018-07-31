package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"

	pp "github.com/nci/gsky/worker/gdalprocess"
	pb "github.com/nci/gsky/worker/gdalservice"

	_ "net/http/pprof"

	"os"
	"os/signal"
	"syscall"

	"github.com/kavu/go_reuseport"
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
	executable := flag.String("exec", "", "Executable filepath")
	debug := flag.Bool("debug", false, "verbose logging")
	flag.Parse()

	var procPool *pp.ProcessPool
	if *executable != pp.BuiltinProcessExec {
		psize := *poolSize
		// avoid double counting self as one of workers
		if len(*executable) == 0 {
			psize--
		}
		if psize < 0 {
			psize = 0
		}
		tmpProcPool, err := pp.CreateProcessPool(psize, *executable, *port, *debug)
		if err != nil {
			log.Printf("Failed to create process pool: %v", err)
			os.Exit(2)
		}
		procPool = tmpProcPool

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
	} else {
		procPool = &pp.ProcessPool{[]*pp.Process{}, make(chan *pp.Task, 400), make(chan *pp.ErrorMsg)}
	}

	useBuiltinProcess := len(*executable) == 0 || *executable == pp.BuiltinProcessExec
	if useBuiltinProcess {
		pp.RegisterGDALDrivers()
		go func() {
			for task := range procPool.TaskQueue {
				out := pp.GdalBuiltinProcess(task.Payload, *debug)
				task.Resp <- out
			}
		}()
	}

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
