package processor

// #include "ogr_api.h"
// #include "ogr_srs_api.h"
// #cgo pkg-config: gdal
import "C"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unsafe"

	"github.com/edisonguo/jet"
	geo "github.com/nci/geometry"
	"github.com/nci/gsky/utils"
)

// ISOFormat is the string used to format Go ISO times
const ISOFormat = "2006-01-02T15:04:05.000Z"
const DefaultMaxLogLength = 3000
const DrillTargetSRS = "EPSG:4326"

type FileList struct {
	Files []string `json:"files"`
}

type DrillIndexer struct {
	Context     context.Context
	In          chan *GeoDrillRequest
	Out         chan *GeoDrillGranule
	Error       chan error
	APIAddress  string
	IdentityTol float64
	DpTol       float64
	Approx      bool
}

func NewDrillIndexer(ctx context.Context, apiAddr string, identityTol float64, dpTol float64, approx bool, errChan chan error) *DrillIndexer {
	return &DrillIndexer{
		Context:     ctx,
		In:          make(chan *GeoDrillRequest, 100),
		Out:         make(chan *GeoDrillGranule, 100),
		Error:       errChan,
		APIAddress:  apiAddr,
		IdentityTol: identityTol,
		DpTol:       dpTol,
		Approx:      approx,
	}
}

type TiledResponse struct {
	IndexerID   int
	NumIndexers int
	IndexerTime time.Duration
	Metadata    *MetadataResponse
}

func (p *DrillIndexer) Run(verbose bool) {
	if verbose {
		defer log.Printf("Drill Indexer done")
	}
	defer close(p.Out)

	isInit := true
	for geoReq := range p.In {
		var feat geo.Feature
		err := json.Unmarshal([]byte(geoReq.Geometry), &feat)
		if err != nil {
			p.sendError(fmt.Errorf("Drill Indexer: Problem unmarshalling GeoJSON object: %v", geoReq.Geometry))
			return
		}

		ns := geoReq.NameSpaces
		if geoReq.Mask != nil {
			for _, v := range geoReq.Mask.IDExpressions.VarList {
				ns = append(ns, v)
			}
		}
		namespaces := strings.Join(ns, ",")

		reqURL := p.getRequestURL(geoReq, namespaces)
		featWKT := feat.Geometry.MarshalWKT()

		if isInit {
			if geoReq.MetricsCollector != nil {
				if len(geoReq.MetricsCollector.Info.Indexer.URL.RawURL) == 0 {
					geoReq.MetricsCollector.Info.Indexer.URL.RawURL = reqURL
				}

				if len(geoReq.MetricsCollector.Info.Indexer.Geometry) == 0 {
					geoReq.MetricsCollector.Info.Indexer.Geometry = featWKT
				}

				if len(geoReq.MetricsCollector.Info.Indexer.SRS) == 0 {
					geoReq.MetricsCollector.Info.Indexer.SRS = geoReq.CRS
				}
			}

			isInit = false
		}

		if verbose {
			logRequestGeometry("Drill Indexer: original request: ", reqURL, featWKT)
		}

		tiledGeoms, err := getTiledGeometries(feat.Geometry.MarshalWKT(), geoReq, verbose)
		if err != nil || len(tiledGeoms) == 0 {
			tiledGeoms = append(tiledGeoms, featWKT)
		}

		if verbose && err != nil {
			log.Printf("Drill Indexer: %v", err)
		}

		tiledResChanSize := len(tiledGeoms)
		if tiledResChanSize > 100 {
			tiledResChanSize = 100
		}

		tiledRes := make(chan *TiledResponse, tiledResChanSize)

		go func() {
			defer close(tiledRes)
			for ig, geomWKT := range tiledGeoms {
				postBody := url.Values{"wkt": {geomWKT}}
				if verbose {
					logRequestGeometry(fmt.Sprintf("Drill Indexer(%d/%d): ", ig+1, len(tiledGeoms)), reqURL, geomWKT)
				}

				start := time.Now()
				resp, err := http.PostForm(reqURL, postBody)
				if err != nil {
					p.sendError(fmt.Errorf("Drill Indexer: POST request to %s failed. Error: %v", reqURL, err))
					continue
				}

				body, err := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					p.sendError(fmt.Errorf("Drill Indexer: Error parsing response body from %s. Error: %v", reqURL, err))
					break
				}

				indexTime := time.Since(start)
				if geoReq.MetricsCollector != nil {
					geoReq.MetricsCollector.Info.Indexer.Duration += indexTime
				}

				var metadata MetadataResponse
				err = json.Unmarshal(body, &metadata)
				if err != nil {
					fmt.Println(string(body))
					p.sendError(fmt.Errorf("Drill Indexer: Problem parsing JSON response from %s. Error: %v", reqURL, err))
					break
				}

				if len(metadata.Error) > 0 {
					fmt.Printf("Drill Indexer error: %v", string(body))
					p.sendError(fmt.Errorf("Drill Indexer error: %v", metadata.Error))
					break
				}

				if p.checkCancellation() {
					return
				}
				tiledRes <- &TiledResponse{IndexerID: ig, NumIndexers: len(tiledGeoms), IndexerTime: indexTime, Metadata: &metadata}
			}
		}()

		var template *jet.Template
		if geoReq.Mask != nil {
			path := "."
			view := jet.NewSet(jet.SafeWriter(func(w io.Writer, b []byte) {
				w.Write(b)
			}), path, "/")

			template, err = view.GetTemplate(geoReq.VRTURL)
			if err != nil {
				log.Printf("Drill Indexer error: %v", err)
				p.sendError(fmt.Errorf("Drill Indexer error: %v", err))
				return
			}
		}

		isFirst := true
		dedupGranules := make(map[string]bool)
		for res := range tiledRes {
			p.processDatasets(res, geoReq, template, dedupGranules, verbose, &isFirst)
			if p.checkCancellation() {
				break
			}
		}

		if geoReq.MetricsCollector != nil {
			geoReq.MetricsCollector.Info.Indexer.NumFiles += len(dedupGranules)
			geoReq.MetricsCollector.Info.Indexer.NumGranules += len(dedupGranules)
		}

		if verbose {
			log.Printf("Drill Indexer: total effective GDAL subdatasets: %v", len(dedupGranules))
		}
	}
}

