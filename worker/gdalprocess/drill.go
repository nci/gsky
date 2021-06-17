package gdalprocess

// #include "gdal.h"
// #include "gdal_alg.h"
// #include "ogr_api.h"
// #include "ogr_srs_api.h"
// #include "cpl_string.h"
// #cgo pkg-config: gdal
import "C"

import (
	"fmt"
	"log"
	"math"
	"sort"
	"syscall"
	"unsafe"

	"encoding/json"

	geo "github.com/nci/geometry"
	pb "github.com/nci/gsky/worker/gdalservice"
)

type DrillFileDescriptor struct {
	SrcBBox []int32
	DstBBox []int32
	Mask    []uint8
}

var cWGS84WKT = C.CString(`GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9108"]],AUTHORITY["EPSG","4326"]]","proj4":"+proj=longlat +ellps=WGS84 +towgs84=0,0,0,0,0,0,0 +no_defs `)

func DrillDataset(in *pb.GeoRPCGranule) *pb.Result {

	var feat geo.Feature
	err := json.Unmarshal([]byte(in.Geometry), &feat)
	if err != nil {
		msg := fmt.Sprintf("Problem unmarshalling geometry %v", in)
		log.Println(msg)
		return &pb.Result{Error: msg}
	}
	geomGeoJSON, err := json.Marshal(feat.Geometry)
	if err != nil {
		msg := fmt.Sprintf("Problem marshaling GeoJSON geometry: %v", err)
		log.Println(msg)
		return &pb.Result{Error: msg}
	}

	if len(in.VRT) > 0 {
		vrtMgr, err := NewVRTManager([]byte(in.VRT))
		if err != nil {
			msg := fmt.Sprintf("VRT Manager error: %v", err)
			log.Printf(msg)
			return &pb.Result{Error: msg}
		}
		in.Path = vrtMgr.DSFileName

		defer vrtMgr.Close()
	}

	cPath := C.CString(in.Path)
	defer C.free(unsafe.Pointer(cPath))
	ds := C.GDALOpen(cPath, C.GDAL_OF_READONLY)
	if ds == nil {
		msg := fmt.Sprintf("GDAL could not open dataset: %s", in.Path)
		log.Println(msg)
		return &pb.Result{Error: msg}
	}
	defer C.GDALClose(ds)

	cGeom := C.CString(string(geomGeoJSON))
	defer C.free(unsafe.Pointer(cGeom))
	geom := C.OGR_G_CreateGeometryFromJson(cGeom)
	if geom == nil {
		msg := fmt.Sprintf("Geometry %s could not be parsed", in.Geometry)
		log.Println(msg)
		return &pb.Result{Error: msg}
	}

	selSRS := C.OSRNewSpatialReference(cWGS84WKT)
	defer C.OSRDestroySpatialReference(selSRS)

	C.OGR_G_AssignSpatialReference(geom, selSRS)

	res := readData(ds, float64(in.Width), float64(in.Height), in.Bands, geom, int(in.BandStrides), int(in.DrillDecileCount), int(in.PixelCount), in.ClipUpper, in.ClipLower)
	C.OGR_G_DestroyGeometry(geom)
	return res
}

