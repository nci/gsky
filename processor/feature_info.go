package processor

import (
	"context"
	"fmt"
	"github.com/nci/gsky/utils"
	"sort"
	"strings"
	"time"
)

type featureInfo struct {
	Raster     []utils.Raster
	Namespaces []string
	DsFiles    []string
	DsDates    []string
}

func GetFeatureInfo(ctx context.Context, params utils.WMSParams, conf *utils.Config, configMap map[string]*utils.Config, verbose bool) (string, error) {
	ftInfo, err := getRaster(ctx, params, conf, configMap, verbose)
	if err != nil {
		return "", err
	}

	out := `"bands": {`

	if len(ftInfo.Raster) == 1 {
		if rs, ok := ftInfo.Raster[0].(*utils.ByteRaster); ok {
			if rs.NameSpace == "ZoomOut" {
				for i, ns := range ftInfo.Namespaces {
					out += fmt.Sprintf(`"%s":"zoom in to view"`, ns)
					if i < len(ftInfo.Namespaces)-1 {
						out += ","
					}
				}
				out += `}`
				return out, nil
			}

			if rs.NameSpace == utils.EmptyTileNS {
				return "", fmt.Errorf("data unavailable")
			}
		}
	}

	width, height, _, err := utils.ValidateRasterSlice(ftInfo.Raster)
	if err != nil {
		return "", err
	}

	x := *params.X
	y := *params.Y

	offset := y*width + x
	if offset >= width*height {
		return "", fmt.Errorf("x or y out of bound")
	}

	for i, ns := range ftInfo.Namespaces {
		r := ftInfo.Raster[i]
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
		if i < len(ftInfo.Namespaces)-1 {
			out += ","
		}
	}

	if len(ftInfo.DsDates) > 0 {
		out += `, "data_available_for_dates":[`
		for i, ts := range ftInfo.DsDates {
			out += fmt.Sprintf(`"%s"`, ts)
			if i < len(ftInfo.DsDates)-1 {
				out += ","
			}
		}
		out += `]`
	}

	if len(ftInfo.DsFiles) > 0 {
		prefix := ""
		idx, _ := utils.GetLayerIndex(params, conf)
		if len(conf.Layers[idx].FeatureInfoDataLinkUrl) > 0 {
			prefix = conf.Layers[idx].FeatureInfoDataLinkUrl
			if prefix[len(prefix)-1] != '/' {
				prefix += "/"
			}
		}
		out += `, "data_links":[`
		for i, file := range ftInfo.DsFiles {

			out += fmt.Sprintf(`"%s%s"`, prefix, file)
			if i < len(ftInfo.DsFiles)-1 {
				out += ","
			}
		}
		out += `]`
	}
	out += `}`
	return out, nil
}

