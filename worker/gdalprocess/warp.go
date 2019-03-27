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
#cgo pkg-config: gdal

typedef struct {

    GDALTransformerInfo sTI;

    double   adfSrcGeoTransform[6];
    double   adfSrcInvGeoTransform[6];

    void     *pSrcTransformArg;
    GDALTransformerFunc pSrcTransformer;

    void     *pReprojectArg;
    GDALTransformerFunc pReproject;

    double   adfDstGeoTransform[6];
    double   adfDstInvGeoTransform[6];

    void     *pDstTransformArg;
    GDALTransformerFunc pDstTransformer;

} GenImgProjTransformInfo;

void *createGeoLocTransformer(const char *srcProjRef, const char **geoLocOpts, const char *dstProjRef, double *dstGeot) {
	GenImgProjTransformInfo *psInfo = (GenImgProjTransformInfo *)GDALCreateGenImgProjTransformer3(srcProjRef, NULL, dstProjRef, dstGeot);
	if(!psInfo) {
		return NULL;
	}

	psInfo->pSrcTransformArg = GDALCreateGeoLocTransformer(NULL, (char **)geoLocOpts, 0);
	if(psInfo->pSrcTransformArg == NULL)
        {
            GDALDestroyGenImgProjTransformer(psInfo);
            return NULL;
        }
        psInfo->pSrcTransformer = GDALGeoLocTransform;

	return psInfo;
}

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

int warp_operation_fast(const char *srcFilePath, char *srcProjRef, double *srcGeot, const char **geoLocOpts, const char *dstProjRef, double *dstGeot, int dstXImageSize, int dstYImageSize, int band, void **dstBuf, int *dstBufSize, int *dstBbox, double *noData, GDALDataType *dType)
{
	GDALDatasetH hSrcDS = GDALOpen(srcFilePath, GA_ReadOnly);
        if(!hSrcDS) {
                return 1;
        }

	if(srcProjRef == NULL) {
		srcProjRef = (char *)GDALGetProjectionRef(hSrcDS);
		if(strlen(srcProjRef) == 0) {
			srcProjRef = "GEOGCS[\"WGS 84\",DATUM[\"WGS_1984\",SPHEROID[\"WGS 84\",6378137,298.257223563,AUTHORITY[\"EPSG\",\"7030\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6326\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9108\"]],AUTHORITY[\"EPSG\",\"4326\"]]\",\"proj4\":\"+proj=longlat +ellps=WGS84 +towgs84=0,0,0,0,0,0,0 +no_defs \"";
		}
	}

        GDALRasterBandH hBand = GDALGetRasterBand(hSrcDS, band);
	if(!hBand) {
		GDALClose(hSrcDS);
		return 2;
	}

	double _srcGeot[6];
	if(srcGeot == NULL) {
		srcGeot = _srcGeot;
		GDALGetGeoTransform(hSrcDS, srcGeot);
	}

	void *hTransformArg  = NULL;
	GDALTransformerFunc pTransFunc = GDALGenImgProjTransform;
	int hasGeoLoc = geoLocOpts != NULL;
	if(!hasGeoLoc) {
		hTransformArg = GDALCreateGenImgProjTransformer3(srcProjRef, srcGeot, dstProjRef, dstGeot);
		if(!hTransformArg) {
			GDALClose(hSrcDS);
			return 3;
		}
	} else {
		hTransformArg = createGeoLocTransformer(srcProjRef, geoLocOpts, dstProjRef, dstGeot);
		if(!hTransformArg) {
			GDALClose(hSrcDS);
			return 3;
		}
	}

	double geotOut[6];
	int nPixels;
	int nLines;
	double bbox[4];
	int err = GDALSuggestedWarpOutput2(hSrcDS, pTransFunc, hTransformArg, geotOut, &nPixels, &nLines, bbox, 0);

	int nOverviews = GDALGetOverviewCount(hBand);
	int useOverview = 0;
	if(!hasGeoLoc && err == CE_None && nOverviews > 0) {
		double targetRatio = 1.0 / geotOut[1];
		if(targetRatio > 1.0) {
			int srcXSize = GDALGetRasterXSize(hSrcDS);
			int srcYSize = GDALGetRasterYSize(hSrcDS);

			int iOvr = -1;
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
				hBand = GDALGetOverview(hBand, iOvr);
				int ovrXSize = GDALGetRasterBandXSize(hBand);
        			int ovrYSize = GDALGetRasterBandYSize(hBand);

				srcGeot[1] *= srcXSize / (double)ovrXSize;
			        srcGeot[2] *= srcXSize / (double)ovrXSize;
                                srcGeot[4] *= srcYSize / (double)ovrYSize;
                                srcGeot[5] *= srcYSize / (double)ovrYSize;

				void *hOvrTransformArg = GDALCreateGenImgProjTransformer3(srcProjRef, srcGeot, dstProjRef, dstGeot);
				if(hOvrTransformArg) {
					GDALDestroyGenImgProjTransformer(hTransformArg);
					hTransformArg = hOvrTransformArg;
				}
			}
		}
	}

	int dstXOff = 0;
	int dstYOff = 0;

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

	void *hApproxTransformArg = GDALCreateApproxTransformer(pTransFunc, hTransformArg, 0.125);

        int srcXSize = GDALGetRasterBandXSize(hBand);
        int srcYSize = GDALGetRasterBandYSize(hBand);

        int srcXBlockSize, srcYBlockSize;
        GDALGetBlockSize(hBand, &srcXBlockSize, &srcYBlockSize);

        int nXBlocks = (srcXSize + srcXBlockSize - 1) / srcXBlockSize;
        int nYBlocks = (srcYSize + srcYBlockSize - 1) / srcYBlockSize;

        void **blockList = (void **)malloc(nXBlocks * nYBlocks * sizeof(void *));
        memset(blockList, 0, nXBlocks * nYBlocks * sizeof(void *));

        *dType = GDALGetRasterDataType(hBand);
	const GDALDataType srcDataType = *dType;
        const int srcDataSize = GDALGetDataTypeSizeBytes(*dType);

	const int supportedDataType = *dType == GDT_Byte || *dType == GDT_Int16 || *dType == GDT_UInt16 || *dType == GDT_Float32;
	if(!supportedDataType) {
		*dType = GDT_Float32;
	}

        const int dataSize = GDALGetDataTypeSizeBytes(*dType);

	*dstBufSize = dstXSize * dstYSize * dataSize;
	*dstBuf = malloc(*dstBufSize);

	*noData = GDALGetRasterNoDataValue(hBand, NULL);
	GDALCopyWords(noData, GDT_Float64, 0, *dstBuf, *dType, dataSize, dstXSize * dstYSize);

        double *dx = (double *)malloc(2 * dstXSize * sizeof(double));
        double *dy = (double *)malloc(dstXSize * sizeof(double));
        double *dz = (double *)malloc(dstXSize * sizeof(double));
        int *bSuccess = (int *)malloc(dstXSize * sizeof(int));

        int iDstX, iDstY;
        for(iDstX = 0; iDstX < dstXSize; iDstX++) {
                dx[dstXSize+iDstX] = iDstX + 0.5 + dstXOff;
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
                        const int iSrcX = (int)(dx[iDstX] + 1.0e-10);
                        const int iSrcY = (int)(dy[iDstX] + 1.0e-10);
                        if(iSrcX >= srcXSize || iSrcY >= srcYSize) continue;

                        int iXBlock = iSrcX / srcXBlockSize;
                        int iYBlock = iSrcY / srcYBlockSize;
                        int iBlock = iXBlock + iYBlock * nXBlocks;

                        if(!blockList[iBlock]) {
                                blockList[iBlock] = malloc(srcXBlockSize * srcYBlockSize * srcDataSize);
                                err = GDALReadBlock(hBand, iXBlock, iYBlock, blockList[iBlock]);
				if(err != CE_None) continue;
                        }

                        int iXBlockOff = iSrcX % srcXBlockSize;
                        int iYBlockOff = iSrcY % srcYBlockSize;
                        int iBlockOff = (iXBlockOff + iYBlockOff * srcXBlockSize) * srcDataSize;

                        int iDstOff = (iDstY * dstXSize + iDstX) * dataSize;
			if(supportedDataType) {
				memcpy(*dstBuf + iDstOff, blockList[iBlock] + iBlockOff, dataSize);
			} else {
				GDALCopyWords(blockList[iBlock] + iBlockOff, srcDataType, srcDataSize, *dstBuf + iDstOff, *dType, dataSize, 1);
			}
                }
        }

	dstBbox[0] = dstXOff;
        dstBbox[1] = dstYOff;
        dstBbox[2] = dstXSize;
        dstBbox[3] = dstYSize;

	int iBlock;
        for(iBlock = 0; iBlock < nXBlocks * nYBlocks; iBlock++) {
                if(blockList[iBlock]) {
                        free(blockList[iBlock]);
                }
        }

        free(blockList);
        free(dx);
        free(dy);
        free(dz);
        free(bSuccess);

	GDALDestroyApproxTransformer(hApproxTransformArg);
       	GDALDestroyGenImgProjTransformer(hTransformArg);

	GDALClose(hSrcDS);
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

