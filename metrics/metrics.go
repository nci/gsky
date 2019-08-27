package metrics

//#include "ogr_api.h"
//#include "ogr_srs_api.h"
//#cgo pkg-config: gdal
import "C"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"time"
	"unsafe"

	"github.com/nci/gsky/utils"
)

type URLInfo struct {
	RawURL string            `json:"raw_url"`
	Host   string            `json:"host"`
	Path   string            `json:"path"`
	Query  map[string]string `json:"query"`
}

type IndexerInfo struct {
	Duration     time.Duration `json:"duration"`
	URL          URLInfo       `json:"url"`
	Geometry     string        `json:"geometry"`
	SRS          string        `json:"-"`
	GeometryArea float64       `json:"geometry_area"`
	NumFiles     int           `json:"num_files"`
	NumGranules  int           `json:"num_granules"`
}

type RPCInfo struct {
	Duration         time.Duration `json:"duration"`
	NumTiledGranules int           `json:"num_tiled_granules"`
	BytesRead        int64         `json:"bytes_read"`
	UserTime         int64         `json:"user_time"`
	SysTime          int64         `json:"sys_time"`
}

type MetricsInfo struct {
	ReqTime     string        `json:"req_time"`
	ReqDuration time.Duration `json:"req_duration"`
	URL         URLInfo       `json:"url"`
	RemoteAddr  string        `json:"remote_addr"`
	RemoteHost  string        `json:"remote_host"`
	RemotePort  string        `json:"remote_port"`
	HTTPStatus  int           `json:"http_status"`
	Indexer     *IndexerInfo  `json:"indexer"`
	RPC         *RPCInfo      `json:"rpc"`
}

type MetricsCollector struct {
	Info   *MetricsInfo
	logger Logger
}

func NewMetricsCollector(logger Logger) *MetricsCollector {
	return &MetricsCollector{
		Info: &MetricsInfo{
			Indexer: &IndexerInfo{},
			RPC:     &RPCInfo{},
		},
		logger: logger,
	}
}

func (m *MetricsCollector) Log() {
	if m.logger != nil {
		m.logger.Log(m.Info)
	}
}

func (i *MetricsInfo) ToJSON() (string, error) {
	i.normaliseNetworkAddr(i.RemoteAddr)
	i.normaliseURLs()
	err := i.normaliseGeometry()
	if err != nil {
		log.Printf("metrics: normaliseGeometry() error: %v", err)
	}

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	err = enc.Encode(i)
	if err == nil {
		return buf.String(), nil
	} else {
		return "", err
	}
}

func (i *MetricsInfo) normaliseNetworkAddr(addr string) {
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		i.RemoteHost = host
		i.RemotePort = port
	} else {
		i.RemoteHost = addr
	}
}

func (i *MetricsInfo) normaliseURLs() {
	err := i.normaliseURL(&i.URL)
	if err != nil {
		log.Printf("metrics: normaliseUrl() error: %v", err)
	}

	if i.Indexer != nil {
		err = i.normaliseURL(&i.Indexer.URL)
		if err != nil {
			log.Printf("metrics: indexer: normaliseUrl() error: %v", err)
		}
	}
}

func (i *MetricsInfo) normaliseURL(u *URLInfo) error {
	r, err := url.Parse(u.RawURL)
	if err != nil {
		return err
	}

	u.Host = r.Host
	u.Path = r.Path
	query, err := utils.ParseQuery(r.RawQuery)
	if err != nil {
		return err
	}

	if u.Query == nil {
		u.Query = make(map[string]string)
	}
	for k, v := range query {
		if len(v) == 1 {
			u.Query[k] = v[0]
		} else if len(v) > 1 {
			u.Query[k] = fmt.Sprintf("%v", v)
		} else {
			u.Query[k] = ""
		}
	}
	return nil
}

func (i *MetricsInfo) normaliseGeometry() error {
	if i.Indexer != nil && len(i.Indexer.Geometry) == 0 {
		i.Indexer.Geometry = "POLYGON EMPTY"
		return nil
	} else if i.Indexer != nil && len(i.Indexer.Geometry) > 0 {
		// WPS case
		if i.Indexer.GeometryArea > 0 && i.Indexer.SRS == "EPSG:4326" {
			return nil
		}

		geomWktTmpC := C.CString(i.Indexer.Geometry)
		//OGR_G_CreateFromWkt() internally updates the pointer. We need to back it up for freeing the heap later.
		geomWktC := geomWktTmpC

		var geom C.OGRGeometryH
		C.OGR_G_CreateFromWkt(&geomWktTmpC, nil, &geom)
		C.free(unsafe.Pointer(geomWktC))
		if geom == nil {
			return fmt.Errorf("Failed to create geometry from wkt: %v", i.Indexer.Geometry)
		}

		srcSRS := C.OSRNewSpatialReference(nil)
		srsC := C.CString(i.Indexer.SRS)
		C.OSRSetFromUserInput(srcSRS, srsC)
		C.free(unsafe.Pointer(srsC))

		dstSRS := C.OSRNewSpatialReference(nil)
		srsC = C.CString("EPSG:4326")
		C.OSRSetFromUserInput(dstSRS, srsC)
		C.free(unsafe.Pointer(srsC))

		trans := C.OCTNewCoordinateTransformation(srcSRS, dstSRS)
		ret := C.OGR_G_Transform(geom, trans)

		if ret == C.OGRERR_NONE {
			var dstGeomWkt *C.char
			C.OGR_G_ExportToWkt(geom, &dstGeomWkt)
			i.Indexer.Geometry = C.GoString(dstGeomWkt)
			C.free(unsafe.Pointer(dstGeomWkt))
			i.Indexer.GeometryArea = float64(C.OGR_G_Area(geom))
		} else {
			return fmt.Errorf("Failed to transform geometry: %v", ret)
		}

		C.OSRDestroySpatialReference(srcSRS)
		C.OSRDestroySpatialReference(dstSRS)
		C.OGR_G_DestroyGeometry(geom)

	}
	return nil
}
