package gdalprocess

/*
#cgo pkg-config: gdal

#include "warper.hxx"
#include "gdal.h"
*/
import "C"

/*
// This is a reference implementation of warp.
// We leave this code here for debugging and comparsion purposes.
int warp_operation(GDALDatasetH hSrcDS, GDALDatasetH hDstDS, int band)
{
	const char *srcProjRef;
	int err;
	GDALWarpOptions *psWOptions;

	psWOptions = GDALCreateWarpOptions();
	psWOptions->nBandCount = 1;
	psWOptions->panSrcBands = (int *) CPLMalloc(sizeof(int) * 1);
	psWOptions->panSrcBands[0] = band;
	psWOptions->panDstBands = (int *) CPLMalloc(sizeof(int) * 1);
	psWOptions->panDstBands[0] = 1;

	srcProjRef = GDALGetProjectionRef(hSrcDS);
	if(strlen(srcProjRef) == 0) {
		srcProjRef = "GEOGCS[\"WGS 84\",DATUM[\"WGS_1984\",SPHEROID[\"WGS 84\",6378137,298.257223563,AUTHORITY[\"EPSG\",\"7030\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6326\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9108\"]],AUTHORITY[\"EPSG\",\"4326\"]]\",\"proj4\":\"+proj=longlat +ellps=WGS84 +towgs84=0,0,0,0,0,0,0 +no_defs \"";
	}

	err = GDALReprojectImage(hSrcDS, srcProjRef, hDstDS, GDALGetProjectionRef(hDstDS), GRA_NearestNeighbour, 0.0, 0.0, NULL, NULL, psWOptions);
	GDALDestroyWarpOptions(psWOptions);

	return err;
}
*/

import (
	"fmt"
	"log"
	"math"
	"reflect"
	"syscall"
	"unsafe"

	pb "github.com/nci/gsky/worker/gdalservice"
)

const SizeofUint16 = 2
const SizeofInt16 = 2
const SizeofFloat32 = 4

var GDALTypes = map[C.GDALDataType]string{0: "Unkown", 1: "Byte", 2: "UInt16", 3: "Int16",
	4: "UInt32", 5: "Int32", 6: "Float32", 7: "Float64",
	8: "CInt16", 9: "CInt32", 10: "CFloat32", 11: "CFloat64",
	12: "TypeCount"}

func ComputeReprojectExtent(in *pb.GeoRPCGranule) *pb.Result {
	srcFileC := C.CString(in.Path)
	defer C.free(unsafe.Pointer(srcFileC))

	hSrcDS := C.GDALOpenEx(srcFileC, C.GDAL_OF_READONLY|C.GDAL_OF_VERBOSE_ERROR, nil, nil, nil)
	if hSrcDS == nil {
		return &pb.Result{Error: fmt.Sprintf("Failed to open existing dataset: %v", in.Path)}
	}
	defer C.GDALClose(hSrcDS)

	dstProjRefC := C.CString(in.DstSRS)
	defer C.free(unsafe.Pointer(dstProjRefC))

	hTransformArg := C.GDALCreateGenImgProjTransformer(hSrcDS, nil, nil, dstProjRefC, C.int(0), C.double(0), C.int(0))
	if hTransformArg == nil {
		return &pb.Result{Error: fmt.Sprintf("GDALCreateGenImgProjTransformer() failed")}
	}
	defer C.GDALDestroyGenImgProjTransformer(hTransformArg)

	psInfo := (*C.GDALTransformerInfo)(hTransformArg)

	var padfGeoTransformOut [6]C.double
	var pnPixels, pnLines C.int
	gerr := C.GDALSuggestedWarpOutput(hSrcDS, psInfo.pfnTransform, hTransformArg, &padfGeoTransformOut[0], &pnPixels, &pnLines)
	if gerr != 0 {
		return &pb.Result{Error: fmt.Sprintf("GDALSuggestedWarpOutput() failed")}
	}

	xRes := float64(padfGeoTransformOut[1])
	yRes := float64(math.Abs(float64(padfGeoTransformOut[5])))

	xMin := in.DstGeot[0]
	yMin := in.DstGeot[1]
	xMax := in.DstGeot[2]
	yMax := in.DstGeot[3]

	nPixels := int((xMax - xMin + xRes/2.0) / xRes)
	nLines := int((yMax - yMin + yRes/2.0) / yRes)

	out := make([]int, 2)
	out[0] = nPixels
	out[1] = nLines

	header := *(*reflect.SliceHeader)(unsafe.Pointer(&out))
	intSize := int(unsafe.Sizeof(int(0)))
	header.Len *= intSize
	header.Cap *= intSize
	dBytes := *(*[]uint8)(unsafe.Pointer(&header))

	dBytesCopy := make([]uint8, len(dBytes))
	for i := 0; i < len(dBytes); i++ {
		dBytesCopy[i] = dBytes[i]
	}
	return &pb.Result{Raster: &pb.Raster{Data: dBytesCopy, NoData: 0, RasterType: "Int"}, Error: "OK"}
}

