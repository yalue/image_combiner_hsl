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

// Implements the color interface. Stores the H, S, and L components,
// respectively. *This will panic if the slice doesn't contain at least 3
// components.* Values after the first 3 are ignored. Each component is a
// fraction out of 0xffff.
type HSLColor []uint16

// Utility function to convert the 3 16-bit values to fractional components.
func (c HSLColor) HSLComponents() (float64, float64, float64) {
	h := float64(c[0]) / float64(0xffff)
	s := float64(c[1]) / float64(0xffff)
	l := float64(c[2]) / float64(0xffff)
	return h, s, l
}

func (c HSLColor) String() string {
	h, s, l := c.HSLComponents()
	return fmt.Sprintf("(%f, %f, %f)", h, s, l)
}

// Converts a given arbitrary RGB color to a single brightness value.
func convertToBrightness(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return float64(r+g+b) / (3.0 * 65535.0)
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

// Linearly maps a floating point value in [0, 1] to [0, 0xffff]. Clamps v to
// be in the range [0, 1].
func scaleTo16Bit(v float64) uint16 {
	return uint16(clamp(v) * float64(0xffff))
}

// Returns R, G, B, given a particular hue value.
func hueToRGB(h float64) (float64, float64, float64) {
	r := math.Abs((h*6.0)-3.0) - 1.0
	g := 2.0 - math.Abs((h*6.0)-2.0)
	b := 2.0 - math.Abs((h*6.0)-4.0)
	return clamp(r), clamp(g), clamp(b)
}

// I based this code off of the snippet here:
// https://gist.github.com/mathebox/e0805f72e7db3269ec22
func (c HSLColor) RGBA() (r, g, b, a uint32) {
	h, s, l := c.HSLComponents()
	r1, g1, b1 := hueToRGB(h)
	chroma := (1.0 - math.Abs(2.0*l-1)) * s
	r1 = (r1-0.5)*chroma + l
	g1 = (g1-0.5)*chroma + l
	b1 = (b1-0.5)*chroma + l
	r = uint32(scaleTo16Bit(r1))
	g = uint32(scaleTo16Bit(g1))
	b = uint32(scaleTo16Bit(b1))
	a = 0xffff
	return
}

// Implements the image interface. Internally uses HSL representation for each
// pixel.
type HSLImage struct {
	// We'll keep the HSL pixel data in a single slice to avoid any possible
	// padding if we use a slice of color structs instead. (This is why
	// HSLColor is a slice, rather than a struct.)
	pixels []uint16
	w, h   int
}

func (h *HSLImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, h.w, h.h)
}

func (h *HSLImage) ColorModel() color.Model {
	return color.RGBA64Model
}

// Returns the HSLColor corresponding to the pixel at (x, y), or a separate,
// black, HSLColor if the coordinate is outside of the image boundaries.
func (h *HSLImage) HSLPixel(x, y int) HSLColor {
	if (x < 0) || (y < 0) || (x >= h.w) || (y >= h.h) {
		return HSLColor([]uint16{0, 0, 0})
	}
	i := 3 * (y*h.w + x)
	return HSLColor(h.pixels[i : i+3])
}

func (h *HSLImage) At(x, y int) color.Color {
	return h.HSLPixel(x, y)
}

// Takes another image and sets a component of each of this image's pixels
// based on the brightness of each pixel in pic. The "componentOffset" must be
// 0 if setting hue, 1 if setting saturation, and 2 if setting luminosity.
func (h *HSLImage) SetComponent(pic image.Image, componentOffset int) error {
	if (componentOffset < 0) || (componentOffset > 2) {
		return fmt.Errorf("Invalid component offset: %d", componentOffset)
	}
	bounds := pic.Bounds().Canon()
	localX := 0
	localY := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		localX = 0
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			hslPixel := h.HSLPixel(localX, localY)
			// Convert the new component from the grayscale brightness of the
			// pixel in the source pic.
			newValue := scaleTo16Bit(convertToBrightness(pic.At(x, y)))
			hslPixel[componentOffset] = newValue
			localX++
		}
		localY++
	}
	return nil
}

// "Rotates" the hue value of each pixel in the image forward by the given
// amount.
func (h *HSLImage) AdjustHue(adjustment float64) {
	for y := 0; y < h.h; y++ {
		for x := 0; x < h.w; x++ {
			hslPixel := h.HSLPixel(x, y)
			// We'll just let this wrap around to take care of the rotation.
			hslPixel[0] += scaleTo16Bit(adjustment)
		}
	}
}

func newHSLImage(w, h int) (*HSLImage, error) {
	if (w <= 0) || (h <= 0) {
		return nil, fmt.Errorf("Image bounds must be positive")
	}
	return &HSLImage{
		w:      w,
		h:      h,
		pixels: make([]uint16, 3*w*h),
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

	componentNames := []string{"hue", "saturation", "luminosity"}
	for i, filename := range imageFiles {
		componentName := componentNames[i]
		fmt.Printf("Setting %s using %s...\n", componentName, filename)
		f, e = os.Open(filename)
		if e != nil {
			return nil, fmt.Errorf("Failed opening file %s: %s", filename, e)
		}
		pic, _, e = image.Decode(f)
		if e != nil {
			f.Close()
			return nil, fmt.Errorf("Failed decoding image %s: %s", filename, e)
		}
		e = combined.SetComponent(pic, i)
		if e != nil {
			f.Close()
			return nil, fmt.Errorf("Failed setting %s: %s", componentName, e)
		}
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
	fmt.Printf("Writing combined image to %s\n", oName)
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
