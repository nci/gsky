package processor

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
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
			rasters, err := dp.processDeps(geoReq, verbose)
			if err != nil {
				close(m.In)
				return m.Out
			}

			polyLimiter.Increase()
			m.In <- rasters

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

func (dp *TilePipeline) GetFileList(geoReq *GeoTileRequest, verbose bool) chan *GeoTileGranule {
	i := NewTileIndexer(dp.Context, dp.MASAddress, dp.Error)

	if dp.CurrentLayer != nil && len(dp.CurrentLayer.InputLayers) > 0 {
		otherVars, hasFusedBand, err := dp.checkFusedBandNames(geoReq)
		if err != nil {
			dp.Error <- err
			close(i.In)
			return i.Out
		}

		if hasFusedBand {
			grans, err := dp.getDepFileList(geoReq, verbose)
			if err != nil {
				close(i.In)
				return i.Out
			}

			for _, g := range grans {
				i.Out <- g
			}

			if len(otherVars) == 0 {
				close(i.In)
				return i.Out
			}
		}
	}

	go func() {
		i.In <- geoReq
		close(i.In)
	}()

	go i.Run(verbose)
	return i.Out
}

func (dp *TilePipeline) getDepFileList(geoReq *GeoTileRequest, verbose bool) ([]*GeoTileGranule, error) {
	errChan := make(chan error, 100)
	var grans []*GeoTileGranule

	depLayers, err := dp.findDepLayers()
	if err != nil {
		return nil, err
	}
	dp.prepareInputGeoRequests(geoReq, depLayers)

	for idx, reqCtx := range depLayers {
		tp := InitTilePipeline(dp.Context, reqCtx.Service.MASAddress, reqCtx.Service.WorkerNodes, reqCtx.Layer.MaxGrpcRecvMsgSize, reqCtx.Layer.WmsPolygonShardConcLimit, reqCtx.Service.MaxGrpcBufferSize, errChan)

		req := reqCtx.GeoReq
		for gran := range tp.GetFileList(req, verbose) {
			select {
			case err := <-errChan:
				return nil, fmt.Errorf("Error in the fusion pipeline(%d of %d): %v", idx+1, len(depLayers), err)
			case <-tp.Context.Done():
				return nil, fmt.Errorf("Context cancelled in fusion pipeline(%d of %d)", idx+1, len(depLayers))
			default:
				grans = append(grans, gran)
			}
		}

		if verbose {
			log.Printf("fusion pipeline(%d of %d) tile indexer done", idx+1, len(depLayers))
		}
	}

	return grans, nil

}

func (dp *TilePipeline) processDeps(geoReq *GeoTileRequest, verbose bool) ([]*FlexRaster, error) {
	errChan := make(chan error, 100)
	var rasters []*FlexRaster

	depLayers, err := dp.findDepLayers()
	if err != nil {
		return nil, err
	}
	dp.prepareInputGeoRequests(geoReq, depLayers)

	var timestamp time.Time
	if geoReq.StartTime != nil {
		timestamp = *geoReq.StartTime
	} else {
		timestamp = time.Now().UTC()
	}

	for idx, reqCtx := range depLayers {
		tp := InitTilePipeline(dp.Context, reqCtx.Service.MASAddress, reqCtx.Service.WorkerNodes, reqCtx.Layer.MaxGrpcRecvMsgSize, reqCtx.Layer.WmsPolygonShardConcLimit, reqCtx.Service.MaxGrpcBufferSize, errChan)

		req := reqCtx.GeoReq
		select {
		case res := <-tp.Process(req, verbose):
			timeDelta := time.Second * time.Duration(-idx)
			hasScaleParams := !(req.ScaleParams.Offset == 0 && req.ScaleParams.Scale == 0 && req.ScaleParams.Clip == 0)
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
					flex := getFlexRaster(j, timestamp.Add(timeDelta), geoReq, norm[j])
					rasters = append(rasters, flex)
				}
			} else {
				for j := range res {
					flex := getFlexRaster(j, timestamp.Add(timeDelta), geoReq, res[j])
					rasters = append(rasters, flex)
				}
			}

			if verbose {
				log.Printf("fusion pipeline(%d of %d) done", idx+1, len(depLayers))
			}

		case err := <-errChan:
			return nil, fmt.Errorf("Error in the fusion pipeline(%d of %d): %v", idx+1, len(depLayers), err)
		case <-tp.Context.Done():
			return nil, fmt.Errorf("Context cancelled in fusion pipeline(%d of %d)", idx+1, len(depLayers))
		}
	}

	if len(rasters) == 0 {
		for idx, _ := range depLayers {
			emptyRaster := &utils.ByteRaster{Data: make([]uint8, geoReq.Height*geoReq.Width), NoData: 0, Height: geoReq.Height, Width: geoReq.Width, NameSpace: utils.EmptyTileNS}
			emptyFlex := getFlexRaster(idx, timestamp, geoReq, emptyRaster)
			rasters = append(rasters, emptyFlex)
		}
	}

	return rasters, nil

}

