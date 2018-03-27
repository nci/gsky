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

func register_gdal_drivers() {
	// This is a bit nasty, but this is one way to work out which
	// drivers are present in the GDAL shared library. We then
	// load the drivers of interest and then load all of the
	// drivers. This places common drivers at the front of the
	// driver list.
	var have_netCDF, have_HDF4, have_HDF5, have_JP2OpenJPEG bool
	var have_GTiff bool

	// Find out which drivers are present
	C.GDALAllRegister()
	for i := 0; i < int(C.GDALGetDriverCount()); i++ {
		driver := C.GDALGetDriver(C.int(i));
		switch (C.GoString(C.GDALGetDriverShortName(driver))) {
		case "netCDF":
			have_netCDF = true
		case "HDF4":
			have_HDF4 = true
		case "HDF5":
			have_HDF5 = true
		case "JP2OpenJPEG":
			have_JP2OpenJPEG = true
		case "GTiff":
			have_GTiff = true
		}
	}

	// De-register all the drivers again
	for i := 0; i < int(C.GDALGetDriverCount()); i++ {
		driver := C.GDALGetDriver(C.int(i));
		C.GDALDeregisterDriver(driver)
	}

	// Register these drivers first for higher performance when
	// opening files (drivers are interrogated in a linear scan)
	if have_netCDF {
		C.GDALRegister_netCDF()
	}
	if have_HDF4 {
		C.GDALRegister_HDF4()
	}
	if have_HDF5 {
		C.GDALRegister_HDF5()
	}
	if have_JP2OpenJPEG {
		C.GDALRegister_JP2OpenJPEG()
	}
	if have_GTiff {
		C.GDALRegister_GTiff()
	}
	// Now register everything else
	C.GDALAllRegister()
}

func main() {
	debug := flag.Bool("debug", false, "verbose logging")
	sock := flag.String("sock", "", "unix socket path")
	flag.Parse()

	register_gdal_drivers()

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
