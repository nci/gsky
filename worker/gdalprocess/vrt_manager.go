package gdalprocess

/*
#include "gdal.h"
#include "cpl_vsi.h"
#cgo pkg-config: gdal

VSILFILE *wrap_VSIFileFromMemBuffer(char *filename, char *data, unsigned long dataLength) {
  return VSIFileFromMemBuffer(filename, data, dataLength, 0);
}
*/
import "C"

import (
	"encoding/xml"
	"fmt"
	"math/rand"
	"strings"
	"unsafe"
)

type VRTDataset struct {
	XMLName        xml.Name         `xml:"VRTDataset"`
	RasterXSize    int              `xml:"rasterXSize,attr"`
	RasterYSize    int              `xml:"rasterYSize,attr"`
	SRS            string           `xml:"SRS"`
	GeoTransform   string           `xml:"GeoTransform"`
	VRTRasterBands []*VRTRasterBand `xml:"VRTRasterBand"`
}

type VRTRasterBand struct {
	XMLName               xml.Name           `xml:"VRTRasterBand"`
	DataType              string             `xml:"dataType,attr"`
	Band                  int                `xml:"band,attr"`
	SubClass              string             `xml:"subClass,attr"`
	NoDataValue           float64            `xml:"NoDataValue"`
	PixelFunctionType     string             `xml:"PixelFunctionType"`
	PixelFunctionLanguage string             `xml:"PixelFunctionLanguage"`
	PixelFunctionCode     *PixelFunctionCode `xml:"PixelFunctionCode"`
	SimpleSources         []*SimpleSource    `xml:"SimpleSource"`
}

type SimpleSource struct {
	MetadataTemplate int    `xml:"metadata-template,attr"`
	SourceFileName   string `xml:"SourceFilename"`
}

type PixelFunctionCode struct {
	XMLName xml.Name `xml:"PixelFunctionCode"`
	Text    string   `xml:",cdata"`
}

type VRTManager struct {
	DSFileName string
	vrtC       *C.char
}

func NewVRTManager(vrt []byte) (*VRTManager, error) {
	var vrtDS VRTDataset
	err := xml.Unmarshal(vrt, &vrtDS)
	if err != nil {
		return nil, err
	}

	vrtMgr := &VRTManager{}

	newVRT := vrt
	foundMDTemplate := false
	for _, band := range vrtDS.VRTRasterBands {
		for _, source := range band.SimpleSources {
			if source.MetadataTemplate == 1 {
				pathC := C.CString(source.SourceFileName)
				ds := C.GDALOpen(pathC, C.GDAL_OF_READONLY)
				C.free(unsafe.Pointer(pathC))
				if ds == nil {
					return nil, fmt.Errorf("GDAL could not open dataset: %s", source.SourceFileName)
				}

				projRefC := C.GDALGetProjectionRef(ds)
				vrtDS.SRS = C.GoString(projRefC)
				if len(vrtDS.SRS) == 0 {
					vrtDS.SRS = "GEOGCS[\"WGS 84\",DATUM[\"WGS_1984\",SPHEROID[\"WGS 84\",6378137,298.257223563,AUTHORITY[\"EPSG\",\"7030\"]],TOWGS84[0,0,0,0,0,0,0],AUTHORITY[\"EPSG\",\"6326\"]],PRIMEM[\"Greenwich\",0,AUTHORITY[\"EPSG\",\"8901\"]],UNIT[\"degree\",0.0174532925199433,AUTHORITY[\"EPSG\",\"9108\"]],AUTHORITY[\"EPSG\",\"4326\"]]\",\"proj4\":\"+proj=longlat +ellps=WGS84 +towgs84=0,0,0,0,0,0,0 +no_defs \""
				}

				vrtDS.RasterXSize = int(C.GDALGetRasterXSize(ds))
				vrtDS.RasterYSize = int(C.GDALGetRasterYSize(ds))

				var geot [6]float64
				C.GDALGetGeoTransform(ds, (*C.double)(&geot[0]))

				var geotStr []string
				for _, v := range geot {
					geotStr = append(geotStr, fmt.Sprintf("%.5f", v))
				}
				vrtDS.GeoTransform = strings.Join(geotStr, ",")

				hBand := C.GDALGetRasterBand(ds, 1)
				band.NoDataValue = float64(C.GDALGetRasterNoDataValue(hBand, nil))

				dataTypeC := C.GDALGetDataTypeName(C.GDALGetRasterDataType(hBand))
				band.DataType = C.GoString(dataTypeC)
				C.GDALClose(ds)

				newVRT, err = xml.MarshalIndent(vrtDS, " ", "  ")
				if err != nil {
					return nil, err
				}

				foundMDTemplate = true
				break
			}
		}
		if foundMDTemplate {
			break
		}
	}

	newVRTC := C.CString(string(newVRT))
	vsiFile := fmt.Sprintf("/vsimem/file%04d.vrt", rand.Intn(1000))
	vsiFileC := C.CString(vsiFile)
	vrtLen := C.strlen(newVRTC)
	vsiFileH := C.wrap_VSIFileFromMemBuffer(vsiFileC, newVRTC, vrtLen)
	C.VSIFCloseL(vsiFileH)
	C.free(unsafe.Pointer(vsiFileC))

	vrtMgr.vrtC = newVRTC
	vrtMgr.DSFileName = vsiFile

	return vrtMgr, nil
}

func (mgr *VRTManager) Close() {
	if len(mgr.DSFileName) > 0 {
		fileC := C.CString(mgr.DSFileName)
		C.VSIUnlink(fileC)
		C.free(unsafe.Pointer(fileC))
	}

	if mgr.vrtC != nil {
		C.free(unsafe.Pointer(mgr.vrtC))
	}
}