func WarpRaster(in *pb.GeoRPCGranule) *pb.Result {
	filePathC := C.CString(in.Path)
	defer C.free(unsafe.Pointer(filePathC))

	var dstProjRefC *C.char
	if len(in.DstSRS) > 0 {
		dstProjRefC = C.CString(in.DstSRS)
		defer C.free(unsafe.Pointer(dstProjRefC))
	} else {
		dstProjRefC = nil
	}

	dump := func(msg interface{}) string {
		log.Println(
			"warp", in.Path,
			"band", in.Bands[0],
			"width", in.Width,
			"height", in.Height,
			"geotransform", in.DstGeot,
			"srs", in.DstSRS,
			"error", msg,
		)
		return fmt.Sprintf("%v", msg)
	}

	var geoLocOpts []*C.char
	var pGeoLoc **C.char
	if len(in.GeoLocOpts) > 0 {
		for _, opt := range in.GeoLocOpts {
			geoLocOpts = append(geoLocOpts, C.CString(opt))
		}

		for _, opt := range geoLocOpts {
			defer C.free(unsafe.Pointer(opt))
		}
		geoLocOpts = append(geoLocOpts, nil)

		pGeoLoc = &geoLocOpts[0]
	} else {
		pGeoLoc = nil
	}

	var srcProjRefC *C.char
	if len(in.SrcSRS) > 0 {
		srcProjRefC = C.CString(in.SrcSRS)
		defer C.free(unsafe.Pointer(srcProjRefC))
	} else {
		srcProjRefC = nil
	}

	var pSrcGeot *C.double
	if len(in.SrcGeot) > 0 {
		pSrcGeot = (*C.double)(&in.SrcGeot[0])
	} else {
		pSrcGeot = nil
	}

	var dstBboxC [4]C.int
	var dstBufSize C.int
	var dstBufC unsafe.Pointer
	var noData float64
	var dType C.GDALDataType
	var bytesReadC C.size_t

	var resUsage0, resUsage1 syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &resUsage0)
	cErr := C.warp_operation_fast(filePathC, srcProjRefC, pSrcGeot, pGeoLoc, dstProjRefC, (*C.double)(&in.DstGeot[0]), C.int(in.Width), C.int(in.Height), C.int(in.Bands[0]), C.int(in.SRSCf), (*unsafe.Pointer)(&dstBufC), (*C.int)(&dstBufSize), (*C.int)(&dstBboxC[0]), (*C.double)(&noData), (*C.GDALDataType)(&dType), &bytesReadC)
	syscall.Getrusage(syscall.RUSAGE_SELF, &resUsage1)

	metrics := &pb.WorkerMetrics{
		BytesRead: int64(bytesReadC),
		UserTime:  resUsage1.Utime.Nano() - resUsage0.Utime.Nano(),
		SysTime:   resUsage1.Stime.Nano() - resUsage0.Stime.Nano(),
	}

	if cErr != 0 {
		return &pb.Result{Error: dump(fmt.Sprintf("warp_operation() fail: %v", int(cErr))), Metrics: metrics}
	}

	dstBbox := make([]int32, len(dstBboxC))
	for i, v := range dstBboxC {
		dstBbox[i] = int32(v)
	}

	bboxCanvas := C.GoBytes(dstBufC, dstBufSize)
	C.free(dstBufC)

	var rasterType string
	if C.int(dType) == 100 {
		rasterType = "SignedByte"
	} else {
		rasterType = GDALTypes[dType]
	}

	return &pb.Result{Raster: &pb.Raster{Data: bboxCanvas, NoData: noData, RasterType: rasterType, Bbox: dstBbox}, Error: "OK", Metrics: metrics}
}
