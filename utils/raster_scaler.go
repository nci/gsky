package utils

import (
	"fmt"
)

type ScaleParams struct {
	Offset float64
	Scale  float64
	Clip   float64
}

func scale(r Raster, params ScaleParams) (*ByteRaster, error) {
	scale := float32(params.Scale)
	if scale <= 0.0 {
		if params.Clip <= 0.0 {
			scale = float32(1.0)
		} else {
			scale = float32(float32(254.0) / float32(params.Clip))
		}
	}

	switch t := r.(type) {
	case *ByteRaster:
		noData := uint8(t.NoData)
		offset := uint8(params.Offset)
		clip := uint8(params.Clip)

		for i, value := range t.Data {
			if value == noData {
				t.Data[i] = 0xFF
			} else {
				value += offset
				if value > clip {
					value = clip
				}
				if value < 0 {
					value = 0
				}
				t.Data[i] = uint8(float32(value) * scale)
			}
		}
		return &ByteRaster{t.Data, t.Height, t.Width, t.NoData}, nil

	case *Int16Raster:
		out := &ByteRaster{NoData: t.NoData, Data: make([]uint8, t.Height*t.Width), Width: t.Width, Height: t.Height}
		noData := int16(t.NoData)
		offset := int16(params.Offset)
		clip := int16(params.Clip)
		for i, value := range t.Data {
			if value == noData {
				out.Data[i] = 0xFF
			} else {
				value += offset
				if value > clip {
					value = clip
				}
				if value < 0 {
					value = 0
				}
				out.Data[i] = uint8(float32(value) * scale)
			}
		}
		return out, nil

	case *UInt16Raster:
		out := &ByteRaster{NoData: t.NoData, Data: make([]uint8, t.Height*t.Width), Width: t.Width, Height: t.Height}
		noData := uint16(t.NoData)
		offset := uint16(params.Offset)
		clip := uint16(params.Clip)
		for i, value := range t.Data {
			if value == noData {
				out.Data[i] = 0xFF
			} else {
				value += offset
				if value > clip {
					value = clip
				}
				if value < 0 {
					value = 0
				}
				out.Data[i] = uint8(float32(value) * scale)
			}
		}
		return out, nil

	case *Float32Raster:
		out := &ByteRaster{NoData: t.NoData, Data: make([]uint8, t.Height*t.Width), Width: t.Width, Height: t.Height}
		noData := float32(t.NoData)
		offset := float32(params.Offset)
		clip := float32(params.Clip)
		for i, value := range t.Data {
			if value == noData {
				out.Data[i] = 0xFF
			} else {
				value += offset
				if value > clip {
					value = clip
				}
				if value < 0.0 {
					value = 0.0
				}
				out.Data[i] = uint8(value * scale)
			}
		}
		return out, nil

	default:
		return &ByteRaster{}, fmt.Errorf("Raster type not implemented")
	}
}

func Scale(rs []Raster, params ScaleParams) ([]*ByteRaster, error) {
	out := make([]*ByteRaster, len(rs))

	for i, r := range rs {
		br, err := scale(r, params)
		if err != nil {
			return out, err
		}
		out[i] = br
	}

	return out, nil
}
