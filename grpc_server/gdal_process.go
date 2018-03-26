package main

// #include "gdal.h"
// #cgo LDFLAGS: -lgdal
import "C"

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	gp "./gdalprocess"
	pb "./gdalservice"
	"github.com/golang/protobuf/proto"
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

func dataHandler(conn net.Conn, debug bool) {
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

	// TODO "This is ugly"
	if in.Geot == nil && in.Geometry == "" {
		out = gp.ExtractGDALInfo(in)
	} else if in.Geot == nil {
		out = gp.DrillDataset(in)
	} else {
		out = gp.WarpRaster(in, debug)
	}

	err = sendOutput(out, conn)
	if err != nil {
		log.Println(err)
	}

}

func main() {
	debug := flag.Bool("debug", false, "verbose logging")
	sock := flag.String("sock", "", "unix socket path")
	flag.Parse()

	C.GDALAllRegister()

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

		dataHandler(conn, *debug)
	}
}
