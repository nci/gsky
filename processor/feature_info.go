package processor

import (
	"context"
	"fmt"
	"github.com/nci/gsky/utils"
	"regexp"
	"sort"
	"strings"
	"time"
)

func GetFeatureInfo(ctx context.Context, params utils.WMSParams, conf *utils.Config, verbose bool) (string, error) {
	raster, namespaces, dsFiles, err := getRaster(ctx, params, conf, verbose)
	if err != nil {
		return "", err
	}

	out := `"bands": {`

	if len(raster) == 1 {
		if rs, ok := raster[0].(*utils.ByteRaster); ok {
			if rs.NameSpace == "ZoomOut" {
				for i, ns := range namespaces {
					out += fmt.Sprintf(`"%s":"zoom in to view"`, ns)
					if i < len(namespaces)-1 {
						out += ","
					}
				}
				out += `}`
				return out, nil
			}

			if rs.NameSpace == "EmptyTile" {
				return "", fmt.Errorf("data unavailable")
			}
		}
	}

	width, height, _, err := utils.ValidateRasterSlice(raster)
	if err != nil {
		return "", err
	}

	x := *params.X
	y := *params.Y

	offset := y*width + x
	if offset >= width*height {
		return "", fmt.Errorf("x or y out of bound")
	}

	for i, ns := range namespaces {
		r := raster[i]
		var valueStr string

		switch t := r.(type) {
		case *utils.ByteRaster:
			noData := uint8(t.NoData)
			value := t.Data[offset]
			if value == noData {
				valueStr = `"n/a"`
			} else {
				valueStr = fmt.Sprintf("%v", value)
			}

		case *utils.Int16Raster:
			noData := int16(t.NoData)
			value := t.Data[offset]
			if value == noData {
				valueStr = `"n/a"`
			} else {
				valueStr = fmt.Sprintf("%v", value)
			}

		case *utils.UInt16Raster:
			noData := uint16(t.NoData)
			value := t.Data[offset]
			if value == noData {
				valueStr = `"n/a"`
			} else {
				valueStr = fmt.Sprintf("%v", value)
			}

		case *utils.Float32Raster:
			noData := float32(t.NoData)
			value := t.Data[offset]
			if value == noData {
				valueStr = `"n/a"`
			} else {
				valueStr = fmt.Sprintf("%v", value)
			}
		}

		out += fmt.Sprintf(`"%s": %s`, ns, valueStr)
		if i < len(namespaces)-1 {
			out += ","
		}
	}

	if len(dsFiles) > 0 {
		prefix := ""
		idx, _ := utils.GetLayerIndex(params, conf)
		if len(conf.Layers[idx].FeatureInfoDataLinkUrl) > 0 {
			prefix = conf.Layers[idx].FeatureInfoDataLinkUrl
			if prefix[len(prefix)-1] != '/' {
				prefix += "/"
			}
		}
		out += `, "data_links":[`
		for i, file := range dsFiles {

			out += fmt.Sprintf(`"%s%s"`, prefix, file)
			if i < len(dsFiles)-1 {
				out += ","
			}
		}

		out += `]`
	}
	out += `}`
	return out, nil
}

