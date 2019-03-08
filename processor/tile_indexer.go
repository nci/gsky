package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"math"
)

type DatasetAxis struct {
	Name   string `json:"name"`
	Params []float64 `json:"params"`
	Strides []int `json:"strides"`
	Shape  []int `json:"shape"`
	Grid   string    `json:"grid"`
	IntersectionIdx []int
	Order           int
	Aggregate       int
}

type GDALDataset struct {
	DSName       string      `json:"ds_name"`
	NameSpace    string      `json:"namespace"`
	ArrayType    string      `json:"array_type"`
	TimeStamps   []time.Time `json:"timestamps"`
	Polygon      string      `json:"polygon"`
	Means        []float64   `json:"means"`
	SampleCounts []int       `json:"sample_counts"`
	NoData       float64     `json:"nodata"`
	Axes         []*DatasetAxis `json:"axes"`
}

type MetadataResponse struct {
	Error        string        `json:"error"`
	Files        []string      `json:"files"`
	GDALDatasets []GDALDataset `json:"gdal"`
}

type TileIndexer struct {
	Context    context.Context
	In         chan *GeoTileRequest
	Out        chan *GeoTileGranule
	Error      chan error
	APIAddress string
	QueryLimit int
}

func NewTileIndexer(ctx context.Context, apiAddr string, errChan chan error) *TileIndexer {
	return &TileIndexer{
		Context:    ctx,
		In:         make(chan *GeoTileRequest, 100),
		Out:        make(chan *GeoTileGranule, 100),
		Error:      errChan,
		APIAddress: apiAddr,
	}
}

func BBox2WKT(bbox []float64) string {
	// BBox xMin, yMin, xMax, yMax
	return fmt.Sprintf("POLYGON ((%f %f, %f %f, %f %f, %f %f, %f %f))", bbox[0], bbox[1], bbox[2], bbox[1], bbox[2], bbox[3], bbox[0], bbox[3], bbox[0], bbox[1])
}

func (p *TileIndexer) Run(verbose bool) {
	if verbose {
		defer log.Printf("tile indexer done")
	}
	defer close(p.Out)

	for geoReq := range p.In {
		select {
		case <-p.Context.Done():
			p.Error <- fmt.Errorf("Tile indexer context has been cancel: %v", p.Context.Err())
			return
		default:
			var wg sync.WaitGroup
			var url string

			if len(geoReq.NameSpaces) == 0 {
				geoReq.NameSpaces = append(geoReq.NameSpaces, "")
			}

			nameSpaces := strings.Join(geoReq.NameSpaces, ",")
			if geoReq.EndTime == nil {
				url = strings.Replace(fmt.Sprintf("http://%s%s?intersects&metadata=gdal&time=%s&srs=%s&wkt=%s&namespace=%s&nseg=%d&limit=%d", p.APIAddress, geoReq.Collection, geoReq.StartTime.Format(ISOFormat), geoReq.CRS, BBox2WKT(geoReq.BBox), nameSpaces, geoReq.PolygonSegments, geoReq.QueryLimit), " ", "%20", -1)
			} else {
				url = strings.Replace(fmt.Sprintf("http://%s%s?intersects&metadata=gdal&time=%s&until=%s&srs=%s&wkt=%s&namespace=%s&nseg=%d&limit=%d", p.APIAddress, geoReq.Collection, geoReq.StartTime.Format(ISOFormat), geoReq.EndTime.Format(ISOFormat), geoReq.CRS, BBox2WKT(geoReq.BBox), nameSpaces, geoReq.PolygonSegments, geoReq.QueryLimit), " ", "%20", -1)
			}
			if verbose {
				log.Println(url)
			}

			wg.Add(1)
			go URLIndexGet(p.Context, url, geoReq, p.Error, p.Out, &wg, verbose)
			if geoReq.Mask != nil {
				maskCollection := geoReq.Mask.DataSource
				if len(maskCollection) == 0 {
					maskCollection = geoReq.Collection
				}

				if maskCollection != geoReq.Collection || geoReq.Mask.ID != nameSpaces {
					if geoReq.EndTime == nil {
						url = strings.Replace(fmt.Sprintf("http://%s%s?intersects&metadata=gdal&time=%s&srs=%s&wkt=%s&namespace=%s&nseg=%d&limit=%d", p.APIAddress, maskCollection, geoReq.StartTime.Format(ISOFormat), geoReq.CRS, BBox2WKT(geoReq.BBox), geoReq.Mask.ID, geoReq.PolygonSegments, geoReq.QueryLimit), " ", "%20", -1)
					} else {
						url = strings.Replace(fmt.Sprintf("http://%s%s?intersects&metadata=gdal&time=%s&until=%s&srs=%s&wkt=%s&namespace=%s&nseg=%d&limit=%d", p.APIAddress, maskCollection, geoReq.StartTime.Format(ISOFormat), geoReq.EndTime.Format(ISOFormat), geoReq.CRS, BBox2WKT(geoReq.BBox), geoReq.Mask.ID, geoReq.PolygonSegments, geoReq.QueryLimit), " ", "%20", -1)
					}
					if verbose {
						log.Println(url)
					}

					wg.Add(1)
					go URLIndexGet(p.Context, url, geoReq, p.Error, p.Out, &wg, verbose)
				}
			}
			wg.Wait()
		}
	}
}

