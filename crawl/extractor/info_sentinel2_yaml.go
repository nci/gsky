package extractor

// #include <stdio.h>
// #include <stdlib.h>
// #include "gdal.h"
// #include "ogr_srs_api.h" /* for SRS calls */
// #include "cpl_string.h"
// #cgo pkg-config: gdal
//char *getProj4Text(char *projWKT)
//{
//	char *pszProj4;
//	char *result;
//	OGRSpatialReferenceH hSRS;
//
//	hSRS = OSRNewSpatialReference(projWKT);
//	OSRExportToProj4(hSRS, &pszProj4);
//	result = strdup(pszProj4);
//
//	OSRDestroySpatialReference(hSRS);
//	CPLFree(pszProj4);
//
//	return result;
//}
import "C"

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"
	"unsafe"
)

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
			Geo_ref_Points struct {
				Ll struct {
					X string
					Y string
				}
				Lr struct {
					X string
					Y string
				}
				Ul struct {
					X string
					Y string
				}
				Ur struct {
					X string
					Y string
				}
			}
			Spatial_reference string
		}
	}

	Image struct {
		Bands map[string]*ArdBand
	}
}

func ExtractSentinel2Yaml(filename string) (*GeoFile, error) {
	rawData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
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

	projWkt := ard.Grid_spatial.Projection.Spatial_reference

	cProjWKT := C.CString(projWkt)
	cProj4 := C.getProj4Text(cProjWKT)
	C.free(unsafe.Pointer(cProjWKT))
	proj4 := C.GoString(cProj4)
	C.free(unsafe.Pointer(cProj4))

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
			Polygon: fmt.Sprintf("POLYGON ((%s %s,%s %s,%s %s,%s %s,%s %s))",
				ard.Grid_spatial.Projection.Geo_ref_Points.Ul.X,
				ard.Grid_spatial.Projection.Geo_ref_Points.Ul.Y,
				ard.Grid_spatial.Projection.Geo_ref_Points.Ll.X,
				ard.Grid_spatial.Projection.Geo_ref_Points.Ll.Y,
				ard.Grid_spatial.Projection.Geo_ref_Points.Lr.X,
				ard.Grid_spatial.Projection.Geo_ref_Points.Lr.Y,
				ard.Grid_spatial.Projection.Geo_ref_Points.Ur.X,
				ard.Grid_spatial.Projection.Geo_ref_Points.Ur.Y,
				ard.Grid_spatial.Projection.Geo_ref_Points.Ul.X,
				ard.Grid_spatial.Projection.Geo_ref_Points.Ul.Y),

			ProjWKT: projWkt,
			Proj4:   proj4,
		}

		geoFile.DataSets = append(geoFile.DataSets, ds)
	}

	return &geoFile, nil
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
