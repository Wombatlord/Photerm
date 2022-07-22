package photerm

import (
	"image"

	"github.com/nfnt/resize"
)

// These are interfaces to the CLI struct
// for interacting with its fields.
type FocusView interface {
	GetYOrigin() int
	GetHeight() int
	GetXOrigin() int
	GetWidth() int
	GetRegion() Region
}

type ScaleFactors interface {
	GetScale() float64
	GetSquash() float64
}

type PathSpec interface {
	GetPath() string
	GetStdIn() bool
}

// Region fields define the area of an image that will be rendered.
// Essentially crops the image post scaling if values are non-default.
type Region struct{ Left, Top, Right, Btm int }

// OutputBoundsOf consumes a Cli value and returns pixel width, height tuple
func OutputDimsOf(scales ScaleFactors, img image.Image) (w, h uint) {
	height := float64(uint(img.Bounds().Max.Y))
	width := float64(uint(img.Bounds().Max.X))

	scale := scales.GetScale()
	ratio := width / height * scales.GetSquash()

	return uint(scale * height * ratio), uint(scale * height)
}

// ScaleImg does global scale and makes boyz wide
func ScaleImg(img image.Image, sf ScaleFactors) image.Image {
	w, h := OutputDimsOf(sf, img)
	return resize.Resize(w, h, img, resize.Lanczos2)
}

// ScaleTransform is ScaleImg wrapped as a pipeline step, i.e.
// an async generator that has an input and an output.
func ScaleTransform(
	out chan<- image.Image,
	in <-chan image.Image,
	sf ScaleFactors,
) {
	defer close(out)
	for img := range in {
		out <- ScaleImg(img, sf)
	}
}

// AppendScalingStep attaches the scaling pipeline step to the image buffer
func AppendScalingStep(in <-chan image.Image, sf ScaleFactors) <-chan image.Image {
	out := make(chan image.Image)
	go ScaleTransform(out, in, sf)

	return out
}
