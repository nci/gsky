package processor

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nci/gsky/metrics"
	"github.com/nci/gsky/utils"
)

type featureInfo struct {
	Raster     []utils.Raster
	Namespaces []string
	DsFiles    []string
	DsDates    []string
}

func GetFeatureInfo(ctx context.Context, params utils.WMSParams, conf *utils.Config, configMap map[string]*utils.Config, verbose bool, metricsCollector *metrics.MetricsCollector) (string, error) {
	ftInfo, err := getRaster(ctx, params, conf, configMap, verbose, metricsCollector)
	if err != nil {
		return "", err
	}

	out := `"bands": {`

	hasData := true
	if len(ftInfo.Raster) == 1 {
		if rs, ok := ftInfo.Raster[0].(*utils.ByteRaster); ok {
			var msg string
			switch rs.NameSpace {
			case "ZoomOut":
				msg = "zoom in to view"
				hasData = false
			case utils.EmptyTileNS:
				msg = "n/a"
				hasData = false
			}

			if !hasData {
				for i, ns := range ftInfo.Namespaces {
					out += fmt.Sprintf(`"%s":"%s"`, ns, msg)
					if i < len(ftInfo.Namespaces)-1 {
						out += ","
					}
				}
				out += `}`
			}
		}
	}

	if hasData {
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
			case *utils.SignedByteRaster:
				noData := int8(t.NoData)
				value := t.Data[offset]
				if value == noData {
					valueStr = `"n/a"`
				} else {
					valueStr = fmt.Sprintf("%v", value)
				}

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
		out += `}`
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

	return out, nil
}

func getRaster(ctx context.Context, params utils.WMSParams, conf *utils.Config, configMap map[string]*utils.Config, verbose bool, metricsCollector *metrics.MetricsCollector) (*featureInfo, error) {
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

	bbox, err := utils.GetCanonicalBbox(*params.CRS, params.BBox)
	if err != nil {
		bbox = params.BBox
	}
	reqRes := utils.GetPixelResolution(bbox, *params.Width, *params.Height)

	// We construct a 2x2 image corresponding to an infinitesimal bounding box
	// to approximate a pixel.
	// We observed several order of magnitude of performance improvement as a
	// result of such an approximation.
	xmin := params.BBox[0] + float64(*params.X)*reqRes
	ymin := params.BBox[3] - float64(*params.Y)*reqRes

	xmax := params.BBox[0] + float64(*params.X+1)*reqRes
	ymax := params.BBox[3] - float64(*params.Y-1)*reqRes

	*params.Height = 2
	*params.Width = 2

	*params.X = 0
	*params.Y = 1

	params.BBox = []float64{xmin, ymin, xmax, ymax}

	geoReq := &GeoTileRequest{ConfigPayLoad: ConfigPayLoad{NameSpaces: namespaces,
		BandExpr:            bandExpr,
		Mask:                styleLayer.Mask,
		ZoomLimit:           styleLayer.ZoomLimit,
		PolygonSegments:     conf.Layers[idx].WmsPolygonSegments,
		GrpcConcLimit:       conf.Layers[idx].GrpcWmsConcPerNode,
		QueryLimit:          -1,
		UserSrcSRS:          conf.Layers[idx].UserSrcSRS,
		UserSrcGeoTransform: conf.Layers[idx].UserSrcGeoTransform,
		AxisMapping:         conf.Layers[idx].WmsAxisMapping,
		MasQueryHint:        conf.Layers[idx].MasQueryHint,
		SRSCf:               conf.Layers[idx].SRSCf,
		FusionUnscale:       1,
		MetricsCollector:    metricsCollector,
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

	masAddress := styleLayer.MASAddress
	hasOverview := len(styleLayer.Overviews) > 0
	if hasOverview {
		iOvr := utils.FindLayerBestOverview(styleLayer, reqRes, true)
		if iOvr >= 0 {
			ovr := styleLayer.Overviews[iOvr]
			geoReq.Collection = ovr.DataSource
			masAddress = ovr.MASAddress
		}
	}

	if !hasOverview && styleLayer.ZoomLimit != 0.0 && reqRes > styleLayer.ZoomLimit {
		ftInfo.Namespaces = bandExpr.ExprNames
		ftInfo.Raster = []utils.Raster{&utils.ByteRaster{NameSpace: "ZoomOut"}}
		return ftInfo, nil
	}

	ctx, ctxCancel := context.WithCancel(ctx)
	defer ctxCancel()
	errChan := make(chan error, 100)

	var outRaster []utils.Raster
	tp := InitTilePipeline(ctx, masAddress, conf.ServiceConfig.WorkerNodes, conf.Layers[idx].MaxGrpcRecvMsgSize, conf.Layers[idx].WmsPolygonShardConcLimit, conf.ServiceConfig.MaxGrpcBufferSize, errChan)
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
		currTime, _ := time.Parse(ISOFormat, conf.Layers[idx].Dates[len(conf.Layers[idx].Dates)-1])
		step := 60*24*conf.Layers[idx].StepDays + 60*conf.Layers[idx].StepHours + conf.Layers[idx].StepMinutes
		if step <= 0 {
			step = 1
		}
		timeStep := time.Minute * time.Duration(step)
		currTime = currTime.Add(timeStep)
		geoReq.EndTime = &currTime
	}

	tp = InitTilePipeline(ctx, conf.ServiceConfig.MASAddress, conf.ServiceConfig.WorkerNodes, conf.Layers[idx].MaxGrpcRecvMsgSize, conf.Layers[idx].WmsPolygonShardConcLimit, conf.ServiceConfig.MaxGrpcBufferSize, errChan)
	tp.CurrentLayer = styleLayer
	tp.DataSources = configMap

	indexerOut, err := tp.GetFileList(geoReq, verbose)
	if err != nil {
		return nil, err
	}

	var pixelFiles []*GeoTileGranule
	timestampLookup := make(map[time.Time]bool)
	for _, geo := range indexerOut {
		if geo.NameSpace == utils.EmptyTileNS {
			continue
		}

		tm := time.Unix(int64(geo.TimeStamp), 0).UTC()
		normalizedTm := time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, time.UTC)
		if _, found := timestampLookup[normalizedTm]; found {
			continue
		}

		timestampLookup[normalizedTm] = true
		pixelFiles = append(pixelFiles, geo)
	}

	sort.Slice(pixelFiles, func(i, j int) bool { return pixelFiles[i].TimeStamp >= pixelFiles[j].TimeStamp })

	var topDsDates []string
	dateFormat := "2006-01-02"
	if conf.Layers[idx].FeatureInfoMaxAvailableDates != 0 {
		maxDates := conf.Layers[idx].FeatureInfoMaxAvailableDates
		if maxDates < 0 {
			maxDates = len(pixelFiles)
		}
		for i := range pixelFiles[:maxDates] {
			ts := pixelFiles[maxDates-1-i].TimeStamp
			tm := time.Unix(int64(ts), 0).UTC()
			normalizedTm := time.Date(tm.Year(), tm.Month(), tm.Day(), 0, 0, 0, 0, time.UTC).Format(dateFormat)
			topDsDates = append(topDsDates, normalizedTm)
		}
		ftInfo.DsDates = topDsDates
	}

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