func WarpRaster(in *pb.GeoRPCGranule, debug bool) *pb.Result {
	filePathC := C.CString(in.Path)
	defer C.free(unsafe.Pointer(filePathC))

	dstProjRefC := C.CString(in.DstSRS)
	defer C.free(unsafe.Pointer(dstProjRefC))

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
	cErr := C.warp_operation_fast(filePathC, srcProjRefC, pSrcGeot, pGeoLoc, dstProjRefC, (*C.double)(&in.DstGeot[0]), C.int(in.Width), C.int(in.Height), C.int(in.Bands[0]), (*unsafe.Pointer)(&dstBufC), (*C.int)(&dstBufSize), (*C.int)(&dstBboxC[0]), (*C.double)(&noData), (*C.GDALDataType)(&dType))
	if cErr != 0 {
		return &pb.Result{Error: dump(fmt.Sprintf("warp_operation() fail: %v", int(cErr)))}
	}

	if debug {
		dump("debug")
	}

	dstBbox := make([]int32, len(dstBboxC))
	for i, v := range dstBboxC {
		dstBbox[i] = int32(v)
	}

	bboxCanvas := C.GoBytes(dstBufC, dstBufSize)
	C.free(dstBufC)

	return &pb.Result{Raster: &pb.Raster{Data: bboxCanvas, NoData: noData, RasterType: GDALTypes[dType], Bbox: dstBbox}, Error: "OK"}
}
