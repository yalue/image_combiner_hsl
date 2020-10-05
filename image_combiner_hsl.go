// The image_combiner_hsl program takes three input images and maps the images,
// respectively, to the hue, saturation, and luminosity components of the HSL
// color representation. If the input images are not grayscale, they will be
// converted to grayscale.
package main

import (
	"flag"
	"fmt"
	_ "github.com/spakin/netpbm"
	_ "golang.org/x/image/bmp"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"os"
)

// Implements the color interface. The output image will be composed of this.
// Implements the color interface. All three components are expected to be in
// the range [0.0, 1.0].
type HSLColor struct {
	H float32
	S float32
	L float32
}

func (c HSLColor) String() string {
	return fmt.Sprintf("(%f, %f, %f)", c.H, c.S, c.L)
}

// Converts a given arbitrary RGB color to a single brightness value.
func convertToBrightness(c color.Color) float32 {
	r, g, b, _ := c.RGBA()
	return float32(r+g+b) / (3.0 * 65535.0)
}

// This function sets the H value of the given HSLColor, returning the modified
// HSLColor. It's written this way, rather than as a method of HSLColor, so
// that the function can be passed to HSLImage's setComponent method.
func setH(c HSLColor, value color.Color) HSLColor {
	c.H = convertToBrightness(value)
	return c
}

// See setH
func setS(c HSLColor, value color.Color) HSLColor {
	c.S = convertToBrightness(value)
	return c
}

// See setH
func setL(c HSLColor, value color.Color) HSLColor {
	c.L = convertToBrightness(value)
	return c
}

func clamp(v float64) float64 {
	if v <= 0.0 {
		return 0.0
	}
	if v >= 1.0 {
		return 1.0
	}
	return v
}

// Returns R, G, B, given a particular hue value.
func hueToRGB(h float64) (float64, float64, float64) {
	r := math.Abs((h*6.0)-3.0) - 1.0
	g := 2.0 - math.Abs((h*6.0)-2.0)
	b := 2.0 - math.Abs((h*6.0)-4.0)
	return clamp(r), clamp(g), clamp(b)
}

// Linearly maps a floating point value in [0, 1] to [0, 0xffff].
func scaleTo16Bit(v float64) uint32 {
	tmp := uint32(v * float64(0xffff))
	if tmp > 0xffff {
		tmp = 0xffff
	}
	return tmp
}

// I based this code off of the snippet here:
// https://gist.github.com/mathebox/e0805f72e7db3269ec22
func (c HSLColor) RGBA() (r, g, b, a uint32) {
	r1, g1, b1 := hueToRGB(float64(c.H))
	chroma := (1.0 - math.Abs(2.0*float64(c.L)-1)) * float64(c.S)
	l := float64(c.L)
	r1 = (r1-0.5)*chroma + l
	g1 = (g1-0.5)*chroma + l
	b1 = (b1-0.5)*chroma + l
	r = scaleTo16Bit(r1)
	g = scaleTo16Bit(g1)
	b = scaleTo16Bit(b1)
	a = 0xffff
	return
}

type HSLImage struct {
	pixels []HSLColor
	w, h   int
}

func (h *HSLImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, h.w, h.h)
}

func (h *HSLImage) ColorModel() color.Model {
	return color.RGBA64Model
}

// Returns the index into h's pixel array corresponding to the pixel at x, y,
// or 0 if x, y is outside of the image's bounds.
func (h *HSLImage) safeIndex(x, y int) int {
	if (x < 0) || (y < 0) || (x >= h.w) || (y >= h.h) {
		return 0
	}
	return y*h.w + x
}

func (h *HSLImage) At(x, y int) color.Color {
	return h.pixels[h.safeIndex(x, y)]
}

// Takes another image and sets a component of each of this image's pixels
// based on the brightness of each pixel in pic. The provided "pic" can not be
// larger than h in either dimension. Requires a "setter" function, which will
// be called to obtain the updated value of each HSL pixel.
func (h *HSLImage) SetComponent(pic image.Image,
	setter func(HSLColor, color.Color) HSLColor) {
	bounds := pic.Bounds().Canon()
	localX := 0
	localY := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		localX = 0
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			i := h.safeIndex(localX, localY)
			// Here is where we call the provided "setter" function to update
			// the components of h.pixels[i]. In practice, setter will always
			// be setH, setS, or setL.
			h.pixels[i] = setter(h.pixels[i], pic.At(x, y))
			localX++
		}
		localY++
	}
}

// "Rotates" the hue value of each pixel in the image forward by the given
// amount.
func (h *HSLImage) AdjustHue(adjustment float64) {
	for i := range h.pixels {
		oldHue := float64(h.pixels[i].H)
		_, newHue := math.Modf(oldHue + adjustment)
		h.pixels[i].H = float32(newHue)
	}
}

