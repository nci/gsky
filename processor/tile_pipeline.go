package processor

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/nci/gsky/utils"
)

type TilePipeline struct {
	Context               context.Context
	Error                 chan error
	RPCAddress            []string
	MaxGrpcRecvMsgSize    int
	PolygonShardConcLimit int
	MASAddress            string
	MaxGrpcBufferSize     int
	CurrentLayer          *utils.Layer
	DataSources           map[string]*utils.Config
}

type GeoReqContext struct {
	Service    *utils.ServiceConfig
	Layer      *utils.Layer
	StyleLayer *utils.Layer
	GeoReq     *GeoTileRequest
	MASAddress string
}

const fusedBandName = "fuse"

func InitTilePipeline(ctx context.Context, masAddr string, rpcAddr []string, maxGrpcRecvMsgSize int, polygonShardConcLimit int, maxGrpcBufferSize int, errChan chan error) *TilePipeline {
	return &TilePipeline{
		Context:               ctx,
		Error:                 errChan,
		RPCAddress:            rpcAddr,
		MaxGrpcRecvMsgSize:    maxGrpcRecvMsgSize,
		PolygonShardConcLimit: polygonShardConcLimit,
		MASAddress:            masAddr,
		MaxGrpcBufferSize:     maxGrpcBufferSize,
	}

}

func (dp *TilePipeline) Process(geoReq *GeoTileRequest, verbose bool) chan []utils.Raster {
	masAddress := dp.MASAddress
	if geoReq.Overview != nil {
		dataSource := geoReq.Collection
		spatialExtent := geoReq.SpatialExtent
		endTime := geoReq.EndTime

		geoReq.Collection = geoReq.Overview.DataSource
		geoReq.SpatialExtent = geoReq.Overview.SpatialExtent
		if !geoReq.Overview.Accum {
			geoReq.EndTime = nil
		} else {
			step := time.Minute * time.Duration(60*24*geoReq.Overview.StepDays+60*geoReq.Overview.StepHours+geoReq.Overview.StepMinutes)
			et := geoReq.StartTime.Add(step)
			geoReq.EndTime = &et
		}

		dp.MASAddress = geoReq.Overview.MASAddress
		hasData := dp.HasFiles(geoReq, verbose)
		dp.MASAddress = masAddress
		masAddress = geoReq.Overview.MASAddress
		if !hasData {
			geoReq.Collection = dataSource
			geoReq.SpatialExtent = spatialExtent
			geoReq.EndTime = endTime
			masAddress = dp.MASAddress
		}
	}

	i := NewTileIndexer(dp.Context, masAddress, dp.Error)
	m := NewRasterMerger(dp.Context, dp.Error)
	grpcTiler := NewRasterGRPC(dp.Context, dp.RPCAddress, dp.MaxGrpcRecvMsgSize, dp.PolygonShardConcLimit, dp.MaxGrpcBufferSize, dp.Error)

	grpcTiler.In = i.Out
	m.In = grpcTiler.Out

	go m.Run(geoReq.BandExpr, verbose)

	varList := geoReq.BandExpr.VarList
	if dp.CurrentLayer != nil && len(dp.CurrentLayer.InputLayers) > 0 {
		otherVars, hasFusedBand, supportTimeWeighted, err := dp.checkFusedBandNames(geoReq)
		if err != nil {
			dp.sendError(err)
			close(m.In)
			return m.Out
		}

		if hasFusedBand {
			var aggTime time.Duration
			if geoReq.StartTime != nil && geoReq.EndTime != nil {
				aggTime = geoReq.EndTime.Sub(*geoReq.StartTime)
			}

			var weightedGeoReqs []*GeoTileRequest
			if supportTimeWeighted {
				for k, axis := range geoReq.Axes {
					if k != utils.WeightedTimeAxis {
						continue
					}

					if len(axis.InValues) < 2 {
						continue
					}

					for _, val := range axis.InValues {
						gr := &GeoTileRequest{}
						startTime := time.Unix(int64(val), 0).UTC()
						gr.StartTime = &startTime
						var endTime time.Time
						if geoReq.EndTime != nil {
							endTime = startTime.Add(aggTime)
							gr.EndTime = &endTime
						}
						weightedGeoReqs = append(weightedGeoReqs, gr)
					}
					break
				}
			}
			isTimeWeighted := len(weightedGeoReqs) > 0
			if !isTimeWeighted {
				weightedGeoReqs = append(weightedGeoReqs, geoReq)
			}

			for iw, wgr := range weightedGeoReqs {
				geoReq.StartTime = wgr.StartTime
				geoReq.EndTime = wgr.EndTime
				if isTimeWeighted {
					geoReq.FusionUnscale = 1
				}
				rasters, err := dp.processDeps(geoReq, verbose)
				if err != nil {
					dp.sendError(err)
					close(m.In)
					return m.Out
				}

				if isTimeWeighted {
					for _, raster := range rasters {
						raster.NameSpace = fmt.Sprintf("%s_%d", raster.NameSpace, iw)
					}
				}

				m.In <- rasters
			}

			if len(otherVars) == 0 {
				close(m.In)
				return m.Out
			}
			varList = otherVars
		}
	}

	go func() {
		i.In <- geoReq
		close(i.In)
	}()

	go i.Run(verbose)
	go grpcTiler.Run(varList, verbose)

	return m.Out
}

