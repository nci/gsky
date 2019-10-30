package processor

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/nci/gsky/utils"
)

const SizeofUint16 = 2
const SizeofInt16 = 2
const SizeofFloat32 = 4

type RasterMerger struct {
	Context context.Context
	In      chan []*FlexRaster
	Out     chan []utils.Raster
	Error   chan error
}

func NewRasterMerger(ctx context.Context, errChan chan error) *RasterMerger {
	return &RasterMerger{
		Context: ctx,
		In:      make(chan []*FlexRaster, 100),
		Out:     make(chan []utils.Raster, 100),
		Error:   errChan,
	}
}

func MergeMaskedRaster(r *FlexRaster, canvasMap map[string]*FlexRaster, mask []bool) (err error) {
	switch r.Type {
	case "SignedByte":
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&canvasMap[r.NameSpace].Data))
		canvas := *(*[]int8)(unsafe.Pointer(&headr))

		header := *(*reflect.SliceHeader)(unsafe.Pointer(&r.Data))
		data := *(*[]int8)(unsafe.Pointer(&header))
		nodata := int8(r.NoData)
		if r.TimeStamp < canvasMap[r.NameSpace].TimeStamp {
			iSrc := 0
			for ir := 0; ir < r.DataHeight; ir++ {
				for ic := 0; ic < r.DataWidth; ic++ {
					val := data[iSrc]
					iDst := (ir+r.OffY)*r.Width + ic + r.OffX
					if val != nodata && !mask[iSrc] && canvas[iDst] == nodata {
						canvas[iDst] = val
					}
					iSrc++
				}
			}
		} else {
			iSrc := 0
			for ir := 0; ir < r.DataHeight; ir++ {
				for ic := 0; ic < r.DataWidth; ic++ {
					val := data[iSrc]
					if val != nodata && !mask[iSrc] {
						iDst := (ir+r.OffY)*r.Width + ic + r.OffX
						canvas[iDst] = val
					}
					iSrc++
				}
			}
			canvasMap[r.NameSpace].TimeStamp = r.TimeStamp
		}

	case "Byte":
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&canvasMap[r.NameSpace].Data))
		canvas := *(*[]uint8)(unsafe.Pointer(&headr))

		header := *(*reflect.SliceHeader)(unsafe.Pointer(&r.Data))
		data := *(*[]uint8)(unsafe.Pointer(&header))
		nodata := uint8(r.NoData)
		if r.TimeStamp < canvasMap[r.NameSpace].TimeStamp {
			iSrc := 0
			for ir := 0; ir < r.DataHeight; ir++ {
				for ic := 0; ic < r.DataWidth; ic++ {
					val := data[iSrc]
					iDst := (ir+r.OffY)*r.Width + ic + r.OffX
					if val != nodata && !mask[iSrc] && canvas[iDst] == nodata {
						canvas[iDst] = val
					}
					iSrc++
				}
			}
		} else {
			iSrc := 0
			for ir := 0; ir < r.DataHeight; ir++ {
				for ic := 0; ic < r.DataWidth; ic++ {
					val := data[iSrc]
					if val != nodata && !mask[iSrc] {
						iDst := (ir+r.OffY)*r.Width + ic + r.OffX
						canvas[iDst] = val
					}
					iSrc++
				}
			}
			canvasMap[r.NameSpace].TimeStamp = r.TimeStamp
		}
	case "Int16":
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&canvasMap[r.NameSpace].Data))
		headr.Len /= SizeofInt16
		headr.Cap /= SizeofInt16
		canvas := *(*[]int16)(unsafe.Pointer(&headr))

		header := *(*reflect.SliceHeader)(unsafe.Pointer(&r.Data))
		header.Len /= SizeofInt16
		header.Cap /= SizeofInt16
		data := *(*[]int16)(unsafe.Pointer(&header))
		nodata := int16(r.NoData)

		if r.TimeStamp < canvasMap[r.NameSpace].TimeStamp {
			iSrc := 0
			for ir := 0; ir < r.DataHeight; ir++ {
				for ic := 0; ic < r.DataWidth; ic++ {
					val := data[iSrc]
					iDst := (ir+r.OffY)*r.Width + ic + r.OffX
					if val != nodata && !mask[iSrc] && canvas[iDst] == nodata {
						canvas[iDst] = val
					}
					iSrc++
				}
			}
		} else {
			iSrc := 0
			for ir := 0; ir < r.DataHeight; ir++ {
				for ic := 0; ic < r.DataWidth; ic++ {
					val := data[iSrc]
					if val != nodata && !mask[iSrc] {
						iDst := (ir+r.OffY)*r.Width + ic + r.OffX
						canvas[iDst] = val
					}
					iSrc++
				}
			}
			canvasMap[r.NameSpace].TimeStamp = r.TimeStamp
		}
	case "UInt16":
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&canvasMap[r.NameSpace].Data))
		headr.Len /= SizeofUint16
		headr.Cap /= SizeofUint16
		canvas := *(*[]uint16)(unsafe.Pointer(&headr))

		header := *(*reflect.SliceHeader)(unsafe.Pointer(&r.Data))
		header.Len /= SizeofUint16
		header.Cap /= SizeofUint16
		data := *(*[]uint16)(unsafe.Pointer(&header))
		nodata := uint16(r.NoData)

		if r.TimeStamp < canvasMap[r.NameSpace].TimeStamp {
			iSrc := 0
			for ir := 0; ir < r.DataHeight; ir++ {
				for ic := 0; ic < r.DataWidth; ic++ {
					val := data[iSrc]
					iDst := (ir+r.OffY)*r.Width + ic + r.OffX
					if val != nodata && !mask[iSrc] && canvas[iDst] == nodata {
						canvas[iDst] = val
					}
					iSrc++
				}
			}
		} else {
			iSrc := 0
			for ir := 0; ir < r.DataHeight; ir++ {
				for ic := 0; ic < r.DataWidth; ic++ {
					val := data[iSrc]
					if val != nodata && !mask[iSrc] {
						iDst := (ir+r.OffY)*r.Width + ic + r.OffX
						canvas[iDst] = val
					}
					iSrc++
				}
			}
			canvasMap[r.NameSpace].TimeStamp = r.TimeStamp
		}
	case "Float32":
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&canvasMap[r.NameSpace].Data))
		headr.Len /= SizeofFloat32
		headr.Cap /= SizeofFloat32
		canvas := *(*[]float32)(unsafe.Pointer(&headr))

		header := *(*reflect.SliceHeader)(unsafe.Pointer(&r.Data))
		header.Len /= SizeofFloat32
		header.Cap /= SizeofFloat32
		data := *(*[]float32)(unsafe.Pointer(&header))
		nodata := float32(r.NoData)

		if r.TimeStamp < canvasMap[r.NameSpace].TimeStamp {
			iSrc := 0
			for ir := 0; ir < r.DataHeight; ir++ {
				for ic := 0; ic < r.DataWidth; ic++ {
					val := data[iSrc]
					iDst := (ir+r.OffY)*r.Width + ic + r.OffX
					if val != nodata && !mask[iSrc] && canvas[iDst] == nodata {
						canvas[iDst] = val
					}
					iSrc++
				}
			}
		} else {
			iSrc := 0
			for ir := 0; ir < r.DataHeight; ir++ {
				for ic := 0; ic < r.DataWidth; ic++ {
					val := data[iSrc]
					if val != nodata && !mask[iSrc] {
						iDst := (ir+r.OffY)*r.Width + ic + r.OffX
						canvas[iDst] = val
					}
					iSrc++
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
	case "SignedByte":
		out := make([]int8, size)
		fill := int8(noDataValue)
		for i := 0; i < size; i++ {
			out[i] = fill
		}
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&out))
		return *(*[]uint8)(unsafe.Pointer(&headr))
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
		headr.Len *= SizeofInt16
		headr.Cap *= SizeofInt16
		return *(*[]uint8)(unsafe.Pointer(&headr))
	case "UInt16":
		out := make([]uint16, size)
		fill := uint16(noDataValue)
		for i := 0; i < size; i++ {
			out[i] = fill
		}
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&out))
		headr.Len *= SizeofUint16
		headr.Cap *= SizeofUint16
		return *(*[]uint8)(unsafe.Pointer(&headr))
	case "Float32":
		out := make([]float32, size)
		fill := float32(noDataValue)
		for i := 0; i < size; i++ {
			out[i] = fill
		}
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&out))
		headr.Len *= SizeofFloat32
		headr.Cap *= SizeofFloat32
		return *(*[]uint8)(unsafe.Pointer(&headr))
	default:
		return []uint8{}
	}

}