func (dp *TilePipeline) findDepLayers() ([]*GeoReqContext, error) {
	var layers []*GeoReqContext
	for _, refLayer := range dp.CurrentLayer.InputLayers {
		config, found := dp.DataSources[refLayer.NameSpace]
		if !found {
			return layers, fmt.Errorf("namespace %s not found referenced by %s", refLayer.NameSpace, refLayer.Name)
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

func (dp *TilePipeline) prepareInputGeoRequests(geoReq *GeoTileRequest, depLayers []*GeoReqContext) {
	for _, ctx := range depLayers {
		styleLayer := ctx.StyleLayer
		ctx.GeoReq = &GeoTileRequest{ConfigPayLoad: ConfigPayLoad{NameSpaces: styleLayer.RGBExpressions.VarList,
			ScaleParams: ScaleParams{Offset: styleLayer.OffsetValue,
				Scale: styleLayer.ScaleValue,
				Clip:  styleLayer.ClipValue,
			},

			BandExpr:        styleLayer.RGBExpressions,
			Mask:            styleLayer.Mask,
			Palette:         styleLayer.Palette,
			ZoomLimit:       geoReq.ZoomLimit,
			PolygonSegments: geoReq.PolygonSegments,
			GrpcConcLimit:   geoReq.GrpcConcLimit,
			QueryLimit:      geoReq.QueryLimit,
		},
			Collection: styleLayer.DataSource,
			CRS:        geoReq.CRS,
			BBox:       geoReq.BBox,
			Height:     geoReq.Height,
			Width:      geoReq.Width,
			StartTime:  geoReq.StartTime,
			EndTime:    geoReq.EndTime,
		}
	}
}

func getFlexRaster(idx int, timestamp time.Time, req *GeoTileRequest, raster utils.Raster) *FlexRaster {
	namespace := fmt.Sprintf(fusedBandName+"%d", idx)
	flex := &FlexRaster{ConfigPayLoad: ConfigPayLoad{NameSpaces: req.ConfigPayLoad.NameSpaces}, NameSpace: namespace, TimeStamp: float64(timestamp.Unix()), Polygon: "dummy_polygon", Height: req.Height, Width: req.Width, DataHeight: req.Height, DataWidth: req.Width}
	switch t := raster.(type) {
	case *utils.ByteRaster:
		flex.Type = "Byte"
		flex.NoData = t.NoData
		flex.Data = t.Data

	case *utils.Int16Raster:
		flex.Type = "Int16"
		flex.NoData = t.NoData
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&t.Data))
		headr.Len *= SizeofInt16
		headr.Cap *= SizeofInt16
		flex.Data = *(*[]uint8)(unsafe.Pointer(&headr))

	case *utils.UInt16Raster:
		flex.Type = "UInt16"
		flex.NoData = t.NoData
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&t.Data))
		headr.Len *= SizeofUint16
		headr.Cap *= SizeofUint16
		flex.Data = *(*[]uint8)(unsafe.Pointer(&headr))

	case *utils.Float32Raster:
		flex.Type = "Float32"
		flex.NoData = t.NoData
		headr := *(*reflect.SliceHeader)(unsafe.Pointer(&t.Data))
		headr.Len *= SizeofFloat32
		headr.Cap *= SizeofFloat32
		flex.Data = *(*[]uint8)(unsafe.Pointer(&headr))

	}

	return flex
}

func (dp *TilePipeline) checkFusedBandNames(geoReq *GeoTileRequest) ([]string, bool, error) {
	var otherVars []string
	hasFusedBand := false
	for _, ns := range geoReq.BandExpr.VarList {
		if len(ns) > len(fusedBandName) && ns[:len(fusedBandName)] == fusedBandName {
			bandIdx64, err := strconv.ParseInt(ns[len(fusedBandName):], 10, 64)
			if err != nil {
				return nil, false, fmt.Errorf("invalid namespace: %v", ns)
			}

			bandIdx := int(bandIdx64)
			if bandIdx < 0 || bandIdx >= len(dp.CurrentLayer.InputLayers) {
				return nil, false, fmt.Errorf("namespace out of range: %v", ns)
			}
			hasFusedBand = true
			continue
		}
		otherVars = append(otherVars, ns)
	}
	return otherVars, hasFusedBand, nil
}
