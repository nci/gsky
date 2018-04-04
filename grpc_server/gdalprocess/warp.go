package gdalprocess

// #include "gdal.h"
// #include "gdalwarper.h"
// #include "gdal_alg.h"
// #include "ogr_api.h"
// #include "ogr_srs_api.h"
// #include "cpl_string.h"
// #cgo LDFLAGS: -lgdal
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
	"unsafe"

	pb "github.com/nci/gsky/grpc_server/gdalservice"
	"reflect"
)

const SIZE_OF_UINT16 = 2
const SIZE_OF_INT16 = 2
const SIZE_OF_FLOAT32 = 4

var GDALTypes map[C.GDALDataType]string = map[C.GDALDataType]string{0: "Unkown", 1: "Byte", 2: "UInt16", 3: "Int16",
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

	return &pb.Result{Raster: &pb.Raster{Data: canvas, NoData: nodata, RasterType: GDALTypes[dType]}, Error: "OK"}
}
