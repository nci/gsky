package processor

import (
	"../utils"
	"fmt"
)

type TilesContainer struct {
	RasterType   string
	ByteBands    map[string][]*ByteRaster
	Int16Bands   map[string][]*Int16Raster
	UInt16Bands  map[string][]*UInt16Raster
	Float32Bands map[string][]*Float32Raster
}

func NewTilesContainer() TilesContainer {
	return TilesContainer{"", make(map[string][]*ByteRaster),
		make(map[string][]*Int16Raster), make(map[string][]*UInt16Raster),
		make(map[string][]*Float32Raster)}
}

type RasterStitcher struct {
	In    chan Raster
	Out   chan []utils.Raster
	Error chan error
}

func NewRasterStitcher(errChan chan error) *RasterStitcher {
	return &RasterStitcher{
		In:    make(chan Raster, 100),
		Out:   make(chan []utils.Raster, 100),
		Error: errChan,
	}
}

func (stch *RasterStitcher) Run() {
	defer close(stch.Out)

	bands := NewTilesContainer()
	var nameSpaces []string
	canvasX := 0
	canvasY := 0

	for raster := range stch.In {
		// We need consistent results on
		// * Type
		// * ConfigPayload
		switch t := raster.(type) {
		case *ByteRaster:
			if t.OffX+t.Width > canvasX {
				canvasX = t.OffX + t.Width
			}

			if t.OffY+t.Height > canvasY {
				canvasY = t.OffY + t.Height
			}

			// Handle empty tiles
			if t.NameSpace == "EmptyTile" {
				continue
			}

			if bands.RasterType == "" {
				bands.RasterType = "ByteRaster"
			}

			if bands.RasterType != "ByteRaster" {
				stch.Error <- fmt.Errorf("Mixed raster types detected!")
				return
			}
			bands.ByteBands[t.NameSpace] = append(bands.ByteBands[t.NameSpace], t)
			nameSpaces = t.NameSpaces
		case *Int16Raster:
			if bands.RasterType == "" {
				bands.RasterType = "Int16Raster"
			}
			if bands.RasterType != "Int16Raster" {
				stch.Error <- fmt.Errorf("Mixed raster types detected!")
				return
			}

			if t.OffX+t.Width > canvasX {
				canvasX = t.OffX + t.Width
			}
			if t.OffY+t.Height > canvasY {
				canvasY = t.OffY + t.Height
			}

			bands.Int16Bands[t.NameSpace] = append(bands.Int16Bands[t.NameSpace], t)
			nameSpaces = t.NameSpaces
		case *UInt16Raster:
			if bands.RasterType == "" {
				bands.RasterType = "UInt16Raster"
			}
			if bands.RasterType != "UInt16Raster" {
				stch.Error <- fmt.Errorf("Mixed raster types detected!")
				return
			}

			if t.OffX+t.Width > canvasX {
				canvasX = t.OffX + t.Width
			}
			if t.OffY+t.Height > canvasY {
				canvasY = t.OffY + t.Height
			}

			bands.UInt16Bands[t.NameSpace] = append(bands.UInt16Bands[t.NameSpace], t)
			nameSpaces = t.NameSpaces
		case *Float32Raster:
			if bands.RasterType == "" {
				bands.RasterType = "Float32Raster"
			}
			if bands.RasterType != "Float32Raster" {
				stch.Error <- fmt.Errorf("Mixed raster types detected!")
				return
			}

			if t.OffX+t.Width > canvasX {
				canvasX = t.OffX + t.Width
			}
			if t.OffY+t.Height > canvasY {
				canvasY = t.OffY + t.Height
			}

			bands.Float32Bands[t.NameSpace] = append(bands.Float32Bands[t.NameSpace], t)
			nameSpaces = t.NameSpaces
		default:
			stch.Error <- fmt.Errorf("Not Implemented")
			return
		}
	}

	if len(nameSpaces) == 0 {
		stch.Out <- []utils.Raster{&utils.ByteRaster{Data: make([]uint8, canvasX*canvasY), Height: canvasY, Width: canvasX}}
		return
	}

	out := make([]utils.Raster, len(nameSpaces))

	for i, nameSpace := range nameSpaces {
		switch bands.RasterType {
		case "ByteRaster":
			if len(bands.ByteBands[nameSpace]) == 0 {
				stch.Error <- fmt.Errorf("Empty namespace %s on ByteRaster", nameSpace)
				return
			}
			dst := &utils.ByteRaster{NoData: bands.ByteBands[nameSpace][0].NoData, Data: make([]uint8, canvasX*canvasY),
				Width: canvasX, Height: canvasY}

			for _, raster := range bands.ByteBands[nameSpace] {
				for j := 0; j < raster.Height; j++ {
					for i := 0; i < raster.Width; i++ {
						dst.Data[(raster.OffY+j)*dst.Width+(raster.OffX+i)] = raster.Data[j*raster.Width+i]
					}
				}
			}
			out[i] = dst

		case "Int16Raster":
			if len(bands.Int16Bands[nameSpace]) == 0 {
				stch.Error <- fmt.Errorf("Empty namespace %s on Int16Raster", nameSpace)
				return
			}
			dst := &utils.Int16Raster{NoData: bands.Int16Bands[nameSpace][0].NoData, Data: make([]int16, canvasX*canvasY),
				Width: canvasX, Height: canvasY}

			for _, raster := range bands.Int16Bands[nameSpace] {
				for j := 0; j < raster.Height; j++ {
					for i := 0; i < raster.Width; i++ {
						dst.Data[(raster.OffY+j)*dst.Width+(raster.OffX+i)] = raster.Data[j*raster.Width+i]
					}
				}
			}
			out[i] = dst

		case "UInt16Raster":
			if len(bands.UInt16Bands[nameSpace]) == 0 {
				stch.Error <- fmt.Errorf("Empty namespace %s on UInt16Raster", nameSpace)
				return
			}
			dst := &utils.UInt16Raster{NoData: bands.UInt16Bands[nameSpace][0].NoData, Data: make([]uint16, canvasX*canvasY),
				Width: canvasX, Height: canvasY}

			for _, raster := range bands.UInt16Bands[nameSpace] {
				for j := 0; j < raster.Height; j++ {
					for i := 0; i < raster.Width; i++ {
						dst.Data[(raster.OffY+j)*dst.Width+(raster.OffX+i)] = raster.Data[j*raster.Width+i]
					}
				}
			}
			out[i] = dst

		case "Float32Raster":
			if len(bands.Float32Bands[nameSpace]) == 0 {
				stch.Error <- fmt.Errorf("Empty namespace %s on Float32Raster", nameSpace)
				return
			}
			dst := &utils.Float32Raster{NoData: bands.Float32Bands[nameSpace][0].NoData, Data: make([]float32, canvasX*canvasY),
				Width: canvasX, Height: canvasY}

			for _, raster := range bands.Float32Bands[nameSpace] {
				for j := 0; j < raster.Height; j++ {
					for i := 0; i < raster.Width; i++ {
						dst.Data[(raster.OffY+j)*dst.Width+(raster.OffX+i)] = raster.Data[j*raster.Width+i]
					}
				}
			}

			out[i] = dst

		default:
			stch.Error <- fmt.Errorf("%s not implemented", bands.RasterType)
			return
		}

	}

	stch.Out <- out
}