func ProcessRasterStack(rasterStack map[float64][]*FlexRaster, maskMap map[float64][]bool, canvasMap map[string]*FlexRaster) (map[string]*FlexRaster, error) {
	var keys []float64
	for k := range rasterStack {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] > keys[j] })

	var err error
	for _, geoStamp := range keys {
		for _, r := range rasterStack[geoStamp] {
			if _, ok := canvasMap[r.NameSpace]; !ok {
				// Raster namespace doesn't have a canvas yet
				canvasMap[r.NameSpace] = &FlexRaster{TimeStamp: 0, ConfigPayLoad: r.ConfigPayLoad,
					NoData: r.NoData, Data: initNoDataSlice(r.Type, r.NoData, r.Width*r.Height),
					Height: r.Height, Width: r.Width, OffX: r.OffX, OffY: r.OffY,
					Type: r.Type, NameSpace: r.NameSpace}
			}
			if mask, ok := maskMap[geoStamp]; ok {
				err = MergeMaskedRaster(r, canvasMap, mask)
			} else {
				err = MergeMaskedRaster(r, canvasMap, make([]bool, r.Height*r.Width))
			}

			if err != nil {
				return canvasMap, err

			}
		}
		delete(rasterStack, geoStamp)
	}
	return canvasMap, nil
}

