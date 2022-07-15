package photerm

import (
	"fmt"
	"image"
	"log"
	"os"
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

// Just experimenting and exploring abstraction.
// Encapsulates functionality related to loading & processing frames
// Holds individual frames in the image field.
type FrameCache struct {
	imageFiles  []os.DirEntry
	imageFile   *os.File
	image       image.Image
	imageFormat string
	frameErr    error
}

func (fc *FrameCache) LoadImageFiles(path PathSpec) []os.DirEntry {
	fc.imageFiles, fc.frameErr = os.ReadDir(path.GetPath())
	if fc.frameErr != nil {
		log.Fatalf("LOAD ERR: %s", fc.frameErr)
	}
	return fc.imageFiles
}

func (fc *FrameCache) DecodeFrame(frame *os.File) {
	fc.image, fc.imageFormat, fc.frameErr = image.Decode(frame)
	if fc.frameErr != nil {
		log.Fatalf("DECODE ERR: %s", fc.frameErr)
	}
	_ = frame.Close()
}

func (fc *FrameCache) LoadImageFile(f os.DirEntry, path PathSpec) *os.File {
	fc.imageFile, fc.frameErr = os.Open(fmt.Sprintf("%s%s", path.GetPath(), f.Name()))
	if fc.frameErr != nil {
		log.Fatal(fc.frameErr)
	}
	return fc.imageFile
}

func (fc *FrameCache) GetImage() image.Image {
	return fc.image
}

// Returns the extension of a file, eg. .json, .jpg, .mp4
func (fc *FrameCache) checkExtension(f os.DirEntry) string {
	return strings.Split(f.Name(), ".")[1]
}

// BufferImageDir runs asynchronously to load files into memory
// Each file is sent into imageBuffer to be consumed elsewhere.
// This is an example of a generator pattern in golang.
func (fc *FrameCache) BufferImageDir(path PathSpec, sf ScaleFactors) <-chan image.Image {

	// Define the asynchronous work that will process data and populate the generator
	// Anonymous func takes the write side of a channel
	work := func(results chan<- image.Image) {
		for _, file := range fc.imageFiles {

			// Ignore serialised args file / source mp4 and proceed with iteration
			if fc.checkExtension(file) != "jpg" {
				continue
			}

			// Load & Decode the .jpg file into an image.Image
			imgFile := fc.LoadImageFile(file, path)
			fc.DecodeFrame(imgFile)

			// Scale image, then read image.Image into channel.
			results <- ScaleImg(fc.image, sf)
		}
		// Close the channel once all files have been read into it.
		close(results)
	}

	// blocking code
	imageBuffer := make(chan image.Image, len(fc.imageFiles))
	// non-blocking
	go work(imageBuffer)

	// Return the read side of the channel
	return imageBuffer
}