func (dp *TilePipeline) GetFileList(geoReq *GeoTileRequest, verbose bool) ([]*GeoTileGranule, error) {
	var totalGrans []*GeoTileGranule
	i := NewTileIndexer(dp.Context, dp.MASAddress, dp.Error)

	if dp.CurrentLayer != nil && len(dp.CurrentLayer.InputLayers) > 0 {
		otherVars, hasFusedBand, _, err := dp.checkFusedBandNames(geoReq)
		if err != nil {
			return nil, err
		}

		if hasFusedBand {
			grans, err := dp.getDepFileList(geoReq, verbose)

			if err != nil {
				return nil, err
			}

			for _, g := range grans {
				totalGrans = append(totalGrans, g)
			}

			if len(otherVars) == 0 {
				return totalGrans, nil
			}
		}
	}

	go func() {
		i.In <- geoReq
		close(i.In)
	}()

	go i.Run(verbose)

	for gran := range i.Out {
		select {
		case err := <-dp.Error:
			return nil, err
		case <-dp.Context.Done():
			return nil, fmt.Errorf("Context cancelled in fusion tile indexer")
		default:
			totalGrans = append(totalGrans, gran)
		}
	}

	return totalGrans, nil
}

func (dp *TilePipeline) HasFiles(geoReq *GeoTileRequest, verbose bool) bool {
	mask := geoReq.Mask
	queryLimit := geoReq.QueryLimit

	geoReq.Mask = nil
	geoReq.QueryLimit = 1

	hasData := false
	indexerOut, err := dp.GetFileList(geoReq, verbose)
	if err == nil {
		for _, geo := range indexerOut {
			if geo.NameSpace != utils.EmptyTileNS {
				hasData = true
				break
			}
		}
	}

	geoReq.Mask = mask
	geoReq.QueryLimit = queryLimit

	return hasData
}

