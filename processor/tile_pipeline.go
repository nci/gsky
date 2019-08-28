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

	i := NewTileIndexer(dp.Context, dp.MASAddress, dp.Error)
	m := NewRasterMerger(dp.Context, dp.Error)
	grpcTiler := NewRasterGRPC(dp.Context, dp.RPCAddress, dp.MaxGrpcRecvMsgSize, dp.PolygonShardConcLimit, dp.MaxGrpcBufferSize, dp.Error)

	grpcTiler.In = i.Out
	m.In = grpcTiler.Out

	polyLimiter := NewConcLimiter(dp.PolygonShardConcLimit)
	go m.Run(polyLimiter, geoReq.BandExpr, verbose)

	varList := geoReq.BandExpr.VarList
	if dp.CurrentLayer != nil && len(dp.CurrentLayer.InputLayers) > 0 {
		otherVars, hasFusedBand, err := dp.checkFusedBandNames(geoReq)
		if err != nil {
			dp.Error <- err
			close(m.In)
			return m.Out
		}

		if hasFusedBand {
			var aggTime time.Duration
			if geoReq.StartTime != nil && geoReq.EndTime != nil {
				aggTime = geoReq.EndTime.Sub(*geoReq.StartTime)
			}

			var weightedGeoReqs []*GeoTileRequest
			for k, axis := range geoReq.Axes {
				if k != "weighted_time" {
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
			isTimeWeighted := len(weightedGeoReqs) > 0
			if !isTimeWeighted {
				weightedGeoReqs = append(weightedGeoReqs, geoReq)
			}

			for iw, wgr := range weightedGeoReqs {
				geoReq.StartTime = wgr.StartTime
				geoReq.EndTime = wgr.EndTime
				rasters, err := dp.processDeps(geoReq, verbose)
				if err != nil {
					dp.Error <- err
					close(m.In)
					return m.Out
				}

				if isTimeWeighted {
					for _, raster := range rasters {
						raster.NameSpace = fmt.Sprintf("%s_%d", raster.NameSpace, iw)
					}
				}

				polyLimiter.Increase()
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
	go grpcTiler.Run(polyLimiter, varList, verbose)

	return m.Out
}

func (dp *TilePipeline) GetFileList(geoReq *GeoTileRequest, verbose bool) ([]*GeoTileGranule, error) {
	var totalGrans []*GeoTileGranule
	i := NewTileIndexer(dp.Context, dp.MASAddress, dp.Error)

	if dp.CurrentLayer != nil && len(dp.CurrentLayer.InputLayers) > 0 {
		otherVars, hasFusedBand, err := dp.checkFusedBandNames(geoReq)
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
						log.Printf("fusion pipeline(%d of %d): skip processing, requsted time range [%v, %v] out of layer time range [%v, %v]", idx+1, len(depLayers), geoReq.StartTime, geoReq.EndTime, t0, t1)
					}
					continue
				}
			}

		}

		tp := InitTilePipeline(dp.Context, reqCtx.MASAddress, reqCtx.Service.WorkerNodes, reqCtx.Layer.MaxGrpcRecvMsgSize, reqCtx.Layer.WmsPolygonShardConcLimit, reqCtx.Service.MaxGrpcBufferSize, errChan)

		select {
		case res := <-tp.Process(req, verbose):
			timeDelta := time.Second * time.Duration(-idx)
			hasScaleParams := !(req.ScaleParams.Offset == 0 && req.ScaleParams.Scale == 0 && req.ScaleParams.Clip == 0)
			allFilled := true
			if hasScaleParams {
				scaleParams := utils.ScaleParams{Offset: req.ScaleParams.Offset,
					Scale: req.ScaleParams.Scale,
					Clip:  req.ScaleParams.Clip,
				}

				norm, err := utils.Scale(res, scaleParams)
				if err != nil {
					return nil, fmt.Errorf("fusion pipeline(%d of %d) utils.Scale error: %v", idx+1, len(depLayers), err)
				}

				if len(norm) == 0 || norm[0].Width == 0 || norm[0].Height == 0 || norm[0].NameSpace == "EmptyTile" {
					if verbose {
						log.Printf("fusion pipeline(%d of %d): empty tile", idx+1, len(depLayers))
					}
					break
				}

				for j := range norm {
					norm[j].NoData = 0xFF
					flex, filled := getFlexRaster(j, timestamp.Add(timeDelta), geoReq, norm[j])
					rasters = append(rasters, flex)
					if !filled {
						allFilled = filled
					}
				}
			} else {
				for j := range res {
					flex, filled := getFlexRaster(j, timestamp.Add(timeDelta), geoReq, res[j])
					rasters = append(rasters, flex)
					if !filled {
						allFilled = filled
					}

				}
			}

			if verbose {
				log.Printf("fusion pipeline(%d of %d) done", idx+1, len(depLayers))
			}

			if allFilled {
				if verbose {
					log.Printf("fusion pipeline(%d of %d) early stopping, all pixels are filled", idx+1, len(depLayers))
				}
				return rasters, nil
			}

		case err := <-errChan:
			return nil, fmt.Errorf("Error in the fusion pipeline(%d of %d): %v", idx+1, len(depLayers), err)
		case <-tp.Context.Done():
			return nil, fmt.Errorf("Context cancelled in fusion pipeline(%d of %d)", idx+1, len(depLayers))
		}
	}

	if len(rasters) == 0 {
		for idx := 0; idx < len(geoReq.BandExpr.Expressions); idx++ {
			emptyRaster := &utils.ByteRaster{Data: make([]uint8, geoReq.Height*geoReq.Width), NoData: 0, Height: geoReq.Height, Width: geoReq.Width, NameSpace: utils.EmptyTileNS}
			emptyFlex, _ := getFlexRaster(idx, timestamp, geoReq, emptyRaster)
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

	granCount := 0
	for idx, reqCtx := range depLayers {
		tp := InitTilePipeline(dp.Context, reqCtx.MASAddress, reqCtx.Service.WorkerNodes, reqCtx.Layer.MaxGrpcRecvMsgSize, reqCtx.Layer.WmsPolygonShardConcLimit, reqCtx.Service.MaxGrpcBufferSize, errChan)

		req := reqCtx.GeoReq
		grans, err := tp.GetFileList(req, verbose)
		if err != nil {
			return nil, fmt.Errorf("fusion pipeline(%d of %d) tile indexer error: %v", idx+1, len(depLayers), err)
		}

		for _, g := range grans {
			totalGrans = append(totalGrans, g)
			if geoReq.QueryLimit > 0 {
				if g.NameSpace != utils.EmptyTileNS {
					granCount++
				}

				if granCount >= geoReq.QueryLimit {
					if verbose {
						log.Printf("fusion pipeline(%d of %d) tile indexer early stopping, query limit reached", idx+1, len(depLayers))
					}
					return totalGrans, nil
				}
			}
		}

		if verbose {
			log.Printf("fusion pipeline(%d of %d) tile indexer done", idx+1, len(depLayers))
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
			UserSrcGeoTransform: geoReq.UserSrcGeoTransform,
			UserSrcSRS:          geoReq.UserSrcSRS,
			NoReprojection:      geoReq.NoReprojection,
			AxisMapping:         geoReq.AxisMapping,
			MasQueryHint:        geoReq.MasQueryHint,
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

		masAddress := styleLayer.MASAddress
		if useOverview {
			hasOverview := len(styleLayer.Overviews) > 0
			if hasOverview {
				iOvr := utils.FindLayerBestOverview(styleLayer, geoReq.ReqRes, true)
				if iOvr >= 0 {
					ovr := styleLayer.Overviews[iOvr]
					ctx.GeoReq.Collection = ovr.DataSource
					masAddress = ovr.MASAddress

				}
			}
		}
		ctx.MASAddress = masAddress
	}
}

func getFlexRaster(idx int, timestamp time.Time, req *GeoTileRequest, raster utils.Raster) (*FlexRaster, bool) {
	namespace := fmt.Sprintf(fusedBandName+"%d", idx)
	flex := &FlexRaster{ConfigPayLoad: ConfigPayLoad{NameSpaces: req.ConfigPayLoad.NameSpaces}, NameSpace: namespace, TimeStamp: float64(timestamp.Unix()), Polygon: "dummy_polygon", Height: req.Height, Width: req.Width, DataHeight: req.Height, DataWidth: req.Width}

	allFilled := true
	switch t := raster.(type) {
	case *utils.SignedByteRaster:
		flex.Type = "SignedByte"
		flex.NoData = t.NoData
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&t.Data))
		flex.Data = *(*[]uint8)(unsafe.Pointer(&headr))
		noData := int8(t.NoData)
		for i := range t.Data {
			if t.Data[i] == noData {
				allFilled = false
				break
			}
		}

	case *utils.ByteRaster:
		flex.Type = "Byte"
		flex.NoData = t.NoData
		flex.Data = t.Data
		noData := uint8(t.NoData)
		for i := range t.Data {
			if t.Data[i] == noData {
				allFilled = false
				break
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
		for i := range t.Data {
			if t.Data[i] == noData {
				allFilled = false
				break
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
		for i := range t.Data {
			if t.Data[i] == noData {
				allFilled = false
				break
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
		for i := range t.Data {
			if t.Data[i] == noData {
				allFilled = false
				break
			}
		}

	}

	return flex, allFilled
}

func (dp *TilePipeline) checkFusedBandNames(geoReq *GeoTileRequest) ([]string, bool, error) {
	var otherVars []string
	hasFusedBand := false
	for _, ns := range geoReq.BandExpr.VarList {
		if len(ns) > len(fusedBandName) && ns[:len(fusedBandName)] == fusedBandName {
			fusedNs := strings.Split(ns[len(fusedBandName):], "_")[0]
			_, err := strconv.ParseInt(fusedNs, 10, 64)
			if err != nil {
				return nil, false, fmt.Errorf("invalid namespace: %v", ns)
			}

			hasFusedBand = true
			continue
		}
		otherVars = append(otherVars, ns)
	}
	return otherVars, hasFusedBand, nil
}
