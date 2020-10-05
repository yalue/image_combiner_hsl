Image Combiner HSL
==================

This program is similar to my other
[image combiner project](https://github.com/yalue/image_combiner), in that it
merges several input images into a single output image. However, while
`image_combiner` allows mapping an arbitrary number of images to colors in an
output image, `image_combiner_hsl` requires exactly three input images, which
are mapped to the hue, saturation, and luminosity components in the HSL color
scheme.

Like the project upon which it was based, this project also supports merging
images of multiple different input formats, but always produces a .jpg output.
It is intended to support higher-resolution images (it will only load up to one
input image at a time), and uses 16-bit color internally.

Compilation
-----------

Ensure that you have `go` installed. Navigate to the directory containing this
README and run `go build`.

Usage
-----

The program requires several command-line options: the three images mapping to
the hue, saturation, and luminosity components, along with the name of the
output image. Additionally, hue can be "rotated" by an amount between 0 and
1.0.  See the following example, or run with `-help` to see all of the options:

```bash
./image_combiner_hsl \
    -H examples/gradient_diagonal.png \
    -S examples/gradient_horizontal.jpg \
    -L examples/hello_world.gif \
    -o hsl_hello_world.jpg
```

The resolution of the output image will match that of the largest input image.
If input images differ in resolution, the top-left corner of each image will be
aligned to the top-left corner of the output image. (Input images will not be
resized by this program. There are many ways to make them match, and it's
easier to leave the resizing process as a responsibility of the user.)