func (dp *TilePipeline) processDeps(geoReq *GeoTileRequest, verbose bool) ([]*FlexRaster, error) {
	errChan := make(chan error, 100)
	var rasters []*FlexRaster

	depLayers, err := dp.findDepLayers()
	if err != nil {
		return nil, err
	}
	dp.prepareInputGeoRequests(geoReq, depLayers, true)

	var timestamp time.Time
	if geoReq.StartTime != nil {
		timestamp = *geoReq.StartTime
	} else {
		timestamp = time.Now().UTC()
	}

	var normRaster utils.Raster
	for idx, reqCtx := range depLayers {
		req := reqCtx.GeoReq
		if len(reqCtx.Layer.EffectiveStartDate) > 0 && len(reqCtx.Layer.EffectiveEndDate) > 0 {
			t0, e0 := time.Parse(utils.ISOFormat, reqCtx.Layer.EffectiveStartDate)
			t1, e1 := time.Parse(utils.ISOFormat, reqCtx.Layer.EffectiveEndDate)
			if e0 == nil && e1 == nil {
				ts0 := t0.Unix()
				ts1 := t1.Unix()

				reqT0 := int64(-1)
				if geoReq.StartTime != nil {
					reqT0 = geoReq.StartTime.Unix()
				}

				reqT1 := int64(-1)
				if geoReq.EndTime != nil {
					reqT1 = geoReq.EndTime.Unix()
				}

				isInRange := reqT0 >= ts0 && reqT0 <= ts1 || reqT1 >= ts0 && reqT1 <= ts1
				if !isInRange {
					if verbose {
						log.Printf("fusion pipeline '%v' (%d of %d): skip processing, requsted time range [%v, %v] out of layer time range [%v, %v]", reqCtx.Layer.Name, idx+1, len(depLayers), geoReq.StartTime, geoReq.EndTime, t0, t1)
					}
					continue
				}
			}

		}

		tp := InitTilePipeline(dp.Context, reqCtx.MASAddress, reqCtx.Service.WorkerNodes, reqCtx.Layer.MaxGrpcRecvMsgSize, reqCtx.Layer.WmsPolygonShardConcLimit, reqCtx.Service.MaxGrpcBufferSize, errChan)
		tp.CurrentLayer = reqCtx.StyleLayer
		tp.DataSources = dp.DataSources

		select {
		case res := <-tp.Process(req, verbose):
			timeDelta := time.Second * time.Duration(-idx)
			hasScaleParams := !(req.ScaleParams.Offset == 0 && req.ScaleParams.Scale == 0 && req.ScaleParams.Clip == 0)
			allFilled := false
			if req.FusionUnscale == 0 && hasScaleParams {
				scaleParams := utils.ScaleParams{Offset: req.ScaleParams.Offset,
					Scale: req.ScaleParams.Scale,
					Clip:  req.ScaleParams.Clip,
				}

				norm, err := utils.Scale(res, scaleParams)
				if err != nil {
					return nil, fmt.Errorf("fusion pipeline '%v' (%d of %d) utils.Scale error: %v", reqCtx.Layer.Name, idx+1, len(depLayers), err)
				}

				if len(norm) == 0 || norm[0].Width == 0 || norm[0].Height == 0 || norm[0].NameSpace == "EmptyTile" {
					if verbose {
						log.Printf("fusion pipeline '%v' (%d of %d): empty tile", reqCtx.Layer.Name, idx+1, len(depLayers))
					}
					break
				}

				for j := range norm {
					norm[j].NoData = 0xFF
					flex, filled := getFlexRaster(j, timestamp.Add(timeDelta), geoReq, norm[j], nil)
					if flex != nil {
						rasters = append(rasters, flex)
						if !filled {
							allFilled = filled
						}
					}
				}
			} else {
				for j := range res {
					if idx == 0 && j == 0 {
						normRaster = res[0]
					}
					flex, filled := getFlexRaster(j, timestamp.Add(timeDelta), geoReq, res[j], normRaster)
					if flex != nil {
						rasters = append(rasters, flex)
						if !filled {
							allFilled = filled
						}
					}
				}
			}

			if verbose {
				log.Printf("fusion pipeline '%v' (%d of %d) done", reqCtx.Layer.Name, idx+1, len(depLayers))
			}

			if allFilled {
				if verbose {
					log.Printf("fusion pipeline '%v' (%d of %d) early stopping, all pixels are filled", reqCtx.Layer.Name, idx+1, len(depLayers))
				}
				return rasters, nil
			}

		case err := <-errChan:
			return nil, fmt.Errorf("Error in the fusion pipeline '%v' (%d of %d): %v", reqCtx.Layer.Name, idx+1, len(depLayers), err)
		case <-tp.Context.Done():
			return nil, fmt.Errorf("Context cancelled in fusion pipeline '%v' (%d of %d)", reqCtx.Layer.Name, idx+1, len(depLayers))
		}
	}

	if len(rasters) == 0 {
		for idx := 0; idx < len(geoReq.BandExpr.ExprNames); idx++ {
			emptyRaster := &utils.ByteRaster{Data: make([]uint8, geoReq.Height*geoReq.Width), NoData: 0, Height: geoReq.Height, Width: geoReq.Width, NameSpace: utils.EmptyTileNS + "_dummy"}
			emptyFlex, _ := getFlexRaster(idx, timestamp, geoReq, emptyRaster, nil)
			rasters = append(rasters, emptyFlex)
		}
	}

	return rasters, nil

}