func URLIndexGet(ctx context.Context, url string, geoReq *GeoTileRequest, errChan chan error, out chan *GeoTileGranule, wg *sync.WaitGroup, verbose bool) {
	defer wg.Done()

	resp, err := http.Get(url)
	if err != nil {
		errChan <- fmt.Errorf("GET request to %s failed. Error: %v", url, err)
		out <- &GeoTileGranule{ConfigPayLoad: ConfigPayLoad{NameSpaces: []string{"EmptyTile"}, ScaleParams: geoReq.ScaleParams, Palette: geoReq.Palette}, Path: "NULL", NameSpace: "EmptyTile", RasterType: "Byte", TimeStamp: 0, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, OffX: geoReq.OffX, OffY: geoReq.OffY, CRS: geoReq.CRS}
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errChan <- fmt.Errorf("Error parsing response body from %s. Error: %v", url, err)
		out <- &GeoTileGranule{ConfigPayLoad: ConfigPayLoad{NameSpaces: []string{"EmptyTile"}, ScaleParams: geoReq.ScaleParams, Palette: geoReq.Palette}, Path: "NULL", NameSpace: "EmptyTile", RasterType: "Byte", TimeStamp: 0, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, OffX: geoReq.OffX, OffY: geoReq.OffY, CRS: geoReq.CRS}
		return
	}
	var metadata MetadataResponse
	err = json.Unmarshal(body, &metadata)
	if err != nil {
		errChan <- fmt.Errorf("Problem parsing JSON response from %s. Error: %v", url, err)
		out <- &GeoTileGranule{ConfigPayLoad: ConfigPayLoad{NameSpaces: []string{"EmptyTile"}, ScaleParams: geoReq.ScaleParams, Palette: geoReq.Palette}, Path: "NULL", NameSpace: "EmptyTile", RasterType: "Byte", TimeStamp: 0, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, OffX: geoReq.OffX, OffY: geoReq.OffY, CRS: geoReq.CRS}
		return
	}

	if verbose {
		log.Printf("tile indexer: %d files", len(metadata.GDALDatasets))
	}

	switch len(metadata.GDALDatasets) {
	case 0:
		if len(metadata.Error) > 0 {
			log.Printf("Indexer returned error: %v", string(body))
			errChan <- fmt.Errorf("Indexer returned error: %v", metadata.Error)
		}
		out <- &GeoTileGranule{ConfigPayLoad: ConfigPayLoad{NameSpaces: []string{"EmptyTile"}, ScaleParams: geoReq.ScaleParams, Palette: geoReq.Palette}, Path: "NULL", NameSpace: "EmptyTile", RasterType: "Byte", TimeStamp: 0, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, OffX: geoReq.OffX, OffY: geoReq.OffY, CRS: geoReq.CRS}
	default:
		geoReq.Axes = make(map[string]*GeoTileAxis)

		_s := 12332.22
		geoReq.Axes["time"] = &GeoTileAxis{Start: &_s}

		_t1 := 1100.0
		_t2 := 5000.0
		geoReq.Axes["level"] = &GeoTileAxis{Start: &_t1, End: &_t2, Order: 1}

		for _, ds := range metadata.GDALDatasets {
			if len(ds.TimeStamps) < 32 { continue } //debug

			isOutRange := false
			for _, axis := range ds.Axes {
				tileAxis, found := geoReq.Axes[axis.Name]
				axis.Order = tileAxis.Order
				axis.Aggregate = tileAxis.Aggregate
				if found {
					if axis.Grid == "enum" {
						if len(axis.Params) > 1 {
							if tileAxis.Start != nil && tileAxis.End == nil {
								startVal := *tileAxis.Start
								if startVal < axis.Params[0] || startVal > axis.Params[len(axis.Params)-1] {
									isOutRange = true
									break
								}

								axisIdx := 0
								for iv, val := range axis.Params {
									if startVal >= val {
										if iv == len(axis.Params) - 1 {
											axisIdx = iv
											break
										}							

										if math.Abs(startVal - val) <= math.Abs(startVal - axis.Params[iv+1]) {
											axisIdx = iv
											break
										} else {
											axisIdx = iv + 1
											break
										}
									}
								}

								axis.IntersectionIdx = append(axis.IntersectionIdx,  axisIdx)
							} else if tileAxis.Start != nil && tileAxis.End != nil {
								startVal := *tileAxis.Start
								endVal := *tileAxis.End
								if endVal < axis.Params[0] || startVal > axis.Params[len(axis.Params)-1] {
									isOutRange = true
									break
								}
								for iv, val := range axis.Params {
									if val >= startVal && val < endVal {
										axis.IntersectionIdx = append(axis.IntersectionIdx,  iv)
									}
								}

							}

						}
					} else if axis.Grid == "default" {
						for it, t := range ds.TimeStamps {
							if t.Equal(*geoReq.StartTime) || geoReq.EndTime != nil && t.After(*geoReq.StartTime) && t.Before(*geoReq.EndTime) {
								axis.IntersectionIdx = append(axis.IntersectionIdx,  it)
							}
						}

						if len(axis.IntersectionIdx) == 0 {
							isOutRange = true
							break
						}
					}

					for i := range axis.IntersectionIdx {
						axis.IntersectionIdx[i] *= axis.Strides[0]
					}
				}
			}

			if isOutRange {
				continue
			}

			log.Printf("%v, %v, %v", *ds.Axes[0], *ds.Axes[1], len(ds.TimeStamps))
			axisIdxCnt := make([]int, len(ds.Axes))
			timeStampStrides := make([]int, len(ds.Axes))
			timeStampStrides[len(ds.Axes)-1] = 1

			for i := len(ds.Axes) - 2; i >= 0; i-- {
				timeStampStrides[i] = timeStampStrides[i+1] * len(ds.Axes[i+1].IntersectionIdx)
			}

			totalTimeStamps := 0
			for i := 0; i < len(ds.Axes); i++ {
				totalTimeStamps += len(ds.Axes[i].IntersectionIdx)
			}

			for axisIdxCnt[0] < len(ds.Axes[0].IntersectionIdx) {
				bandIdx := 1
				timeStamp := 0
				for i := 0; i < len(axisIdxCnt); i++ {
					bandIdx += ds.Axes[i].IntersectionIdx[axisIdxCnt[i]]

					iTimeStamp := axisIdxCnt[i]
					if ds.Axes[i].Order != 0 {
						iTimeStamp = len(ds.Axes[i].IntersectionIdx) - axisIdxCnt[i] - 1
					}

					timeStamp += iTimeStamp * timeStampStrides[i]
				}

				timeStamp = totalTimeStamps - timeStamp

				log.Printf("    %v, %v, %v   %v", axisIdxCnt, bandIdx, timeStamp, len(ds.TimeStamps))

				out <- &GeoTileGranule{ConfigPayLoad: geoReq.ConfigPayLoad, Path: ds.DSName, NameSpace: ds.NameSpace, RasterType: ds.ArrayType, TimeStamp: float64(timeStamp), BandIdx: bandIdx, Polygon: ds.Polygon, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, CRS: geoReq.CRS}

				ia := len(ds.Axes) - 1
				axisIdxCnt[ia]++
				for ; ia > 0 && axisIdxCnt[ia] >= len(ds.Axes[ia].IntersectionIdx); ia-- {
					axisIdxCnt[ia] = 0
					axisIdxCnt[ia-1]++
				}
			}

			/*
			for _, axis := range ds.Axes {
				if axis.Name != "time" { continue }

				bandIdx := 0
				for _, idx := range axis.IntersectionIdx {
					bandIdx = idx + 1
					timeStamp := float64(idx)
					if axis.Order != 0 {
						timeStamp = -timeStamp
					}

					out <- &GeoTileGranule{ConfigPayLoad: geoReq.ConfigPayLoad, Path: ds.DSName, NameSpace: ds.NameSpace, RasterType: ds.ArrayType, TimeStamp: timeStamp, BandIdx: bandIdx, Polygon: ds.Polygon, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, CRS: geoReq.CRS}
				}
			}
			*/

			//log.Printf("%v, %v, %v", *ds.Axes[0], *ds.Axes[1], len(ds.TimeStamps))

			/*
			for _, t := range ds.TimeStamps {
				if t.Equal(*geoReq.StartTime) || geoReq.EndTime != nil && t.After(*geoReq.StartTime) && t.Before(*geoReq.EndTime) {
					out <- &GeoTileGranule{ConfigPayLoad: geoReq.ConfigPayLoad, Path: ds.DSName, NameSpace: ds.NameSpace, RasterType: ds.ArrayType, TimeStamps: ds.TimeStamps, TimeStamp: t, Polygon: ds.Polygon, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, OffX: geoReq.OffX, OffY: geoReq.OffY, CRS: geoReq.CRS}
				}
			}
			*/
		}
	}
}
