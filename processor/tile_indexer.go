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
)

type GDALDataset struct {
	DSName       string      `json:"ds_name"`
	NameSpace    string      `json:"namespace"`
	ArrayType    string      `json:"array_type"`
	TimeStamps   []time.Time `json:"timestamps"`
	Polygon      string      `json:"polygon"`
	Means        []float64   `json:"means"`
	SampleCounts []int       `json:"sample_counts"`
	NoData       float64     `json:"nodata"`
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

func (p *TileIndexer) Run() {
	defer close(p.Out)
	for geoReq := range p.In {
		select {
		case <-p.Context.Done():
			p.Error <- fmt.Errorf("Tile indexer context has been cancel: %v", p.Context.Err())
			return
		default:
			xRes := (geoReq.BBox[2] - geoReq.BBox[0]) / float64(geoReq.Width)
			if geoReq.ZoomLimit != 0.0 && xRes > geoReq.ZoomLimit {
				p.Out <- &GeoTileGranule{ConfigPayLoad: ConfigPayLoad{NameSpaces: []string{"OutOfZoom"}, Palette: geoReq.Palette}, Path: "NULL", NameSpace: "OutOfZoom", RasterType: "Byte", TimeStamps: nil, TimeStamp: *geoReq.StartTime, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, OffX: geoReq.OffX, OffY: geoReq.OffY, CRS: geoReq.CRS}
				// TODO this needs a proper fix
				// If the index returns too quick the gRPC connections are not ready yet
				time.Sleep(10 * time.Millisecond)
				continue
			}

			var wg sync.WaitGroup
			var url string

			if len(geoReq.NameSpaces) == 0 {
				geoReq.NameSpaces = append(geoReq.NameSpaces, "")
			}

			nameSpaces := strings.Join(geoReq.NameSpaces, ",")
			if geoReq.EndTime == nil {
				url = strings.Replace(fmt.Sprintf("http://%s%s?intersects&metadata=gdal&time=%s&srs=%s&wkt=%s&namespace=%s&nseg=%d", p.APIAddress, geoReq.Collection, geoReq.StartTime.Format(ISOFormat), geoReq.CRS, BBox2WKT(geoReq.BBox), nameSpaces, geoReq.PolygonSegments), " ", "%20", -1)
			} else {
				url = strings.Replace(fmt.Sprintf("http://%s%s?intersects&metadata=gdal&time=%s&until=%s&srs=%s&wkt=%s&namespace=%s&nseg=%d", p.APIAddress, geoReq.Collection, geoReq.StartTime.Format(ISOFormat), geoReq.EndTime.Format(ISOFormat), geoReq.CRS, BBox2WKT(geoReq.BBox), nameSpaces, geoReq.PolygonSegments), " ", "%20", -1)
			}
			log.Println(url)

			wg.Add(1)
			go URLIndexGet(p.Context, url, geoReq, p.Error, p.Out, &wg)
			if geoReq.Mask != nil {
				maskCollection := geoReq.Mask.DataSource
				if len(maskCollection) == 0 {
					maskCollection = geoReq.Collection
				}

				if maskCollection != geoReq.Collection || geoReq.Mask.ID != nameSpaces {
					if geoReq.EndTime == nil {
						url = strings.Replace(fmt.Sprintf("http://%s%s?intersects&metadata=gdal&time=%s&srs=%s&wkt=%s&namespace=%s&nseg=%d", p.APIAddress, maskCollection, geoReq.StartTime.Format(ISOFormat), geoReq.CRS, BBox2WKT(geoReq.BBox), geoReq.Mask.ID, geoReq.PolygonSegments), " ", "%20", -1)
					} else {
						url = strings.Replace(fmt.Sprintf("http://%s%s?intersects&metadata=gdal&time=%s&until=%s&srs=%s&wkt=%s&namespace=%s&nseg=%d", p.APIAddress, maskCollection, geoReq.StartTime.Format(ISOFormat), geoReq.EndTime.Format(ISOFormat), geoReq.CRS, BBox2WKT(geoReq.BBox), geoReq.Mask.ID, geoReq.PolygonSegments), " ", "%20", -1)
					}
					log.Println(url)

					wg.Add(1)
					go URLIndexGet(p.Context, url, geoReq, p.Error, p.Out, &wg)
				}
			}
			wg.Wait()
		}
	}
}

func URLIndexGet(ctx context.Context, url string, geoReq *GeoTileRequest, errChan chan error, out chan *GeoTileGranule, wg *sync.WaitGroup) {
	defer wg.Done()

	resp, err := http.Get(url)
	if err != nil {
		errChan <- fmt.Errorf("GET request to %s failed. Error: %v", url, err)
		out <- &GeoTileGranule{ConfigPayLoad: ConfigPayLoad{NameSpaces: []string{"EmptyTile"}, ScaleParams: geoReq.ScaleParams, Palette: geoReq.Palette}, Path: "NULL", NameSpace: "EmptyTile", RasterType: "Byte", TimeStamps: nil, TimeStamp: *geoReq.StartTime, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, OffX: geoReq.OffX, OffY: geoReq.OffY, CRS: geoReq.CRS}
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errChan <- fmt.Errorf("Error parsing response body from %s. Error: %v", url, err)
		out <- &GeoTileGranule{ConfigPayLoad: ConfigPayLoad{NameSpaces: []string{"EmptyTile"}, ScaleParams: geoReq.ScaleParams, Palette: geoReq.Palette}, Path: "NULL", NameSpace: "EmptyTile", RasterType: "Byte", TimeStamps: nil, TimeStamp: *geoReq.StartTime, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, OffX: geoReq.OffX, OffY: geoReq.OffY, CRS: geoReq.CRS}
		return
	}
	var metadata MetadataResponse
	err = json.Unmarshal(body, &metadata)
	if err != nil {
		errChan <- fmt.Errorf("Problem parsing JSON response from %s. Error: %v", url, err)
		out <- &GeoTileGranule{ConfigPayLoad: ConfigPayLoad{NameSpaces: []string{"EmptyTile"}, ScaleParams: geoReq.ScaleParams, Palette: geoReq.Palette}, Path: "NULL", NameSpace: "EmptyTile", RasterType: "Byte", TimeStamps: nil, TimeStamp: *geoReq.StartTime, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, OffX: geoReq.OffX, OffY: geoReq.OffY, CRS: geoReq.CRS}
		return
	}

	switch len(metadata.GDALDatasets) {
	case 0:
		if len(metadata.Error) > 0 {
			log.Printf("Indexer returned error: %v", string(body))
			errChan <- fmt.Errorf("Indexer returned error: %v", metadata.Error)
		}
		out <- &GeoTileGranule{ConfigPayLoad: ConfigPayLoad{NameSpaces: []string{"EmptyTile"}, ScaleParams: geoReq.ScaleParams, Palette: geoReq.Palette}, Path: "NULL", NameSpace: "EmptyTile", RasterType: "Byte", TimeStamps: nil, TimeStamp: *geoReq.StartTime, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, OffX: geoReq.OffX, OffY: geoReq.OffY, CRS: geoReq.CRS}
	default:
		for _, ds := range metadata.GDALDatasets {
			for _, t := range ds.TimeStamps {
				if t.Equal(*geoReq.StartTime) || geoReq.EndTime != nil && t.After(*geoReq.StartTime) && t.Before(*geoReq.EndTime) {
					out <- &GeoTileGranule{ConfigPayLoad: ConfigPayLoad{NameSpaces: geoReq.NameSpaces, Mask: geoReq.Mask, ScaleParams: geoReq.ScaleParams, Palette: geoReq.Palette, Timeout: geoReq.Timeout, GrpcConcLimit: geoReq.GrpcConcLimit}, Path: ds.DSName, NameSpace: ds.NameSpace, RasterType: ds.ArrayType, TimeStamps: ds.TimeStamps, TimeStamp: t, Polygon: ds.Polygon, BBox: geoReq.BBox, Height: geoReq.Height, Width: geoReq.Width, OffX: geoReq.OffX, OffY: geoReq.OffY, CRS: geoReq.CRS}
				}
			}
		}
	}
}
