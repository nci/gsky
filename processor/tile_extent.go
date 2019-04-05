package processor

//#include "ogr_srs_api.h"
//#cgo pkg-config: gdal
import "C"

import (
	"fmt"
	"log"
	"math/rand"
	"reflect"
	"unsafe"

	pb "github.com/nci/gsky/worker/gdalservice"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func ComputeReprojectionExtent(ctx context.Context, geoReq *GeoTileRequest, masAddress string, workerNodes []string, epsg int, bbox []float64, verbose bool) (int, int, error) {
	errChan := make(chan error, 100)
	indexer := NewTileIndexer(ctx, masAddress, errChan)
	go func() {
		indexer.In <- geoReq
		close(indexer.In)
	}()

	go indexer.Run(verbose)

	fileLookup := make(map[string]bool)
	var indexGrans []*GeoTileGranule
	for gran := range indexer.Out {
		select {
		case err := <-errChan:
			return -1, -1, err
		case <-ctx.Done():
			return -1, -1, ctx.Err()
		default:
			if _, found := fileLookup[gran.RawPath]; !found {
				fileLookup[gran.RawPath] = true
				indexGrans = append(indexGrans, gran)
			}
		}
	}

	const DefaultRecvMsgSize = 100 * 1024 * 1024
	const DefaultConcLimit = 16

	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(DefaultRecvMsgSize)),
	}

	var conns []*grpc.ClientConn
	for _, worker := range workerNodes {
		conn, err := grpc.Dial(worker, opts...)
		if err != nil {
			log.Printf("gRPC connection problem: %v", err)
			continue
		}
		defer conn.Close()
		conns = append(conns, conn)
	}

	cLimiter := NewConcLimiter(DefaultConcLimit * len(conns))
	type OutputSize struct {
		Width  int
		Height int
	}

	if verbose {
		log.Printf("tile_extent: total files: %v", len(indexGrans))
	}

	hSRS := C.OSRNewSpatialReference(nil)
	defer C.OSRDestroySpatialReference(hSRS)
	crsC := C.CString(geoReq.CRS)
	defer C.free(unsafe.Pointer(crsC))
	C.OSRSetFromUserInput(hSRS, crsC)
	var projWKTC *C.char
	defer C.free(unsafe.Pointer(projWKTC))
	C.OSRExportToWkt(hSRS, &projWKTC)
	projWKT := C.GoString(projWKTC)

	workerStart := rand.Intn(len(conns))

	outChan := make(chan *OutputSize, len(indexGrans))
	for ig, gran := range indexGrans {
		select {
		case <-ctx.Done():
			return -1, -1, fmt.Errorf("tile_extent: gRPC context has been canceled: %v", ctx.Err())
		default:
			if verbose {
				if (ig+1)%int(len(indexGrans)/10+1) == 0 {
					log.Printf("tile_extent: %v of %v done", ig+1, len(indexGrans))
				}
			}

			cLimiter.Increase()
			go func(g *GeoTileGranule, conc *ConcLimiter, iTile int) {
				defer conc.Decrease()
				c := pb.NewGDALClient(conns[(iTile+workerStart)%len(conns)])

				dsPath := g.Path
				if dsPath == "NULL" {
					dsPath = g.RawPath
				}
				granule := &pb.GeoRPCGranule{Operation: "extent", Path: dsPath, DstSRS: projWKT, DstGeot: bbox}
				res, err := c.Process(ctx, granule)
				if err != nil {
					errChan <- err
					return
				}

				header := *(*reflect.SliceHeader)(unsafe.Pointer(&res.Raster.Data))
				intSize := int(unsafe.Sizeof(int(0)))

				header.Len /= intSize
				header.Cap /= intSize

				data := *(*[]int)(unsafe.Pointer(&header))

				outChan <- &OutputSize{Width: data[0], Height: data[1]}
			}(gran, cLimiter, ig)
		}
	}

	maxWidth := -1
	maxHeight := -1

	iTile := 0
	allTilesDone := false
	for {
		select {
		case err := <-errChan:
			return -1, -1, err
		case <-ctx.Done():
			return -1, -1, fmt.Errorf("Context canceled: %v", ctx.Err())
		case outSize := <-outChan:
			if outSize.Width > maxWidth {
				maxWidth = outSize.Width
			}

			if outSize.Height > maxHeight {
				maxHeight = outSize.Height
			}

			iTile++
			if iTile == len(indexGrans) {
				allTilesDone = true
				break
			}
		}

		if allTilesDone {
			break
		}

	}

	if maxWidth <= 0 || maxHeight <= 0 {
		return -1, -1, fmt.Errorf("unable to compute extent: width(%v), height(%v)", maxWidth, maxHeight)
	}

	return maxWidth, maxHeight, nil
}
