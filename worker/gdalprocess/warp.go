package gdalprocess

/*
This is a fast implementation of the GDAL warp operation.
The performance improvements over the original warp are as follows:
1) If the down-sampling algorithm is nearest neighbour, we will be able
to reduce the FLOPS of the warp operation by down sampling the source
band before warping. This is achieved by only loading the data blocks
corresponding to the input pixel coordinates.
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
#include "uv.h"
#cgo pkg-config: gdal libuv

typedef struct {
	char *srcPath;
	int band;
	int iOvr;
	int xBlock, yBlock; 
	void *blockBuf;
	uv_sem_t *sem;
} AsyncBlockReq;

int roundCoord(double coord, int maxExtent) {
	int c;
	if(coord < 0) {
		c = 0;
	} else {
		c = (int)(coord + 1e-10);
		if(c > maxExtent - 1) {
			c = maxExtent - 1;
		}
	}
	return c;
}

void async_read_block(uv_work_t *req) {
	AsyncBlockReq *blockReq = (AsyncBlockReq *) req->data;

	GDALDatasetH hDS = GDALOpen(blockReq->srcPath, GA_ReadOnly);
        GDALRasterBandH hBand = GDALGetRasterBand(hDS, blockReq->band);
	if(blockReq->iOvr >= 0) {
		hBand = GDALGetOverview(hBand, blockReq->iOvr);
	}

	GDALReadBlock(hBand, blockReq->xBlock, blockReq->yBlock, blockReq->blockBuf);
	GDALClose(hDS);
	
	uv_sem_post(blockReq->sem);
}

void async_read_block_done(uv_work_t *req, int status) {}

int warp_operation_fast(const char *srcPath, GDALDatasetH hSrcDS, GDALDatasetH hDstDS, int band, void *dstBuf, int *dstBbox)
{
	const char *srcProjRef = GDALGetProjectionRef(hSrcDS);
	if(strlen(srcProjRef) == 0) {
		srcProjRef = "GEOGCS[\"WGS 84\",DATUM[\"WGS_1984\",SPHEROID[\"WGS 84\",6378137,298.257223563,AUTHORITY[\"EPSG\",\"7030\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6326\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9108\"]],AUTHORITY[\"EPSG\",\"4326\"]]\",\"proj4\":\"+proj=longlat +ellps=WGS84 +towgs84=0,0,0,0,0,0,0 +no_defs \"";
	}
	const char *dstProjRef = GDALGetProjectionRef(hDstDS);

        GDALRasterBandH hBand = GDALGetRasterBand(hSrcDS, band);
	if(!hBand) {
		return 1;
	}

	void *hTransformArg = GDALCreateGenImgProjTransformer(hSrcDS, srcProjRef, hDstDS, dstProjRef, TRUE, 0.0, 0);
	if(!hTransformArg) {
		return 2;
	}

	double geotOut[6];
	int nPixels;
	int nLines;
	double bbox[4];
	int err = GDALSuggestedWarpOutput2(hSrcDS, GDALGenImgProjTransform, hTransformArg, geotOut, &nPixels, &nLines, bbox, 0);

	int iOvr = -1;
	int nOverviews = GDALGetOverviewCount(hBand);
	int useOverview = 0;
	if(err == CE_None && nOverviews > 0) {
		double targetRatio = 1.0 / geotOut[1];
		if(targetRatio > 1.0) {
			int srcXSize = GDALGetRasterXSize(hSrcDS);
			int srcYSize = GDALGetRasterYSize(hSrcDS);

			for(; iOvr < nOverviews - 1; iOvr++) {
				GDALRasterBandH hOvr = GDALGetOverview(hBand, iOvr);
				GDALRasterBandH hOvrNext = GDALGetOverview(hBand, iOvr+1);

				double ovrRatio = 1.0;
				if(iOvr >= 0) {
					ovrRatio = (double)srcXSize / GDALGetRasterBandXSize(hOvr);
				}
				double nextOvrRatio = (double)srcXSize / GDALGetRasterBandXSize(hOvrNext);

                        	if(ovrRatio < targetRatio && nextOvrRatio > targetRatio) break;

				double diff = ovrRatio - targetRatio;
				if(diff > -1e-1 && diff < 1e-1) break;
			}

			if(iOvr >= 0) {
				double geot[6];
				GDALGetGeoTransform(hSrcDS, geot);

				hBand = GDALGetOverview(hBand, iOvr);
				int ovrXSize = GDALGetRasterBandXSize(hBand);
        			int ovrYSize = GDALGetRasterBandYSize(hBand);

				geot[1] *= srcXSize / (double)ovrXSize;
			        geot[2] *= srcXSize / (double)ovrXSize;
                                geot[4] *= srcYSize / (double)ovrYSize;
                                geot[5] *= srcYSize / (double)ovrYSize;

				double dstGeot[6];
				GDALGetGeoTransform(hDstDS, dstGeot);

				void *hOvrTransformArg = GDALCreateGenImgProjTransformer3(srcProjRef, geot, dstProjRef, dstGeot);
				if(hOvrTransformArg) {
					GDALDestroyGenImgProjTransformer(hTransformArg);
					hTransformArg = hOvrTransformArg;
				}
			}
		}
	}

	int dstXOff = 0;
	int dstYOff = 0;

	int dstXImageSize = GDALGetRasterXSize(hDstDS);
        int dstYImageSize = GDALGetRasterYSize(hDstDS);

	int dstXSize = dstXImageSize;
	int dstYSize = dstYImageSize;

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

	void *hApproxTransformArg = GDALCreateApproxTransformer(GDALGenImgProjTransform, hTransformArg, 0.125);

        int srcXSize = GDALGetRasterBandXSize(hBand);
        int srcYSize = GDALGetRasterBandYSize(hBand);

        int srcXBlockSize, srcYBlockSize;
        GDALGetBlockSize(hBand, &srcXBlockSize, &srcYBlockSize);

        int nXBlocks = (srcXSize + srcXBlockSize - 1) / srcXBlockSize;
        int nYBlocks = (srcYSize + srcYBlockSize - 1) / srcYBlockSize;

        uv_work_t **blockList = (uv_work_t **)malloc(nXBlocks * nYBlocks * sizeof(uv_work_t *));
        memset(blockList, 0, nXBlocks * nYBlocks * sizeof(uv_work_t *));

        GDALDataType dType = GDALGetRasterDataType(hBand);
        int dataSize = GDALGetDataTypeSizeBytes(dType);

        double *dx = (double *)malloc(2 * dstXSize * sizeof(double));
        double *dy = (double *)malloc(dstXSize * sizeof(double));
        double *dz = (double *)malloc(dstXSize * sizeof(double));
        int *bSuccess = (int *)malloc(dstXSize * sizeof(int));

        int iDstX, iDstY;
        for(iDstX = 0; iDstX < dstXSize; iDstX++) {
                dx[dstXSize+iDstX] = iDstX + 0.5 + dstXOff;
        }

	uv_loop_t *loop = uv_default_loop();

	for(iDstY = 0; iDstY < dstYSize; iDstY++) {
                memcpy(dx, dx + dstXSize, dstXSize * sizeof(double));
                const double dfY = iDstY + 0.5 + dstYOff;
                for(iDstX = 0; iDstX < dstXSize; iDstX++) {
                        dy[iDstX] = dfY;
                }
                memset(dz, 0, dstXSize * sizeof(double));

                GDALApproxTransform(hApproxTransformArg, TRUE, dstXSize, dx, dy, dz, bSuccess);

                for(iDstX = 0; iDstX < dstXSize; iDstX++) {
                        if(!bSuccess[iDstX]) continue;
                        if(dx[iDstX] < 0 || dy[iDstX] < 0) continue;
                        const iSrcX = (int)(dx[iDstX] + 1.0e-10);
                        const iSrcY = (int)(dy[iDstX] + 1.0e-10);
                        if(iSrcX >= srcXSize || iSrcY >= srcYSize) continue;

                        int iXBlock = iSrcX / srcXBlockSize;
                        int iYBlock = iSrcY / srcYBlockSize;
                        int iBlock = iXBlock + iYBlock * nXBlocks;

                        if(!blockList[iBlock]) {
				AsyncBlockReq *blockReq = (AsyncBlockReq *)malloc(sizeof(AsyncBlockReq));
				blockReq->srcPath = srcPath;
				blockReq->band = band;
				blockReq->iOvr = iOvr;
				blockReq->xBlock = iXBlock;
				blockReq->yBlock = iYBlock;
				blockReq->blockBuf = malloc(srcXBlockSize * srcYBlockSize * dataSize);
				
				uv_sem_t *sem = (uv_sem_t *)malloc(sizeof(uv_sem_t));
				uv_sem_init(sem, 0);
				blockReq->sem = sem;

				uv_work_t *req = (uv_work_t *)malloc(sizeof(uv_work_t));
				req->data = (void *) blockReq;

				blockList[iBlock] = req;
                        }
                }
        }

        int iBlock;
        for(iBlock = 0; iBlock < nXBlocks * nYBlocks; iBlock++) {
                if(blockList[iBlock]) {
			uv_queue_work(loop, blockList[iBlock], async_read_block, async_read_block_done);
                }
        }

	for(iBlock = 0; iBlock < nXBlocks * nYBlocks; iBlock++) {
                if(blockList[iBlock]) {
			AsyncBlockReq *blockReq = (AsyncBlockReq *) blockList[iBlock]->data;
			uv_sem_wait(blockReq->sem);
                }
        }

	for(iDstY = 0; iDstY < dstYSize; iDstY++) {
                memcpy(dx, dx + dstXSize, dstXSize * sizeof(double));
                const double dfY = iDstY + 0.5 + dstYOff;
                for(iDstX = 0; iDstX < dstXSize; iDstX++) {
                        dy[iDstX] = dfY;
                }
                memset(dz, 0, dstXSize * sizeof(double));

                GDALApproxTransform(hApproxTransformArg, TRUE, dstXSize, dx, dy, dz, bSuccess);

                for(iDstX = 0; iDstX < dstXSize; iDstX++) {
                        if(!bSuccess[iDstX]) continue;
                        if(dx[iDstX] < 0 || dy[iDstX] < 0) continue;
                        const iSrcX = (int)(dx[iDstX] + 1.0e-10);
                        const iSrcY = (int)(dy[iDstX] + 1.0e-10);
                        if(iSrcX >= srcXSize || iSrcY >= srcYSize) continue;

                        int iXBlock = iSrcX / srcXBlockSize;
                        int iYBlock = iSrcY / srcYBlockSize;
                        int iBlock = iXBlock + iYBlock * nXBlocks;

                        int iXBlockOff = iSrcX % srcXBlockSize;
                        int iYBlockOff = iSrcY % srcYBlockSize;
                        int iBlockOff = (iXBlockOff + iYBlockOff * srcXBlockSize) * dataSize;

                        int iDstOff = ((iDstY + dstYOff) * dstXImageSize + iDstX + dstXOff) * dataSize;

			AsyncBlockReq *blockReq = (AsyncBlockReq *) blockList[iBlock]->data;
                        memcpy(dstBuf + iDstOff, blockReq->blockBuf + iBlockOff, dataSize);

                }
        }

	dstBbox[0] = dstXOff;
        dstBbox[1] = dstYOff;
        dstBbox[2] = dstXSize;
        dstBbox[3] = dstYSize;

        for(iBlock = 0; iBlock < nXBlocks * nYBlocks; iBlock++) {
                if(blockList[iBlock]) {
			AsyncBlockReq *blockReq = (AsyncBlockReq *) blockList[iBlock]->data;
                        free(blockReq->blockBuf);
			uv_sem_destroy(blockReq->sem);
			free(blockList[iBlock]);
                }
        }

	uv_loop_close(loop);
	//free(loop);

        free(blockList);
        free(dx);
        free(dy);
        free(dz);
        free(bSuccess);

	GDALDestroyApproxTransformer(hApproxTransformArg);
        GDALDestroyGenImgProjTransformer(hTransformArg);

	return 0;
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
	cErr := C.warp_operation_fast(filePathCStr, hSrcDS, hDstDS, C.int(in.Bands[0]), unsafe.Pointer(&canvas[0]), (*C.int)(&dstBboxC[0]))
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