func readData(ds C.GDALDatasetH, rasterXSize float64, rasterYSize float64, bands []int32, geom C.OGRGeometryH, bandStrides int, decileCount int, pixelCount int, clipUpper float32, clipLower float32) *pb.Result {
	nCols := 1 + decileCount

	avgs := []*pb.TimeSeries{}

	dsDscr, err := getDrillFileDescriptor(ds, geom, rasterXSize, rasterYSize)
	if err != nil {
		return &pb.Result{Error: err.Error()}
	}

	// it is safe to assume all data bands have same data type and nodata value
	bandH := C.GDALGetRasterBand(ds, C.int(1))
	dType := C.GDALGetRasterDataType(bandH)

	dSize := C.GDALGetDataTypeSizeBytes(dType)
	if dSize == 0 {
		err := fmt.Errorf("GDAL data type not implemented")
		return &pb.Result{Error: err.Error()}
	}

	if bandStrides <= 0 {
		bandStrides = 1
	}

	nodata := float32(C.GDALGetRasterNoDataValue(bandH, nil))
	metrics := &pb.WorkerMetrics{}

	var resUsage0, resUsage1 syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &resUsage0)

	// If we have a lot of bands, one may want to seek an approximate algorithm
	// to speed up the computation especially the RasterIO operation.
	// The approximate algorithm implemented here is linear interpolation between
	// the points in between the range with size specified by bandStrides.
	// For example, if bandStrides is 3. We then proceed as follows:
	// 1) Load band 1 and compute average for band 1 (i.e. avg1)
	// 2) Load band 3 and compute average for band 3 (i.e. avg3)
	// 3) Linearly interpolate avg2 using avg1 and avg3
	for ibBgn := 0; ibBgn < len(bands); ibBgn += bandStrides {
		ibEnd := ibBgn + bandStrides
		if ibEnd > len(bands) {
			ibEnd = len(bands)
		}

		bandsRead := []int32{bands[ibBgn], bands[ibEnd-1]}
		if bandStrides == 1 {
			bandsRead = bandsRead[:1]
		}

		effectiveNBands := len(bandsRead)

		dataBuf := make([]float32, dsDscr.DstBBox[2]*dsDscr.DstBBox[3]*int32(effectiveNBands))
		C.GDALDatasetRasterIO(ds, C.GF_Read, C.int(dsDscr.SrcBBox[0]), C.int(dsDscr.SrcBBox[1]), C.int(dsDscr.SrcBBox[2]), C.int(dsDscr.SrcBBox[3]), unsafe.Pointer(&dataBuf[0]), C.int(dsDscr.DstBBox[2]), C.int(dsDscr.DstBBox[3]), C.GDT_Float32, C.int(effectiveNBands), (*C.int)(unsafe.Pointer(&bandsRead[0])), 0, 0, 0)
		metrics.BytesRead += int64(len(dataBuf)) * int64(dSize)

		boundAvgs := make([]*pb.TimeSeries, effectiveNBands*nCols)
		bandSize := int(dsDscr.DstBBox[2] * dsDscr.DstBBox[3])
		for iBand := 0; iBand < effectiveNBands; iBand++ {
			bandOffset := iBand * bandSize

			sum := float32(0)
			total := int32(0)

			for i := 0; i < bandSize; i++ {
				if dsDscr.Mask[i] == 255 && dataBuf[i+bandOffset] != nodata {
					val := dataBuf[i+bandOffset]
					if pixelCount != 0 {
						total++
					}

					if val < clipLower || val > clipUpper {
						continue
					}
					if pixelCount == 0 {
						sum += val
						total++
					} else {
						sum += 1.0
					}
				}
			}

			iRes := iBand * nCols
			if total > 0 {
				boundAvgs[iRes] = &pb.TimeSeries{Value: float64(sum / float32(total)), Count: total}
			} else {
				boundAvgs[iRes] = &pb.TimeSeries{Value: 0, Count: 0}
			}

			if nCols > 1 {
				if total > 0 {
					deciles := computeDeciles(decileCount, dataBuf, bandSize, bandOffset, nodata, dsDscr)
					for ic := 0; ic < len(deciles); ic++ {
						iRes++
						boundAvgs[iRes] = &pb.TimeSeries{Value: float64(deciles[ic]), Count: 1}
					}
				} else {
					for ic := 0; ic < decileCount; ic++ {
						iRes++
						boundAvgs[iRes] = &pb.TimeSeries{Value: 0, Count: 0}
					}
				}
			}
		}

		avgs = append(avgs, boundAvgs[:nCols]...)

		if bandStrides > 2 && len(boundAvgs) > nCols {
			var beta []float64
			var count []float64
			for ic := 0; ic < nCols; ic++ {
				beta_ := (boundAvgs[ic+nCols].Value - boundAvgs[ic].Value) / float64(bandStrides-1)
				beta = append(beta, beta_)

				count_ := math.Round(float64(boundAvgs[ic].Count+boundAvgs[ic+nCols].Count) / float64(2))
				count = append(count, count_)
			}
			for ip := 1; ip < bandStrides-1; ip++ {
				for ic := 0; ic < nCols; ic++ {
					beta_ := beta[ic]
					val := boundAvgs[ic].Value + float64(ip)*beta_
					avgs = append(avgs, &pb.TimeSeries{Value: val, Count: int32(count[ic])})
				}
			}
		}

		if len(boundAvgs) > nCols {
			avgs = append(avgs, boundAvgs[len(boundAvgs)-nCols:]...)
		}

	}
	syscall.Getrusage(syscall.RUSAGE_SELF, &resUsage1)
	metrics.UserTime = resUsage1.Utime.Nano() - resUsage0.Utime.Nano()
	metrics.SysTime = resUsage1.Stime.Nano() - resUsage0.Stime.Nano()

	nRows := len(avgs) / nCols
	return &pb.Result{TimeSeries: avgs, Raster: &pb.Raster{NoData: float64(nodata)}, Shape: []int32{int32(nRows), int32(nCols)}, Error: "OK", Metrics: metrics}
}

