package processor

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"
)

func TestComputeReprojectionExtent(t *testing.T) {

	masAddress := "127.0.0.1:8888"
	workerNodes := []string{"127.0.0.1:6000"}
	collection := "/g/data2/tc43/modis-fc/v310/tiles/monthly/cover"
	namespaces := []string{"bare_soil", "phot_veg", "nphot_veg"}

	const ISOFormat = "2006-01-02T15:04:05.000Z"
	startTime, _ := time.Parse(ISOFormat, "2018-01-01T00:00:00.000Z")
	testURL := strings.Replace(fmt.Sprintf("http://%s%s?timestamps&time=%s&since=%s&namespace=%s", masAddress, collection, "", "", namespaces), " ", "%20", -1)
	_, err := http.Get(testURL)
	masOnline := err == nil

	if !masOnline {
		t.Skip("MAS endpoint is unavailable. Skipping tests that require MAS connection")
		return
	}

	ctx := context.Background()
	bbox := []float64{-179, -80, 180, 80}
	geoReq := &GeoTileRequest{ConfigPayLoad: ConfigPayLoad{NameSpaces: namespaces,
		PolygonSegments: 10,
		ZoomLimit:       0.0,
	},
		Collection: collection,
		CRS:        "EPSG:4326",
		StartTime:  &startTime,
		BBox:       bbox,
		Width:      1,
	}

	width, height, err := ComputeReprojectionExtent(ctx, geoReq, masAddress, workerNodes, 4326, bbox, true)
	if err != nil {
		t.Errorf("failed to compute projection extent: %v", err)
		return
	}

	expectedWidth := 121717
	expectedHeight := 54247
	if width != 121717 || height != 54247 {
		t.Errorf("unexpected extent: expected (width:121717, height:54247), actual (width:%v, height:%v)", expectedWidth, expectedHeight)
	}
}
