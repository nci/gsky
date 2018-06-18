package utils

// #include "gdal.h"
// #include "ogr_srs_api.h"
// #cgo pkg-config: gdal
import "C"

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"unsafe"
)

type Raster interface {
	GetNoData() float64
}

type ByteRaster struct {
	Data          []uint8
	Height, Width int
	NoData        float64
}

func (r *ByteRaster) GetNoData() float64 {
	return r.NoData
}

type Int16Raster struct {
	Data          []int16
	Height, Width int
	NoData        float64
}

func (r *Int16Raster) GetNoData() float64 {
	return r.NoData
}

type UInt16Raster struct {
	Data          []uint16
	Height, Width int
	NoData        float64
}

func (r *UInt16Raster) GetNoData() float64 {
	return r.NoData
}

type Float32Raster struct {
	Data          []float32
	Height, Width int
	NoData        float64
}

func (r *Float32Raster) GetNoData() float64 {
	return r.NoData
}

func EncodePNG(br []*ByteRaster, palette *Palette) ([]byte, error) {
	buf := new(bytes.Buffer)
	canvas := image.NewRGBA(image.Rect(0, 0, br[0].Width, br[0].Height))

	switch len(br) {
	case 1:
		if palette != nil {
			plt, err := GradientRGBAPalette(palette)
			if err != nil {
				return buf.Bytes(), err
			}

			for x := 0; x < br[0].Width; x++ {
				for y := 0; y < br[0].Height; y++ {
					if br[0].Data[y*br[0].Width+x] != 0xFF {
						canvas.Set(x, y, plt[br[0].Data[y*br[0].Width+x]])
					}
				}
			}
		}

	case 3:
		rasterR := br[0]
		rasterG := br[1]
		rasterB := br[2]

		if rasterR == nil || rasterG == nil || rasterB == nil {
			return []byte{}, fmt.Errorf("At least one of the bands is nil")
		}

		var start int
		for i := 0; i < rasterR.Width*rasterR.Height; i++ {
			if rasterR.Data[i] != 0xFF || rasterG.Data[i] != 0xFF || rasterB.Data[i] != 0xFF {
				start = i * 4
				canvas.Pix[start] = rasterR.Data[i]
				canvas.Pix[start+1] = rasterG.Data[i]
				canvas.Pix[start+2] = rasterB.Data[i]
				canvas.Pix[start+3] = 0xff
			}
		}

	default:
		return []byte{}, fmt.Errorf("Cannot encode other than 1 or 3 namespaces into a PNG: Received %d", len(br))
	}

	err := png.Encode(buf, canvas)

	return buf.Bytes(), err
}

func ValidateRasterSlice(rs []Raster) (int, int, string, error) {
	var width, height int
	var rasterType string
	var err error

	for _, r := range rs {
		switch t := r.(type) {
		case *ByteRaster:
			if rasterType == "" {
				rasterType = "Byte"
			} else if rasterType != "Byte" {
				err = fmt.Errorf("Mixed types")
			}

			if width == 0 {
				width = t.Width
			} else if width != t.Width {
				err = fmt.Errorf("Mixed width sizes")
			}

			if height == 0 {
				height = t.Height
			} else if height != t.Height {
				err = fmt.Errorf("Mixed height sizes")
			}
		case *Int16Raster:
			if rasterType == "" {
				rasterType = "Int16"
			} else if rasterType != "Int16" {
				err = fmt.Errorf("Mixed types")
			}

			if width == 0 {
				width = t.Width
			} else if width != t.Width {
				err = fmt.Errorf("Mixed width sizes")
			}

			if height == 0 {
				height = t.Height
			} else if height != t.Height {
				err = fmt.Errorf("Mixed height sizes")
			}
		case *UInt16Raster:
			if rasterType == "" {
				rasterType = "UInt16"
			} else if rasterType != "UInt16" {
				err = fmt.Errorf("Mixed types")
			}

			if width == 0 {
				width = t.Width
			} else if width != t.Width {
				err = fmt.Errorf("Mixed width sizes")
			}

			if height == 0 {
				height = t.Height
			} else if height != t.Height {
				err = fmt.Errorf("Mixed height sizes")
			}
		case *Float32Raster:
			if rasterType == "" {
				rasterType = "Float32"
			} else if rasterType != "Float32" {
				err = fmt.Errorf("Mixed types")
			}

			if width == 0 {
				width = t.Width
			} else if width != t.Width {
				err = fmt.Errorf("Mixed width sizes")
			}

			if height == 0 {
				height = t.Height
			} else if height != t.Height {
				err = fmt.Errorf("Mixed height sizes")
			}
		default:
			err = fmt.Errorf("Raster type not implemented")
		}
	}
	return width, height, rasterType, err
}