func computeDeciles(decileCount int, dataBuf []float32, bandSize int, bandOffset int, nodata float32, dsDscr *DrillFileDescriptor) []float32 {
	deciles := make([]float32, decileCount)

	var buf []float32
	for i := 0; i < bandSize; i++ {
		if dsDscr.Mask[i] == 255 && dataBuf[i+bandOffset] != nodata {
			buf = append(buf, dataBuf[i+bandOffset])
		}
	}

	sort.Slice(buf, func(i, j int) bool { return buf[i] <= buf[j] })
	step := len(buf) / (decileCount + 1)
	if step > 0 {
		isEven := len(buf)%(decileCount+1) == 0

		for i := 0; i < decileCount; i++ {
			iStep := (i + 1) * step
			de := buf[iStep]
			if isEven {
				de = (buf[iStep] + buf[iStep+1]) / 2.0
			}

			deciles[i] = de
		}
	} else {
		padding := make(map[int]int)
		for i := 0; i < decileCount; i++ {
			idx := i % len(buf)
			if _, found := padding[idx]; !found {
				padding[idx] = 0
			}
			padding[idx]++
		}

		idx := 0
		for i := 0; i < len(buf); i++ {
			for p := 0; p < padding[i]; p++ {
				deciles[idx] = buf[i]
				idx++
			}
		}
	}

	return deciles
}

func createMask(ds C.GDALDatasetH, g C.OGRGeometryH, geoTrans []float64, bbox []int32) ([]uint8, error) {
	canvas := make([]uint8, bbox[2]*bbox[3])

	memStr := fmt.Sprintf("MEM:::DATAPOINTER=%d,PIXELS=%d,LINES=%d,DATATYPE=Byte", unsafe.Pointer(&canvas[0]), bbox[2], bbox[3])
	memStrC := C.CString(memStr)
	defer C.free(unsafe.Pointer(memStrC))
	hDstDS := C.GDALOpen(memStrC, C.GA_Update)
	if hDstDS == nil {
		return nil, fmt.Errorf("Couldn't create memory driver")
	}
	defer C.GDALClose(hDstDS)

	var gdalErr C.CPLErr
	if gdalErr = C.GDALSetProjection(hDstDS, C.GDALGetProjectionRef(ds)); gdalErr != 0 {
		return nil, fmt.Errorf("Couldn't set a projection in the mem raster %v", gdalErr)
	}

	if gdalErr = C.GDALSetGeoTransform(hDstDS, (*C.double)(&geoTrans[0])); gdalErr != 0 {
		return nil, fmt.Errorf("Couldn't set the geotransform on the destination dataset %v", gdalErr)
	}

	ic := C.OGR_G_Clone(g)
	defer C.OGR_G_DestroyGeometry(ic)

	geomBurnValue := C.double(255)
	panBandList := []C.int{C.int(1)}
	pahGeomList := []C.OGRGeometryH{ic}

	opts := []*C.char{C.CString("ALL_TOUCHED=TRUE"), nil}
	defer C.free(unsafe.Pointer(opts[0]))

	if gdalErr = C.GDALRasterizeGeometries(hDstDS, 1, &panBandList[0], 1, &pahGeomList[0], nil, nil, &geomBurnValue, &opts[0], nil, nil); gdalErr != 0 {
		return nil, fmt.Errorf("GDALRasterizeGeometry error %v", gdalErr)
	}

	return canvas, nil
}

func envelopePolygon(hDS C.GDALDatasetH, xSize float64, ySize float64) (C.OGRGeometryH, error) {
	geoTrans := make([]float64, 6)
	C.GDALGetGeoTransform(hDS, (*C.double)(&geoTrans[0]))

	var ulX, ulY C.double
	C.GDALApplyGeoTransform((*C.double)(&geoTrans[0]), C.double(0), C.double(0), &ulX, &ulY)
	var lrX, lrY C.double
	C.GDALApplyGeoTransform((*C.double)(&geoTrans[0]), C.double(xSize), C.double(ySize), &lrX, &lrY)

	polyWKT := fmt.Sprintf("POLYGON ((%f %f,%f %f,%f %f,%f %f,%f %f))", ulX, ulY,
		ulX, lrY,
		lrX, lrY,
		lrX, ulY,
		ulX, ulY)

	ppszData := C.CString(polyWKT)
	ppszDataTmp := ppszData

	var hGeom C.OGRGeometryH
	hSRS := C.OSRNewSpatialReference(C.GDALGetProjectionRef(hDS))

	// OGR_G_CreateFromWkt intrnally updates &ppszData pointer value
	errC := C.OGR_G_CreateFromWkt(&ppszData, hSRS, &hGeom)

	C.OSRRelease(hSRS)
	C.free(unsafe.Pointer(ppszDataTmp))

	if errC != C.OGRERR_NONE {
		return nil, fmt.Errorf("failed to compute envelope polygon: %v", polyWKT)
	}

	return hGeom, nil
}