func getRaster(ctx context.Context, params utils.WMSParams, conf *utils.Config, configMap map[string]*utils.Config, verbose bool) (*featureInfo, error) {
	ftInfo := &featureInfo{}

	idx, err := utils.GetLayerIndex(params, conf)
	if err != nil {
		return nil, fmt.Errorf("Malformed WMS GetFeatureInfo request: %v", err)
	}
	if params.Time == nil {
		return nil, fmt.Errorf("Request should contain a valid time.")
	}
	if params.CRS == nil {
		return nil, fmt.Errorf("Request should contain a valid ISO 'crs/srs' parameter.")
	}
	if len(params.BBox) != 4 {
		return nil, fmt.Errorf("Request should contain a valid 'bbox' parameter.")
	}

	xRes := (params.BBox[2] - params.BBox[0]) / float64(*params.Width)
	yRes := (params.BBox[3] - params.BBox[1]) / float64(*params.Height)
	reqRes := xRes
	if yRes > reqRes {
		reqRes = yRes
	}

	styleIdx, err := utils.GetLayerStyleIndex(params, conf, idx)
	if err != nil {
		return nil, err
	}

	styleLayer := &conf.Layers[idx]
	if styleIdx >= 0 {
		styleLayer = &conf.Layers[idx].Styles[styleIdx]
	}

	var namespaces []string
	var bandExpr *utils.BandExpressions
	if len(styleLayer.FeatureInfoBands) > 0 {
		namespaces = styleLayer.FeatureInfoExpressions.VarList
		bandExpr = styleLayer.FeatureInfoExpressions
	} else if len(conf.Layers[idx].FeatureInfoBands) > 0 {
		namespaces = conf.Layers[idx].FeatureInfoExpressions.VarList
		bandExpr = conf.Layers[idx].FeatureInfoExpressions
	} else {
		namespaces = styleLayer.RGBExpressions.VarList
		bandExpr = styleLayer.RGBExpressions
	}

	if conf.Layers[idx].ZoomLimit != 0.0 && reqRes > conf.Layers[idx].ZoomLimit {
		ftInfo.Namespaces = bandExpr.ExprNames
		ftInfo.Raster = []utils.Raster{&utils.ByteRaster{NameSpace: "ZoomOut"}}
		return ftInfo, nil
	}

	if params.Height == nil || params.Width == nil {
		return nil, fmt.Errorf("Request should contain valid 'width' and 'height' parameters.")
	}
	if *params.Height > conf.Layers[idx].WmsMaxHeight || *params.Width > conf.Layers[idx].WmsMaxWidth {
		return nil, fmt.Errorf("Requested width/height is too large, max width:%d, height:%d", conf.Layers[idx].WmsMaxWidth, conf.Layers[idx].WmsMaxHeight)
	}

	if params.X == nil || params.Y == nil {
		return nil, fmt.Errorf("Request should contain valid 'x' and 'y' parameters.")
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
		return nil, fmt.Errorf("Invalid data source")
	}

	// We construct a 2x2 image corresponding to an infinitesimal bounding box
	// to approximate a pixel.
	// We observed several order of magnitude of performance improvement as a
	// result of such an approximation.
	xmin := params.BBox[0] + float64(*params.X)*xRes
	ymin := params.BBox[3] - float64(*params.Y)*yRes

	xmax := params.BBox[0] + float64(*params.X+1)*xRes
	ymax := params.BBox[3] - float64(*params.Y-1)*xRes

	*params.Height = 2
	*params.Width = 2

	*params.X = 0
	*params.Y = 1

	params.BBox = []float64{xmin, ymin, xmax, ymax}

	geoReq := &GeoTileRequest{ConfigPayLoad: ConfigPayLoad{NameSpaces: namespaces,
		BandExpr:            bandExpr,
		Mask:                styleLayer.Mask,
		ZoomLimit:           conf.Layers[idx].ZoomLimit,
		PolygonSegments:     conf.Layers[idx].WmsPolygonSegments,
		GrpcConcLimit:       conf.Layers[idx].GrpcWmsConcPerNode,
		QueryLimit:          -1,
		UserSrcSRS:          conf.Layers[idx].UserSrcSRS,
		UserSrcGeoTransform: conf.Layers[idx].UserSrcGeoTransform,
	},
		Collection: styleLayer.DataSource,
		CRS:        *params.CRS,
		BBox:       params.BBox,
		Height:     *params.Height,
		Width:      *params.Width,
		StartTime:  params.Time,
		EndTime:    endTime,
	}

	if len(params.Axes) > 0 {
		geoReq.Axes = make(map[string]*GeoTileAxis)
		for _, axis := range params.Axes {
			geoReq.Axes[axis.Name] = &GeoTileAxis{Start: axis.Start, End: axis.End, InValues: axis.InValues, Order: axis.Order, Aggregate: axis.Aggregate}
		}
	}

	if params.BandExpr != nil {
		geoReq.ConfigPayLoad.NameSpaces = params.BandExpr.VarList
		geoReq.ConfigPayLoad.BandExpr = params.BandExpr
	}

	ctx, ctxCancel := context.WithCancel(ctx)
	defer ctxCancel()
	errChan := make(chan error, 100)

	var outRaster []utils.Raster
	tp := InitTilePipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, conf.Layers[idx].MaxGrpcRecvMsgSize, conf.Layers[idx].WmsPolygonShardConcLimit, conf.ServiceConfig.MaxGrpcBufferSize, errChan)
	tp.CurrentLayer = styleLayer
	tp.DataSources = configMap

	select {
	case res := <-tp.Process(geoReq, verbose):
		outRaster = res
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	ftInfo.Raster = outRaster
	ftInfo.Namespaces = bandExpr.ExprNames
	if conf.Layers[idx].FeatureInfoMaxAvailableDates == 0 && conf.Layers[idx].FeatureInfoMaxDataLinks == 0 {
		return ftInfo, nil
	}

	if conf.Layers[idx].FeatureInfoMaxAvailableDates != 0 {
		geoReq.StartTime = &time.Time{}
		//		geoReq.EndTime = nil
	}

	tp = InitTilePipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, conf.Layers[idx].MaxGrpcRecvMsgSize, conf.Layers[idx].WmsPolygonShardConcLimit, conf.ServiceConfig.MaxGrpcBufferSize, errChan)
	tp.CurrentLayer = styleLayer
	tp.DataSources = configMap

	indexerOut := tp.GetFileList(geoReq, verbose)

	var pixelFiles []*GeoTileGranule
	timestampLookup := make(map[time.Time]bool)
	for geo := range indexerOut {
		select {
		case err := <-errChan:
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if geo.NameSpace == utils.EmptyTileNS {
				continue
			}

			tm := time.Unix(int64(geo.TimeStamp), 0)
			normalizedTm := time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, time.UTC)
			if _, found := timestampLookup[normalizedTm]; found {
				continue
			}

			pixelFiles = append(pixelFiles, geo)
		}
	}

	sort.Slice(pixelFiles, func(i, j int) bool { return pixelFiles[i].TimeStamp >= pixelFiles[j].TimeStamp })

	var topDsDates []string
	if conf.Layers[idx].FeatureInfoMaxAvailableDates != 0 {
		for i, ds := range pixelFiles {
			tm := time.Unix(int64(ds.TimeStamp), 0)
			normalizedTm := time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, time.UTC).Format(utils.ISOFormat)
			topDsDates = append(topDsDates, normalizedTm)

			if conf.Layers[idx].FeatureInfoMaxAvailableDates > 0 && i+1 >= conf.Layers[idx].FeatureInfoMaxAvailableDates {
				break
			}
		}
		ftInfo.DsDates = topDsDates
	}

	fmt.Printf("xxxxxxxxxxx %v, %v, %v", len(pixelFiles), conf.Layers[idx].FeatureInfoMaxAvailableDates, topDsDates)

	var topDsFiles []string
	if conf.Layers[idx].FeatureInfoMaxDataLinks == 0 {
		return ftInfo, nil
	}

	fileDedup := make(map[string]bool)
	for i, ds := range pixelFiles {
		dsFile := ds.RawPath
		if len(dsFile) == 0 {
			continue
		}

		_, found := fileDedup[dsFile]
		if found {
			continue
		}
		fileDedup[dsFile] = true

		if strings.Index(dsFile, styleLayer.DataSource) >= 0 {
			offset := 0
			if styleLayer.DataSource[len(styleLayer.DataSource)-1] != '/' {
				offset = 1
			}
			dsFile = dsFile[len(styleLayer.DataSource)+offset:]
			topDsFiles = append(topDsFiles, dsFile)

			if conf.Layers[idx].FeatureInfoMaxDataLinks > 0 && i+1 >= conf.Layers[idx].FeatureInfoMaxDataLinks {
				break
			}
		}
	}
	ftInfo.DsFiles = topDsFiles
	return ftInfo, nil
}
