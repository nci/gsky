package utils

import (
	"bytes"
	//"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
)

const tSize = 256

func GetEmptyTile(imageFilename string, height, width int) ([]byte, error) {
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))

	if len(imageFilename) > 0 {
		infile, err := os.Open(imageFilename)
		if err != nil {
			return nil, err
		}
		defer infile.Close()

		// Decode will figure out what type of image is in the file on its own.
		// We just have to be sure all the image packages we want are imported.
		tile, _, err := image.Decode(infile)
		if err != nil {
			return nil, err
		}

		for x := 0; x < width; x += tSize {
			for y := 0; y < height; y += tSize {
				draw.Draw(canvas, image.Rect(x, y, x+tSize, y+tSize), tile, image.ZP, draw.Src)
			}
		}
	}

	buf := new(bytes.Buffer)
	err := png.Encode(buf, canvas)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), err
}
