package gdalprocess

import (
	"os"
	"testing"

	"github.com/nci/gsky/utils"
	pb "github.com/nci/gsky/worker/gdalservice"
)

func TestComputeReprojectExtent(t *testing.T) {
	if _, err := os.Stat("/g/data2/tc43/modis-fc/v310/tiles/monthly/cover/FC_Monthly_Medoid.v310.MCD43A4.h29v12.2017.006.nc"); os.IsNotExist(err) {
		t.Skip("Test data file is unavailable. Skipping tests")
		return
	}

	raster := utils.ByteRaster{NameSpace: "test-ns", Data: make([]uint8, 25), Width: 5, Height: 5}
	geot := []float64{-179, 0.359, 0, 80, 0, -0.16}
	rs := []utils.Raster{&raster}
	_, tempFile, _ := utils.EncodeGdalOpen("/tmp", 256, 256, "geotiff", geot, 4326, rs, 1000, 1000, 1)
	defer os.Remove(tempFile)

	geo := &pb.GeoRPCGranule{Path: "NETCDF:\"/g/data2/tc43/modis-fc/v310/tiles/monthly/cover/FC_Monthly_Medoid.v310.MCD43A4.h29v12.2017.006.nc\":phot_veg", EPSG: 4326, Geot: geot}
	res := ComputeReprojectExtent(geo)
	expected := []uint8{210, 75, 0, 0, 0, 0, 0, 0, 188, 33, 0, 0, 0, 0, 0, 0}
	for i, val := range res.Raster.Data {
		if val != expected[i] {
			t.Errorf("unexpected value %v, %v:", expected, res.Raster.Data)
			return
		}
	}

}
