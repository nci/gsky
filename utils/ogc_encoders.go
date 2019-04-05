package utils

// #include "gdal.h"
// #include "ogr_srs_api.h"
// #cgo pkg-config: gdal
import "C"

import (
	"bytes"
	"context"
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
	NameSpace     string
	Data          []uint8
	Height, Width int
	NoData        float64
}

func (r *ByteRaster) GetNoData() float64 {
	return r.NoData
}

type Int16Raster struct {
	NameSpace     string
	Data          []int16
	Height, Width int
	NoData        float64
}

func (r *Int16Raster) GetNoData() float64 {
	return r.NoData
}

type UInt16Raster struct {
	NameSpace     string
	Data          []uint16
	Height, Width int
	NoData        float64
}

func (r *UInt16Raster) GetNoData() float64 {
	return r.NoData
}

type Float32Raster struct {
	NameSpace     string
	Data          []float32
	Height, Width int
	NoData        float64
}

func (r *Float32Raster) GetNoData() float64 {
	return r.NoData
}

const EmptyTileNS = "EmptyTile"

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
		} else {
			var start int
			for i := 0; i < br[0].Width*br[0].Height; i++ {
				val := br[0].Data[i]
				if val != 0xFF {
					start = i * 4
					canvas.Pix[start] = val
					canvas.Pix[start+1] = val
					canvas.Pix[start+2] = val
					canvas.Pix[start+3] = 0xff
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

	if width <= 0 || height <= 0 {
		err = fmt.Errorf("data unavailable")
	}

	return width, height, rasterType, err
}

var GDALTypes = map[string]C.GDALDataType{"Unkown": 0, "Byte": 1, "UInt16": 2, "Int16": 3,
	"UInt32": 4, "Int32": 5, "Float32": 6, "Float64": 7,
	"CInt16": 8, "CInt32": 9, "CFloat32": 10, "CFloat64": 11,
	"TypeCount": 12}

func GetDummyGDALDatasetH() C.GDALDatasetH {
	var handle C.GDALDatasetH
	return handle
}

func GetDriverNameFromFormat(format string) (string, error) {
	var driverName string
	switch strings.ToLower(format) {
	case "geotiff":
		driverName = "GTiff"
	case "netcdf":
		driverName = "netCDF"
	default:
		return "", fmt.Errorf("Unsupported encoding format: %v", format)
	}

	return driverName, nil
}

func EncodeGdalOpen(tempDir string, blockXSize int, blockYSize int, format string, geot []float64, epsg int, rs []Raster, width int, height int, bands int) (C.GDALDatasetH, string, error) {
	_, _, rType, err := ValidateRasterSlice(rs)
	if err != nil {
		return nil, "", fmt.Errorf("Error validating raster: %v", err)
	}

	driverName, err := GetDriverNameFromFormat(format)
	if err != nil {
		return nil, "", err
	}

	var driverOptions []*C.char
	switch strings.ToLower(format) {
	case "geotiff":
		driverOptions = append(driverOptions, C.CString("COMPRESS=PACKBITS"))
		driverOptions = append(driverOptions, C.CString("TILED=YES"))
		driverOptions = append(driverOptions, C.CString("BIGTIFF=YES"))
		driverOptions = append(driverOptions, C.CString("INTERLEAVE=BAND"))
		driverOptions = append(driverOptions, C.CString(fmt.Sprintf("BLOCKXSIZE=%d", blockXSize)))
		driverOptions = append(driverOptions, C.CString(fmt.Sprintf("BLOCKYSIZE=%d", blockYSize)))
	case "netcdf":
		driverOptions = append(driverOptions, C.CString("COMPRESS=DEFLATE"))
		driverOptions = append(driverOptions, C.CString("ZLEVEL=6"))
	default:
		return nil, "", fmt.Errorf("Unsupported encoding format: %v", format)
	}

	for _, opt := range driverOptions {
		defer C.free(unsafe.Pointer(opt))
	}

	// NULL pointer is used to terminate the point array by gdal
	driverOptions = append(driverOptions, nil)

	var driverNameC = C.CString(driverName)
	hDriver := C.GDALGetDriverByName(driverNameC)

	tempFileHandle, err := ioutil.TempFile(tempDir, "raster_")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create raster temp file: %v\n", err)
	}
	tempFileHandle.Close()

	tempFile := tempFileHandle.Name()
	tempFileC := C.CString(tempFile)
	defer C.free(unsafe.Pointer(tempFileC))
	hDstDS := C.GDALCreate(hDriver, tempFileC, C.int(width), C.int(height), C.int(bands), GDALTypes[rType], &driverOptions[0])
	if hDstDS == nil {
		os.Remove(tempFile)
		return nil, "", fmt.Errorf("Error creating raster")
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

	return hDstDS, tempFile, nil
}

func EncodeGdal(hDstDS C.GDALDatasetH, rs []Raster, xOff int, yOff int) ([]string, error) {
	_, _, _, err := ValidateRasterSlice(rs)
	if err != nil {
		return []string{}, fmt.Errorf("Error validating raster: %v", err)
	}

	resNameSpaceC := C.CString("long_name")
	defer C.free(unsafe.Pointer(resNameSpaceC))

	bandNames := make([]string, len(rs))
	for i, r := range rs {
		hBand := C.GDALGetRasterBand(hDstDS, C.int(i+1))
		gerr := C.CPLErr(0)
		switch t := r.(type) {
		case *ByteRaster:
			bandNames[i] = t.NameSpace
			if isEmptyTile(t.NameSpace) {
				continue
			}
			C.GDALSetRasterNoDataValue(hBand, C.double(t.NoData))
			varNameC := C.CString(t.NameSpace)
			gerr = C.GDALSetMetadataItem(C.GDALMajorObjectH(hBand), resNameSpaceC, varNameC, nil)
			C.free(unsafe.Pointer(varNameC))
			if gerr != 0 {
				break
			}

			gerr = C.GDALRasterIO(hBand, C.GF_Write, C.int(xOff), C.int(yOff), C.int(t.Width), C.int(t.Height), unsafe.Pointer(&t.Data[0]), C.int(t.Width), C.int(t.Height), C.GDT_Byte, 0, 0)

		case *Int16Raster:
			bandNames[i] = t.NameSpace
			if isEmptyTile(t.NameSpace) {
				continue
			}
			C.GDALSetRasterNoDataValue(hBand, C.double(t.NoData))
			varNameC := C.CString(t.NameSpace)
			gerr = C.GDALSetMetadataItem(C.GDALMajorObjectH(hBand), resNameSpaceC, varNameC, nil)
			C.free(unsafe.Pointer(varNameC))
			if gerr != 0 {
				break
			}

			gerr = C.GDALRasterIO(hBand, C.GF_Write, C.int(xOff), C.int(yOff), C.int(t.Width), C.int(t.Height), unsafe.Pointer(&t.Data[0]), C.int(t.Width), C.int(t.Height), C.GDT_Int16, 0, 0)

		case *UInt16Raster:
			bandNames[i] = t.NameSpace
			if isEmptyTile(t.NameSpace) {
				continue
			}
			C.GDALSetRasterNoDataValue(hBand, C.double(t.NoData))
			varNameC := C.CString(t.NameSpace)
			gerr = C.GDALSetMetadataItem(C.GDALMajorObjectH(hBand), resNameSpaceC, varNameC, nil)
			C.free(unsafe.Pointer(varNameC))
			if gerr != 0 {
				break
			}

			gerr = C.GDALRasterIO(hBand, C.GF_Write, C.int(xOff), C.int(yOff), C.int(t.Width), C.int(t.Height), unsafe.Pointer(&t.Data[0]), C.int(t.Width), C.int(t.Height), C.GDT_UInt16, 0, 0)

		case *Float32Raster:
			bandNames[i] = t.NameSpace
			if isEmptyTile(t.NameSpace) {
				continue
			}
			C.GDALSetRasterNoDataValue(hBand, C.double(t.NoData))
			varNameC := C.CString(t.NameSpace)
			gerr = C.GDALSetMetadataItem(C.GDALMajorObjectH(hBand), resNameSpaceC, varNameC, nil)
			C.free(unsafe.Pointer(varNameC))
			if gerr != 0 {
				break
			}

			gerr = C.GDALRasterIO(hBand, C.GF_Write, C.int(xOff), C.int(yOff), C.int(t.Width), C.int(t.Height), unsafe.Pointer(&t.Data[0]), C.int(t.Width), C.int(t.Height), C.GDT_Float32, 0, 0)

		default:
			C.GDALClose(hDstDS)
			return []string{}, fmt.Errorf("Unsupported gdal data type")
		}

		if gerr != 0 {
			C.GDALClose(hDstDS)
			return []string{}, fmt.Errorf("Error writing raster band: %d, xOff: %d, yOff:%d", i, xOff, yOff)
		}
	}

	return bandNames, nil

}

func EncodeGdalMerge(ctx context.Context, hDstDS C.GDALDatasetH, format string, workerTempFileName string, widthList []int, heightList []int, xOffList []int, yOffList []int) error {
	driverName, err := GetDriverNameFromFormat(format)
	if err != nil {
		return err
	}

	tempFileC := C.CString(workerTempFileName)
	defer C.free(unsafe.Pointer(tempFileC))

	driverList := []*C.char{C.CString(driverName)}
	defer C.free(unsafe.Pointer(driverList[0]))

	hSrcDS := C.GDALOpenEx(tempFileC, C.GDAL_OF_READONLY, &driverList[0], nil, nil)
	if hSrcDS == nil {
		return fmt.Errorf("Failed to reopen existing dataset: %v", workerTempFileName)
	}
	defer C.GDALClose(hSrcDS)

	nBands := int(C.GDALGetRasterCount(hDstDS))
	for ib := 0; ib < nBands; ib++ {
		hSrcBand := C.GDALGetRasterBand(hSrcDS, C.int(ib+1))
		hDstBand := C.GDALGetRasterBand(hDstDS, C.int(ib+1))
		dataType := C.GDALGetRasterDataType(hSrcBand)
		dataSize := int(C.GDALGetDataTypeSizeBytes(dataType))
		if dataSize == 0 {
			return fmt.Errorf("GDAL data type not implemented")
		}

		iBgn := 0
		for iBgn < len(xOffList) {
			xOff := xOffList[iBgn]
			yOff := yOffList[iBgn]
			width := widthList[iBgn]
			height := heightList[iBgn]

			iOff := iBgn + 1
			for ; iOff < iBgn+32 && iOff < len(xOffList); iOff++ {
				if heightList[iOff] != height {
					break
				}

				if yOffList[iOff] != yOff {
					break
				}

				width += widthList[iOff]
			}

			select {
			case <-ctx.Done():
				return fmt.Errorf("EncodeGdalMerge context canceled: %v", ctx.Err())
			default:
				dataBuf := make([]uint8, dataSize*width*height)
				gerr := C.GDALRasterIO(hSrcBand, C.GF_Read, C.int(xOff), C.int(yOff), C.int(width), C.int(height), unsafe.Pointer(&dataBuf[0]), C.int(width), C.int(height), dataType, 0, 0)
				if gerr != 0 {
					return fmt.Errorf("Error reading raster band: %d, xOff: %d, yOff:%d", ib, xOff, yOff)
				}

				gerr = C.GDALRasterIO(hDstBand, C.GF_Write, C.int(xOff), C.int(yOff), C.int(width), C.int(height), unsafe.Pointer(&dataBuf[0]), C.int(width), C.int(height), dataType, 0, 0)
				if gerr != 0 {
					return fmt.Errorf("Error writing raster band: %d, xOff: %d, yOff:%d", ib, xOff, yOff)
				}

				iBgn = iOff
			}
		}

	}

	return nil
}

func EncodeGdalFlush(hDstDS C.GDALDatasetH) {
	C.GDALFlushCache(hDstDS)
}

func EncodeGdalClose(hDstDS *C.GDALDatasetH) {
	if hDstDS != nil && *hDstDS != nil {
		C.GDALClose(*hDstDS)
	}
}

func RemoveGdalTempFile(tempFile string) {
	os.Remove(tempFile)
}

// ExtractEPSGCode parses an SRS string and gets
// the EPSG code
func ExtractEPSGCode(srs string) (int, error) {
	return strconv.Atoi(srs[5:])
}

func isEmptyTile(namespace string) bool {
	return len(namespace) >= len(EmptyTileNS) && namespace[:len(EmptyTileNS)] == EmptyTileNS
}
