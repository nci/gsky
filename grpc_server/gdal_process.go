package main

// #include "gdal.h"
// #include "gdal_frmts.h"
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

	gp "github.com/nci/gsky/grpc_server/gdalprocess"
	pb "github.com/nci/gsky/grpc_server/gdalservice"
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

func registerGDALDrivers() {
	// This is a bit nasty, but this is one way to work out which
	// drivers are present in the GDAL shared library. We then
	// load the drivers of interest and then load all of the
	// drivers. This places common drivers at the front of the
	// driver list.
	var haveNetCDF, haveHDF4, haveHDF5, haveJP2OpenJPEG bool
	var haveGTiff bool

	// Find out which drivers are present
	C.GDALAllRegister()
	for i := 0; i < int(C.GDALGetDriverCount()); i++ {
		driver := C.GDALGetDriver(C.int(i));
		switch (C.GoString(C.GDALGetDriverShortName(driver))) {
		case "netCDF":
			haveNetCDF = true
		case "HDF4":
			haveHDF4 = true
		case "HDF5":
			haveHDF5 = true
		case "JP2OpenJPEG":
			haveJP2OpenJPEG = true
		case "GTiff":
			haveGTiff = true
		}
	}

	// De-register all the drivers again
	for i := 0; i < int(C.GDALGetDriverCount()); i++ {
		driver := C.GDALGetDriver(C.int(i));
		C.GDALDeregisterDriver(driver)
	}

	// Register these drivers first for higher performance when
	// opening files (drivers are interrogated in a linear scan)
	if haveNetCDF {
		C.GDALRegister_netCDF()
	}
	if haveHDF4 {
		C.GDALRegister_HDF4()
	}
	if haveHDF5 {
		C.GDALRegister_HDF5()
	}
	if haveJP2OpenJPEG {
		C.GDALRegister_JP2OpenJPEG()
	}
	if haveGTiff {
		C.GDALRegister_GTiff()
	}
	// Now register everything else
	C.GDALAllRegister()
}

func main() {
	debug := flag.Bool("debug", false, "verbose logging")
	sock := flag.String("sock", "", "unix socket path")
	flag.Parse()

	registerGDALDrivers()

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