func getRaster(ctx context.Context, params utils.WMSParams, conf *utils.Config, verbose bool) ([]utils.Raster, []string, []string, error) {
	idx, err := utils.GetLayerIndex(params, conf)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Malformed WMS GetMap request: %v", err)
	}
	if params.Time == nil {
		return nil, nil, nil, fmt.Errorf("Request should contain a valid time.")
	}
	if params.CRS == nil {
		return nil, nil, nil, fmt.Errorf("Request should contain a valid ISO 'crs/srs' parameter.")
	}
	if len(params.BBox) != 4 {
		return nil, nil, nil, fmt.Errorf("Request should contain a valid 'bbox' parameter.")
	}

	xRes := (params.BBox[2] - params.BBox[0]) / float64(*params.Width)
	yRes := (params.BBox[3] - params.BBox[1]) / float64(*params.Height)
	reqRes := xRes
	if yRes > reqRes {
		reqRes = yRes
	}

	if conf.Layers[idx].ZoomLimit != 0.0 && reqRes > conf.Layers[idx].ZoomLimit {
		return []utils.Raster{&utils.ByteRaster{NameSpace: "ZoomOut"}}, conf.Layers[idx].RGBProducts, nil, nil
	}

	if params.Height == nil || params.Width == nil {
		return nil, nil, nil, fmt.Errorf("Request should contain valid 'width' and 'height' parameters.")
	}
	if *params.Height > conf.Layers[idx].WmsMaxHeight || *params.Width > conf.Layers[idx].WmsMaxWidth {
		return nil, nil, nil, fmt.Errorf("Requested width/height is too large, max width:%d, height:%d", conf.Layers[idx].WmsMaxWidth, conf.Layers[idx].WmsMaxHeight)
	}

	if params.X == nil || params.Y == nil {
		return nil, nil, nil, fmt.Errorf("Request should contain valid 'x' and 'y' parameters.")
	}
	if strings.ToUpper(*params.CRS) == "EPSG:4326" && *params.Version == "1.3.0" {
		params.BBox = []float64{params.BBox[1], params.BBox[0], params.BBox[3], params.BBox[2]}
	}
	if strings.ToUpper(*params.CRS) == "CRS:84" && *params.Version == "1.3.0" {
		*params.CRS = "EPSG:4326"
	}

	var endTime *time.Time
	if conf.Layers[idx].Accum == true {
		step := time.Minute * time.Duration(60*24*conf.Layers[idx].StepDays+60*conf.Layers[idx].StepHours+conf.Layers[idx].StepMinutes)
		eT := params.Time.Add(step)
		endTime = &eT
	}

	if len(conf.Layers[idx].DataSource) == 0 {
		return nil, nil, nil, fmt.Errorf("Invalid data source")
	}

	geoReq := &GeoTileRequest{ConfigPayLoad: ConfigPayLoad{NameSpaces: conf.Layers[idx].RGBProducts,
		Mask:            conf.Layers[idx].Mask,
		ZoomLimit:       conf.Layers[idx].ZoomLimit,
		PolygonSegments: conf.Layers[idx].WmsPolygonSegments,
		GrpcConcLimit:   conf.Layers[idx].GrpcWmsConcPerNode,
		QueryLimit:      -1,
	},
		Collection: conf.Layers[idx].DataSource,
		CRS:        *params.CRS,
		BBox:       params.BBox,
		Height:     *params.Height,
		Width:      *params.Width,
		StartTime:  params.Time,
		EndTime:    endTime,
	}

	ctx, ctxCancel := context.WithCancel(ctx)
	defer ctxCancel()
	errChan := make(chan error)

	var outRaster []utils.Raster
	tp := InitTilePipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, conf.Layers[idx].MaxGrpcRecvMsgSize, conf.Layers[idx].WmsPolygonShardConcLimit, errChan)
	select {
	case res := <-tp.Process(geoReq, verbose):
		outRaster = res
	case err := <-errChan:
		return nil, nil, nil, err
	case <-ctx.Done():
		return nil, nil, nil, ctx.Err()
	}

	if conf.Layers[idx].FeatureInfoMaxDataLinks < 1 {
		return outRaster, conf.Layers[idx].RGBProducts, nil, nil
	}

	x, y, err := utils.GetCoordinates(params)
	if err != nil {
		return nil, nil, nil, err
	}

	indexer := NewTileIndexer(ctx, conf.ServiceConfig.MASAddress, errChan)
	go func() {
		geoReq.Mask = nil
		geoReq.BBox = []float64{x, y, x + 1e-4, y + 1e-4}
		indexer.In <- geoReq
		close(indexer.In)
	}()

	go indexer.Run(verbose)

	var pixelFiles []*GeoTileGranule
	for geo := range indexer.Out {
		select {
		case err := <-errChan:
			return nil, nil, nil, err
		case <-ctx.Done():
			return nil, nil, nil, ctx.Err()
		default:
			if geo.NameSpace == "EmptyTile" {
				continue
			}

			pixelFiles = append(pixelFiles, geo)
		}
	}

	sort.Slice(pixelFiles, func(i, j int) bool { return pixelFiles[i].TimeStamp.Unix() < pixelFiles[j].TimeStamp.Unix() })

	var topDsFiles []string
	fileDedup := make(map[string]bool)
	reGdalDs := regexp.MustCompile(`[a-zA-Z0-9\-_]+:"(.*)":.*`)
	for i, ds := range pixelFiles {
		dsFile := ds.Path
		if len(dsFile) == 0 {
			continue
		}

		matches := reGdalDs.FindStringSubmatch(ds.Path)
		if len(matches) > 1 {
			dsFile = matches[1]
		}

		_, found := fileDedup[dsFile]
		if found {
			continue
		}
		fileDedup[dsFile] = true

		if strings.Index(dsFile, conf.Layers[idx].DataSource) >= 0 {
			offset := 0
			if conf.Layers[idx].DataSource[len(conf.Layers[idx].DataSource)-1] != '/' {
				offset = 1
			}
			dsFile = dsFile[len(conf.Layers[idx].DataSource)+offset:]
			topDsFiles = append(topDsFiles, dsFile)

			if i+1 >= conf.Layers[idx].FeatureInfoMaxDataLinks {
				break
			}
		}
	}

	return outRaster, conf.Layers[idx].RGBProducts, topDsFiles, nil
}