func (dp *TilePipeline) getDepFileList(geoReq *GeoTileRequest, verbose bool) ([]*GeoTileGranule, error) {
	errChan := make(chan error, 100)
	var totalGrans []*GeoTileGranule

	depLayers, err := dp.findDepLayers()
	if err != nil {
		return nil, err
	}
	dp.prepareInputGeoRequests(geoReq, depLayers, false)

	type LayerGrans struct {
		idx    int
		reqCtx *GeoReqContext
		grans  []*GeoTileGranule
	}

	granList := make(chan *LayerGrans, len(depLayers))
	errList := make(chan error, len(depLayers))
	cLimiter := NewConcLimiter(4)
	for idx, reqCtx := range depLayers {
		cLimiter.Increase()
		go func(idx int, reqCtx *GeoReqContext) {
			defer cLimiter.Decrease()
			tp := InitTilePipeline(dp.Context, reqCtx.MASAddress, reqCtx.Service.WorkerNodes, reqCtx.Layer.MaxGrpcRecvMsgSize, reqCtx.Layer.WmsPolygonShardConcLimit, reqCtx.Service.MaxGrpcBufferSize, errChan)
			tp.CurrentLayer = reqCtx.StyleLayer
			tp.DataSources = dp.DataSources

			req := reqCtx.GeoReq
			grans, err := tp.GetFileList(req, verbose)
			if err != nil {
				select {
				case errList <- fmt.Errorf("fusion pipeline '%v' (%d of %d) tile indexer error: %v", reqCtx.Layer.Name, idx+1, len(depLayers), err):
				default:
				}
				return
			}
			granList <- &LayerGrans{idx: idx, reqCtx: reqCtx, grans: grans}
			if verbose {
				log.Printf("fusion pipeline '%v' (%d of %d) tile indexer done", reqCtx.Layer.Name, idx+1, len(depLayers))
			}
		}(idx, reqCtx)
	}

	granCount := 0
	for il := 0; il < len(depLayers); il++ {
		select {
		case ctx := <-granList:
			for _, g := range ctx.grans {
				totalGrans = append(totalGrans, g)
				if geoReq.QueryLimit > 0 {
					if g.NameSpace != utils.EmptyTileNS {
						granCount++
					}

					if granCount >= geoReq.QueryLimit {
						if verbose {
							log.Printf("fusion pipeline '%v' (%d of %d) tile indexer early stopping, query limit reached", ctx.reqCtx.Layer.Name, ctx.idx+1, len(depLayers))
						}
						return totalGrans, nil
					}
				}
			}

		case err := <-errList:
			return nil, err
		}
	}

	return totalGrans, nil
}

func (dp *TilePipeline) findDepLayers() ([]*GeoReqContext, error) {
	var layers []*GeoReqContext
	for _, refLayer := range dp.CurrentLayer.InputLayers {
		refNameSpace := refLayer.NameSpace
		if len(refNameSpace) == 0 {
			if len(dp.CurrentLayer.NameSpace) == 0 {
				refNameSpace = "."
			} else {
				refNameSpace = dp.CurrentLayer.NameSpace
			}
		}

		config, found := dp.DataSources[refNameSpace]
		if !found {
			return layers, fmt.Errorf("namespace %s not found referenced by %s", refNameSpace, refLayer.Name)
		}

		params := utils.WMSParams{Layers: []string{refLayer.Name}}

		layerIdx, err := utils.GetLayerIndex(params, config)
		if err != nil {
			return layers, err
		}

		styleLayer := &config.Layers[layerIdx]
		layer := styleLayer
		if len(refLayer.Styles) > 0 {
			params.Styles = []string{refLayer.Styles[0].Name}
			styleIdx, err := utils.GetLayerStyleIndex(params, config, layerIdx)
			if err != nil {
				return layers, err
			}

			if styleIdx >= 0 {
				styleLayer = &config.Layers[layerIdx].Styles[styleIdx]
			}

		} else {
			if len(styleLayer.Styles) == 1 {
				styleLayer = &config.Layers[layerIdx].Styles[0]
			} else if len(styleLayer.Styles) > 0 {
				return layers, fmt.Errorf("referenced layer %s has multiple styles", refLayer.Name)
			}
		}
		layers = append(layers, &GeoReqContext{Service: &config.ServiceConfig, Layer: layer, StyleLayer: styleLayer})
	}

	return layers, nil
}

