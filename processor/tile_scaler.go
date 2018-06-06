package processor

type RasterScaler struct {
	In    chan Raster
	Out   chan *ByteRaster
	Error chan error
}

func NewRasterScaler(errChan chan error) *RasterScaler {
	return &RasterScaler{
		In:    make(chan Raster, 100),
		Out:   make(chan *ByteRaster, 100),
		Error: errChan,
	}
}

func (scl *RasterScaler) Run() {
	defer close(scl.Out)

	for raster := range scl.In {
		switch t := raster.(type) {
		case *ByteRaster:

			noData := uint8(t.NoData)
			scale := t.ScaleParams.Scale
			clip := uint8(t.ScaleParams.Clip)

			for i, value := range t.Data {
				if value == noData {
					t.Data[i] = 0xFF
				} else {
					if value > clip {
						value = clip
					}
					if value < 0 {
						value = 0
					}
					t.Data[i] = uint8(float64(value) * scale)
				}
			}
			scl.Out <- t

		case *Int16Raster:
			out := &ByteRaster{ConfigPayLoad: ConfigPayLoad{NameSpaces: t.NameSpaces, ScaleParams: t.ScaleParams, Palette: t.Palette},
				NoData: t.NoData, Data: make([]uint8, t.Height*t.Width), Width: t.Width, Height: t.Height,
				OffX: t.OffX, OffY: t.OffY, NameSpace: t.NameSpace}
			noData := int16(t.NoData)
			clip := int16(t.ScaleParams.Clip)
			for i, value := range t.Data {
				if value == noData {
					out.Data[i] = 0xFF
				} else {
					if value > clip {
						value = clip
					}
					if value < 0 {
						value = 0
					}
					out.Data[i] = uint8(float32(value) * 254.0 / float32(clip))
				}
			}
			scl.Out <- out

		case *UInt16Raster:
			out := &ByteRaster{ConfigPayLoad: ConfigPayLoad{NameSpaces: t.NameSpaces, ScaleParams: t.ScaleParams, Palette: t.Palette},
				NoData: t.NoData, Data: make([]uint8, t.Height*t.Width), Width: t.Width, Height: t.Height,
				OffX: t.OffX, OffY: t.OffY, NameSpace: t.NameSpace}
			noData := uint16(t.NoData)
			clip := uint16(t.ScaleParams.Clip)
			for i, value := range t.Data {
				if value == noData {
					out.Data[i] = 0xFF
				} else {
					if value > clip {
						value = clip
					}
					if value < 0 {
						value = 0
					}
					out.Data[i] = uint8(float32(value) * 254.0 / float32(clip))
				}
			}
			scl.Out <- out

		case *Float32Raster:
			out := &ByteRaster{ConfigPayLoad: ConfigPayLoad{NameSpaces: t.NameSpaces, ScaleParams: t.ScaleParams, Palette: t.Palette},
				NoData: t.NoData, Data: make([]uint8, t.Height*t.Width), Width: t.Width, Height: t.Height,
				OffX: t.OffX, OffY: t.OffY, NameSpace: t.NameSpace}

			noData := float32(t.NoData)
			scale := float32(t.ScaleParams.Scale)
			offset := uint8(t.ScaleParams.Offset)
			clip := float32(t.ScaleParams.Clip)

			for i, value := range t.Data {
				if value == noData {
					out.Data[i] = 0xFF
				} else {
					value += float32(offset)
					if value > clip {
						value = clip
					}
					if value < 0 {
						value = 0
					}
					out.Data[i] = uint8(value * scale)
				}
			}
			scl.Out <- out
		}
	}
}