func (p *DrillIndexer) processDatasets(res *TiledResponse, geoReq *GeoDrillRequest, template *jet.Template, dedupGranules map[string]bool, verbose bool, isFirst *bool) {
	metadata := res.Metadata
	switch len(metadata.GDALDatasets) {
	case 0:
		p.Out <- &GeoDrillGranule{"NULL", utils.EmptyTileNS, "Byte", nil, geoReq.Geometry, geoReq.CRS, "", nil, nil, 0, false, 0, 0, 0, 0, 0, geoReq.MetricsCollector}
	default:
		var grans []*GeoDrillGranule
		var effectiveDatasets []*GDALDataset
		for _, ds := range metadata.GDALDatasets {
			if _, found := dedupGranules[ds.DSName+ds.NameSpace]; found {
				continue
			}
			dedupGranules[ds.DSName+ds.NameSpace] = true

			grans = append(grans, &GeoDrillGranule{ds.DSName, ds.NameSpace, ds.ArrayType, ds.TimeStamps, geoReq.Geometry, geoReq.CRS, "", ds.Means, ds.SampleCounts, ds.NoData, p.Approx, geoReq.ClipUpper, geoReq.ClipLower, geoReq.RasterXSize, geoReq.RasterYSize, geoReq.GrpcConcLimit, geoReq.MetricsCollector})
			effectiveDatasets = append(effectiveDatasets, ds)
		}
		if len(grans) == 0 {
			return
		}

		if verbose {
			log.Printf("Drill Indexer(%d/%d) time: %v, GDAL subdatasets, retrieved: %v, effective: %v", res.IndexerID+1, res.NumIndexers, res.IndexerTime, len(metadata.GDALDatasets), len(grans))
		}

		if geoReq.Mask == nil {
			for _, gran := range grans {
				if p.checkCancellation() {
					return
				}
				p.Out <- gran
			}

		} else {
			granMaskGroups := make(map[string][]*GeoDrillGranule)
			for ids, ds := range effectiveDatasets {
				keyComps := []string{ds.Polygon}
				for _, ts := range ds.TimeStamps {
					keyComps = append(keyComps, fmt.Sprintf("%v", ts))
				}
				key := strings.Join(keyComps, "_")

				granMaskGroups[key] = append(granMaskGroups[key], grans[ids])
			}

			dataNSLookup := make(map[string]bool)
			for _, ns := range geoReq.NameSpaces {
				dataNSLookup[ns] = true
			}

			maskNSLookup := make(map[string]int)
			for iv, ns := range geoReq.Mask.IDExpressions.VarList {
				maskNSLookup[ns] = iv
			}

			for _, granMasks := range granMaskGroups {
				var dataGrans []*GeoDrillGranule
				maskGrans := make([]*GeoDrillGranule, len(maskNSLookup))
				nMaskGrans := 0
				nDataGrans := 0
				for _, gran := range granMasks {
					if nDataGrans < len(dataNSLookup) {
						if _, found := dataNSLookup[gran.NameSpace]; found {
							dataGrans = append(dataGrans, gran)
							nDataGrans++
						}
					}

					if nMaskGrans < len(maskNSLookup) {
						if iv, found := maskNSLookup[gran.NameSpace]; found {
							maskGrans[iv] = gran
							nMaskGrans++
						}
					}
				}

				granErrMsgs := func(grans []*GeoDrillGranule) string {
					var dsPaths []string
					for idg, dg := range grans {
						if idg >= 20 {
							dsPaths = append(dsPaths, " ...")
							break
						}
						dsPaths = append(dsPaths, dg.Path)
					}
					return fmt.Sprintf("data namespaces(%v):%v, data grans(%v):%v", len(dataNSLookup), dataNSLookup, len(grans), strings.Join(dsPaths, ","))

				}
				if nDataGrans != len(dataNSLookup) {
					msg := "Drill Indexer error: duplicated data granules for spatial-temporal hash key"
					msgLog := fmt.Sprintf("%s, %s", msg, granErrMsgs(dataGrans))
					log.Printf(msgLog)
					p.sendError(fmt.Errorf(msg))
					return
				}

				if nMaskGrans != len(maskNSLookup) {
					msg := "Drill Indexer error: duplicated mask granules for spatial-temporal hash key"
					msgLog := fmt.Sprintf("%s, %s", msg, granErrMsgs(maskGrans))
					log.Printf(msgLog)
					p.sendError(fmt.Errorf(msg))
					return
				}

				for _, dg := range dataGrans {

					type GranuleInfo struct {
						Data  *GeoDrillGranule
						Masks []*GeoDrillGranule
					}

					granInfo := &GranuleInfo{Data: dg}
					for _, mg := range maskGrans {
						granInfo.Masks = append(granInfo.Masks, mg)
					}

					var resBuf bytes.Buffer
					vars := make(jet.VarMap)
					if err := template.Execute(&resBuf, vars, granInfo); err != nil {
						msg := fmt.Sprintf("Drill Indexer VRT error: %v", err)
						log.Printf(msg)
						p.sendError(fmt.Errorf(msg))
						return
					}
					dg.VRT = resBuf.String()

					if *isFirst && verbose {
						log.Printf("Drill Indexer VRT: %v, %v", dg.Path, dg.VRT)
						*isFirst = false
					}

					if p.checkCancellation() {
						return
					}
					p.Out <- dg
				}
			}
		}
	}
}

