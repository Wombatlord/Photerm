package photerm

import (
	"image"
	"log"
	"os/exec"
	"strings"

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

// Proxy to ffmpeg CLI for generating sequential jpgs from an mp4.
func Mp4ToFrames(p PathSpec) {
	// Split path into constituent strings
	dest := strings.Split(p.GetPath(), "/")
	// re-join all but the final element to construct the path minus the target mp4
	destDir := strings.Join(dest[0:len(dest)-1], "/")

	// construct the ffmpeg command & run it to convert mp4 to indvidual images saved in destDir
	// images are named in ascending order, starting at 00000.jpg
	c := exec.Command("ffmpeg", "-i", p.GetPath(), "-vf", "fps=24", destDir+"/%05d.jpg")
	err := c.Run()
	// catch any errors from the ffmpeg call.
	if err != nil {
		log.Fatal(err)
	}
}