func getDrillFileDescriptor(ds C.GDALDatasetH, g C.OGRGeometryH, rasterXSize float64, rasterYSize float64) (*DrillFileDescriptor, error) {
	gCopy := C.OGR_G_Buffer(g, C.double(0.0), C.int(30))
	if C.OGR_G_IsEmpty(gCopy) == C.int(1) {
		gCopy = C.OGR_G_Clone(g)
	}

	defer C.OGR_G_DestroyGeometry(gCopy)

	if C.GoString(C.GDALGetProjectionRef(ds)) != "" {
		desSRS := C.OSRNewSpatialReference(C.GDALGetProjectionRef(ds))
		defer C.OSRDestroySpatialReference(desSRS)
		srcSRS := C.OSRNewSpatialReference(cWGS84WKT)
		defer C.OSRDestroySpatialReference(srcSRS)
		C.OSRSetAxisMappingStrategy(srcSRS, C.OAMS_TRADITIONAL_GIS_ORDER)
		C.OSRSetAxisMappingStrategy(desSRS, C.OAMS_TRADITIONAL_GIS_ORDER)
		trans := C.OCTNewCoordinateTransformation(srcSRS, desSRS)
		C.OGR_G_Transform(gCopy, trans)
		C.OCTDestroyCoordinateTransformation(trans)
	}

	xSize := float64(C.GDALGetRasterXSize(ds))
	ySize := float64(C.GDALGetRasterYSize(ds))

	fileEnv, err := envelopePolygon(ds, xSize, ySize)
	if err != nil {
		return nil, err
	}
	defer C.OGR_G_DestroyGeometry(fileEnv)

	inters := C.OGR_G_Intersection(gCopy, fileEnv)
	defer C.OGR_G_DestroyGeometry(inters)

	var env C.OGREnvelope
	C.OGR_G_GetEnvelope(inters, &env)

	geot := make([]float64, 6)
	gdalErr := C.GDALGetGeoTransform(ds, (*C.double)(&geot[0]))
	if gdalErr != 0 {
		return nil, fmt.Errorf("Couldn't get the geotransform from the source dataset %v", gdalErr)
	}

	srcBBox, err := getPixelLineBBox(geot, &env)
	if err != nil {
		return nil, err
	}

	if srcBBox[0] >= int32(xSize) {
		srcBBox[0] = int32(xSize) - 1
	}
	if srcBBox[2]+srcBBox[0] >= int32(xSize) {
		srcBBox[2] = int32(xSize) - srcBBox[0]
	}

	if srcBBox[1] >= int32(ySize) {
		srcBBox[1] = int32(ySize) - 1
	}
	if srcBBox[3]+srcBBox[1] >= int32(ySize) {
		srcBBox[3] = int32(ySize) - srcBBox[1]
	}

	if srcBBox[2] <= 3 && srcBBox[3] <= 3 {
		numPixels := int(srcBBox[2] * srcBBox[3])
		mask := make([]uint8, numPixels)
		for i := 0; i < numPixels; i++ {
			mask[i] = 255
		}
		return &DrillFileDescriptor{srcBBox, srcBBox, mask}, nil
	}

	dstGeot := make([]float64, len(geot))
	copy(dstGeot, geot)

	dstGeot[0] += dstGeot[1] * float64(srcBBox[0])
	dstGeot[3] += dstGeot[5] * float64(srcBBox[1])

	if srcBBox[2] <= 3 || srcBBox[3] <= 3 {
		rasterXSize = 0
		rasterYSize = 0
	}

	hasNewGeot := false
	if (rasterXSize > 0 && rasterXSize <= 1) || (rasterYSize > 0 && rasterYSize <= 1) {
		dstXSize := xSize
		dstYSize := ySize

		if rasterXSize > 0 && rasterXSize <= 1 {
			dstXSize = float64(int(xSize*rasterXSize + 0.5))
		}

		if rasterYSize > 0 && rasterYSize <= 1 {
			dstYSize = float64(int(ySize*rasterYSize + 0.5))
		}

		if rasterXSize <= 0 && rasterYSize > 0 {
			dstXSize = float64(int(float64(dstYSize)*xSize/ySize + 0.5))
		} else if rasterXSize > 0 && rasterYSize <= 0 {
			dstYSize = float64(int(float64(dstXSize)*ySize/xSize + 0.5))
		}

		if dstXSize < 1 {
			dstXSize = 1
		}

		if dstYSize < 1 {
			dstYSize = 1
		}

		dstGeot[1] *= xSize / dstXSize
		dstGeot[5] *= ySize / dstYSize

		hasNewGeot = true
	} else if rasterXSize > 1 || rasterYSize > 1 {
		var dstEnv C.OGREnvelope
		C.OGR_G_GetEnvelope(gCopy, &dstEnv)

		envMinX := float64(dstEnv.MinX)
		envMaxX := float64(dstEnv.MaxX)
		envMinY := float64(dstEnv.MinY)
		envMaxY := float64(dstEnv.MaxY)

		var dstXSize, dstYSize float64
		if rasterXSize > 1 && rasterYSize > 1 {
			dstXSize = rasterXSize
			dstYSize = rasterYSize
		} else {
			var dstSize float64
			if rasterXSize > 1 {
				dstSize = rasterXSize
			} else {
				dstSize = rasterYSize
			}

			if xSize >= ySize {
				dstXSize = dstSize
				rasterXSize = dstSize
				rasterYSize = 0
			} else {
				dstYSize = dstSize
				rasterXSize = 0
				rasterYSize = dstSize
			}
		}

		if rasterXSize > 1 && rasterYSize <= 0 {
			dstYSize = rasterXSize * ySize / xSize
		}

		if rasterYSize > 1 && rasterXSize <= 0 {
			dstXSize = rasterYSize * xSize / ySize
		}

		if dstXSize < 1 {
			dstXSize = 1
		}

		if dstYSize < 1 {
			dstYSize = 1
		}

		xRes := (envMaxX - envMinX) / dstXSize
		if xRes > math.Abs(geot[1]) {
			dstGeot[1] = math.Copysign(xRes, geot[1])
			hasNewGeot = true
		}

		yRes := (envMaxY - envMinY) / dstYSize
		if yRes > math.Abs(geot[5]) {
			dstGeot[5] = math.Copysign(yRes, geot[5])
			hasNewGeot = true
		}

	} else if rasterXSize > 0 || rasterYSize > 0 {
		return nil, fmt.Errorf("unsupported raster size: %v, %v", rasterXSize, rasterYSize)
	}

	var dstBBox []int32
	if !hasNewGeot {
		dstBBox = srcBBox
	} else {
		dstBBox, err = getPixelLineBBox(dstGeot, &env)
		if err != nil {
			dstBBox = srcBBox
		}
	}

	//log.Printf("xSize:%v, ySize:%v, srcBBox:%v, dstBBox:%v, srcGeot:%v, dstGeot:%v",
	//  xSize, ySize, srcBBox, dstBBox, geot, dstGeot)
	mask, err := createMask(ds, gCopy, dstGeot, dstBBox)
	return &DrillFileDescriptor{srcBBox, dstBBox, mask}, err
}