func ComputeMask(mask *utils.Mask, data []byte, rType string) (out []bool, err error) {
	if len(mask.Value) == 0 {
		if len(mask.BitTests) == 0 {
			err = fmt.Errorf("Please specify either mask.Value or mask.BitTests")
			return
		} else if len(mask.BitTests)%2 != 0 {
			err = fmt.Errorf("The entries in mask.BitTests must be in pairs")
			return
		}
	}

	header := *(*reflect.SliceHeader)(unsafe.Pointer(&data))

	switch rType {
	case "SignedByte":
		data := *(*[]int8)(unsafe.Pointer(&header))
		out = make([]bool, len(data))
		if len(mask.Value) > 0 {
			maskValue64, _ := strconv.ParseUint(mask.Value, 2, 8)
			maskValue := int8(maskValue64)
			for i, val := range data {
				if (val & maskValue) > 0 {
					out[i] = true
				}
			}
		} else {
			for i, val := range data {
				for j := 0; j < len(mask.BitTests); j += 2 {
					maskFilter64, _ := strconv.ParseInt(mask.BitTests[j], 2, 8)
					maskFilter := int8(maskFilter64)

					maskValue64, _ := strconv.ParseInt(mask.BitTests[j+1], 2, 8)
					maskValue := int8(maskValue64)

					if (val & maskFilter) == maskValue {
						out[i] = true
						break
					}
				}
			}
		}
	case "Byte":
		data := *(*[]uint8)(unsafe.Pointer(&header))
		out = make([]bool, len(data))
		if len(mask.Value) > 0 {
			maskValue64, _ := strconv.ParseUint(mask.Value, 2, 8)
			maskValue := uint8(maskValue64)
			for i, val := range data {
				if (val & maskValue) > 0 {
					out[i] = true
				}
			}
		} else {
			for i, val := range data {
				for j := 0; j < len(mask.BitTests); j += 2 {
					maskFilter64, _ := strconv.ParseInt(mask.BitTests[j], 2, 8)
					maskFilter := uint8(maskFilter64)

					maskValue64, _ := strconv.ParseInt(mask.BitTests[j+1], 2, 8)
					maskValue := uint8(maskValue64)

					if (val & maskFilter) == maskValue {
						out[i] = true
						break
					}
				}
			}
		}
	case "Int16":
		header.Len /= SizeofInt16
		header.Cap /= SizeofInt16
		data := *(*[]int16)(unsafe.Pointer(&header))
		out = make([]bool, len(data))
		if len(mask.Value) > 0 {
			maskValue64, _ := strconv.ParseInt(mask.Value, 2, 16)
			maskValue := int16(maskValue64)
			for i, val := range data {
				if (val & maskValue) > 0 {
					out[i] = true
				}
			}
		} else {
			for i, val := range data {
				for j := 0; j < len(mask.BitTests); j += 2 {
					maskFilter64, _ := strconv.ParseInt(mask.BitTests[j], 2, 16)
					maskFilter := int16(maskFilter64)

					maskValue64, _ := strconv.ParseInt(mask.BitTests[j+1], 2, 16)
					maskValue := int16(maskValue64)

					if (val & maskFilter) == maskValue {
						out[i] = true
						break
					}
				}
			}
		}
	case "UInt16":
		header.Len /= SizeofUint16
		header.Cap /= SizeofUint16
		data := *(*[]uint16)(unsafe.Pointer(&header))
		out = make([]bool, len(data))
		if len(mask.Value) > 0 {
			maskValue64, _ := strconv.ParseUint(mask.Value, 2, 16)
			maskValue := uint16(maskValue64)
			for i, val := range data {
				if (val & maskValue) > 0 {
					out[i] = true
				}
			}
		} else {
			for i, val := range data {
				for j := 0; j < len(mask.BitTests); j += 2 {
					maskFilter64, _ := strconv.ParseInt(mask.BitTests[j], 2, 16)
					maskFilter := uint16(maskFilter64)

					maskValue64, _ := strconv.ParseInt(mask.BitTests[j+1], 2, 16)
					maskValue := uint16(maskValue64)

					if (val & maskFilter) == maskValue {
						out[i] = true
						break
					}
				}
			}
		}
	default:
		err = fmt.Errorf("Type %s cannot contain a bit mask", rType)

	}
	return
}

