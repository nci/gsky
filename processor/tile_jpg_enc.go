package processor

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"time"
	"utils"
)

type JPGEncoder struct {
	In    chan *ByteRaster
	Out   chan []byte
	Error chan error
}

func NewJPGEncoder(errChan chan error) *JPGEncoder {
	return &JPGEncoder{
		In:    make(chan *ByteRaster, 100),
		Out:   make(chan []byte, 100),
		Error: errChan,
	}
}

func (enc *JPGEncoder) Run() {
	start := time.Now()

	bands := map[string]*ByteRaster{}
	var nameSpaces []string
	palette := []color.RGBA{}
	for raster := range enc.In {
		bands[raster.NameSpace] = raster
		nameSpaces = raster.NameSpaces
		if raster.Palette != nil {
			palette, _ = GradientRGBAPalette(raster.Palette)
		}

	}

	switch len(nameSpaces) {
	case 1:
		buf := new(bytes.Buffer)
		if nameSpaces[0] == "OutOfZoom" {
			f, _ := os.Open(utils.EtcDir + "/zoom.png")
			io.Copy(buf, f)
			enc.Out <- buf.Bytes()
		}
		raster := bands[nameSpaces[0]]
		dst := image.NewRGBA(image.Rect(0, 0, raster.Width, raster.Height))
		for x := 0; x < raster.Width; x++ {
			for y := 0; y < raster.Height; y++ {
				if raster.Data[y*raster.Width+x] != 0x00 {
					dst.Set(x, y, palette[raster.Data[y*raster.Width+x]])
				}
			}
		}

		var opt jpeg.Options
		opt.Quality = 80
		err := jpeg.Encode(buf, dst, &opt)
		if err != nil {
			enc.Error <- err
		}
		enc.Out <- buf.Bytes()
	case 3:
		rasterR := bands[nameSpaces[0]]
		rasterG := bands[nameSpaces[1]]
		rasterB := bands[nameSpaces[2]]

		if rasterR == nil || rasterG == nil || rasterB == nil {
			enc.Error <- fmt.Errorf("At least one of the bands is nil")
			enc.Out <- nil
			return
		}

		nodataR := uint8(rasterR.NoData)
		nodataG := uint8(rasterG.NoData)
		nodataB := uint8(rasterB.NoData)
		canvas := image.NewRGBA(image.Rect(0, 0, rasterR.Width, rasterR.Height))
		var start int
		for i := 0; i < rasterR.Width*rasterR.Height; i++ {
			if rasterR.Data[i] != nodataR && rasterR.Data[i] != 0x00 || rasterG.Data[i] != nodataG && rasterG.Data[i] != 0x00 || rasterB.Data[i] != nodataB && rasterB.Data[i] != 0x00 {
				start = i * 4
				canvas.Pix[start] = rasterR.Data[i]
				canvas.Pix[start+1] = rasterG.Data[i]
				canvas.Pix[start+2] = rasterB.Data[i]
				canvas.Pix[start+3] = 0xff
			}
		}

		buf := new(bytes.Buffer)
		var opt jpeg.Options
		opt.Quality = 80
		err := jpeg.Encode(buf, canvas, &opt)
		if err != nil {
			enc.Error <- err
		}
		enc.Out <- buf.Bytes()
	default:
		enc.Error <- fmt.Errorf("Cannot encode other than 1 or 3 namespaces into a PNG: Received %d", len(nameSpaces))
		enc.Out <- nil
	}

	fmt.Println("PNG Encoder Time", time.Since(start))
}
