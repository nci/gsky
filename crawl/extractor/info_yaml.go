package extractor

// #include <stdio.h>
// #include <stdlib.h>
// #include "gdal.h"
// #include "ogr_srs_api.h" /* for SRS calls */
// #include "cpl_string.h"
// #cgo pkg-config: gdal
//char *getWktText(char *projWKT, int mode)
//{
//	char *pszProjWkt;
//	char *result;
//	OGRSpatialReferenceH hSRS;
//
//	hSRS = OSRNewSpatialReference(NULL);
//	OGRErr err = OSRSetFromUserInput(hSRS, projWKT);
//	if(err != OGRERR_NONE) {
//		OSRDestroySpatialReference(hSRS);
//		return NULL;
//	}
//
//	if(mode == 0) {
//		err = OSRExportToWkt(hSRS, &pszProjWkt);
//	} else {
//		err = OSRExportToProj4(hSRS, &pszProjWkt);
//	}
//
//	if(err != OGRERR_NONE) {
//		OSRDestroySpatialReference(hSRS);
//		return NULL;
//	}
//
//	result = strdup(pszProjWkt);
//
//	OSRDestroySpatialReference(hSRS);
//	CPLFree(pszProjWkt);
//
//	return result;
//}
import "C"

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

func ExtractYaml(filename string, family string) (*GeoFile, error) {
	var geoFile *GeoFile
	var err error
	if family == "sentinel2" {
		geoFile, err = ExtractSentinel2Yaml(filename)
	} else if family == "landsat" {
		geoFile, err = ExtractLandsatYaml(filename)
	} else {
		return nil, fmt.Errorf("unsupported yaml family: %s", family)
	}

	if err != nil {
		return nil, err
	}

	fStat, fErr := os.Lstat(filename)
	if fErr != nil {
		geoFile.PosixInfo = &PosixInfo{}
	} else {
		geoFile.PosixInfo = GetPosixInfo(filename, fStat)
		geoFile.PosixInfo.FilePath = ""
	}
	return geoFile, nil
}

func ExtractSentinel2Yaml(filename string) (*GeoFile, error) {
	rawData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	type ArdBand struct {
		Info struct {
			Geotransform []float64
			Height       int
			Width        int
		}

		Path string
	}

	type ArdMetadata struct {
		Format struct {
			Name string
		}

		Extent struct {
			Center_dt string
		}

		Grid_spatial struct {
			Projection struct {
				Valid_data struct {
					Coordinates [][][]string
				}
				Spatial_reference string
			}
		}

		Image struct {
			Bands map[string]*ArdBand
		}
	}

	ard := ArdMetadata{}

	err = yaml.Unmarshal(rawData, &ard)
	if err != nil {
		return nil, err
	}

	dsPath, _ := filepath.Split(filename)
	geoFile := GeoFile{FileName: filename, Driver: ard.Format.Name}

	timestampFormat := "2006-01-02T15:04:05Z"
	timestamp, err := time.ParseInLocation(timestampFormat, ard.Extent.Center_dt, time.UTC)
	if err != nil {
		log.Printf("invalid timestamp: %v", err)
	}

	srs := ard.Grid_spatial.Projection.Spatial_reference

	cSrs := C.CString(srs)

	cProjWkt := C.getWktText(cSrs, 0)
	projWkt := C.GoString(cProjWkt)
	C.free(unsafe.Pointer(cProjWkt))

	cProj4 := C.getWktText(cSrs, 1)
	proj4 := C.GoString(cProj4)
	C.free(unsafe.Pointer(cProj4))

	C.free(unsafe.Pointer(cSrs))

	var points []string
	for _, coord := range ard.Grid_spatial.Projection.Valid_data.Coordinates[0] {
		point := fmt.Sprintf("%s %s", coord[0], coord[1])
		points = append(points, point)
	}
	polygon := "POLYGON ((" + strings.Join(points, ",") + "))"

	for ns, aband := range ard.Image.Bands {
		ds := &GeoMetaData{
			DataSetName:  filepath.Join(dsPath, aband.Path),
			NameSpace:    ns,
			Type:         getBandDataType(ns),
			RasterCount:  1,
			TimeStamps:   []time.Time{timestamp},
			XSize:        int32(aband.Info.Width),
			YSize:        int32(aband.Info.Height),
			GeoTransform: aband.Info.Geotransform,
			Polygon:      polygon,
			ProjWKT:      projWkt,
			Proj4:        proj4,
		}

		geoFile.DataSets = append(geoFile.DataSets, ds)
	}

	return &geoFile, nil
}

