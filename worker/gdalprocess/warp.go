package gdalprocess

/*
This is a fast implementation of the GDAL warp operation.
The performance improvements over the original warp are as follows:
1) If the down-sampling algorithm is nearest neighbour, we will be able
to reduce the FLOPS of the warp operation by down sampling the source
band before warping.
2) As a result of the downsampling at step 1), GDAL's RasterIO will
automatically take advantage of overviews if applicable.
3) The target window projected from the source band is likely to be
small when we zoom out. Thus we do not have to warp over the entire
target buffer but the subwindow projected from the source band.
4) Since we now only warp over a subwindow, we will only need to send
the subwindow of data over the network, which results in large
reduction of overheads in grpc (de-)serialisation and network traffic.
*/

/*
#include "gdal.h"
#include "gdalwarper.h"
#include "gdal_alg.h"
#include "ogr_api.h"
#include "ogr_srs_api.h"
#include "cpl_string.h"
#include "gdal_utils.h"
#cgo pkg-config: gdal
GDALDatasetH subsampleSrcDS(GDALDatasetH hSrcDS, const char *srcProjRef, GDALDatasetH hDstDS, const char *dstProjRef, int band, int *bSubsample)
{
	*bSubsample = 0;
	void *hTransformArg = GDALCreateReprojectionTransformer(srcProjRef, dstProjRef);

	int srcXSize = GDALGetRasterXSize(hSrcDS);
	int srcYSize = GDALGetRasterYSize(hSrcDS);

	int subsampleXSize = srcXSize;
	int subsampleYSize = srcYSize;

	double dstXSize = (double)GDALGetRasterXSize(hDstDS);
	double dstYSize = (double)GDALGetRasterYSize(hDstDS);

	double dstGeot[6];
	GDALGetGeoTransform(hDstDS, dstGeot);
	double dx[] = {dstGeot[0], dstGeot[0]+(dstXSize+0.5)*dstGeot[1]};
	double dy[] = {dstGeot[3], dstGeot[3]+(dstXSize+0.5)*dstGeot[5]};
	double dz[] = {0.0, 0.0};
	int bSuccess[] = {0, 0};
	GDALReprojectionTransform(hTransformArg, TRUE, 2, dx, dy, dz, bSuccess);
	GDALDestroyReprojectionTransformer(hTransformArg);

	if(bSuccess[0] == FALSE || bSuccess[1] == FALSE) {
		*bSubsample = 0;
		return NULL;
	}

	double dstXRes = (dx[1] - dx[0]) / dstXSize;
	double dstYRes = (dy[1] - dy[0]) / dstYSize;

	double srcGeot[6];
	GDALGetGeoTransform(hSrcDS, srcGeot);

	subsampleXSize = (int)(srcXSize * srcGeot[1] / dstXRes + 0.5);
	subsampleYSize = (int)(srcYSize * srcGeot[5] / dstYRes + 0.5);

	*bSubsample = subsampleXSize > 1 && subsampleXSize < 0.9 * srcXSize && subsampleYSize > 1 && subsampleYSize < 0.9 * srcYSize;
	if(!(*bSubsample)) {
		return NULL;
	}

	char bandStr[32];
	sprintf(bandStr, "%d", band);

	char xSizeStr[32];
	sprintf(xSizeStr, "%d", subsampleXSize);

	char ySizeStr[32];
	sprintf(ySizeStr, "%d", subsampleYSize);

	char *opts = NULL;
	opts = CSLAddString(opts, "-b");
	opts = CSLAddString(opts, bandStr);
	opts = CSLAddString(opts, "-outsize");
	opts = CSLAddString(opts, xSizeStr);
	opts = CSLAddString(opts, ySizeStr);

	GDALTranslateOptions *psOptions;
	psOptions = GDALTranslateOptionsNew(opts, NULL);

	GDALDatasetH hOutDS = GDALTranslate("/vsimem/tmp.vrt", hSrcDS, psOptions, NULL);
	GDALTranslateOptionsFree(psOptions);

	if(hOutDS == NULL) {
		*bSubsample = 0;
		return NULL;
	}

	*bSubsample = 1;
	return hOutDS;
}

int roundCoord(double coord, int maxExtent) {
	int c = (int)coord;
	if(c < 0) {
		c = 0;
	} else if(c > maxExtent - 1) {
		c = maxExtent - 1;
	}
	return c;
}

int warp_operation_fast(GDALDatasetH hSrcDS, GDALDatasetH hDstDS, int band, int *dstBbox)
{
	const char *srcProjRef = GDALGetProjectionRef(hSrcDS);
	if(strlen(srcProjRef) == 0) {
		srcProjRef = "GEOGCS[\"WGS 84\",DATUM[\"WGS_1984\",SPHEROID[\"WGS 84\",6378137,298.257223563,AUTHORITY[\"EPSG\",\"7030\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6326\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9108\"]],AUTHORITY[\"EPSG\",\"4326\"]]\",\"proj4\":\"+proj=longlat +ellps=WGS84 +towgs84=0,0,0,0,0,0,0 +no_defs \"";
	}
	const char *dstProjRef = GDALGetProjectionRef(hDstDS);

	int bSubsample = 0;
	GDALDatasetH hOutDS = subsampleSrcDS(hSrcDS, srcProjRef, hDstDS, dstProjRef, band, &bSubsample);
	if(bSubsample) {
		hSrcDS = hOutDS;
		band = 1;
	}

	void *hTransformArg = GDALCreateGenImgProjTransformer(hSrcDS, srcProjRef, hDstDS, dstProjRef, TRUE, 0.0, 0);
	if(hTransformArg == NULL) {
		if(bSubsample) {
			GDALClose(hSrcDS);
		}
		return 1;
	}

	double geotOut[6];
	int nPixels;
	int nLines;
	double bbox[4];
	int err = GDALSuggestedWarpOutput2(hSrcDS, GDALGenImgProjTransform, hTransformArg, geotOut, &nPixels, &nLines, bbox, 0);

	int dstXOff = 0;
	int dstYOff = 0;
	int dstXSize = GDALGetRasterXSize(hDstDS);
	int dstYSize = GDALGetRasterYSize(hDstDS);

	if(err == CE_None) {
		int minX, minY, maxX, maxY;
		minX = roundCoord(bbox[0], dstXSize);
		minY = roundCoord(bbox[1], dstYSize);
		maxX = roundCoord(bbox[2]+0.5, dstXSize);
		maxY = roundCoord(bbox[3]+0.5, dstYSize);

		dstXOff = minX;
		dstYOff = minY;
		dstXSize = maxX - minX + 1;
		dstYSize = maxY - minY + 1;
	}

	GDALWarpOptions *psWOptions;

	psWOptions = GDALCreateWarpOptions();
	psWOptions->nBandCount = 1;
	psWOptions->panSrcBands = (int *) CPLMalloc(sizeof(int) * 1);
	psWOptions->panSrcBands[0] = band;
	psWOptions->panDstBands = (int *) CPLMalloc(sizeof(int) * 1);
	psWOptions->panDstBands[0] = 1;

	psWOptions->hSrcDS = hSrcDS;
	psWOptions->hDstDS = hDstDS;

	psWOptions->eResampleAlg = GRA_NearestNeighbour;

	psWOptions->pTransformerArg = GDALCreateApproxTransformer(GDALGenImgProjTransform, hTransformArg, 0.25);
        psWOptions->pfnTransformer = GDALApproxTransform;

	GDALWarpOperationH warper = GDALCreateWarpOperation(psWOptions);
	err = GDALWarpRegion(warper, dstXOff, dstYOff, dstXSize, dstYSize, 0, 0, 0, 0);

	GDALDestroyApproxTransformer(psWOptions->pTransformerArg);
	GDALDestroyGenImgProjTransformer(hTransformArg);
	GDALDestroyWarpOptions(psWOptions);
	GDALDestroyWarpOperation(warper);

	dstBbox[0] = dstXOff;
	dstBbox[1] = dstYOff;
	dstBbox[2] = dstXSize;
	dstBbox[3] = dstYSize;

	if(bSubsample) {
		GDALClose(hSrcDS);
	}

	return err;
}

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
import "C"

import (
	"fmt"
	"log"
	"math"
	"reflect"
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

func initNoDataSlice(rType string, noDataValue float64, ssize int32) []uint8 {
	size := int(ssize)
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

func ComputeReprojectExtent(in *pb.GeoRPCGranule) *pb.Result {
	srcFileC := C.CString(in.Path)
	defer C.free(unsafe.Pointer(srcFileC))

	hSrcDS := C.GDALOpenEx(srcFileC, C.GDAL_OF_READONLY|C.GDAL_OF_VERBOSE_ERROR, nil, nil, nil)
	if hSrcDS == nil {
		return &pb.Result{Error: fmt.Sprintf("Failed to open existing dataset: %v", in.Path)}
	}
	defer C.GDALClose(hSrcDS)

	hSRS := C.OSRNewSpatialReference(nil)
	defer C.OSRDestroySpatialReference(hSRS)
	C.OSRImportFromEPSG(hSRS, C.int(in.EPSG))
	var projWKT *C.char
	defer C.free(unsafe.Pointer(projWKT))
	C.OSRExportToWkt(hSRS, &projWKT)

	hTransformArg := C.GDALCreateGenImgProjTransformer(hSrcDS, nil, nil, projWKT, C.int(0), C.double(0), C.int(0))
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

	xMin := in.Geot[0]
	yMin := in.Geot[1]
	xMax := in.Geot[2]
	yMax := in.Geot[3]

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

func WarpRaster(in *pb.GeoRPCGranule, debug bool) *pb.Result {
	filePathCStr := C.CString(in.Path)
	defer C.free(unsafe.Pointer(filePathCStr))

	dump := func(msg interface{}) string {
		log.Println(
			"warp", in.Path,
			"band", in.Bands[0],
			"width", in.Width,
			"height", in.Height,
			"geotransform", in.Geot,
			"error", msg,
		)
		return fmt.Sprintf("%v", msg)
	}

	hSrcDS := C.GDALOpen(filePathCStr, C.GA_ReadOnly)
	if hSrcDS == nil {
		return &pb.Result{Error: dump("GDALOpen() fail")}
	}
	defer C.GDALClose(hSrcDS)

	bandH := C.GDALGetRasterBand(hSrcDS, C.int(in.Bands[0]))
	if bandH == nil {
		return &pb.Result{Error: dump("GDALGetRasterBand() fail")}
	}
	nodata := float64(C.GDALGetRasterNoDataValue(bandH, nil))

	dType := C.GDALGetRasterDataType(bandH)

	canvas := initNoDataSlice(GDALTypes[dType], nodata, in.Width*in.Height)
	memStr := C.CString(fmt.Sprintf("MEM:::DATAPOINTER=%d,PIXELS=%d,LINES=%d,DATATYPE=%s", unsafe.Pointer(&canvas[0]), C.int(in.Width), C.int(in.Height), GDALTypes[dType]))
	defer C.free(unsafe.Pointer(memStr))
	hDstDS := C.GDALOpen(memStr, C.GA_Update)
	defer C.GDALClose(hDstDS)

	hSRS := C.OSRNewSpatialReference(nil)
	defer C.OSRDestroySpatialReference(hSRS)
	C.OSRImportFromEPSG(hSRS, C.int(in.EPSG))
	var projWKT *C.char
	defer C.free(unsafe.Pointer(projWKT))
	C.OSRExportToWkt(hSRS, &projWKT)

	C.GDALSetProjection(hDstDS, projWKT)
	C.GDALSetGeoTransform(hDstDS, (*C.double)(&in.Geot[0]))

	var dstBboxC [4]C.int
	cErr := C.warp_operation_fast(hSrcDS, hDstDS, C.int(in.Bands[0]), (*C.int)(&dstBboxC[0]))
	if cErr != 0 {
		return &pb.Result{Error: dump("warp_operation() fail")}
	}

	if debug {
		dump("debug")
	}

	dstBbox := make([]int32, len(dstBboxC))
	for i, v := range dstBboxC {
		dstBbox[i] = int32(v)
	}

	bboxCanvas := canvas
	if float64(dstBbox[2]*dstBbox[3]) < 0.95*float64(in.Width*in.Height) {
		bboxCanvas = initNoDataSlice(GDALTypes[dType], nodata, dstBbox[2]*dstBbox[3])
		gdalErr := C.GDALDatasetRasterIO(hDstDS, C.GF_Read, dstBboxC[0], dstBboxC[1], dstBboxC[2], dstBboxC[3], unsafe.Pointer(&bboxCanvas[0]), dstBboxC[2], dstBboxC[3], dType, 1, nil, 0, 0, 0)
		if gdalErr != C.CE_None {
			bboxCanvas = canvas
			dstBbox = []int32{0, 0, in.Width, in.Height}
		}
	} else {
		bboxCanvas = canvas
		dstBbox = []int32{0, 0, in.Width, in.Height}
	}

	return &pb.Result{Raster: &pb.Raster{Data: bboxCanvas, NoData: nodata, RasterType: GDALTypes[dType], Bbox: dstBbox}, Error: "OK"}
}
