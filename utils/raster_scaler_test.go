package utils

import (
	"testing"
)

func assert(t *testing.T, out *ByteRaster, expected *ByteRaster, err error) {
	if err != nil {
		t.Errorf("byte raster test failed,  %v", err)
	}
	for i := range out.Data {
		if out.Data[i] != expected.Data[i] {
			t.Errorf("byte raster test failed, expecting %v, actual %v", expected.Data, out.Data)
		}
	}
}

func testByteRaster(t *testing.T) {
	inRaster := make([]Raster, 1)

	sp := ScaleParams{Offset: 1, Scale: 1, Clip: 1000}

	inRaster[0] = &ByteRaster{Data: []uint8{uint8(1), uint8(2)}}
	expOut := &ByteRaster{Data: []uint8{uint8(2), uint8(3)}}
	out, err := Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &ByteRaster{Data: []uint8{uint8(1), uint8(2)}}
	sp = ScaleParams{Offset: 0, Scale: 0, Clip: 2}
	expOut = &ByteRaster{Data: []uint8{uint8(127), uint8(254)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &ByteRaster{Data: []uint8{uint8(1), uint8(2)}}
	sp = ScaleParams{Offset: 3, Scale: 2, Clip: 1000}
	expOut = &ByteRaster{Data: []uint8{uint8(8), uint8(10)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &ByteRaster{Data: []uint8{uint8(1), uint8(2)}}
	sp = ScaleParams{Offset: 3, Scale: 2, Clip: 2}
	expOut = &ByteRaster{Data: []uint8{uint8(4), uint8(4)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)
}

func testInt16Raster(t *testing.T) {
	inRaster := make([]Raster, 1)

	sp := ScaleParams{Offset: 1, Scale: 1, Clip: 1000}

	inRaster[0] = &Int16Raster{Data: []int16{int16(1), int16(2)}, Height: 2, Width: 1}
	expOut := &ByteRaster{Data: []uint8{uint8(2), uint8(3)}}
	out, err := Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &Int16Raster{Data: []int16{int16(1), int16(2)}, Height: 2, Width: 1}
	sp = ScaleParams{Offset: 0, Scale: 0, Clip: 2}
	expOut = &ByteRaster{Data: []uint8{uint8(127), uint8(254)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &Int16Raster{Data: []int16{int16(1), int16(2)}, Height: 2, Width: 1}
	sp = ScaleParams{Offset: 3, Scale: 2, Clip: 1000}
	expOut = &ByteRaster{Data: []uint8{uint8(8), uint8(10)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &Int16Raster{Data: []int16{int16(1), int16(2)}, Height: 2, Width: 1}
	sp = ScaleParams{Offset: 3, Scale: 2, Clip: 2}
	expOut = &ByteRaster{Data: []uint8{uint8(4), uint8(4)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &Int16Raster{Data: []int16{int16(-100), int16(-200)}, Height: 2, Width: 1}
	sp = ScaleParams{Offset: 3, Scale: 2, Clip: 2}
	expOut = &ByteRaster{Data: []uint8{uint8(0), uint8(0)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)
}

func testUInt16Raster(t *testing.T) {
	inRaster := make([]Raster, 1)

	sp := ScaleParams{Offset: 1, Scale: 1, Clip: 1000}

	inRaster[0] = &UInt16Raster{Data: []uint16{uint16(1), uint16(2)}, Height: 2, Width: 1}
	expOut := &ByteRaster{Data: []uint8{uint8(2), uint8(3)}}
	out, err := Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &UInt16Raster{Data: []uint16{uint16(1), uint16(2)}, Height: 2, Width: 1}
	sp = ScaleParams{Offset: 0, Scale: 0, Clip: 2}
	expOut = &ByteRaster{Data: []uint8{uint8(127), uint8(254)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &UInt16Raster{Data: []uint16{uint16(1), uint16(2)}, Height: 2, Width: 1}
	sp = ScaleParams{Offset: 3, Scale: 2, Clip: 1000}
	expOut = &ByteRaster{Data: []uint8{uint8(8), uint8(10)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &UInt16Raster{Data: []uint16{uint16(1), uint16(2)}, Height: 2, Width: 1}
	sp = ScaleParams{Offset: 3, Scale: 2, Clip: 2}
	expOut = &ByteRaster{Data: []uint8{uint8(4), uint8(4)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)
}

func testFloat32Raster(t *testing.T) {
	inRaster := make([]Raster, 1)

	sp := ScaleParams{Offset: 1, Scale: 1, Clip: 1000}

	inRaster[0] = &Float32Raster{Data: []float32{float32(1), float32(2)}, Height: 2, Width: 1}
	expOut := &ByteRaster{Data: []uint8{uint8(2), uint8(3)}}
	out, err := Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &Float32Raster{Data: []float32{float32(1), float32(2)}, Height: 2, Width: 1}
	sp = ScaleParams{Offset: 0, Scale: 0, Clip: 2}
	expOut = &ByteRaster{Data: []uint8{uint8(127), uint8(254)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &Float32Raster{Data: []float32{float32(1), float32(2)}, Height: 2, Width: 1}
	sp = ScaleParams{Offset: 3, Scale: 2, Clip: 1000}
	expOut = &ByteRaster{Data: []uint8{uint8(8), uint8(10)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &Float32Raster{Data: []float32{float32(1), float32(2)}, Height: 2, Width: 1}
	sp = ScaleParams{Offset: 3, Scale: 2, Clip: 2}
	expOut = &ByteRaster{Data: []uint8{uint8(4), uint8(4)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)

	inRaster[0] = &Float32Raster{Data: []float32{float32(-100), float32(-200)}, Height: 2, Width: 1}
	sp = ScaleParams{Offset: 3, Scale: 2, Clip: 2}
	expOut = &ByteRaster{Data: []uint8{uint8(0), uint8(0)}}
	out, err = Scale(inRaster, sp)
	assert(t, out[0], expOut, err)
}

func TestScale(t *testing.T) {
	testByteRaster(t)
	testInt16Raster(t)
	testUInt16Raster(t)
	testFloat32Raster(t)
}
