package processor

import (
	"github.com/nci/gsky/utils"
	"image/color"
)

// InterpolateUint8 interpolates the value of a
// byte between two numbers 'a' and 'b' by
// especifying a length and a position 'i'
// along that length.
func InterpolateUint8(a, b uint8, i, sectionLength int) uint8 {
	return a + uint8((i * (int(b) - int(a)) / sectionLength))
}

// InterpolateColor returns an RGBA color where
// the R, G, B, and A components have been
// interpolated from the 'a' and 'b' colors
func InterpolateColor(a, b color.RGBA, i, sectionLength int) color.RGBA {
	return color.RGBA{InterpolateUint8(a.R, b.R, i, sectionLength),
		InterpolateUint8(a.G, b.G, i, sectionLength),
		InterpolateUint8(a.B, b.B, i, sectionLength),
		255}
}

// GradientPalette returns a palette of 256 colors
// creating an interpolation that goes though
// a list of provided colours.
func GradientRGBAPalette(palette *utils.Palette) ([]color.RGBA, error) {
	if palette == nil {
		return nil, nil
	}

	ramp := make([]color.RGBA, 256)

	if palette.Interpolate {
		bins := len(palette.Colours) - 1
		sectionLength := 256 / bins
		bonus := 256 - (sectionLength * bins)
		bonusArr := make([]int, bins)
		for i := 0; i < bonus; i++ {
			bonusArr[i] = 1
		}

		index := 0
		for section, upperColour := range palette.Colours[1:] {
			for i := 0; i < sectionLength+bonusArr[section]; i++ {
				ramp[index] = InterpolateColor(palette.Colours[section], upperColour, i, sectionLength)
				index++
			}
		}
	} else {
		bins := len(palette.Colours)
		sectionLength := 256 / bins
		bonus := 256 - (sectionLength * bins)
		bonusArr := make([]int, bins)
		for i := 0; i < bonus; i++ {
			bonusArr[i] = 1
		}

		index := 0
		for section, colour := range palette.Colours {
			for i := 0; i < sectionLength+bonusArr[section]; i++ {
				ramp[index] = colour
				index++
			}
		}
	}

	return ramp, nil
}
