package processor

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	pb "github.com/nci/gsky/grpc_server/gdalservice"
	"github.com/golang/protobuf/ptypes"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type Overview struct {
	XSize int32 `json:"x_size"`
	YSize int32 `json:"y_size"`
}

type GeoMetaData struct {
	DataSetName  string         `json:"ds_name"`
	NameSpace    string         `json:"namespace,omitempty"`
	Type         string         `json:"array_type"`
	RasterCount  int32          `json:"raster_count"`
	TimeStamps   []time.Time    `json:"timestamps"`
	Heights      []float64      `json:"heights,omitempty"`
	Overviews    []*pb.Overview `json:"overviews,omitempty"`
	XSize        int32          `json:"x_size"`
	YSize        int32          `json:"y_size"`
	GeoTransform []float64      `json:"geotransform"`
	Polygon      string         `json:"polygon"`
	ProjWKT      string         `json:"proj_wkt"`
	Proj4        string         `json:"proj4"`
}

type GeoFile struct {
	FileName string        `json:"filename,omitempty"`
	Driver   string        `json:"file_type"`
	DataSets []GeoMetaData `json:"geo_metadata"`
}

type GeoInfoGRPC struct {
	Context context.Context
	In      chan string
	Out     chan *GeoFile
	Error   chan error
	Clients []string
}

func NewInfoGRPC(ctx context.Context, serverAddress []string, errChan chan error) *GeoInfoGRPC {
	return &GeoInfoGRPC{
		Context: ctx,
		In:      make(chan string, 100),
		Out:     make(chan *GeoFile, 100),
		Error:   errChan,
		Clients: serverAddress,
	}
}

func (gi *GeoInfoGRPC) Run() {
	defer close(gi.Out)

	conns := make([]*grpc.ClientConn, len(gi.Clients))
	for i, client := range gi.Clients {
		conn, err := grpc.Dial(client, grpc.WithInsecure())
		if err != nil {
			log.Fatalf("gRPC connection problem: %v", err)
		}
		defer conn.Close()
		conns[i] = conn
	}

	// Concurrency limited to the number of gRPC workers
	cl := NewConcLimiter(16)
	for gran := range gi.In {
		select {
		case <-gi.Context.Done():
			fmt.Println(gi.Context.Err())
			gi.Error <- fmt.Errorf("Tile gRPC context has been cancel: %v", gi.Context.Err())
			return
		default:
			cl.Increase()
			go func(g string) {
				defer cl.Decrease()

				c := pb.NewGDALClient(conns[rand.Intn(len(conns))])
				r, err := c.Process(gi.Context, &pb.GeoRPCGranule{Path: g})
				if err != nil {
					fmt.Println(err)
					gi.Error <- err
					return
				}
				a := r.Info
				ds := make([]GeoMetaData, len(a.DataSets))
				for i, d := range a.DataSets {
					ts := make([]time.Time, len(d.TimeStamps))
					for j, pbt := range d.TimeStamps {
						t, err := ptypes.Timestamp(pbt)
						if err != nil {
							panic(err)
						}
						ts[j] = t
					}
					ds[i] = GeoMetaData{DataSetName: d.DatasetName, NameSpace: d.NameSpace, Type: d.Type,
						RasterCount: d.RasterCount, TimeStamps: ts, Heights: d.Height,
						Overviews: d.Overviews, XSize: d.XSize, YSize: d.YSize,
						GeoTransform: d.GeoTransform, Polygon: d.Polygon, ProjWKT: d.ProjWKT,
						Proj4: d.Proj4}
				}
				out := &GeoFile{FileName: a.FileName, Driver: a.Driver, DataSets: ds}
				gi.Out <- out
			}(gran)
		}
	}

	cl.Wait()
}