func (p *DrillIndexer) getRequestURL(geoReq *GeoDrillRequest, namespaces string) string {
	startTimeStr := ""
	if !time.Time.IsZero(geoReq.StartTime) {
		startTimeStr = geoReq.StartTime.Format(ISOFormat)
	}
	return strings.Replace(fmt.Sprintf("http://%s%s?intersects&metadata=gdal&time=%s&until=%s&srs=%s&namespace=%s&identitytol=%f&dptol=%f", p.APIAddress, geoReq.Collection, startTimeStr, geoReq.EndTime.Format(ISOFormat), geoReq.CRS, namespaces, p.IdentityTol, p.DpTol), " ", "%20", -1)

}

func (p *DrillIndexer) sendError(err error) {
	select {
	case p.Error <- err:
	default:
	}
}

func (p *DrillIndexer) checkCancellation() bool {
	select {
	case <-p.Context.Done():
		p.sendError(fmt.Errorf("Drill Indexer: context has been cancel: %v", p.Context.Err()))
		return true
	case err := <-p.Error:
		p.sendError(err)
		return true
	default:
		return false
	}
}

func getTiledGeometries(geomWKT string, geoReq *GeoDrillRequest, verbose bool) ([]string, error) {
	if geoReq.IndexTileXSize <= 0.0 && geoReq.IndexTileYSize <= 0.0 {
		return nil, nil
	}

	var tiledGeoms []string
	var err error

	geomC := C.CString(string(geomWKT))
	geomCP := geomC
	var geom C.OGRGeometryH

	// OGR_G_CreateFromWkt intrnally updates &geomC pointer value
	errC := C.OGR_G_CreateFromWkt(&geomC, nil, &geom)
	if errC != C.OGRERR_NONE {
		return nil, fmt.Errorf("Drill Indexer: invalid input geometry")
	}
	C.free(unsafe.Pointer(geomCP))

	geomType := C.OGR_GT_Flatten(C.OGR_G_GetGeometryType(geom))
	hasArea := C.OGR_GT_IsSurface(geomType) != 0 || C.OGR_GT_IsCurve(geomType) != 0 || C.OGR_GT_IsSubClassOf(geomType, C.wkbMultiSurface) != 0 || geomType == C.wkbGeometryCollection
	if !hasArea {
		C.OGR_G_DestroyGeometry(geom)
		return nil, nil
	}

	if geoReq.CRS != DrillTargetSRS {
		srcSRS := C.OSRNewSpatialReference(nil)
		srcSRSC := C.CString(geoReq.CRS)
		C.OSRSetFromUserInput(srcSRS, srcSRSC)
		C.free(unsafe.Pointer(srcSRSC))

		dstSRS := C.OSRNewSpatialReference(nil)
		dstSRSC := C.CString(DrillTargetSRS)
		C.OSRSetFromUserInput(dstSRS, dstSRSC)
		C.free(unsafe.Pointer(dstSRSC))

		C.OSRSetAxisMappingStrategy(srcSRS, C.OAMS_TRADITIONAL_GIS_ORDER)
		trans := C.OCTNewCoordinateTransformation(srcSRS, dstSRS)
		errC := C.OGR_G_Transform(geom, trans)

		C.OCTDestroyCoordinateTransformation(trans)
		C.OSRDestroySpatialReference(dstSRS)
		C.OSRDestroySpatialReference(srcSRS)

		if errC != C.OGRERR_NONE {
			C.OGR_G_DestroyGeometry(geom)
			return nil, fmt.Errorf("Drill Indexer: failed to transform the input geometry from %s to %s", geoReq.CRS, DrillTargetSRS)
		}
	}

	var env C.OGREnvelope
	C.OGR_G_GetEnvelope(geom, &env)
	minX := float64(env.MinX)
	minY := float64(env.MinY)
	maxX := float64(env.MaxX)
	maxY := float64(env.MaxY)

	stepX := geoReq.IndexTileXSize
	stepY := geoReq.IndexTileYSize

	if verbose {
		log.Printf("Drill Indexer: input geometry envelope: (%v, %v), (%v, %v), grid stepX: %v, stepY: %v", minX, minY, maxX, maxY, stepX, stepY)
	}

	for iy := maxY; iy > minY; iy -= stepY {
		for ix := minX; ix < maxX; ix += stepX {
			xMin := ix
			yMax := iy

			xMax := ix + stepX
			if xMax > maxX {
				xMax = maxX
			}

			yMin := iy - stepY
			if yMin < minY {
				yMin = minY
			}

			bbox := BBox2WKT([]float64{xMin, yMin, xMax, yMax})
			bboxC := C.CString(bbox)
			bboxCP := bboxC
			var bboxGeom C.OGRGeometryH
			errC = C.OGR_G_CreateFromWkt(&bboxC, nil, &bboxGeom)
			if errC != C.OGRERR_NONE {
				C.OGR_G_DestroyGeometry(geom)
				return nil, fmt.Errorf("invalid tiled geometry")
			}
			C.free(unsafe.Pointer(bboxCP))

			inters := C.OGR_G_Intersection(geom, bboxGeom)
			C.OGR_G_DestroyGeometry(bboxGeom)
			if inters == nil {
				continue
			}

			if C.OGR_G_IsEmpty(inters) != C.int(0) {
				continue
			}

			var intersWKTC *C.char
			C.OGR_G_ExportToWkt(inters, &intersWKTC)
			intersWKT := C.GoString(intersWKTC)
			C.free(unsafe.Pointer(intersWKTC))
			C.OGR_G_DestroyGeometry(inters)

			tiledGeoms = append(tiledGeoms, intersWKT)
		}
	}

	C.OGR_G_DestroyGeometry(geom)
	return tiledGeoms, err
}

func logRequestGeometry(prefix string, reqURL string, postBodyStr string) {
	maxLogLen := DefaultMaxLogLength
	if len(postBodyStr) < DefaultMaxLogLength {
		maxLogLen = len(postBodyStr)
	}
	log.Printf("%smas_url:%s\tpost_body:%s", prefix, reqURL, postBodyStr[:maxLogLen])
}