func ExtractLandsatYaml(filename string) (*GeoFile, error) {
	rawData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	md := make(map[string]interface{})

	err = yaml.Unmarshal(rawData, &md)
	if err != nil {
		return nil, err
	}

	geoMd := &GeoMetaData{}

	if crsRaw, ok := md["crs"]; ok {
		srs := crsRaw.(string)

		cSrs := C.CString(srs)

		cProjWkt := C.getWktText(cSrs, 0)
		geoMd.ProjWKT = C.GoString(cProjWkt)
		C.free(unsafe.Pointer(cProjWkt))

		cProj4 := C.getWktText(cSrs, 1)
		geoMd.Proj4 = C.GoString(cProj4)
		C.free(unsafe.Pointer(cProj4))

		C.free(unsafe.Pointer(cSrs))
	}

	if geometryRaw, ok := md["geometry"]; ok {
		coordinates := geometryRaw.(map[interface{}]interface{})["coordinates"]
		var points []string
		for _, coordRaw := range coordinates.([]interface{})[0].([]interface{}) {
			coord := coordRaw.([]interface{})
			x := coord[0]
			y := coord[1]
			point := fmt.Sprintf("%f %f", x, y)
			points = append(points, point)
		}
		geoMd.Polygon = "POLYGON ((" + strings.Join(points, ",") + "))"
	}

	if propsRaw, ok := md["properties"]; ok {
		datetimeRaw := propsRaw.(map[interface{}]interface{})["datetime"].(string)
		parts := strings.Split(datetimeRaw, " ")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid datetime format: %v", datetimeRaw)
		}

		datetime := fmt.Sprintf("%sT%sZ", parts[0], parts[1][:len("00:05:18")])
		t, err := time.ParseInLocation("2006-01-02T15:04:05Z", datetime, time.UTC)
		if err == nil {
			geoMd.TimeStamps = []time.Time{t}
		}
	}

	fn, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}

	geoFile := &GeoFile{FileName: fn, Driver: "GTiff"}
	filePath := filepath.Dir(fn)
	if bandsRaw, ok := md["measurements"]; ok {
		bands := bandsRaw.(map[interface{}]interface{})
		for band := range bands {
			ns := band.(string)
			dsName := bands[band].(map[interface{}]interface{})["path"].(string)
			dsPath := filepath.Join(filePath, dsName)

			ds := &GeoMetaData{
				DataSetName:  dsPath,
				NameSpace:    ns,
				Type:         "Int16",
				RasterCount:  1,
				TimeStamps:   geoMd.TimeStamps,
				GeoTransform: []float64{0, 0, 0, 0, 0, 0},
				Polygon:      geoMd.Polygon,
				ProjWKT:      geoMd.ProjWKT,
				Proj4:        geoMd.Proj4,
			}

			geoFile.DataSets = append(geoFile.DataSets, ds)
		}
	}

	return geoFile, nil
}

func getBandDataType(bandName string) string {
	switch bandName {
	case "nbart_contiguity":
		return "Byte"
	case "nbart_red_edge_2":
		return "Int16"
	case "nbart_swir_3":
		return "Int16"
	case "solar_zenith":
		return "Float32"
	case "nbar_contiguity":
		return "Byte"
	case "nbar_red":
		return "Int16"
	case "nbart_nir_1":
		return "Int16"
	case "nbart_red":
		return "Int16"
	case "nbart_red_edge_1":
		return "Int16"
	case "satellite_azimuth":
		return "Float32"
	case "solar_azimuth":
		return "Float32"
	case "timedelta":
		return "Float32"
	case "nbar_blue":
		return "Int16"
	case "nbar_green":
		return "Int16"
	case "nbar_nir_1":
		return "Int16"
	case "nbar_red_edge_2":
		return "Int16"
	case "exiting":
		return "Float32"
	case "fmask":
		return "Byte"
	case "satellite_view":
		return "Float32"
	case "relative_slope":
		return "Float32"
	case "nbar_red_edge_1":
		return "Int16"
	case "nbar_red_edge_3":
		return "Int16"
	case "nbart_nir_2":
		return "Int16"
	case "relative_azimuth":
		return "Float32"
	case "azimuthal_exiting":
		return "Float32"
	case "azimuthal_incident":
		return "Float32"
	case "incident":
		return "Float32"
	case "nbart_red_edge_3":
		return "Int16"
	case "nbar_coastal_aerosol":
		return "Int16"
	case "nbar_swir_2":
		return "Int16"
	case "terrain_shadow":
		return "Byte"
	case "nbart_green":
		return "Int16"
	case "nbart_swir_2":
		return "Int16"
	case "nbar_nir_2":
		return "Int16"
	case "nbar_swir_3":
		return "Int16"
	case "nbart_blue":
		return "Int16"
	case "nbart_coastal_aerosol":
		return "Int16"
	default:
		log.Printf("unknown data type for band: %v", bandName)
		return "Byte"
	}
}
