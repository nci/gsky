package utils

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
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unsafe"
)

func EncodeDap4(w http.ResponseWriter, dataFile string, bandNames []string, verbose bool) error {
	w.Header().Set("Content-Type", "application/vnd.opendap.org.dap4.data")
	defer writeLastChunk(w)

	varNames, axisNames, axisVals, err := getDimensions(bandNames)
	if err != nil {
		handleError(w)
		return err
	}

	dataFileC := C.CString(dataFile)
	defer C.free(unsafe.Pointer(dataFileC))

	driverName := "GTiff"
	driverList := []*C.char{C.CString(driverName)}
	defer C.free(unsafe.Pointer(driverList[0]))

	hSrcDS := C.GDALOpenEx(dataFileC, C.GDAL_OF_READONLY, &driverList[0], nil, nil)
	if hSrcDS == nil {
		handleError(w)
		return fmt.Errorf("Failed to open data file: %v", dataFile)
	}
	defer C.GDALClose(hSrcDS)

	hBand := C.GDALGetRasterBand(hSrcDS, C.int(1))

	width := int(C.GDALGetRasterBandXSize(hBand))
	height := int(C.GDALGetRasterBandYSize(hBand))

	dataType := C.GDALGetRasterDataType(hBand)
	varDataType := getDataType(dataType)
	if len(varDataType) == 0 {
		handleError(w)
		return fmt.Errorf("unknown gdal data type: %v", int(dataType))
	}

	mdrBytes, err := buildMdr(axisNames, axisVals, varNames, varDataType, width, height)
	if err != nil {
		handleError(w)
		return err
	}

	mdrStr := string(mdrBytes)
	if verbose {
		log.Printf("DAP MDR:\n%s", mdrStr)
	}
	mdrStr = strings.Replace(mdrStr, "\n", "", -1)
	mdrBytes = []byte(mdrStr)

	err = writeChunk(w, mdrBytes)
	if err != nil {
		handleError(w)
		return err
	}

	for _, ns := range axisNames {
		axisValsBytes := floatArrToBytes(axisVals[ns])
		err = writeChunk(w, axisValsBytes)
		if err != nil {
			handleError(w)
			return err
		}
	}

	if len(varNames) == 0 {
		return nil
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

	nBands := int(C.GDALGetRasterCount(hSrcDS))
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
				handleError(w)
				return fmt.Errorf("Error reading raster band: %d, xOff: %d, yOff:%d", ib, xOff, yOff)
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
					handleError(w)
					return err
				}
			}
		}

		if verbose {
			progress := nBands / 10
			if progress < 1 {
				progress = 1
			}
			if ib%progress == 0 {
				log.Printf("DAP: %d of %d bands done", ib+1, nBands)
			}
		}
	}

	return nil
}

func buildMdr(axisNames []string, axisVals map[string][]float64, varNames []string, varDataType string, varWidth int, varHeight int) ([]byte, error) {
	mdrTpl := `<Dataset name="D"
  dapVersion="4.0" 
  dmrVersion="1.0" 
  xml:base="file:dap4/test_ce_1.xml"
  xmlns="http://xml.opendap.org/ns/DAP/4.0#"
  xmlns:dap="http://xml.opendap.org/ns/DAP/4.0#">
<Attribute name="_DAP4_Little_Endian" type="UInt8"><Value value="1"/></Attribute>
{{ range $index, $value := .Axes }}
<Dimension name="{{ .Name }}" size="{{ .Size }}"/>
{{ end }}
{{ $length := len .VarNames }} {{ if ne $length 0 }}
<Dimension name="y" size="{{ .VarHeight }}"/>
<Dimension name="x" size="{{ .VarWidth }}"/>
{{ end }}
{{ range $index, $value := .Axes }}
<Float64 name="{{ .Name }}">
<Dim name="{{ .Name }}"/>
</Float64>
{{ end }}
{{ with $ds := . }}
{{ range $index, $value := .VarNames }}
<{{ $ds.VarDataType }} name="{{ $value }}">
{{ range $idx, $val := $ds.Axes }}
<Dim name="{{ $val.Name }}"/>
{{ end }}
<Dim name="y"/>
<Dim name="x"/>
</{{ $ds.VarDataType }}>
{{ end }}
{{ end }}
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
		VarNames    []string
		VarHeight   int
		VarWidth    int
	}

	dsInfo := &DatasetInfo{VarDataType: varDataType, VarNames: varNames, VarHeight: varHeight, VarWidth: varWidth}
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

func getDimensions(dims []string) ([]string, []string, map[string][]float64, error) {
	varLookup := make(map[string]bool)
	varNames := make([]string, 0)

	valsLookup := make(map[string]map[float64]bool)
	axisVals := make(map[string][]float64)
	axisNames := make([]string, 0)

	varNameRegex := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	iVar := 0
	for _, dim := range dims {
		parts := strings.Split(dim, "#")
		if len(parts) > 2 {
			return varNames, axisNames, axisVals, fmt.Errorf("invalid dim format: %v", dim)
		}

		varPart := parts[0]
		if _, found := varLookup[varPart]; !found {
			if varPart != EmptyTileNS {
				varLookup[varPart] = true
				varName := varPart
				if !varNameRegex.MatchString(varName) {
					iVar++
					varName = fmt.Sprintf("var%d", iVar)
				}
				varNames = append(varNames, varName)
			}
		}

		if len(parts) == 1 {
			continue
		}

		dimPart := parts[1]
		axes := strings.Split(dimPart, ",")
		for _, axis := range axes {
			kv := strings.Split(axis, "=")
			if len(kv) != 2 {
				return varNames, axisNames, axisVals, fmt.Errorf("invalid axis format: %v", dim)
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
				timeVal, tErr := time.Parse(ISOFormat, axisVal)
				if tErr != nil {
					return varNames, axisNames, axisVals, fmt.Errorf("unknown data type: %v", dim)
				}
				val = float64(timeVal.Unix())
			}

			if _, found := valsLookup[axisName][val]; !found {
				valsLookup[axisName][val] = true
				axisVals[axisName] = append(axisVals[axisName], val)
			}

		}
	}

	return varNames, axisNames, axisVals, nil
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

func handleError(w http.ResponseWriter) {
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