var GDALTypes = map[string]C.GDALDataType{"Unkown": 0, "Byte": 1, "UInt16": 2, "Int16": 3,
	"UInt32": 4, "Int32": 5, "Float32": 6, "Float64": 7,
	"CInt16": 8, "CInt32": 9, "CFloat32": 10, "CFloat64": 11,
	"TypeCount": 12}

func EncodeGdal(format string, rs []Raster, geot []float64, epsg int) ([]byte, error) {
	var driverName string
	switch strings.ToLower(format) {
	case "geotiff":
		driverName = "GTiff"
	case "netcdf":
		driverName = "netCDF"
	default:
		return []byte{}, fmt.Errorf("Unsupported encoding format: %v", format)
	}

	w, h, rType, err := ValidateRasterSlice(rs)
	if err != nil {
		return []byte{}, fmt.Errorf("Error validating raster %v", err)
	}

	tempFileHandle, err := ioutil.TempFile("", "raster_")
	if err != nil {
		return []byte{}, fmt.Errorf("failed to create raster temp file: %v\n", err)
	}

	tempFile := tempFileHandle.Name()
	defer os.Remove(tempFile)

	C.GDALAllRegister()

	var driverNameC = C.CString(driverName)
	hDriver := C.GDALGetDriverByName(driverNameC)

	hDstDS := C.GDALCreate(hDriver, C.CString(tempFile), C.int(w), C.int(h), C.int(len(rs)), GDALTypes[rType], nil)
	if hDstDS == nil {
		return []byte{}, fmt.Errorf("Error creating raster")
	}

	// Set projection
	hSRS := C.OSRNewSpatialReference(nil)
	defer C.OSRDestroySpatialReference(hSRS)
	C.OSRImportFromEPSG(hSRS, C.int(epsg))
	var projWKT *C.char
	defer C.free(unsafe.Pointer(projWKT))
	C.OSRExportToWkt(hSRS, &projWKT)
	C.GDALSetProjection(hDstDS, projWKT)

	// Set geotransform
	C.GDALSetGeoTransform(hDstDS, (*C.double)(&geot[0]))

	for i, r := range rs {
		hBand := C.GDALGetRasterBand(hDstDS, C.int(i+1))
		gerr := C.CPLErr(0)
		switch t := r.(type) {
		case *ByteRaster:
			C.GDALSetRasterNoDataValue(hBand, C.double(t.NoData))
			gerr = C.GDALRasterIO(hBand, C.GF_Write, 0, 0, C.int(t.Width), C.int(t.Height), unsafe.Pointer(&t.Data[0]), C.int(t.Width), C.int(t.Height), C.GDT_Byte, 0, 0)

		case *Int16Raster:
			C.GDALSetRasterNoDataValue(hBand, C.double(t.NoData))
			gerr = C.GDALRasterIO(hBand, C.GF_Write, 0, 0, C.int(t.Width), C.int(t.Height), unsafe.Pointer(&t.Data[0]), C.int(t.Width), C.int(t.Height), C.GDT_Int16, 0, 0)

		case *UInt16Raster:
			C.GDALSetRasterNoDataValue(hBand, C.double(t.NoData))
			gerr = C.GDALRasterIO(hBand, C.GF_Write, 0, 0, C.int(t.Width), C.int(t.Height), unsafe.Pointer(&t.Data[0]), C.int(t.Width), C.int(t.Height), C.GDT_UInt16, 0, 0)

		case *Float32Raster:
			C.GDALSetRasterNoDataValue(hBand, C.double(t.NoData))
			gerr = C.GDALRasterIO(hBand, C.GF_Write, 0, 0, C.int(t.Width), C.int(t.Height), unsafe.Pointer(&t.Data[0]), C.int(t.Width), C.int(t.Height), C.GDT_Float32, 0, 0)

		default:
			C.GDALClose(hDstDS)
			return []byte{}, fmt.Errorf("Unsupported gdal data type")
		}

		if gerr != 0 {
			C.GDALClose(hDstDS)
			return []byte{}, fmt.Errorf("Error writing raster band: %d", i)
		}
	}

	C.GDALClose(hDstDS)

	f, err := os.Open(tempFile)
	if err != nil {
		return []byte{}, fmt.Errorf("Error opening raster file: %v", tempFile)
	}
	defer f.Close()

	out, err := ioutil.ReadAll(f)
	if err != nil {
		return []byte{}, fmt.Errorf("Error reading raster file: %v", tempFile)
	}

	return out, nil

}

// ExtractEPSGCode parses an SRS string and gets
// the EPSG code
func ExtractEPSGCode(srs string) (int, error) {
	return strconv.Atoi(srs[5:])
}
