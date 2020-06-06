package extractor

import "time"

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
	Overviews    []*Overview    `json:"overviews,omitempty"`
	XSize        int32          `json:"x_size"`
	YSize        int32          `json:"y_size"`
	GeoTransform []float64      `json:"geotransform"`
	Polygon      string         `json:"polygon"`
	ProjWKT      string         `json:"proj_wkt"`
	Proj4        string         `json:"proj4"`
	Mins         []float64      `json:"mins,omitempty"`
	Maxs         []float64      `json:"maxs,omitempty"`
	Means        []float64      `json:"means,omitempty"`
	StdDevs      []float64      `json:"stddevs,omitempty"`
	SampleCounts []int          `json:"sample_counts,omitempty"`
	NoData       float64        `json:"nodata,omitempty"`
	Axes         []*DatasetAxis `json:"axes,omitempty"`
	GeoLocation  *GeoLocInfo    `json:"geo_loc,omitempty"`
}

type GeoLocInfo struct {
	XDataSetName string `json:"x_ds_name"`
	XBand        int    `json:"x_band"`
	YDataSetName string `json:"y_ds_name"`
	YBand        int    `json:"y_band"`
	LineOffset   int    `json:"line_offset"`
	PixelOffset  int    `json:"pixel_offset"`
	PixelStep    int    `json:"pixel_step"`
	LineStep     int    `json:"line_step"`
}

type DatasetAxis struct {
	Name    string    `json:"name"`
	Params  []float64 `json:"params"`
	Strides []int     `json:"strides"`
	Shape   []int     `json:"shape"`
	Grid    string    `json:"grid"`
}

type GeoFile struct {
	FileName string         `json:"filename,omitempty"`
	Driver   string         `json:"file_type"`
	DataSets []*GeoMetaData `json:"geo_metadata"`
}

type PosixInfo struct {
	FilePath string    `json:"file_path"`
	INode    uint64    `json:"inode"`
	Size     int64     `json:"size"`
	MTime    time.Time `json:"mtime"`
	CTime    time.Time `json:"ctime"`
	ID       string    `json:"id"`
}
