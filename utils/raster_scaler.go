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
	case *SignedByteRaster:
		out := &ByteRaster{NameSpace: t.NameSpace, NoData: t.NoData, Data: make([]uint8, t.Height*t.Width), Width: t.Width, Height: t.Height}
		noData := int8(t.NoData)
		offset := int8(params.Offset)
		clip := int8(params.Clip)

		if params.Scale == 0.0 && params.Clip == 0.0 && params.Offset == 0.0 {
			var minVal, maxVal float32
			for i, value := range t.Data {
				if value == noData {
					continue
				}

				val := float32(value)
				if i == 0 {
					minVal = val
					maxVal = val
				} else {
					if val < minVal {
						minVal = val
					}

					if val > maxVal {
						maxVal = val
					}
				}
			}

			if minVal == maxVal {
				maxVal += 0.1
			}

			scale = 254.0 / (maxVal - minVal)
			dfOffset := -minVal

			offset = int8(dfOffset)
			clip = int8(maxVal + dfOffset)
		}

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

	case *ByteRaster:
		noData := uint8(t.NoData)
		offset := uint8(params.Offset)
		clip := uint8(params.Clip)

		if params.Scale == 0.0 && params.Clip == 0.0 && params.Offset == 0.0 {
			var minVal, maxVal float32
			for i, value := range t.Data {
				if value == noData {
					continue
				}

				val := float32(value)
				if i == 0 {
					minVal = val
					maxVal = val
				} else {
					if val < minVal {
						minVal = val
					}

					if val > maxVal {
						maxVal = val
					}
				}
			}

			if minVal == maxVal {
				maxVal += 0.1
			}

			scale = 254.0 / (maxVal - minVal)
			dfOffset := -minVal

			offset = uint8(dfOffset)
			clip = uint8(maxVal + dfOffset)
		}

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
		return &ByteRaster{t.NameSpace, t.Data, t.Height, t.Width, t.NoData}, nil

	case *Int16Raster:
		out := &ByteRaster{NameSpace: t.NameSpace, NoData: t.NoData, Data: make([]uint8, t.Height*t.Width), Width: t.Width, Height: t.Height}
		noData := int16(t.NoData)
		offset := int16(params.Offset)
		clip := int16(params.Clip)

		if params.Scale == 0.0 && params.Clip == 0.0 && params.Offset == 0.0 {
			var minVal, maxVal float32
			for i, value := range t.Data {
				if value == noData {
					continue
				}

				val := float32(value)
				if i == 0 {
					minVal = val
					maxVal = val
				} else {
					if val < minVal {
						minVal = val
					}

					if val > maxVal {
						maxVal = val
					}
				}
			}

			if minVal == maxVal {
				maxVal += 0.1
			}

			scale = 254.0 / (maxVal - minVal)
			dfOffset := -minVal

			offset = int16(dfOffset)
			clip = int16(maxVal + dfOffset)
		}

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
		out := &ByteRaster{NameSpace: t.NameSpace, NoData: t.NoData, Data: make([]uint8, t.Height*t.Width), Width: t.Width, Height: t.Height}
		noData := uint16(t.NoData)
		offset := uint16(params.Offset)
		clip := uint16(params.Clip)

		if params.Scale == 0.0 && params.Clip == 0.0 && params.Offset == 0.0 {
			var minVal, maxVal float32
			for i, value := range t.Data {
				if value == noData {
					continue
				}

				val := float32(value)
				if i == 0 {
					minVal = val
					maxVal = val
				} else {
					if val < minVal {
						minVal = val
					}

					if val > maxVal {
						maxVal = val
					}
				}
			}

			if minVal == maxVal {
				maxVal += 0.1
			}

			scale = 254.0 / (maxVal - minVal)
			dfOffset := -minVal

			offset = uint16(dfOffset)
			clip = uint16(maxVal + dfOffset)
		}

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
		out := &ByteRaster{NameSpace: t.NameSpace, NoData: t.NoData, Data: make([]uint8, t.Height*t.Width), Width: t.Width, Height: t.Height}
		noData := float32(t.NoData)
		offset := float32(params.Offset)
		clip := float32(params.Clip)

		if params.Scale == 0.0 && params.Clip == 0.0 && params.Offset == 0.0 {
			var minVal, maxVal float32
			for i, value := range t.Data {
				if value == noData {
					continue
				}

				if i == 0 {
					minVal = value
					maxVal = value
				} else {
					if value < minVal {
						minVal = value
					}

					if value > maxVal {
						maxVal = value
					}
				}
			}

			if minVal == maxVal {
				maxVal += 0.1
			}

			scale = 254.0 / (maxVal - minVal)
			offset = -minVal

			clip = maxVal + offset
		}

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
