package utils

import (
	"context"
	"os"
	"testing"
)

func TestGetDummyGDALDatasetH(t *testing.T) {
	if GetDummyGDALDatasetH() != nil {
		t.Errorf("Dummy GDALDatasetH handle is non-null")
	}
}

func TestEncodeGdalOpen(t *testing.T) {
	raster := ByteRaster{NameSpace: "test-ns", Data: []uint8{}, Width: 5, Height: 5}
	rs := []Raster{&raster}
	hDstDS, tempFile, err := EncodeGdalOpen("/tmp", 256, 256, "geotiff", []float64{-179, 0.359, 0, 80, 0, -0.16}, 4326, rs, 1000, 1000, 1)
	defer os.Remove(tempFile)

	if err != nil {
		t.Errorf("failed to create gdal file: %v", err)
		return
	}

	if hDstDS == nil {
		t.Errorf("failed to open gdal dataset file")
		return
	}
}

func TestEncodeGdal(t *testing.T) {
	raster := ByteRaster{NameSpace: "test-ns", Data: make([]uint8, 25), Width: 5, Height: 5}
	rs := []Raster{&raster}

	height := 1000
	width := 1000
	hDstDS, tempFile, err := EncodeGdalOpen("/tmp", 256, 256, "geotiff", []float64{-179, 0.359, 0, 80, 0, -0.16}, 4326, rs, width, height, 1)
	defer os.Remove(tempFile)

	for i := 0; i < raster.Height*raster.Width; i++ {
		raster.Data[i] = uint8(i)
	}

	// we test all the four corner cases
	err = EncodeGdal(hDstDS, rs, 0, 0)
	if err != nil {
		t.Errorf("failed to write to gdal dataset file: %v", err)
		return
	}

	err = EncodeGdal(hDstDS, rs, width-raster.Width, 0)
	if err != nil {
		t.Errorf("failed to write to gdal dataset file: %v", err)
		return
	}

	err = EncodeGdal(hDstDS, rs, width-raster.Width, height-raster.Height)
	if err != nil {
		t.Errorf("failed to write to gdal dataset file: %v", err)
		return
	}

	err = EncodeGdal(hDstDS, rs, 0, height-raster.Height)
	if err != nil {
		t.Errorf("failed to write to gdal dataset file: %v", err)
		return
	}
}

func testEncodeGdalFlush(t *testing.T) {
	raster := ByteRaster{NameSpace: "test-ns", Data: []uint8{}, Width: 5, Height: 5}
	rs := []Raster{&raster}
	hDstDS, tempFile, err := EncodeGdalOpen("/tmp", 256, 256, "geotiff", []float64{-179, 0.359, 0, 80, 0, -0.16}, 4326, rs, 1000, 1000, 1)
	defer os.Remove(tempFile)

	newDstDS, err := EncodeGdalFlush(hDstDS, tempFile, "geotiff")
	if err != nil || newDstDS == nil {
		t.Errorf("Failed to re-open existing dataset file: %v", err)
		return
	}
}

func testEncodeGdalMerge(t *testing.T) {
	raster := ByteRaster{NameSpace: "test-ns", Data: []uint8{}, Width: 5, Height: 5}
	rs := []Raster{&raster}

	width := 1000
	height := 1000
	hDstDS, tempFile, err := EncodeGdalOpen("/tmp", 256, 256, "geotiff", []float64{-179, 0.359, 0, 80, 0, -0.16}, 4326, rs, width, height, 1)
	defer os.Remove(tempFile)

	raster2 := ByteRaster{NameSpace: "test-ns", Data: make([]uint8, 100), Width: 10, Height: 10}
	rs2 := []Raster{&raster2}
	_, tempFile2, err := EncodeGdalOpen("/tmp", 256, 256, "geotiff", []float64{-179, 0.359, 0, 80, 0, -0.16}, 4326, rs2, width, height, 1)
	defer os.Remove(tempFile2)

	widthList := []int{5, 5, 5, 5}
	heightList := []int{5, 5, 5, 5}
	xOffList := []int{0, width - 5, width - 5, 0}
	yOffList := []int{0, 0, width - 5, width - 5}

	ctx := context.Background()
	err = EncodeGdalMerge(ctx, hDstDS, "geotiff", tempFile2, widthList, heightList, xOffList, yOffList)
	if err != nil {
		t.Errorf("Failed to merge dataset file: %v", err)
		return
	}
}
