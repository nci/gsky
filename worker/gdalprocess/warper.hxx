#ifndef WRAPER_H
#define WRAPER_H

#include "gdal.h"
#include "gdal_alg.h"
#include "ogr_api.h"
#include "ogr_srs_api.h"
#include "cpl_string.h"
#include "gdal_utils.h"


#ifdef __cplusplus
extern "C" {
#endif

int warp_operation_fast(const char *srcFilePath, char *srcProjRef, double *srcGeot, const char **geoLocOpts, const char *dstProjRef, double *dstGeot, int dstXImageSize, int dstYImageSize, int band, int srsCf, void **dstBuf, int *dstBufSize, int *dstBbox, double *noData, GDALDataType *dType, size_t *bytesRead);

#ifdef __cplusplus
}
#endif


#endif
