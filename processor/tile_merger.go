package processor

import (
	"github.com/nci/gsky/utils"
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
	"strconv"
	"time"
	"unsafe"
)

const SIZE_OF_UINT16 = 2
const SIZE_OF_INT16 = 2
const SIZE_OF_FLOAT32 = 4

type RasterMerger struct {
	In    chan *FlexRaster
	Out   chan Raster
	Error chan error
}

func NewRasterMerger(errChan chan error) *RasterMerger {
	return &RasterMerger{
		In:    make(chan *FlexRaster, 100),
		Out:   make(chan Raster, 100),
		Error: errChan,
	}
}

func MergeMaskedRaster(r *FlexRaster, canvasMap map[string]*FlexRaster, mask []bool) (err error) {
	switch r.Type {
	case "Byte":
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&canvasMap[r.NameSpace].Data))
		canvas := *(*[]uint8)(unsafe.Pointer(&headr))

		header := *(*reflect.SliceHeader)(unsafe.Pointer(&r.Data))
		data := *(*[]uint8)(unsafe.Pointer(&header))
		nodata := uint8(r.NoData)

		// Aren't we looping in ordered dates?
		if r.TimeStamp.Before(canvasMap[r.NameSpace].TimeStamp) {
			for i, val := range data {
				if data[i] != nodata && !mask[i] && canvas[i] == nodata {
					canvas[i] = val
				}
			}
		} else {
			for i, val := range data {
				if val != nodata && !mask[i] {
					canvas[i] = val
				}
			}
			canvasMap[r.NameSpace].TimeStamp = r.TimeStamp
		}
	case "Int16":
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&canvasMap[r.NameSpace].Data))
		headr.Len /= SIZE_OF_INT16
		headr.Cap /= SIZE_OF_INT16
		canvas := *(*[]int16)(unsafe.Pointer(&headr))

		header := *(*reflect.SliceHeader)(unsafe.Pointer(&r.Data))
		header.Len /= SIZE_OF_INT16
		header.Cap /= SIZE_OF_INT16
		data := *(*[]int16)(unsafe.Pointer(&header))
		nodata := int16(r.NoData)

		if r.TimeStamp.Before(canvasMap[r.NameSpace].TimeStamp) {
			for i, val := range data {
				if data[i] != nodata && !mask[i] && canvas[i] == nodata {
					canvas[i] = val
				}
			}
		} else {
			for i, val := range data {
				if val != nodata && !mask[i] {
					canvas[i] = val
				}
			}
			canvasMap[r.NameSpace].TimeStamp = r.TimeStamp
		}
	case "UInt16":
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&canvasMap[r.NameSpace].Data))
		headr.Len /= SIZE_OF_UINT16
		headr.Cap /= SIZE_OF_UINT16
		canvas := *(*[]uint16)(unsafe.Pointer(&headr))

		header := *(*reflect.SliceHeader)(unsafe.Pointer(&r.Data))
		header.Len /= SIZE_OF_UINT16
		header.Cap /= SIZE_OF_UINT16
		data := *(*[]uint16)(unsafe.Pointer(&header))
		nodata := uint16(r.NoData)

		if r.TimeStamp.Before(canvasMap[r.NameSpace].TimeStamp) {
			for i, val := range data {
				if data[i] != nodata && !mask[i] && canvas[i] == nodata {
					canvas[i] = val
				}
			}
		} else {
			for i, val := range data {
				if val != nodata && !mask[i] {
					canvas[i] = val
				}
			}
			canvasMap[r.NameSpace].TimeStamp = r.TimeStamp
		}
	case "Float32":
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&canvasMap[r.NameSpace].Data))
		headr.Len /= SIZE_OF_FLOAT32
		headr.Cap /= SIZE_OF_FLOAT32
		canvas := *(*[]float32)(unsafe.Pointer(&headr))

		header := *(*reflect.SliceHeader)(unsafe.Pointer(&r.Data))
		header.Len /= SIZE_OF_FLOAT32
		header.Cap /= SIZE_OF_FLOAT32
		data := *(*[]float32)(unsafe.Pointer(&header))
		nodata := float32(r.NoData)

		if r.TimeStamp.Before(canvasMap[r.NameSpace].TimeStamp) {
			for i, val := range data {
				if data[i] != nodata && !mask[i] && canvas[i] == nodata {
					canvas[i] = val
				}
			}
		} else {
			for i, val := range data {
				if val != nodata && !mask[i] {
					canvas[i] = val
				}
			}
			canvasMap[r.NameSpace].TimeStamp = r.TimeStamp
		}
	default:
		err = fmt.Errorf("MergeMaskedRaster hasn't been implemented for Raster type %s", r.Type)
	}
	return
}

