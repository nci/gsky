package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type DatasetAxis struct {
	Name               string    `json:"name"`
	Params             []float64 `json:"params"`
	Strides            []int     `json:"strides"`
	Shape              []int     `json:"shape"`
	Grid               string    `json:"grid"`
	IntersectionIdx    []int
	IntersectionValues []float64
	Order              int
	Aggregate          int
}

type GDALDataset struct {
	DSName       string         `json:"ds_name"`
	NameSpace    string         `json:"namespace"`
	ArrayType    string         `json:"array_type"`
	TimeStamps   []time.Time    `json:"timestamps"`
	Polygon      string         `json:"polygon"`
	Means        []float64      `json:"means"`
	SampleCounts []int          `json:"sample_counts"`
	NoData       float64        `json:"nodata"`
	Axes         []*DatasetAxis `json:"axes"`
	IsOutRange   bool
}

type MetadataResponse struct {
	Error        string         `json:"error"`
	Files        []string       `json:"files"`
	GDALDatasets []*GDALDataset `json:"gdal"`
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

			for key, axis := range geoReq.Axes {
				if key == "time" {
					if len(axis.InValues) > 0 {
						var minVal, maxVal float64
						for i, val := range axis.InValues {
							if i == 0 {
								minVal = val
								maxVal = val
							} else {
								if val < minVal {
									minVal = val
								} else if val > maxVal {
									maxVal = val
								}
							}
						}

						minTime := time.Unix(int64(minVal), 0)
						geoReq.StartTime = &minTime

						maxTime := time.Unix(int64(maxVal), 0)
						if maxVal > minVal {
							geoReq.EndTime = &maxTime
						}
					} else {
						if axis.Start != nil {
							minTime := time.Unix(int64(*axis.Start), 0)
							geoReq.StartTime = &minTime
						}

						if axis.End != nil {
							maxTime := time.Unix(int64(*axis.End), 0)
							geoReq.EndTime = &maxTime
						}
					}
					break
				}
			}

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
		/*
			geoReq.Axes = make(map[string]*GeoTileAxis)

			_s := 12332.22
			geoReq.Axes["time"] = &GeoTileAxis{Start: &_s, Aggregate: 1}

			_t1 := 1100.0
			_t2 := 5000.0
			//geoReq.Axes["level"] = &GeoTileAxis{Start: &_t1, End: &_t2, Order: 1, Aggregate: 0}

			geoReq.Axes["level"] = &GeoTileAxis{Start: &_t1, End: &_t2, Order: 1, Aggregate: 0, InValues: []float64 {1100, 4000}}

			//_t1 := 3500.0
			//geoReq.Axes["level"] = &GeoTileAxis{Start: &_t1, Order: 1, Aggregate: 1}
		*/

		for ids, ds := range metadata.GDALDatasets {
			if ids >= 20 {
				break
			} // debug
			if len(ds.TimeStamps) < 32 {
				continue
			} //debug

			isOutRange := false
			for ia, axis := range ds.Axes {
				tileAxis, found := geoReq.Axes[axis.Name]
				if found {
					if axis.Name == "time" && (tileAxis.Start != nil || len(tileAxis.InValues) > 0) {
						axis.Grid = "enum"
						for _, t := range ds.TimeStamps {
							axis.Params = append(axis.Params, float64(t.Unix()))
						}
						ds.Axes[ia] = axis
					}

					axis.Order = tileAxis.Order
					axis.Aggregate = tileAxis.Aggregate

					if axis.Grid == "enum" {
						if len(axis.Params) > 1 {
							if len(tileAxis.InValues) > 0 || (tileAxis.Start != nil && tileAxis.End == nil) {
								iVal := 0
								var startVal float64
								var nVals int

								if len(tileAxis.InValues) > 0 {
									sort.Slice(tileAxis.InValues, func(i, j int) bool { return tileAxis.InValues[i] <= tileAxis.InValues[j] })
									startVal = tileAxis.InValues[0]
									nVals = len(tileAxis.InValues)
								} else {
									startVal = *tileAxis.Start
									nVals = 1
								}
								if startVal < axis.Params[0] || startVal > axis.Params[len(axis.Params)-1] {
									isOutRange = true
									break
								}

								for iv, val := range axis.Params {
									if val >= startVal {
										foundVal := false
										axisIdx := 0
										if math.Abs(startVal-axis.Params[iv-1]) <= math.Abs(startVal-val) {
											axisIdx = iv - 1
											foundVal = true
										} else {
											axisIdx = iv
											foundVal = true
										}

										if foundVal {
											axis.IntersectionIdx = append(axis.IntersectionIdx, axisIdx)
											axis.IntersectionValues = append(axis.IntersectionValues, axis.Params[axisIdx])

											iVal++
											if iVal >= nVals {
												break
											}

											startVal = tileAxis.InValues[iVal]
										}
									}
								}

							} else if tileAxis.Start != nil && tileAxis.End != nil {
								startVal := *tileAxis.Start
								endVal := *tileAxis.End
								if endVal < axis.Params[0] || startVal > axis.Params[len(axis.Params)-1] {
									isOutRange = true
									break
								}
								for iv, val := range axis.Params {
									if val >= startVal && val < endVal {
										axis.IntersectionIdx = append(axis.IntersectionIdx, iv)
										axis.IntersectionValues = append(axis.IntersectionValues, axis.Params[iv])
									}
								}

							}

						}
					} else if axis.Grid == "default" {
						for it, t := range ds.TimeStamps {
							if t.Equal(*geoReq.StartTime) || geoReq.EndTime != nil && t.After(*geoReq.StartTime) && t.Before(*geoReq.EndTime) {
								axis.IntersectionIdx = append(axis.IntersectionIdx, it)
								axis.IntersectionValues = append(axis.IntersectionValues, float64(ds.TimeStamps[it].Unix()))
							}
						}

						if len(axis.IntersectionIdx) == 0 {
							isOutRange = true
							break
						}
					} else {
						errChan <- fmt.Errorf("Indexer error: unknown axis grid type: %v", axis.Grid)
						return
					}

					for i := range axis.IntersectionIdx {
						axis.IntersectionIdx[i] *= axis.Strides[0]
					}
				} else {
					axis.IntersectionIdx = append(axis.IntersectionIdx, 0)
					if axis.Grid == "enum" {
						axis.IntersectionValues = append(axis.IntersectionValues, axis.Params[0])
					} else {
						errChan <- fmt.Errorf("Indexer error: unknown axis grid type: %v", axis.Grid)
						return
					}
				}
			}

			if isOutRange {
				ds.IsOutRange = true
				continue
			}

		}

		bandNameSpaces := make(map[string]map[string]float64)
		var granList []*GeoTileGranule
		for ids, ds := range metadata.GDALDatasets {
			if ids >= 20 {
				break
			} // debug
			if len(ds.TimeStamps) < 32 {
				continue
			} //debug

			if ds.IsOutRange {
				continue
			}

			log.Printf("%v, %v, %v", *ds.Axes[0], *ds.Axes[1], len(ds.TimeStamps))
			axisIdxCnt := make([]int, len(ds.Axes))

			for axisIdxCnt[0] < len(ds.Axes[0].IntersectionIdx) {
				bandIdx := 1
				aggTimeStamp := 0.0
				bandTimeStamp := 0.0

				namespace := ds.NameSpace
				isFirst := true
				hasNonAgg := false
				for i := 0; i < len(axisIdxCnt); i++ {
					bandIdx += ds.Axes[i].IntersectionIdx[axisIdxCnt[i]]

					iTimeStamp := axisIdxCnt[i]
					bandTimeStamp += ds.Axes[i].IntersectionValues[iTimeStamp]

					if ds.Axes[i].Order != 0 {
						iTimeStamp = len(ds.Axes[i].IntersectionIdx) - axisIdxCnt[i] - 1
					}
					aggTimeStamp += ds.Axes[i].IntersectionValues[iTimeStamp]

					if len(ds.Axes[i].IntersectionIdx) > 1 && ds.Axes[i].Aggregate == 0 {
						if isFirst {
							namespace += "#"
							isFirst = false
						} else {
							namespace += ","
						}

						var readableNs string
						if ds.Axes[i].Name == "time" {
							ts := time.Unix(int64(ds.Axes[i].IntersectionValues[axisIdxCnt[i]]), 0)
							readableNs = fmt.Sprintf("%v", ts.Format(ISOFormat))
						} else {
							readableNs = fmt.Sprintf("%v", ds.Axes[i].IntersectionValues[axisIdxCnt[i]])
						}
						namespace += fmt.Sprintf("%s=%v", ds.Axes[i].Name, readableNs)
						hasNonAgg = true
					}
				}

				if hasNonAgg {
					_, found := bandNameSpaces[ds.NameSpace]
					if !found {
						bandNameSpaces[ds.NameSpace] = make(map[string]float64)
					}

					_, bFound := bandNameSpaces[ds.NameSpace][namespace]
					if !bFound {
						bandNameSpaces[ds.NameSpace][namespace] = bandTimeStamp
					}
				}

				gran := &GeoTileGranule{ConfigPayLoad: geoReq.ConfigPayLoad, Path: ds.DSName, NameSpace: namespace, VarNameSpace: ds.NameSpace, RasterType: ds.ArrayType, TimeStamp: float64(aggTimeStamp), BandIdx: bandIdx, Polygon: ds.Polygon, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, CRS: geoReq.CRS}

				log.Printf("    %v, %v,%v,%v,%v   %v", axisIdxCnt, bandIdx, aggTimeStamp, bandTimeStamp, namespace, len(ds.TimeStamps))

				//out <- gran
				granList = append(granList, gran)

				ia := len(ds.Axes) - 1
				axisIdxCnt[ia]++
				for ; ia > 0 && axisIdxCnt[ia] >= len(ds.Axes[ia].IntersectionIdx); ia-- {
					axisIdxCnt[ia] = 0
					axisIdxCnt[ia-1]++
				}
			}

		}

		type _BandNs struct {
			Name string
			Val  float64
		}

		var sortedNameSpaces []string
		for _, ns := range geoReq.NameSpaces {
			bands, found := bandNameSpaces[ns]
			if found {
				bandNsList := make([]*_BandNs, len(bands))
				ib := 0
				for bns, val := range bands {
					bandNsList[ib] = &_BandNs{Name: bns, Val: val}
					ib++
				}

				sort.Slice(bandNsList, func(i, j int) bool { return bandNsList[i].Val <= bandNsList[j].Val })

				for _, bns := range bandNsList {
					sortedNameSpaces = append(sortedNameSpaces, bns.Name)
				}
			} else {
				sortedNameSpaces = append(sortedNameSpaces, ns)
			}
		}

		log.Printf("%#v", bandNameSpaces)
		log.Printf("%#v", sortedNameSpaces)

		newConfigPayLoad := geoReq.ConfigPayLoad
		newConfigPayLoad.NameSpaces = sortedNameSpaces

		log.Printf("%v, %v", geoReq.ConfigPayLoad, newConfigPayLoad)

		log.Printf("total grans: %d", len(granList))

		for _, gran := range granList {
			gran.ConfigPayLoad = newConfigPayLoad
			out <- gran
		}
	}
}