func getPixelLineBBox(geot []float64, env *C.OGREnvelope) ([]int32, error) {
	invGeot := make([]float64, 6)
	gdalErr := C.GDALInvGeoTransform((*C.double)(&geot[0]), (*C.double)(&invGeot[0]))
	if gdalErr == C.int(0) {
		return nil, fmt.Errorf("invert geo transform failed")
	}

	var offMinXC, offMinYC, offMaxXC, offMaxYC C.double
	C.GDALApplyGeoTransform((*C.double)(&invGeot[0]), env.MinX, env.MinY, &offMinXC, &offMinYC)
	C.GDALApplyGeoTransform((*C.double)(&invGeot[0]), env.MaxX, env.MaxY, &offMaxXC, &offMaxYC)

	offMinX := math.Min(float64(offMinXC), float64(offMaxXC))
	offMaxX := math.Max(float64(offMinXC), float64(offMaxXC))
	offMinY := math.Min(float64(offMinYC), float64(offMaxYC))
	offMaxY := math.Max(float64(offMinYC), float64(offMaxYC))

	offsetX := int32(offMinX + 0.5)
	offsetY := int32(offMinY + 0.5)
	countX := int32(offMaxX - offMinX + 0.5)
	countY := int32(offMaxY - offMinY + 0.5)

	if countX == 0 {
		countX++
	}
	if countY == 0 {
		countY++
	}
	if offsetX < 0 {
		offsetX = 0
	}
	if offsetY < 0 {
		offsetY = 0
	}

	return []int32{offsetX, offsetY, countX, countY}, nil
}
