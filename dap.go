package main

// #include "gdal.h"
// #cgo pkg-config: gdal
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unsafe"

	"github.com/nci/gsky/utils"
)

func dapHandler(w http.ResponseWriter, r *http.Request) {
	C.GDALAllRegister()
	log.Printf("dap url: %v", r.URL)

	dataFile := "/local/test_nd_tensor.tiff"
	varName := "var1"
	verbose := true
	writeDap(w, dataFile, varName, verbose)
}

func writeDap(w http.ResponseWriter, dataFile string, varName string, verbose bool) {
	dataFileC := C.CString(dataFile)
	defer C.free(unsafe.Pointer(dataFileC))

	driverName := "GTiff"
	driverList := []*C.char{C.CString(driverName)}
	defer C.free(unsafe.Pointer(driverList[0]))

	hSrcDS := C.GDALOpenEx(dataFileC, C.GDAL_OF_READONLY, &driverList[0], nil, nil)
	if hSrcDS == nil {
		handleError(w, fmt.Errorf("Failed to open data file: %v", dataFile))
		return
	}
	defer C.GDALClose(hSrcDS)

	nBands := int(C.GDALGetRasterCount(hSrcDS))

	nameSpaceC := C.CString("long_name")
	defer C.free(unsafe.Pointer(nameSpaceC))

	var dimsList []string
	for ib := 0; ib < nBands; ib++ {
		hBand := C.GDALGetRasterBand(hSrcDS, C.int(ib+1))
		dimStrC := C.GDALGetMetadataItem(C.GDALMajorObjectH(hBand), nameSpaceC, nil)
		if dimStrC == nil {
			dimsList = []string{}
			break
		}

		dimStr := C.GoString(dimStrC)
		dimsList = append(dimsList, dimStr)
	}

	axisNames, axisVals, err := getDimensions(dimsList)
	if err != nil {
		handleError(w, err)
		return
	}

	hBand := C.GDALGetRasterBand(hSrcDS, C.int(1))

	width := int(C.GDALGetRasterBandXSize(hBand))
	height := int(C.GDALGetRasterBandYSize(hBand))

	dataType := C.GDALGetRasterDataType(hBand)
	varDataType := getDataType(dataType)
	if len(varDataType) == 0 {
		handleError(w, fmt.Errorf("unknown gdal data type: %v", int(dataType)))
		return
	}

	mdrBytes, err := buildMdr(axisNames, axisVals, varName, varDataType, width, height)
	if err != nil {
		handleError(w, err)
		return
	}

	mdrStr := string(mdrBytes)
	if verbose {
		log.Printf("DAP MDR:\n%s", mdrStr)
	}
	mdrStr = strings.Replace(mdrStr, "\n\n", "", -1)
	mdrBytes = []byte(mdrStr)

	w.Header().Set("Content-Type", "application/vnd.opendap.org.dap4.data")
	defer writeLastChunk(w)

	writeChunk(w, mdrBytes)
	for _, ns := range axisNames {
		axisValsBytes := floatArrToBytes(axisVals[ns])
		writeChunk(w, axisValsBytes)
	}

	dataSize := int(C.GDALGetDataTypeSizeBytes(dataType))

	var blockXSizeC, blockYSizeC C.int
	C.GDALGetBlockSize(hBand, &blockXSizeC, &blockYSizeC)
	blockYSize := int(blockYSizeC)

	maxChunkSize := int(float64(0xffffff)/float64(dataSize)) * dataSize
	nYBlocks := int(float64(maxChunkSize) / float64(dataSize*width*blockYSize))
	if nYBlocks < 1 {
		nYBlocks = 1
	}
	blockYSize *= nYBlocks

	for ib := 0; ib < nBands; ib++ {
		hBand := C.GDALGetRasterBand(hSrcDS, C.int(ib+1))
		xOff := 0

		for yOff := 0; yOff < height; yOff += blockYSize {
			ySize := blockYSize
			if yOff+blockYSize > height {
				ySize -= yOff + blockYSize - height
			}
			//log.Printf("%d: %d, %d, %d    %d, %d", ib, xOff, yOff, ySize, height, width)
			dataBuf := make([]uint8, dataSize*width*ySize)
			gerr := C.GDALRasterIO(hBand, C.GF_Read, C.int(xOff), C.int(yOff), C.int(width), C.int(ySize), unsafe.Pointer(&dataBuf[0]), C.int(width), C.int(ySize), dataType, 0, 0)
			if gerr != 0 {
				handleError(w, fmt.Errorf("Error reading raster band: %d, xOff: %d, yOff:%d", ib, xOff, yOff))
				return
			}

			nWords := maxChunkSize
			if nWords > len(dataBuf) {
				nWords = len(dataBuf)
			}

			for iw := 0; iw < len(dataBuf); iw += nWords {
				bufBgn := iw
				bufEnd := iw + nWords
				if bufEnd > len(dataBuf) {
					bufEnd = len(dataBuf)
				}
				//log.Printf("    %d, %d, %d", bufBgn, bufEnd, nWords)
				err := writeChunk(w, dataBuf[bufBgn:bufEnd])
				if err != nil {
					handleError(w, err)
					return
				}
			}
		}

		if verbose {
			progress := nBands / 10
			if progress < 1 {
				progress = 1
			}
			if ib%progress == 0 {
				log.Printf("DAP: %d of %d bands done", ib, nBands)
			}
		}
	}

	return
}

