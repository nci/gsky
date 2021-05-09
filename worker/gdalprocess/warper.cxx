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


#include "warper.hxx"
#include "coordinate_transform_cache.hxx"
#include <utility>
#include <map>
#include <vector>

#include <iostream>

auto coordTransformCache = new CoordinateTransformCache();

struct GenImgProjTransformInfo {

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

    bool     bCheckWithInvertPROJ;
};

void *createGeoLocTransformer(const char *srcProjRef, const char **geoLocOpts, const char *dstProjRef, double *dstGeot) {
	GenImgProjTransformInfo *psInfo = (GenImgProjTransformInfo *)GDALCreateGenImgProjTransformer3(srcProjRef, nullptr, dstProjRef, dstGeot);
	if(!psInfo) {
		return nullptr;
	}

	psInfo->pSrcTransformArg = GDALCreateGeoLocTransformer(nullptr, (char **)geoLocOpts, 0);
	if(psInfo->pSrcTransformArg == nullptr)
        {
            GDALDestroyGenImgProjTransformer(psInfo);
            return nullptr;
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

int warp_operation_fast(const char *srcFilePath, char *srcProjRef, double *srcGeot, const char **geoLocOpts, const char *dstProjRef, double *dstGeot, int dstXImageSize, int dstYImageSize, int band, int srsCf, void **dstBuf, int *dstBufSize, int *dstBbox, double *noData, GDALDataType *dType, int *bytesRead)
{
	*bytesRead = 0;

	GDALDatasetH hSrcDS = nullptr;
	const char *netCDFSig = "NETCDF:";

	if(strncmp(srcFilePath, netCDFSig, strlen(netCDFSig)) && strncmp(srcFilePath+strlen(srcFilePath)-3, ".nc", strlen(".nc"))) {
		hSrcDS = GDALOpenEx(srcFilePath, GA_ReadOnly|GDAL_OF_RASTER, nullptr, nullptr, nullptr);
	} else {
		char bandQuery[20];
		sprintf(bandQuery, "band_query=%d", band);

		const char *srsCfOpt = srsCf > 0 ? "srs_cf=yes" : "srs_cf=no";
		const char *openOpts[] = {"md_query=no", bandQuery, srsCfOpt, NULL};
		const char *drivers[] = {"GSKY_netCDF", NULL};

		hSrcDS = GDALOpenEx(srcFilePath, GA_ReadOnly|GDAL_OF_RASTER, drivers, openOpts, nullptr);
		band = 1;
	}

	if(!hSrcDS) {
		return 1;
	}

	if(srcProjRef == nullptr) {
		srcProjRef = (char *)GDALGetProjectionRef(hSrcDS);
		if(strlen(srcProjRef) == 0) {
			srcProjRef = (char *)"GEOGCS[\"WGS 84\",DATUM[\"WGS_1984\",SPHEROID[\"WGS 84\",6378137,298.257223563,AUTHORITY[\"EPSG\",\"7030\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6326\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9108\"]],AUTHORITY[\"EPSG\",\"4326\"]]\",\"proj4\":\"+proj=longlat +ellps=WGS84 +towgs84=0,0,0,0,0,0,0 +no_defs \"";
		}
	}

        GDALRasterBandH hBand = GDALGetRasterBand(hSrcDS, band);
	if(!hBand) {
		GDALClose(hSrcDS);
		return 2;
	}

	double _srcGeot[6];
	if(srcGeot == nullptr) {
		srcGeot = _srcGeot;
		GDALGetGeoTransform(hSrcDS, srcGeot);
	}

	void *hTransformArg  = nullptr;
	GDALTransformerFunc pTransFunc = GDALGenImgProjTransform;
	int hasGeoLoc = geoLocOpts != nullptr;
	bool hasCoordCache = false;
	if(!hasGeoLoc) {
		TransformKey key = std::make_pair(srcProjRef, dstProjRef);
		hTransformArg = coordTransformCache->get(key);
		if(hTransformArg == nullptr) {
			hTransformArg = GDALCreateGenImgProjTransformer3(srcProjRef, srcGeot, dstProjRef, dstGeot);
			if(!hTransformArg) {
				GDALClose(hSrcDS);
				return 3;
			}
			GenImgProjTransformInfo *psInfo = (GenImgProjTransformInfo *)hTransformArg;
			if(psInfo->pReprojectArg != nullptr) {
				coordTransformCache->put(key, hTransformArg);
				hasCoordCache = true;
			}
		} else {
			GenImgProjTransformInfo *psInfo = (GenImgProjTransformInfo *)hTransformArg;
			memcpy(psInfo->adfSrcGeoTransform, srcGeot,sizeof(psInfo->adfSrcGeoTransform));
			if(!GDALInvGeoTransform(psInfo->adfSrcGeoTransform, psInfo->adfSrcInvGeoTransform)) {
				GDALClose(hSrcDS);
				return 3;
			}

			memcpy(psInfo->adfDstGeoTransform, dstGeot,sizeof(psInfo->adfDstGeoTransform));
			if(!GDALInvGeoTransform(psInfo->adfDstGeoTransform, psInfo->adfDstInvGeoTransform)) {
				GDALClose(hSrcDS);
				return 3;
			}
			hasCoordCache = true;
		}
	} else {
		hTransformArg = createGeoLocTransformer(srcProjRef, geoLocOpts, dstProjRef, dstGeot);
		if(!hTransformArg) {
			GDALClose(hSrcDS);
			return 3;
		}
	}

	if(!dstProjRef) {
		GenImgProjTransformInfo *psInfo = (GenImgProjTransformInfo *)hTransformArg;
		psInfo->pReprojectArg = nullptr;
		psInfo->pReproject = nullptr;
		psInfo->pDstTransformer = psInfo->pSrcTransformer;
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

				GenImgProjTransformInfo *psInfo = (GenImgProjTransformInfo *)hTransformArg;
				memcpy(psInfo->adfSrcGeoTransform, srcGeot,sizeof(psInfo->adfSrcGeoTransform));
				if(!GDALInvGeoTransform(psInfo->adfSrcGeoTransform, psInfo->adfSrcInvGeoTransform)) {
					GDALClose(hSrcDS);
					return 3;
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
        const int nXBlocks = (srcXSize + srcXBlockSize - 1) / srcXBlockSize;
	const int nYBlocks = (srcYSize + srcYBlockSize - 1) / srcYBlockSize;

        *dType = GDALGetRasterDataType(hBand);
	const GDALDataType srcDataType = *dType;
        const int srcDataSize = GDALGetDataTypeSizeBytes(*dType);

	const int supportedDataType = *dType == GDT_Byte || *dType == GDT_Int16 || *dType == GDT_UInt16 || *dType == GDT_Float32;
	if(!supportedDataType) {
		*dType = GDT_Float32;
	}

        const int dataSize = GDALGetDataTypeSizeBytes(*dType);

	*dstBufSize = dstXSize * dstYSize * dataSize;
	uint8_t* pDstBuf = (uint8_t *)malloc(*dstBufSize);
	*dstBuf = pDstBuf;

	*noData = GDALGetRasterNoDataValue(hBand, nullptr);
	GDALCopyWords(noData, GDT_Float64, 0, *dstBuf, *dType, dataSize, dstXSize * dstYSize);

	auto dVec = std::vector<double>();
	dVec.resize(4 * dstXSize);
	double *dx = dVec.data();
	double *dy = dVec.data() + 2 * dstXSize;
	double *dz = dVec.data() + 3 * dstXSize;

	auto sVec = std::vector<int>();
	sVec.resize(dstXSize);
        int *bSuccess = sVec.data();

        for(int iDstX = 0; iDstX < dstXSize; iDstX++) {
                dx[dstXSize+iDstX] = iDstX + 0.5 + dstXOff;
        }

	const int dstPixelXSize = dstXSize / nXBlocks + 1;
	const int dstPixelYSize = dstYSize / nYBlocks + 1;
	const int dstPixelSize = dstPixelXSize * dstPixelYSize;

	auto blockPixelMap = std::map<int, std::pair<std::vector<int>, std::vector<int> > >();

	for(int iDstY = 0; iDstY < dstYSize; iDstY++) {
                memcpy(dx, dx + dstXSize, dstXSize * sizeof(double));
                const double dfY = iDstY + 0.5 + dstYOff;
                for(int iDstX = 0; iDstX < dstXSize; iDstX++) {
                        dy[iDstX] = dfY;
                }
		memset(dz, 0, dstXSize * sizeof(double));

                GDALApproxTransform(hApproxTransformArg, TRUE, dstXSize, dx, dy, dz, bSuccess);

                for(int iDstX = 0; iDstX < dstXSize; iDstX++) {
                        if(!bSuccess[iDstX]) continue;
                        if(dx[iDstX] < 0 || dy[iDstX] < 0) continue;
                        const int iSrcX = (int)(dx[iDstX] + 1.0e-10);
                        const int iSrcY = (int)(dy[iDstX] + 1.0e-10);
                        if(iSrcX >= srcXSize || iSrcY >= srcYSize) continue;

			const int iDst = iDstY * dstXSize + iDstX;
			const int iSrc = iSrcY * srcXSize + iSrcX;

                        const int iXBlock = iSrcX / srcXBlockSize;
                        const int iYBlock = iSrcY / srcYBlockSize;
			const int iBlock = iXBlock + iYBlock * nXBlocks;

			auto it = blockPixelMap.find(iBlock);
			if(it == blockPixelMap.end()) {
				auto srcPixels = std::vector<int>();
				srcPixels.reserve(dstPixelSize);

				auto dstPixels = std::vector<int>();
				dstPixels.reserve(dstPixelSize);

				blockPixelMap[iBlock] = std::make_pair(std::ref(srcPixels), std::ref(dstPixels));
			}
			blockPixelMap[iBlock].first.emplace_back(iSrc);
			blockPixelMap[iBlock].second.emplace_back(iDst);
		}
	}

	auto blockBuffer = std::vector<uint8_t>();
	blockBuffer.resize(srcXBlockSize * srcYBlockSize * srcDataSize);
	uint8_t* blockBuf = blockBuffer.data();

	int nBlocksRead = 0;

	for(const auto& it : blockPixelMap) {
		const int nPixels = it.second.first.size();
		if(nPixels == 0) {
			continue;
		}

		const int iBlock = it.first;
		const int iXBlock = iBlock % nXBlocks;
		const int iYBlock = iBlock / nXBlocks;

		const int err = GDALReadBlock(hBand, iXBlock, iYBlock, blockBuf);
		if(err != CE_None) continue;
		nBlocksRead++;

		for(int i = 0; i < nPixels; i++) {
			const int iSrc = it.second.first[i];
			const int iSrcX = iSrc % srcXSize;
			const int iSrcY = iSrc / srcXSize;

			const int iDst = it.second.second[i];
			const int iDstX = iDst % dstXSize;
			const int iDstY = iDst / dstXSize;

			const int iXBlockOff = iSrcX % srcXBlockSize;
			const int iYBlockOff = iSrcY % srcYBlockSize;
			const int iBlockOff = (iXBlockOff + iYBlockOff * srcXBlockSize) * srcDataSize;

			const int iDstOff = (iDstY * dstXSize + iDstX) * dataSize;
			if(supportedDataType) {
				memcpy(pDstBuf + iDstOff, blockBuf + iBlockOff, dataSize);
			} else {
				GDALCopyWords(blockBuf + iBlockOff, srcDataType, srcDataSize, pDstBuf + iDstOff, *dType, dataSize, 1);
			}
		}
	}

	*bytesRead = srcXBlockSize * srcYBlockSize * srcDataSize * nBlocksRead;

	dstBbox[0] = dstXOff;
        dstBbox[1] = dstYOff;
        dstBbox[2] = dstXSize;
        dstBbox[3] = dstYSize;

	if(*dType == GDT_Byte) {
		const char *pixelType = GDALGetMetadataItem((GDALMajorObjectH)hBand, "PIXELTYPE", "IMAGE_STRUCTURE");
		if(pixelType != nullptr && !strcmp(pixelType, "SIGNEDBYTE")) {
			*dType = (GDALDataType)100;
		}
	}

	GDALDestroyApproxTransformer(hApproxTransformArg);
	if(!hasCoordCache) {
       		GDALDestroyGenImgProjTransformer(hTransformArg);
	}

	GDALClose(hSrcDS);
	return 0;
}
