package img2xterm

import (
	"fmt"
	"math"
	"strconv"
)

const (
	colorUndef       = iota
	colorTransparent = iota
)

type Config struct {
	// Specify width and height of the input image
	Width, Height int

	// True to draw colorized image
	// False to draw in grayscale
	Colorful bool

	// How to compute the distance of colors
	// 'rgb': Use simple RGB linear distance
	// 'yiq': Still linear distance, but in YIQ colorspace (default)
	// 'cie94': Use the CIE94 algorithm, which consumes CPU more
	DistanceAlgorithm string
}

var oldfg, oldbg uint8 = colorUndef, colorUndef
var sequence string

func rawRGB2Pixels(raw []byte) (ret [][3]byte) {
	for cur := 0; cur < len(raw); cur += 3 {
		pixel := [3]byte{raw[cur], raw[cur+1], raw[cur+2]}
		ret = append(ret, pixel)
	}
	return
}

func rawRGB2BrightnessPixels(raw []byte) (ret []float64) {
	for cur := 0; cur < len(raw); cur += 3 {
		r, g, b := raw[cur], raw[cur+1], raw[cur+2]
		bri := (float64(r)*0.299 + float64(g)*0.587 + float64(b)*0.114) / 255.0
		ret = append(ret, bri)
	}
	return
}

// DrawRGB draws RGB byte-array to terminal.
// It takes width and height for recognizing the raw RGB byte-array, and
// colorful to draw blocks with colors.
func DrawRGB(raw []byte, config *Config) {
	var colorful bool = false

	// Prepare drawing settings
	width, height := config.Width, config.Height
	rgb2Xterm := rgb2XtermYIQ
	if config.Colorful {
		colorful = true
		switch config.DistanceAlgorithm {
		case "rgb":
			rgb2Xterm = rgb2XtermRGB
		case "cie94":
			rgb2Xterm = rgb2XtermCIE94
		default:
			rgb2Xterm = rgb2XtermYIQ
		}
	}

	var color1, color2, brightness1, brightness2 uint8
	if colorful {
		// Draw image with color
		pixels := rawRGB2Pixels(raw)
		for y := 0; y < height; y += 2 {
			for x := 0; x < width; x++ {
				// Compute the color of upper block
				r1 := pixels[y*width+x][0]
				g1 := pixels[y*width+x][1]
				b1 := pixels[y*width+x][2]
				color1 = rgb2Xterm(r1, g1, b1)

				// Compute the color of lower block
				if (y + 1) < height {
					r2 := pixels[(y+1)*width+x][0]
					g2 := pixels[(y+1)*width+x][1]
					b2 := pixels[(y+1)*width+x][2]
					color2 = rgb2Xterm(r2, g2, b2)
				} else {
					color2 = colorTransparent
				}

				// Draw one pixel (if needed)
				if color1 != fCache[x][y/2][0] || color2 != fCache[x][y/2][1] {
					dot(x, y/2, color1, color2)
					fCache[x][y/2][0], fCache[x][y/2][1] = color1, color2
				}
			}
			if (y + 2) < height {
				fmt.Println(sequence)
				sequence = ""
			}
		}
	} else {
		// Draw image in grayscale
		pixels := rawRGB2BrightnessPixels(raw)
		for y := 0; y < height; y += 2 {
			for x := 0; x < width; x++ {
				brightness1 = uint8(pixels[y*width+x]*23) + 232
				if (y + 1) < height {
					brightness2 = uint8(pixels[(y+1)*width+x]*23) + 232
				} else {
					brightness2 = colorTransparent
				}
				// Draw one pixel (if needed)
				if brightness1 != fCache[x][y/2][0] || brightness2 != fCache[x][y/2][1] {
					dot(x, y/2, brightness1, brightness2)
					fCache[x][y/2][0], fCache[x][y/2][1] = brightness1, brightness2
				}
			}
			if (y + 2) < height {
				fmt.Println(sequence)
				sequence = ""
			}
		}
	}
}

func dot(x, y int, color1, color2 uint8) {
	// Move cursor
	sequence += "\033[" + strconv.Itoa(y+1) + ";" + strconv.Itoa(x+1) + "H"

	fg, bg := oldfg, oldbg

	// The lower half block "▄"
	var str = "\xe2\x96\x84"

	if color1 == color2 {
		bg = color1
		str = " "
	} else if color2 == colorTransparent {
		// The upper half block "▀"
		str = "\xe2\x96\x80"
		bg, fg = color2, color1
	} else {
		bg, fg = color1, color2
	}

	if bg != oldbg {
		if bg == colorTransparent {
			sequence += "\033[49m"
		} else {
			sequence += "\033[48;5;" + strconv.Itoa(int(bg)) + "m"
		}
	}

	if fg != oldfg {
		if fg == colorUndef {
			sequence += "\033[39m"
		} else {
			sequence += "\033[38;5;" + strconv.Itoa(int(fg)) + "m"
		}
	}

	oldbg, oldfg = bg, fg

	sequence += str
}

func floatMod(x, y float64) float64 {
	return x - y*math.Floor(x/y)
}

func floatMin(x, y float64) float64 {
	if x-y > 0 {
		return y
	}
	return x
}

// AsciiDrawRGB draws a RGB byte-array to terminal.
// It uses only full block characters and without colors and framecache.
func AsciiDrawRGB(raw []byte, config *Config) {
	var chr string
	width, height := config.Width, config.Height
	pixels := rawRGB2BrightnessPixels(raw)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			brightness := pixels[y*width+x]
			bg := brightness*23 + 232
			fg := floatMin(255, bg+1)
			mod := floatMod(bg, 1.0)

			switch {
			case mod < 0.2:
				chr = " "
			case mod < 0.4:
				chr = "░"
			case mod < 0.6:
				chr = "▒"
			case mod < 0.8:
				bg, fg = fg, bg
				chr = "▒"
			default:
				bg, fg = fg, bg
				chr = "░"
			}

			fmt.Printf("\033[48;5;%dm\033[38;5;%dm%s", int(bg), int(fg), chr)
		}
		if (y + 1) < height {
			fmt.Print("\n")
		}
	}
}