func buildMdr(axisNames []string, axisVals map[string][]float64, varName string, varDataType string, varWidth int, varHeight int) ([]byte, error) {
	mdrTpl := `<Dataset name="{{ .VarName }}"
  dapVersion="4.0" 
  dmrVersion="1.0" 
  xml:base="file:dap4/test_ce_1.xml"
  xmlns="http://xml.opendap.org/ns/DAP/4.0#"
  xmlns:dap="http://xml.opendap.org/ns/DAP/4.0#">
<Attribute name="_DAP4_Little_Endian" type="UInt8"><Value value="1"/></Attribute>
{{ range $index, $value := .Axes }}
<Dimension name="dim_{{ .Name }}" size="{{ .Size }}"/>
{{ end }}
{{ range $index, $value := .Axes }}
<Float64 name="{{ .Name }}">
<Dim name="dim_{{ .Name }}"/>
</Float64>
{{ end }}
<{{ .VarDataType }} name="{{ .VarName }}">
{{ range $index, $value := .Axes }}
<Dim name="dim_{{ .Name }}"/>
{{ end }}
<Dim size="{{ .VarHeight }}"/>
<Dim size="{{ .VarWidth }}"/>
</{{ .VarDataType }}>
</Dataset>`

	tpl, err := template.New("template").Parse(mdrTpl)
	if err != nil {
		return []byte{}, fmt.Errorf("Error trying to parse template document: %v", err)
	}

	type AxisInfo struct {
		Name string
		Size int
	}

	type DatasetInfo struct {
		Axes        []*AxisInfo
		VarDataType string
		VarName     string
		VarHeight   int
		VarWidth    int
	}

	dsInfo := &DatasetInfo{VarDataType: varDataType, VarName: varName, VarHeight: varHeight, VarWidth: varWidth}
	dsInfo.Axes = make([]*AxisInfo, len(axisNames))
	for i, ns := range axisNames {
		dsInfo.Axes[i] = &AxisInfo{Name: ns, Size: len(axisVals[ns])}
	}

	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, dsInfo)
	if err != nil {
		return []byte{}, fmt.Errorf("Error executing template: %v\n", err)
	}

	return buf.Bytes(), nil
}

func floatArrToBytes(arr []float64) []byte {
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&arr))
	header.Len *= 8
	header.Cap *= 8
	data := *(*[]byte)(unsafe.Pointer(&header))
	return data
}

func getDimensions(dims []string) ([]string, map[string][]float64, error) {
	valsLookup := make(map[string]map[float64]bool)
	axisVals := make(map[string][]float64)
	axisNames := make([]string, 0)

	for i, dim := range dims {
		parts := strings.Split(dim, "#")
		if len(parts) != 2 {
			if i == 0 {
				return []string{}, make(map[string][]float64), nil
			} else {
				return axisNames, axisVals, fmt.Errorf("invalid dim format: %v", dim)
			}
		}

		dimPart := parts[1]
		axes := strings.Split(dimPart, ",")
		for _, axis := range axes {
			kv := strings.Split(axis, "=")
			if len(kv) != 2 {
				return axisNames, axisVals, fmt.Errorf("invalid axis format: %v", dim)
			}

			axisName := kv[0]
			if _, found := valsLookup[axisName]; !found {
				valsLookup[axisName] = make(map[float64]bool)
				axisVals[axisName] = make([]float64, 0)
				axisNames = append(axisNames, axisName)
			}
			axisVal := kv[1]

			val, err := strconv.ParseFloat(axisVal, 64)
			if err != nil {
				timeVal, tErr := time.Parse(utils.ISOFormat, axisVal)
				if tErr != nil {
					return axisNames, axisVals, fmt.Errorf("unknown data type: %v", dim)
				}
				val = float64(timeVal.Unix())
			}

			if _, found := valsLookup[axisName][val]; !found {
				valsLookup[axisName][val] = true
				axisVals[axisName] = append(axisVals[axisName], val)
			}

		}
	}

	return axisNames, axisVals, nil
}

func writeChunk(w http.ResponseWriter, data []byte) error {
	if len(data) > 0xffffff {
		return fmt.Errorf("exceeding maximum chunk size")
	}

	chunkSize := len(data)

	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(chunkSize))

	//https://github.com/Unidata/netcdf-c/blob/b16eceabe27ef334d4384eae07387cbac18d2afc/libdap4/d4chunk.c
	//#define LAST_CHUNK          (1)
	//#define ERR_CHUNK           (2)
	//#define LITTLE_ENDIAN_CHUNK (4)
	//#define NOCHECKSUM_CHUNK    (8)

	hdr[0] = byte(8 | 4)
	_, err := w.Write(hdr)
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}

func writeLastChunk(w http.ResponseWriter) {
	lastChunk := []byte{1, 0, 0, 0}
	w.Write(lastChunk)
}

func handleError(w http.ResponseWriter, err error) {
	log.Printf("DAP: error: %v", err)
	writeErrChunk(w)
}

func writeErrChunk(w http.ResponseWriter) {
	errChunk := []byte{2, 0, 0, 0}
	w.Write(errChunk)
}

func getDataType(gdalDataType C.GDALDataType) string {
	var GDALTypes = map[C.GDALDataType]string{1: "Byte", 2: "UInt16", 3: "Int16",
		4: "UInt32", 5: "Int32", 6: "Float32", 7: "Float64"}

	dt, found := GDALTypes[gdalDataType]
	if !found {
		return ""
	}

	return dt
}