func (enc *RasterMerger) Run(bandExpr *utils.BandExpressions, verbose bool) {
	if verbose {
		defer log.Printf("tile merger done")
	}
	defer close(enc.Out)

	canvasMap := map[string]*FlexRaster{}
	for inRasters := range enc.In {
		select {
		case <-enc.Context.Done():
			return
		default:
		}

		maskMap := map[float64][]bool{}
		rasterStack := map[float64][]*FlexRaster{}

		for _, r := range inRasters {
			if r == nil {
				continue
			}

			h := fnv.New32a()
			h.Write([]byte(r.Polygon))
			geoStamp := r.TimeStamp + float64(h.Sum32())

			// Raster namespace is identified as Mask
			if r.Mask != nil && r.Mask.ID == r.NameSpace {
				mask, err := ComputeMask(r.Mask, r.Data, r.Type)
				if err != nil {
					enc.sendError(err)
					return
				}
				maskMap[geoStamp] = mask
				if !r.Mask.Inclusive {
					continue
				}

			}

			rasterStack[geoStamp] = append(rasterStack[geoStamp], r)
		}

		if len(rasterStack) > 0 {
			tmpMap, err := ProcessRasterStack(rasterStack, maskMap, canvasMap)
			if err != nil {
				enc.sendError(err)
				return
			}
			canvasMap = tmpMap
		}
	}

	select {
	case <-enc.Context.Done():
		return
	default:
	}

	var nameSpaces []string
	if _, found := canvasMap[utils.EmptyTileNS]; found {
		nameSpaces = append(nameSpaces, utils.EmptyTileNS)
	} else {
		for _, canvas := range canvasMap {
			nameSpaces = canvas.ConfigPayLoad.NameSpaces
			break
		}
	}

	if len(nameSpaces) == 0 {
		enc.Out <- []utils.Raster{&utils.ByteRaster{Data: make([]uint8, 0), Height: 0, Width: 0}}
		return
	}

	hasExpr := (nameSpaces[0] != utils.EmptyTileNS) && (len(bandExpr.Expressions) > 0)

	nOut := len(nameSpaces)

	type Axis2BandVar struct {
		Name string
		Idx  int
	}

	axisNsLookup := make(map[string][]*Axis2BandVar)
	axisList := make([]string, 0)

	if hasExpr {
		for i, ns := range nameSpaces {
			parts := strings.Split(ns, "#")

			var varNs, axisNs string
			if len(parts) > 1 {
				varNs = parts[0]
				axisNs = parts[1]
			} else {
				varNs = ns
				axisNs = "singular"
			}

			_, found := axisNsLookup[axisNs]
			if !found {
				axisNsLookup[axisNs] = make([]*Axis2BandVar, 0)
				axisList = append(axisList, axisNs)
			}

			axisNsLookup[axisNs] = append(axisNsLookup[axisNs], &Axis2BandVar{Name: varNs, Idx: i})

		}

		nAxis := len(axisNsLookup)
		nOut = len(bandExpr.Expressions) * nAxis
	}

	out := make([]utils.Raster, nOut)
	bandVars := make([]*utils.Float32Raster, len(nameSpaces))

	for i, ns := range nameSpaces {
		canvas, found := canvasMap[ns]
		if !found {
			var knownNs []string
			for k, _ := range canvasMap {
				knownNs = append(knownNs, k)
			}
			enc.sendError(fmt.Errorf("unknown namespace: %v, valid namespaces: %v", ns, knownNs))
			return
		}
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&canvas.Data))
		switch canvas.Type {
		case "SignedByte":
			data := *(*[]int8)(unsafe.Pointer(&headr))
			if !hasExpr {
				out[i] = &utils.SignedByteRaster{NoData: canvas.NoData, Data: data,
					Width: canvas.Width, Height: canvas.Height, NameSpace: ns}
			} else {
				varData := make([]float32, len(data))
				for i, val := range data {
					varData[i] = float32(val)
				}
				bandVars[i] = &utils.Float32Raster{NoData: float64(canvas.NoData), Data: varData}
			}

		case "Byte":
			if !hasExpr {
				out[i] = &utils.ByteRaster{NoData: canvas.NoData, Data: canvas.Data,
					Width: canvas.Width, Height: canvas.Height, NameSpace: ns}
			} else {
				data := *(*[]uint8)(unsafe.Pointer(&headr))
				varData := make([]float32, len(data))
				for i, val := range data {
					varData[i] = float32(val)
				}
				bandVars[i] = &utils.Float32Raster{NoData: float64(canvas.NoData), Data: varData}
			}

		case "UInt16":
			headr.Len /= SizeofUint16
			headr.Cap /= SizeofUint16
			data := *(*[]uint16)(unsafe.Pointer(&headr))
			if !hasExpr {
				out[i] = &utils.UInt16Raster{NoData: canvas.NoData, Data: data,
					Width: canvas.Width, Height: canvas.Height, NameSpace: ns}
			} else {
				varData := make([]float32, len(data))
				for i, val := range data {
					varData[i] = float32(val)
				}
				bandVars[i] = &utils.Float32Raster{NoData: float64(canvas.NoData), Data: varData}
			}

		case "Int16":
			headr.Len /= SizeofInt16
			headr.Cap /= SizeofInt16
			data := *(*[]int16)(unsafe.Pointer(&headr))
			if !hasExpr {
				out[i] = &utils.Int16Raster{NoData: canvas.NoData, Data: data,
					Width: canvas.Width, Height: canvas.Height, NameSpace: ns}
			} else {
				varData := make([]float32, len(data))
				for i, val := range data {
					varData[i] = float32(val)
				}
				bandVars[i] = &utils.Float32Raster{NoData: float64(canvas.NoData), Data: varData}
			}

		case "Float32":
			headr.Len /= SizeofFloat32
			headr.Cap /= SizeofFloat32
			data := *(*[]float32)(unsafe.Pointer(&headr))
			if !hasExpr {
				out[i] = &utils.Float32Raster{NoData: canvas.NoData, Data: data,
					Width: canvas.Width, Height: canvas.Height, NameSpace: ns}
			} else {
				varData := make([]float32, len(data))
				for i, val := range data {
					varData[i] = val
				}
				bandVars[i] = &utils.Float32Raster{NoData: float64(canvas.NoData), Data: varData}
			}

		default:
			enc.sendError(fmt.Errorf("raster type %s not recognised", canvas.Type))
			return
		}
	}

	if hasExpr {
		width := canvasMap[nameSpaces[0]].Width
		height := canvasMap[nameSpaces[0]].Height
		noData := bandVars[0].NoData

		iOut := 0
		for iv := range bandExpr.Expressions {
			for _, axisNs := range axisList {
				axisVars := axisNsLookup[axisNs]

				noDataMasks := make([]bool, width*height)
				for i := 0; i < len(noDataMasks); i++ {
					noDataMasks[i] = true
				}

				parameters := make(map[string]interface{})
				for _, v := range axisVars {
					parameters[v.Name] = bandVars[v.Idx].Data

					for j := 0; j < len(noDataMasks); j++ {
						if float64(bandVars[v.Idx].Data[j]) == bandVars[v.Idx].NoData {
							noDataMasks[j] = false
						}
					}
					//log.Printf("   %v: %v, %v", axisNs, v.Name, v.Idx)
				}

				result, err := bandExpr.Expressions[iv].Evaluate(parameters)
				if err != nil {
					enc.sendError(fmt.Errorf("bandExpr '%v' error: %v", bandExpr.ExprText[iv], err))
					return
				}

				outNameSpace := bandExpr.ExprNames[iv]
				if axisNs != "singular" {
					outNameSpace += "#" + axisNs
				}
				//log.Printf(" %v, %v, %v, uuuu %v", iv, iOut, len(out), outNameSpace)

				outRaster := &utils.Float32Raster{NoData: noData, Data: make([]float32, len(noDataMasks)),
					Width: width, Height: height, NameSpace: outNameSpace}
				out[iOut] = outRaster
				iOut++

				resScal, isScal := result.(float32)
				if isScal {
					for i := range outRaster.Data {
						if noDataMasks[i] {
							outRaster.Data[i] = resScal
						} else {
							outRaster.Data[i] = float32(noData)
						}
					}
					continue
				}

				resArr, isArr := result.([]float32)
				if isArr {
					for i := range outRaster.Data {
						if noDataMasks[i] {
							if math.IsInf(float64(resArr[i]), 0) || math.IsNaN(float64(resArr[i])) {
								outRaster.Data[i] = float32(noData)
							} else {
								outRaster.Data[i] = resArr[i]
							}
						} else {
							outRaster.Data[i] = float32(noData)
						}
					}
					continue
				}

				enc.sendError(fmt.Errorf("unknown data type for returned value '%v' for expression '%v'", result, bandExpr.ExprText[iv]))
				return

			}
		}
	}

	enc.Out <- out
}

func (enc *RasterMerger) sendError(err error) {
	select {
	case enc.Error <- err:
	default:
	}
}
