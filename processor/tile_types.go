package processor

import (
	"github.com/nci/gsky/utils"
	"time"
)

type ScaleParams struct {
	Offset float64
	Scale  float64
	Clip   float64
}

type ConfigPayLoad struct {
	NameSpaces  []string
	ScaleParams ScaleParams
	Palette     *utils.Palette
	Mask        *utils.Mask
	ZoomLimit   float64
}

type GeoTileRequest struct {
	ConfigPayLoad
	Collection    string
	CRS           string
	BBox          []float64
	Height, Width int
	OffX, OffY    int
	StartTime     *time.Time
	EndTime       *time.Time
}

type GeoTileGranule struct {
	ConfigPayLoad
	Path          string
	CRS           string
	BBox          []float64
	Height, Width int
	OffX, OffY    int
	NameSpace     string
	TimeStamps    []time.Time
	TimeStamp     time.Time
	Polygon       string
	RasterType    string
}

type FlexRaster struct {
	ConfigPayLoad
	Data          []byte
	Height, Width int
	OffX, OffY    int
	Type          string
	NoData        float64
	NameSpace     string
	TimeStamp     time.Time
	Polygon       string
}

type Raster interface {
	GetNoData() float64
}

type ByteRaster struct {
	ConfigPayLoad
	Data          []uint8
	Height, Width int
	OffX, OffY    int
	NoData        float64
	NameSpace     string
}

func (br *ByteRaster) GetNoData() float64 {
	return br.NoData
}

type UInt16Raster struct {
	ConfigPayLoad
	Data          []uint16
	Height, Width int
	OffX, OffY    int
	NoData        float64
	NameSpace     string
}

func (u16 *UInt16Raster) GetNoData() float64 {
	return u16.NoData
}

type Int16Raster struct {
	ConfigPayLoad
	Data          []int16
	Height, Width int
	OffX, OffY    int
	NoData        float64
	NameSpace     string
}

func (s16 *Int16Raster) GetNoData() float64 {
	return s16.NoData
}

type Float32Raster struct {
	ConfigPayLoad
	Data          []float32
	Height, Width int
	OffX, OffY    int
	NoData        float64
	NameSpace     string
}

func (f32 *Float32Raster) GetNoData() float64 {
	return f32.NoData
}