func newHSLImage(w, h int) (*HSLImage, error) {
	if (w <= 0) || (h <= 0) {
		return nil, fmt.Errorf("Image bounds must be positive")
	}
	return &HSLImage{
		w:      w,
		h:      h,
		pixels: make([]HSLColor, w*h),
	}, nil
}

// Takes 3 image filenames and returns the maximum dimensions of all of them.
func getMaxDimensions(imageFiles []string) (int, int, error) {
	var maxW, maxH, w, h int
	var pic image.Image
	var e error
	var f *os.File
	for _, filename := range imageFiles {
		fmt.Printf("Getting dimensions for %s...\n", filename)
		f, e = os.Open(filename)
		if e != nil {
			return 0, 0, fmt.Errorf("Failed opening %s: %s", filename, e)
		}
		pic, _, e = image.Decode(f)
		if e != nil {
			f.Close()
			return 0, 0, fmt.Errorf("Failed decoding %s: %s", filename, e)
		}
		bounds := pic.Bounds().Canon()
		w = bounds.Dx()
		h = bounds.Dy()
		pic = nil
		f.Close()
		if w > maxW {
			maxW = w
		}
		if h > maxH {
			maxH = h
		}
	}
	return maxW, maxH, nil
}

func combineImages(imageFiles []string, adjustHue float64) (image.Image,
	error) {
	var pic image.Image
	var f *os.File
	if len(imageFiles) != 3 {
		return nil, fmt.Errorf("Need exactly 3 image files, got %d",
			len(imageFiles))
	}
	if (adjustHue < 0) || (adjustHue > 1) {
		return nil, fmt.Errorf("Hue adjustment values must be in [0, 1]")
	}
	w, h, e := getMaxDimensions(imageFiles)
	if e != nil {
		return nil, fmt.Errorf("Failed getting image dimensions: %s", e)
	}
	fmt.Printf("Combining images into a %dx%d image.\n", w, h)
	combined, e := newHSLImage(w, h)
	if e != nil {
		return nil, fmt.Errorf("Failed creating new image: %s", e)
	}

	// We'll index into this array to get the setter function for each
	// component of the HSL color.
	setterFunctions := []func(HSLColor, color.Color) HSLColor{
		setH,
		setS,
		setL,
	}
	componentNames := []string{"hue", "saturation", "luminosity"}

	for i, filename := range imageFiles {
		fmt.Printf("Setting %s using %s...\n", componentNames[i], filename)
		f, e = os.Open(filename)
		if e != nil {
			return nil, fmt.Errorf("Failed opening file %s: %s", filename, e)
		}
		pic, _, e = image.Decode(f)
		if e != nil {
			f.Close()
			return nil, fmt.Errorf("Failed decoding image %s: %s", filename, e)
		}
		combined.SetComponent(pic, setterFunctions[i])
		pic = nil
		f.Close()
	}
	if adjustHue != 0 {
		fmt.Printf("Adjusting hue...\n")
		combined.AdjustHue(adjustHue)
	}
	return combined, nil
}

func printUsage() {
	fmt.Printf("Usage: %s <path for H image> <path for S image> "+
		"<path for L image> <output filename.jpg>\n", os.Args[0])
}

func run() int {
	var hName, sName, lName, oName string
	var adjustHue float64
	flag.StringVar(&hName, "H", "", "The path to the image file to map to "+
		"the hue component. Required.")
	flag.StringVar(&sName, "S", "", "The path to the image file to map to "+
		"the saturation component. Required.")
	flag.StringVar(&lName, "L", "", "The path to the image file to map to "+
		"the luminosity component. Required.")
	flag.StringVar(&oName, "o", "", "The name of the .jpg file to create. "+
		"Required.")
	flag.Float64Var(&adjustHue, "adjust_hue", 0.0, "An amount, in the range "+
		"[0, 1], by which to \"rotate\" hue values.")
	flag.Parse()
	if (hName == "") || (sName == "") || (lName == "") || (oName == "") {
		fmt.Printf("Missing one or more required arguments.\n")
		fmt.Printf("Run with -help for usage information.\n")
		return 1
	}
	outputImage, e := combineImages([]string{hName, sName, lName}, adjustHue)
	if e != nil {
		fmt.Printf("Error combining images: %s\n", e)
		return 1
	}
	outputFile, e := os.Create(oName)
	if e != nil {
		fmt.Printf("Error opening output file: %s\n", e)
		return 1
	}
	defer outputFile.Close()
	options := jpeg.Options{
		Quality: 100,
	}
	e = jpeg.Encode(outputFile, outputImage, &options)
	if e != nil {
		fmt.Printf("Failed creating output JPEG image: %s\n", e)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run())
}
