package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/nci/gsky/utils"
	gp "github.com/nci/gsky/worker/gdalprocess"
	pb "github.com/nci/gsky/worker/gdalservice"
)

func sendOutput(out *pb.Result, conn net.Conn) error {
	outb, err := proto.Marshal(out)
	if err != nil {
		return err
	}

	_, err = conn.Write(outb)
	if err != nil {
		return err
	}

	return nil
}

func dataHandler(conn net.Conn, debug bool, timeout int) {
	defer conn.Close()
	out := &pb.Result{}

	var buf bytes.Buffer
	n, err := io.Copy(&buf, conn)
	if err != nil {
		out.Error = fmt.Sprintf("Error reading data %d from socket: %v", n, err)
		sendOutput(out, conn)
	}

	in := new(pb.GeoRPCGranule)
	err = proto.Unmarshal(buf.Bytes(), in)
	if err != nil {
		out.Error = fmt.Sprintf("Error unmarshaling protobuf request: %v", err)
		sendOutput(out, conn)
	}

	if len(in.Path) == 0 {
		return
	}

	done := make(chan bool, 1)
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)

	go func() {
		switch in.Operation {
		case "warp":
			out = gp.WarpRaster(in, debug)
		case "drill":
			out = gp.DrillDataset(in)
		case "extent":
			out = gp.ComputeReprojectExtent(in)
		case "info":
			out = gp.ExtractGDALInfo(in)
		default:
			out.Error = fmt.Sprintf("Unknown operation: %s", in.Operation)
		}

		done <- true
	}()

	select {
	case <-done:
		timeoutCancel()
	case <-timeoutCtx.Done():
		log.Printf("%v timed out in %v seconds", in.Path, timeout)
		os.Exit(2)
	}

	err = sendOutput(out, conn)
	if err != nil {
		log.Println(err)
	}
}

func init() {
	if _, ok := os.LookupEnv("GOMAXPROCS"); !ok {
		runtime.GOMAXPROCS(2)
	}

	utils.InitGdal()
}

func main() {
	debug := flag.Bool("debug", false, "verbose logging")
	sock := flag.String("sock", "", "unix socket path")
	timeout := flag.Int("timeout", 120, "timeout in seconds")
	flag.Parse()

	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: *sock, Net: "unix"})
	if err != nil {
		log.Fatal(err)
		return
	}
	defer os.Remove(*sock)

	log.Println("Listening on", *sock)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
			return
		}

		dataHandler(conn, *debug, *timeout)
	}
}
