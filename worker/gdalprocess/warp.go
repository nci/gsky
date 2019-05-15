package gdalprocess

// #include "gdal.h"
// #include "gdalwarper.h"
// #include "gdal_alg.h"
// #include "ogr_api.h"
// #include "ogr_srs_api.h"
// #include "cpl_string.h"
// #cgo pkg-config: gdal
// int
// warp_operation(GDALDatasetH hSrcDS, GDALDatasetH hDstDS, int band)
// {
//        const char *srcProjRef;
//        int err;
//        GDALWarpOptions *psWOptions;
//
//        psWOptions = GDALCreateWarpOptions();
//        psWOptions->nBandCount = 1;
//        psWOptions->panSrcBands = (int *) CPLMalloc(sizeof(int) * 1);
//        psWOptions->panSrcBands[0] = band;
//        psWOptions->panDstBands = (int *) CPLMalloc(sizeof(int) * 1);
//        psWOptions->panDstBands[0] = 1;
//
//        srcProjRef = GDALGetProjectionRef(hSrcDS);
//        if(strlen(srcProjRef) == 0) {
//            srcProjRef = "GEOGCS[\"WGS 84\",DATUM[\"WGS_1984\",SPHEROID[\"WGS 84\",6378137,298.257223563,AUTHORITY[\"EPSG\",\"7030\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6326\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9108\"]],AUTHORITY[\"EPSG\",\"4326\"]]\",\"proj4\":\"+proj=longlat +ellps=WGS84 +towgs84=0,0,0,0,0,0,0 +no_defs \"";
//        }
//
//        err = GDALReprojectImage(hSrcDS, srcProjRef, hDstDS, GDALGetProjectionRef(hDstDS), GRA_NearestNeighbour, 0.0, 0.0, NULL, NULL, psWOptions);
//        GDALDestroyWarpOptions(psWOptions);
//
//        return err;
// }
import "C"

import (
	"fmt"
	"log"
	"math"
	"unsafe"

	"reflect"

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

	// TODO pass overview level in Granule
	//ovrSel := C.CString("OVERVIEW_LEVEL=0")
	//defer C.free(unsafe.Pointer(ovrSel))
	//hSrcDS := C.GDALOpenEx(filePathCStr, C.GA_ReadOnly, nil, &ovrSel, nil)

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

	// TODO Remove this code once verified all use cases work
	// This section is in quarantine as it seems GDALWarp doesn't write outside its limits
	// so parts of the canvas remain initialised with zero values instead of the nodata valuek
	/* dSize := C.GDALGetDataTypeSizeBytes(dType)
	if dSize == 0 {
		err := fmt.Errorf("GDAL data type not implemented")
		return &pb.Result{Error: err.Error()}
	}
	canvas := make([]uint8, in.Width*in.Height*dSize)
	*/

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
	cErr := C.warp_operation(hSrcDS, hDstDS, C.int(in.Bands[0]))
	if cErr != 0 {
		return &pb.Result{Error: dump("warp_operation() fail")}
	}

	if debug {
		dump("debug")
	}

	rasterType := GDALTypes[dType]
	if rasterType == "Byte" {
		pixelTypeC := C.CString("PIXELTYPE")
		pixelTypeDomainC := C.CString("IMAGE_STRUCTURE")
		pixelTypeValC := C.GDALGetMetadataItem(C.GDALMajorObjectH(bandH), pixelTypeC, pixelTypeDomainC);
		if pixelTypeValC != nil && C.GoString(pixelTypeValC) == "SIGNEDBYTE" {
			rasterType = "SignedByte"
		}

		C.free(unsafe.Pointer(pixelTypeC))
		C.free(unsafe.Pointer(pixelTypeDomainC))
	}

	return &pb.Result{Raster: &pb.Raster{Data: canvas, NoData: nodata, RasterType: rasterType}, Error: "OK"}
}
