package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/nci/gsky/utils"
)

func serveDap(ctx context.Context, conf *utils.Config, reqURL string, w http.ResponseWriter, query map[string][]string) {
	ceStr := query["dap4.ce"][0]
	ce, err := utils.ParseDap4ConstraintExpr(ceStr)
	if err != nil {
		logDapError(err)
		http.Error(w, fmt.Sprintf("Failed to parse dap4.ce: %v", err), 400)
		return
	}

	if *verbose {
		utils.DumpDap4CE(ce)
	}

	wcsParams, err := dapToWcs(ce, conf)
	if err != nil {
		logDapError(err)
		http.Error(w, fmt.Sprintf("Failed to parse dap4.ce: %v", err), 400)
		return
	}

	serveWCS(ctx, *wcsParams, conf, reqURL, w, query)
}

func dapToWcs(ce *utils.DapConstraints, conf *utils.Config) (*utils.WCSParams, error) {
	defaultBbox := []float64{-180, -90, 180, 90}
	defaultGeoSize := []int{-1, -1}

	wcsParams := &utils.WCSParams{BBox: defaultBbox, Coverages: []string{ce.Dataset}, NoReprojection: true, AxisMapping: 1}
	idx, err := utils.GetCoverageIndex(*wcsParams, conf)
	if err != nil {
		return wcsParams, fmt.Errorf("dataset not found: %v", ce.Dataset)
	}

	layer := conf.Layers[idx]
	if len(layer.DefaultGeoBbox) == 4 {
		defaultBbox = layer.DefaultGeoBbox
	}

	if len(layer.DefaultGeoSize) == 2 {
		defaultGeoSize = layer.DefaultGeoSize
	}

	wcsParams.Service = new(string)
	wcsParams.Request = new(string)
	wcsParams.CRS = new(string)
	wcsParams.Version = new(string)
	wcsParams.Format = new(string)
	wcsParams.Width = new(int)
	wcsParams.Height = new(int)

	*wcsParams.Service = "WCS"
	*wcsParams.Request = "GetCoverage"
	*wcsParams.CRS = "EPSG:4326"
	*wcsParams.Version = "1.0.0"
	*wcsParams.Format = "dap4"
	*wcsParams.Width = defaultGeoSize[1]
	*wcsParams.Height = defaultGeoSize[0]

	var varExpr []string
	for _, vp := range ce.VarParams {
		if vp.IsAxis {
			if vp.Name == "x" {
				isOutRange := *vp.ValStart < defaultBbox[0] || *vp.ValStart > defaultBbox[2]
				if !isOutRange {
					wcsParams.BBox[0] = *vp.ValStart
				}

				if vp.ValEnd == nil {
					continue
				}

				isOutRange = *vp.ValEnd < defaultBbox[0] || *vp.ValEnd > defaultBbox[2]
				if !isOutRange {
					wcsParams.BBox[2] = *vp.ValEnd
				}
				continue
			}

			if vp.Name == "y" {
				isOutRange := *vp.ValStart < defaultBbox[1] || *vp.ValStart > defaultBbox[3]
				if !isOutRange {
					wcsParams.BBox[1] = *vp.ValStart
				}

				if vp.ValEnd == nil {
					continue
				}

				isOutRange = *vp.ValEnd < defaultBbox[1] || *vp.ValEnd > defaultBbox[3]
				if !isOutRange {
					wcsParams.BBox[3] = *vp.ValEnd
				}
				continue
			}

			axisParam := &utils.AxisParam{Name: vp.Name, Start: vp.ValStart, End: vp.ValEnd}
			wcsParams.Axes = append(wcsParams.Axes, axisParam)

		} else {
			varExpr = append(varExpr, vp.Name)
		}
	}

	if len(varExpr) == 0 {
		varExpr = append(varExpr, utils.EmptyTileNS)
		if len(ce.VarParams) > 0 {
			wcsParams.AxisMapping = 0
		}
	}

	bandExpr, err := utils.ParseBandExpressions(varExpr)
	if err != nil {
		return wcsParams, fmt.Errorf("parsing error in main variable expressions: %v", err)
	}
	wcsParams.BandExpr = bandExpr

	return wcsParams, nil
}

func logDapError(err error) {
	log.Printf("DAP: error: %s", err)
}
