package gdalprocess

// #include "gdal.h"
// #include "gdal_frmts.h"
// #cgo pkg-config: gdal
import "C"

import (
	pb "github.com/nci/gsky/worker/gdalservice"
)

func RegisterGDALDrivers() {
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
		driver := C.GDALGetDriver(C.int(i))
		switch C.GoString(C.GDALGetDriverShortName(driver)) {
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
		driver := C.GDALGetDriver(C.int(i))
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
func GdalBuiltinProcess(input *pb.GeoRPCGranule, debug bool) *pb.Result {
	out := new(pb.Result)
	if input.Geot == nil && input.Geometry == "" {
		out = ExtractGDALInfo(input)
	} else if input.Geot == nil {
		out = DrillDataset(input)
	} else if input.Geometry == "<geometry_extent>" {
		out = ComputeReprojectExtent(input)
	} else {
		out = WarpRaster(input, debug)
	}

	return out
}