func initNoDataSlice(rType string, noDataValue float64, size int) []uint8 {
	switch rType {
	case "Byte":
		out := make([]uint8, size)
		fill := uint8(noDataValue)
		for i := 0; i < size; i++ {
			out[i] = fill
		}
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&out))
		return *(*[]uint8)(unsafe.Pointer(&headr))
	case "Int16":
		out := make([]int16, size)
		fill := int16(noDataValue)
		for i := 0; i < size; i++ {
			out[i] = fill
		}
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&out))
		headr.Len *= SIZE_OF_INT16
		headr.Cap *= SIZE_OF_INT16
		return *(*[]uint8)(unsafe.Pointer(&headr))
	case "UInt16":
		out := make([]uint16, size)
		fill := uint16(noDataValue)
		for i := 0; i < size; i++ {
			out[i] = fill
		}
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&out))
		headr.Len *= SIZE_OF_UINT16
		headr.Cap *= SIZE_OF_UINT16
		return *(*[]uint8)(unsafe.Pointer(&headr))
	case "Float32":
		out := make([]float32, size)
		fill := float32(noDataValue)
		for i := 0; i < size; i++ {
			out[i] = fill
		}
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&out))
		headr.Len *= SIZE_OF_FLOAT32
		headr.Cap *= SIZE_OF_FLOAT32
		return *(*[]uint8)(unsafe.Pointer(&headr))
	default:
		return []uint8{}
	}

}

func ProcessRasterStack(rasterStack map[int64][]*FlexRaster, maskMap map[int64][]bool) (canvasMap map[string]*FlexRaster, err error) {
	canvasMap = map[string]*FlexRaster{}

	var keys []int64
	for k := range rasterStack {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	for _, geoStamp := range keys {
		for _, r := range rasterStack[geoStamp] {
			if _, ok := canvasMap[r.NameSpace]; !ok {
				// Raster namespace doesn't have a canvas yet
				canvasMap[r.NameSpace] = &FlexRaster{TimeStamp: time.Time{}, ConfigPayLoad: r.ConfigPayLoad,
					NoData: r.NoData, Data: initNoDataSlice(r.Type, r.NoData, r.Width*r.Height),
					Height: r.Height, Width: r.Width, OffX: r.OffX, OffY: r.OffY,
					Type: r.Type, NameSpace: r.NameSpace}
			}
			if mask, ok := maskMap[geoStamp]; ok {
				err = MergeMaskedRaster(r, canvasMap, mask)
			} else {
				err = MergeMaskedRaster(r, canvasMap, make([]bool, r.Height*r.Width))
			}
		}
		delete(rasterStack, geoStamp)
	}
	return
}

func ComputeMask(mask *utils.Mask, data []byte, rType string) (out []bool, err error) {
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&data))

	switch rType {
	case "Byte":
		data := *(*[]uint8)(unsafe.Pointer(&header))
		out = make([]bool, len(data))
		maskValue64, _ := strconv.ParseUint(mask.Value, 2, 8)
		maskValue := uint8(maskValue64)
		for i, val := range data {
			if (val & maskValue) > 0 {
				out[i] = true
			}
		}
	case "Int16":
		header.Len /= SIZE_OF_INT16
		header.Cap /= SIZE_OF_INT16
		data := *(*[]int16)(unsafe.Pointer(&header))
		out = make([]bool, len(data))
		maskValue64, _ := strconv.ParseInt(mask.Value, 2, 16)
		maskValue := int16(maskValue64)
		for i, val := range data {
			if (val & maskValue) > 0 {
				out[i] = true
			}
		}
	case "UInt16":
		header.Len /= SIZE_OF_UINT16
		header.Cap /= SIZE_OF_UINT16
		data := *(*[]uint16)(unsafe.Pointer(&header))
		out = make([]bool, len(data))
		maskValue64, _ := strconv.ParseUint(mask.Value, 2, 16)
		maskValue := uint16(maskValue64)
		for i, val := range data {
			if (val & maskValue) > 0 {
				out[i] = true
			}
		}
	default:
		err = fmt.Errorf("Type %s cannot contain a bit mask", rType)

	}
	return
}