func (dp *TilePipeline) prepareInputGeoRequests(geoReq *GeoTileRequest, depLayers []*GeoReqContext, useOverview bool) {
	for _, ctx := range depLayers {
		styleLayer := ctx.StyleLayer
		layer := ctx.Layer
		ctx.GeoReq = &GeoTileRequest{ConfigPayLoad: ConfigPayLoad{NameSpaces: styleLayer.RGBExpressions.VarList,
			ScaleParams: ScaleParams{Offset: styleLayer.OffsetValue,
				Scale: styleLayer.ScaleValue,
				Clip:  styleLayer.ClipValue,
			},

			BandExpr:            styleLayer.RGBExpressions,
			Mask:                styleLayer.Mask,
			Palette:             styleLayer.Palette,
			ZoomLimit:           geoReq.ZoomLimit,
			PolygonSegments:     geoReq.PolygonSegments,
			GrpcConcLimit:       geoReq.GrpcConcLimit,
			QueryLimit:          geoReq.QueryLimit,
			UserSrcGeoTransform: layer.UserSrcGeoTransform,
			UserSrcSRS:          layer.UserSrcSRS,
			NoReprojection:      geoReq.NoReprojection,
			AxisMapping:         layer.WmsAxisMapping,
			MasQueryHint:        layer.MasQueryHint,
			ReqRes:              geoReq.ReqRes,
			SRSCf:               layer.SRSCf,
			FusionUnscale:       geoReq.FusionUnscale,
			GrpcTileXSize:       layer.GrpcTileXSize,
			GrpcTileYSize:       layer.GrpcTileYSize,
			IndexTileXSize:      layer.IndexTileXSize,
			IndexTileYSize:      layer.IndexTileYSize,
			SpatialExtent:       layer.SpatialExtent,
			IndexResLimit:       layer.IndexResLimit,
			MetricsCollector:    geoReq.MetricsCollector,
		},
			Collection: styleLayer.DataSource,
			CRS:        geoReq.CRS,
			BBox:       geoReq.BBox,
			Height:     geoReq.Height,
			Width:      geoReq.Width,
			StartTime:  geoReq.StartTime,
			EndTime:    geoReq.EndTime,
			Axes:       geoReq.Axes,
		}

		if useOverview {
			hasOverview := len(styleLayer.Overviews) > 0
			if hasOverview {
				allowExtrapolation := styleLayer.ZoomLimit > 0
				iOvr := utils.FindLayerBestOverview(styleLayer, geoReq.ReqRes, allowExtrapolation)
				if iOvr >= 0 {
					ctx.GeoReq.Overview = &styleLayer.Overviews[iOvr]
				}
			}
		}
		ctx.MASAddress = styleLayer.MASAddress
	}
}

