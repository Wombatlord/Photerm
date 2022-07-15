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

// ScaleImg does global scale and makes boyz wide
func (fc *FrameCache) ScaleImg(sf ScaleFactors) image.Image {
	w, h := OutputDimsOf(sf, fc.image)
	return resize.Resize(w, h, fc.image, resize.Lanczos2)
}

// LoadImageFiles takes a path to a directory of jpgs/pngs and loads them
// into the FrameCache.imageFiles array for enumeration.
func (fc *FrameCache) LoadImageFiles(path PathSpec) []os.DirEntry {
	fc.imageFiles, fc.frameErr = os.ReadDir(path.GetPath())
	if fc.frameErr != nil {
		log.Fatalf("LOAD ERR: %s", fc.frameErr)
	}
	return fc.imageFiles
}

// LoadImageFile takes a path to a single jpg/png and loads it
// into the FrameCache.imageFile field for decoding.
func (fc *FrameCache) LoadImageFile(f os.DirEntry, path PathSpec) *os.File {
	fc.imageFile, fc.frameErr = os.Open(fmt.Sprintf("%s%s", path.GetPath(), f.Name()))
	if fc.frameErr != nil {
		log.Fatal(fc.frameErr)
	}
	return fc.imageFile
}

// DecodeFrame pulls a pointer to a jpg/png from the imageFiles array.
// the *os.File is then decoded into an image.Image for processing and render.
func (fc *FrameCache) DecodeFrame(frame *os.File) {
	fc.image, fc.imageFormat, fc.frameErr = image.Decode(frame)
	if fc.frameErr != nil {
		log.Fatalf("DECODE ERR: %s", fc.frameErr)
	}
	_ = frame.Close()
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
			results <- fc.ScaleImg(sf)
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

// Buffer a single image for non-sequential display
// from a full provided path.
func (fc *FrameCache) BufferImagePath(imageBuffer chan image.Image, path PathSpec, sf ScaleFactors) (err error) {
	imgFile, err := os.Open(path.GetPath())

	if err != nil {
		log.Fatalf("LOAD ERR: %s", err)
	}

	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)

	if err != nil {
		log.Fatalf("DECODE ERR: %s", err)
	}
	// Scale image, then read image.Image into channel.
	img = fc.ScaleImg(sf)
	imageBuffer <- img
	close(imageBuffer)
	return nil
}