func (enc *RasterMerger) Run() {
	defer close(enc.Out)
	maskMap := map[int64][]bool{}
	rasterStack := map[int64][]*FlexRaster{}

	for r := range enc.In {
		h := fnv.New32a()
		h.Write([]byte(r.Polygon))
		geoStamp := r.TimeStamp.UnixNano() + int64(h.Sum32())

		// Raster namespace is identified as Mask
		if r.Mask != nil && r.Mask.Id == r.NameSpace {
			mask, err := ComputeMask(r.Mask, r.Data, r.Type)
			if err != nil {
				enc.Error <- err
				return
			}
			maskMap[geoStamp] = mask
			continue
		}

		rasterStack[geoStamp] = append(rasterStack[geoStamp], r)
	}

	canvasMap, err := ProcessRasterStack(rasterStack, maskMap)
	if err != nil {
		enc.Error <- err
		return
	}

	if len(canvasMap) == 2 && canvasMap["Nadir_Reflectance_Band1"].Type == "Int16" {
		headerB1 := *(*reflect.SliceHeader)(unsafe.Pointer(&canvasMap["Nadir_Reflectance_Band1"].Data))
		headerB1.Len /= SIZE_OF_INT16
		headerB1.Cap /= SIZE_OF_INT16
		DataB1 := *(*[]int16)(unsafe.Pointer(&headerB1))

		headerB2 := *(*reflect.SliceHeader)(unsafe.Pointer(&canvasMap["Nadir_Reflectance_Band2"].Data))
		headerB2.Len /= SIZE_OF_INT16
		headerB2.Cap /= SIZE_OF_INT16
		DataB2 := *(*[]int16)(unsafe.Pointer(&headerB2))

		nodata := float32(canvasMap["Nadir_Reflectance_Band1"].NoData)
		data := make([]float32, len(DataB1))

		for i := range data {
			v1 := float32(DataB1[i])
			v2 := float32(DataB2[i])
			if v1 != nodata && v2 != nodata {
				if (v2-v1)/(v2+v1) <= 0 {
					data[i] = 0.005
				} else {
					data[i] = (v2 - v1) / (v2 + v1)
				}
			}
		}

		dataBytesHdr := *(*reflect.SliceHeader)(unsafe.Pointer(&data))
		dataBytesHdr.Len *= 4
		dataBytesHdr.Cap *= 4
		DataBytes := *(*[]uint8)(unsafe.Pointer(&dataBytesHdr))

		canvas := canvasMap["Nadir_Reflectance_Band1"]
		config := ConfigPayLoad{[]string{"NDVI"}, canvas.ScaleParams, canvas.Palette, canvas.Mask, canvas.ZoomLimit}
		canvasMap["NDVI"] = &FlexRaster{ConfigPayLoad: config, NoData: 0, Data: DataBytes, Type: "Float32",
			Height: canvas.Height, Width: canvas.Width, OffX: canvas.OffX, OffY: canvas.OffY,
			NameSpace: "NDVI"}
		delete(canvasMap, "Nadir_Reflectance_Band1")
		delete(canvasMap, "Nadir_Reflectance_Band2")
	}

	//start := time.Now()

	for _, canvas := range canvasMap {
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&canvas.Data))
		switch canvas.Type {
		case "Byte":
			enc.Out <- &ByteRaster{ConfigPayLoad: canvas.ConfigPayLoad, NoData: canvas.NoData, Data: canvas.Data,
				Height: canvas.Height, Width: canvas.Width, OffX: canvas.OffX, OffY: canvas.OffY,
				NameSpace: canvas.NameSpace}
		case "UInt16":
			headr.Len /= SIZE_OF_UINT16
			headr.Cap /= SIZE_OF_UINT16
			data := *(*[]uint16)(unsafe.Pointer(&headr))
			enc.Out <- &UInt16Raster{ConfigPayLoad: canvas.ConfigPayLoad, NoData: canvas.NoData, Data: data,
				Height: canvas.Height, Width: canvas.Width, OffX: canvas.OffX, OffY: canvas.OffY,
				NameSpace: canvas.NameSpace}
		case "Int16":
			headr.Len /= SIZE_OF_INT16
			headr.Cap /= SIZE_OF_INT16
			data := *(*[]int16)(unsafe.Pointer(&headr))
			enc.Out <- &Int16Raster{ConfigPayLoad: canvas.ConfigPayLoad, NoData: canvas.NoData, Data: data,
				Height: canvas.Height, Width: canvas.Width, OffX: canvas.OffX, OffY: canvas.OffY,
				NameSpace: canvas.NameSpace}
		case "Float32":
			headr.Len /= SIZE_OF_FLOAT32
			headr.Cap /= SIZE_OF_FLOAT32
			data := *(*[]float32)(unsafe.Pointer(&headr))
			enc.Out <- &Float32Raster{ConfigPayLoad: canvas.ConfigPayLoad, NoData: canvas.NoData, Data: data,
				Height: canvas.Height, Width: canvas.Width, OffX: canvas.OffX, OffY: canvas.OffY,
				NameSpace: canvas.NameSpace}
		default:
			enc.Error <- fmt.Errorf("FlexRaster type %s not recognised", canvas.Type)
		}
	}
	//fmt.Println("Merger Time", time.Since(start))
}