func getFlexRaster(idx int, timestamp time.Time, req *GeoTileRequest, raster utils.Raster, normRaster utils.Raster) (*FlexRaster, bool) {
	namespace := fmt.Sprintf(fusedBandName+"%d", idx)
	flex := &FlexRaster{ConfigPayLoad: ConfigPayLoad{NameSpaces: req.ConfigPayLoad.NameSpaces}, NameSpace: namespace, TimeStamp: float64(timestamp.Unix()), Polygon: "dummy_polygon", Height: req.Height, Width: req.Width, DataHeight: req.Height, DataWidth: req.Width}

	allFilled := true
	var normNoData float64
	normalise := false
	switch t := raster.(type) {
	case *utils.SignedByteRaster:
		flex.Type = "SignedByte"
		flex.NoData = t.NoData
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&t.Data))
		flex.Data = *(*[]uint8)(unsafe.Pointer(&headr))
		noData := int8(t.NoData)
		if normRaster != nil {
			if r, ok := normRaster.(*utils.SignedByteRaster); ok {
				normNoData = r.NoData
				if int8(normNoData) != noData {
					normalise = true
					flex.NoData = normNoData
				}
			}
		}
		for i := range t.Data {
			if t.Data[i] == noData {
				allFilled = false
				if !normalise {
					break
				} else {
					t.Data[i] = int8(normNoData)
				}
			}
		}

	case *utils.ByteRaster:
		if t.NameSpace == utils.EmptyTileNS {
			return nil, false
		}
		flex.Type = "Byte"
		flex.NoData = t.NoData
		flex.Data = t.Data
		noData := uint8(t.NoData)
		if normRaster != nil {
			if r, ok := normRaster.(*utils.ByteRaster); ok {
				normNoData = r.NoData
				if uint8(normNoData) != noData {
					normalise = true
					flex.NoData = normNoData
				}
			}
		}
		for i := range t.Data {
			if t.Data[i] == noData {
				allFilled = false
				if !normalise {
					break
				} else {
					t.Data[i] = uint8(normNoData)
				}

			}
		}

	case *utils.Int16Raster:
		flex.Type = "Int16"
		flex.NoData = t.NoData
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&t.Data))
		headr.Len *= SizeofInt16
		headr.Cap *= SizeofInt16
		flex.Data = *(*[]uint8)(unsafe.Pointer(&headr))
		noData := int16(t.NoData)
		if normRaster != nil {
			if r, ok := normRaster.(*utils.Int16Raster); ok {
				normNoData = r.NoData
				if int16(normNoData) != noData {
					normalise = true
					flex.NoData = normNoData
				}
			}
		}
		for i := range t.Data {
			if t.Data[i] == noData {
				allFilled = false
				if !normalise {
					break
				} else {
					t.Data[i] = int16(normNoData)
				}
			}
		}

	case *utils.UInt16Raster:
		flex.Type = "UInt16"
		flex.NoData = t.NoData
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&t.Data))
		headr.Len *= SizeofUint16
		headr.Cap *= SizeofUint16
		flex.Data = *(*[]uint8)(unsafe.Pointer(&headr))
		noData := uint16(t.NoData)
		if normRaster != nil {
			if r, ok := normRaster.(*utils.UInt16Raster); ok {
				normNoData = r.NoData
				if uint16(normNoData) != noData {
					normalise = true
					flex.NoData = normNoData
				}
			}
		}
		for i := range t.Data {
			if t.Data[i] == noData {
				allFilled = false
				if !normalise {
					break
				} else {
					t.Data[i] = uint16(normNoData)
				}
			}
		}

	case *utils.Float32Raster:
		flex.Type = "Float32"
		flex.NoData = t.NoData
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&t.Data))
		headr.Len *= SizeofFloat32
		headr.Cap *= SizeofFloat32
		flex.Data = *(*[]uint8)(unsafe.Pointer(&headr))
		noData := float32(t.NoData)
		if normRaster != nil {
			if r, ok := normRaster.(*utils.Float32Raster); ok {
				normNoData = r.NoData
				if float32(normNoData) != noData {
					normalise = true
					flex.NoData = normNoData
				}
			}
		}
		for i := range t.Data {
			if t.Data[i] == noData {
				allFilled = false
				if !normalise {
					break
				} else {
					t.Data[i] = float32(normNoData)
				}
			}
		}

	}

	return flex, allFilled
}

func (dp *TilePipeline) checkFusedBandNames(geoReq *GeoTileRequest) ([]string, bool, bool, error) {
	var otherVars []string
	hasFusedBand := false
	isTimeWeighted := true
	for _, ns := range geoReq.BandExpr.VarList {
		if len(ns) > len(fusedBandName) && ns[:len(fusedBandName)] == fusedBandName {
			fusedNs := strings.Split(ns[len(fusedBandName):], "_")
			_, err := strconv.ParseInt(fusedNs[0], 10, 64)
			if err != nil {
				return nil, false, false, fmt.Errorf("invalid namespace: %v", ns)
			}

			hasFusedBand = true
			if len(fusedNs) != 2 {
				isTimeWeighted = false
			}
			continue
		}
		otherVars = append(otherVars, ns)
	}
	return otherVars, hasFusedBand, isTimeWeighted, nil
}

func (dp *TilePipeline) sendError(err error) {
	select {
	case dp.Error <- err:
	default:
	}
}
