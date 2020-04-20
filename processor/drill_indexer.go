package processor

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

	"github.com/CloudyKit/jet"
	geo "github.com/nci/geometry"
	"github.com/nci/gsky/utils"
)

// ISOFormat is the string used to format Go ISO times
const ISOFormat = "2006-01-02T15:04:05.000Z"

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

const DefaultMaxLogLength = 3000

func (p *DrillIndexer) Run(verbose bool) {
	defer close(p.Out)
	t0 := time.Now()
	isInit := true
	for geoReq := range p.In {
		var feat geo.Feature
		err := json.Unmarshal([]byte(geoReq.Geometry), &feat)
		if err != nil {
			p.Error <- fmt.Errorf("Problem unmarshalling GeoJSON object: %v", geoReq.Geometry)
			return
		}

		ns := geoReq.NameSpaces
		if geoReq.Mask != nil {
			for _, v := range geoReq.Mask.IDExpressions.VarList {
				ns = append(ns, v)
			}
		}
		namespaces := strings.Join(ns, ",")

		start := time.Now()
		startTimeStr := ""
		if !time.Time.IsZero(geoReq.StartTime) {
			startTimeStr = geoReq.StartTime.Format(ISOFormat)
		}
		reqURL := strings.Replace(fmt.Sprintf("http://%s%s?intersects&metadata=gdal&time=%s&until=%s&srs=%s&namespace=%s&identitytol=%f&dptol=%f", p.APIAddress, geoReq.Collection, startTimeStr, geoReq.EndTime.Format(ISOFormat), geoReq.CRS, namespaces, p.IdentityTol, p.DpTol), " ", "%20", -1)
		featWKT := feat.Geometry.MarshalWKT()
		postBody := url.Values{"wkt": {featWKT}}

		postBodyStr := fmt.Sprintf("%v", postBody)
		maxLogLen := DefaultMaxLogLength
		if len(postBodyStr) < DefaultMaxLogLength {
			maxLogLen = len(postBodyStr)
		}
		if verbose {
			log.Printf("mas_url:%s\tpost_body:%s", reqURL, postBodyStr[:maxLogLen])
		}

		resp, err := http.PostForm(reqURL, postBody)
		if err != nil {
			p.Error <- fmt.Errorf("POST request to %s failed. Error: %v", reqURL, err)
			continue
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			p.Error <- fmt.Errorf("Error parsing response body from %s. Error: %v", reqURL, err)
			continue
		}

		indexTime := time.Since(start)

		var metadata MetadataResponse
		err = json.Unmarshal(body, &metadata)
		if err != nil {
			fmt.Println(string(body))
			p.Error <- fmt.Errorf("Problem parsing JSON response from %s. Error: %v", reqURL, err)
			continue
		}

		if verbose {
			log.Printf("Indexer time: %v, gdal subdatasets: %v", indexTime, len(metadata.GDALDatasets))
		}
		if len(metadata.Error) > 0 {
			fmt.Printf("Indexer returned error: %v", string(body))
			p.Error <- fmt.Errorf("Indexer returned error: %v", metadata.Error)
			continue
		}

		if isInit {
			if geoReq.MetricsCollector != nil {
				defer func() { geoReq.MetricsCollector.Info.Indexer.Duration += time.Since(t0) }()
				if len(geoReq.MetricsCollector.Info.Indexer.URL.RawURL) == 0 {
					geoReq.MetricsCollector.Info.Indexer.URL.RawURL = reqURL
				}

				if len(geoReq.MetricsCollector.Info.Indexer.Geometry) == 0 {
					geoReq.MetricsCollector.Info.Indexer.Geometry = postBody["wkt"][0]
				}

				if len(geoReq.MetricsCollector.Info.Indexer.SRS) == 0 {
					geoReq.MetricsCollector.Info.Indexer.SRS = "EPSG:4326"
				}
				geoReq.MetricsCollector.Info.Indexer.NumFiles += len(metadata.GDALDatasets)
				geoReq.MetricsCollector.Info.Indexer.NumGranules += len(metadata.GDALDatasets)
			}

			isInit = false
		}

		switch len(metadata.GDALDatasets) {
		case 0:
			p.Out <- &GeoDrillGranule{"NULL", utils.EmptyTileNS, "Byte", nil, geoReq.Geometry, geoReq.CRS, "", nil, nil, 0, false, 0, 0, geoReq.MetricsCollector}
		default:
			var grans []*GeoDrillGranule
			for _, ds := range metadata.GDALDatasets {
				grans = append(grans, &GeoDrillGranule{ds.DSName, ds.NameSpace, ds.ArrayType, ds.TimeStamps, geoReq.Geometry, geoReq.CRS, "", ds.Means, ds.SampleCounts, ds.NoData, p.Approx, geoReq.ClipUpper, geoReq.ClipLower, geoReq.MetricsCollector})
			}

			if geoReq.Mask == nil {
				for _, gran := range grans {
					p.Out <- gran
				}

			} else {
				granMaskGroups := make(map[string][]*GeoDrillGranule)
				for ids, ds := range metadata.GDALDatasets {
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

				path := "."
				view := jet.NewSet(jet.SafeWriter(func(w io.Writer, b []byte) {
					w.Write(b)
				}), path, "/")

				template, err := view.GetTemplate(geoReq.VRTURL)
				if err != nil {
					log.Printf("Indexer returned error: %v", err)
					p.Error <- fmt.Errorf("Indexer returned error: %v", err)
					return
				}

				isFirst := true
				for _, granMasks := range granMaskGroups {
					var dataGrans []*GeoDrillGranule
					maskGrans := make([]*GeoDrillGranule, len(maskNSLookup))
					nMaskGrans := 0
					nDataGrans := 0
					for _, gran := range granMasks {
						if _, found := dataNSLookup[gran.NameSpace]; found {
							dataGrans = append(dataGrans, gran)
							nDataGrans++
						}

						if iv, found := maskNSLookup[gran.NameSpace]; found {
							maskGrans[iv] = gran
							nMaskGrans++
						}
					}

					for _, dg := range dataGrans {
						if nDataGrans != len(dataNSLookup) {
							msg := fmt.Sprintf("Indexer returned error: %v, data namespaces=%v, data grans=%v", dg.Path, len(dataNSLookup), nDataGrans)
							log.Printf(msg)
							p.Error <- fmt.Errorf(msg)
							return
						}
						if nMaskGrans != len(maskNSLookup) {
							msg := fmt.Sprintf("Indexer returned error: %v, mask namespaces=%v, mask grans=%v", dg.Path, len(maskNSLookup), nMaskGrans)
							log.Printf(msg)
							p.Error <- fmt.Errorf(msg)
							return
						}

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
						if err = template.Execute(&resBuf, vars, granInfo); err != nil {
							log.Printf("Indexer VRT error: %v", err)
							p.Error <- fmt.Errorf("Indexer VRT error: %v", err)
							return
						}
						dg.VRT = resBuf.String()

						if isFirst && verbose {
							log.Printf("vrt: %v, %v", dg.Path, dg.VRT)
							isFirst = false
						}

						p.Out <- dg
					}
				}
			}
		}
	}
}
