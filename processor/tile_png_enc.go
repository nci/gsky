package processor

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	//"time"
	"fmt"
	"image/draw"
	"log"
)

type PNGEncoder struct {
	In    chan *ByteRaster
	Out   chan []byte
	Error chan error
}

func NewPNGEncoder(errChan chan error) *PNGEncoder {
	return &PNGEncoder{
		In:    make(chan *ByteRaster, 100),
		Out:   make(chan []byte, 100),
		Error: errChan,
	}
}

func (enc *PNGEncoder) Run() {
	defer close(enc.Out)

	//start := time.Now()

	bands := map[string][]*ByteRaster{}
	var nameSpaces []string
	palette := []color.RGBA{}
	var err error
	canvasX := 0
	canvasY := 0
	for raster := range enc.In {
		maxX := raster.OffX + raster.Width
		maxY := raster.OffY + raster.Height
		if maxX > canvasX {
			canvasX = maxX
		}
		if maxY > canvasY {
			canvasY = maxY
		}

		bands[raster.NameSpace] = append(bands[raster.NameSpace], raster)
		nameSpaces = raster.NameSpaces
		if raster.Palette != nil {
			palette, err = GradientRGBAPalette(raster.Palette)
			if err != nil {
				enc.Error <- err
				return
			}
		}

	}

	switch len(nameSpaces) {
	case 1:
		buf := new(bytes.Buffer)
		if nameSpaces[0] == "OutOfZoom" {
			f, err := os.Open("zoom.png")
			if err != nil {
				enc.Error <- fmt.Errorf("missing zoom.png")
				enc.Out <- nil
				return
			}
			io.Copy(buf, f)
			enc.Out <- buf.Bytes()
		}

		dst := image.NewRGBA(image.Rect(0, 0, canvasX, canvasY))
		for _, raster := range bands[nameSpaces[0]] {
			tile := image.NewRGBA(image.Rect(0, 0, raster.Width, raster.Height))
			for x := 0; x < raster.Width; x++ {
				for y := 0; y < raster.Height; y++ {
					if raster.Data[y*raster.Width+x] != 0xFF {
						tile.Set(x, y, palette[raster.Data[y*raster.Width+x]])
					}
				}
			}
			draw.Draw(dst, image.Rect(raster.OffX, canvasY-(raster.OffY+raster.Height), raster.OffX+raster.Width, canvasY-raster.OffY), tile, image.ZP, draw.Src)
		}

		err := png.Encode(buf, dst)
		if err != nil {
			enc.Error <- err
		}
		enc.Out <- buf.Bytes()

	case 3:
		dst := image.NewRGBA(image.Rect(0, 0, canvasX, canvasY))

		if len(bands[nameSpaces[0]]) != len(bands[nameSpaces[1]]) || len(bands[nameSpaces[0]]) != len(bands[nameSpaces[2]]) {
			enc.Error <- fmt.Errorf("Inconsistent length of band namespaces")
			enc.Out <- nil
			return
		}

		for i := 0; i < len(bands[nameSpaces[0]]); i++ {
			rasterR := bands[nameSpaces[0]][i]
			rasterG := bands[nameSpaces[1]][i]
			rasterB := bands[nameSpaces[2]][i]

			if rasterR == nil || rasterG == nil || rasterB == nil {
				enc.Error <- fmt.Errorf("At least one of the bands is nil")
				enc.Out <- nil
				return
			}

			tile := image.NewRGBA(image.Rect(0, 0, canvasX, canvasY))
			var start int
			for i := 0; i < rasterR.Width*rasterR.Height; i++ {
				if rasterR.Data[i] != 0xFF || rasterG.Data[i] != 0xFF || rasterB.Data[i] != 0xFF {
					start = i * 4
					tile.Pix[start] = rasterR.Data[i]
					tile.Pix[start+1] = rasterG.Data[i]
					tile.Pix[start+2] = rasterB.Data[i]
					tile.Pix[start+3] = 0xff
				}
			}
			draw.Draw(dst, image.Rect(rasterR.OffX, canvasY-(rasterR.OffY+rasterR.Height), rasterR.OffX+rasterR.Width, canvasY-rasterR.OffY), tile, image.ZP, draw.Src)
		}

		buf := new(bytes.Buffer)
		err := png.Encode(buf, dst)
		if err != nil {
			enc.Error <- err
		}
		enc.Out <- buf.Bytes()
	default:
		log.Printf("Cannot encode other than 1 or 3 namespaces into a PNG. Received %d namespaces: %v\n", len(nameSpaces), nameSpaces)
		buf := new(bytes.Buffer)
		f, err := os.Open("data_unavailable.png")
		if err != nil {
			enc.Error <- fmt.Errorf("missing data_unavailable.png")
			enc.Out <- nil
			return
		}
		io.Copy(buf, f)
		enc.Out <- buf.Bytes()

	}

	//fmt.Println("PNG Encoder Time", time.Since(start))
}
