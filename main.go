package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"

	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"wombatlord/imagestuff/src/rotato"
	"wombatlord/imagestuff/src/util"

	"github.com/alexflint/go-arg"
	"github.com/nfnt/resize"
)

type Painter string

const (
	Foreground Painter = "\u001b[38;"
	Background         = "\u001b[48;"
	Normalizer         = "\u001b[0m"
)

// RGB paints the string with a true color rgb painter
func RGB(r, g, b byte, p Painter) string {
	return fmt.Sprintf("%s2;%d;%d;%dm", p, r, g, b)
}

type Charset int

const (
	Normal Charset = iota
	TurboGFX
	ASCIIFY
)

// Charsets is the mapping of glyph to brightness levels
// indexed by Charset Arg.
var Charsets = [3][5]string{
	Normal:   {"█", "█", "█", "█", "█"},
	TurboGFX: {" ", "░", "▒", "▓", "█"},
	ASCIIFY:  {" ", ".", "*", "$", "@"},
}

var Ramps = []string{
	Normal: "$@B%8&WM#*oahkbdpqwmZO0QLCJUYXzcvunxrjft/\\|()1{}[]?-_+~<>i!lI;:,^`'. ",
}

const NotSet = 0

type Cli struct {
	Path     string  `arg:"positional" help:"file path for an image" default:"liljeffrey.jpg"`
	Scale    float64 `arg:"-s, --scale" help:"overall image scale" default:"1.0"`
	Squash   float64 `arg:"-w, --wide-boyz" help:"How wide you want it guv? (Widens the image)" default:"1.0"`
	StdInput bool    `arg:"-i, --in" help:"read from stdin"`
	Mode     string  `arg:"-m, --mode" help:"mode selection determines renderer" default:"A"`
	Charset  Charset `arg:"-p, --palette" help:"palette selection determines the character set used by the renderer" default:"0"`
	YOrigin  int     `arg:"--y-org" help:"minimum Y, top of focus" default:"0"`
	Height   int     `arg:"--height" help:"height, vertical size of focus" default:"0"`
	XOrigin  int     `arg:"--x-org" help:"minimum X, left edge of focus" default:"0"`
	Width    int     `arg:"--width" help:"width, width of focus" default:"0"`
	HueAngle float32 `arg:"--hue" help:"hue rotation angle in radians" default:"0.0"`
}

func (c Cli) GetPath() string    { return c.Path }
func (c Cli) GetStdIn() bool     { return c.StdInput }
func (c Cli) GetScale() float64  { return c.Scale }
func (c Cli) GetSquash() float64 { return c.Squash }
func (c Cli) GetYOrigin() int    { return c.YOrigin }
func (c Cli) GetHeight() int     { return c.Height }
func (c Cli) GetXOrigin() int    { return c.XOrigin }
func (c Cli) GetWidth() int      { return c.Width }

func (c *Cli) GetFocusView(img image.Image) FocusView {
	// set defaults as dynamic image size
	if c.Height == NotSet {
		c.Height = img.Bounds().Max.Y
	}
	if c.Width == NotSet {
		c.Width = img.Bounds().Max.X
	}
	// This is width precedence, preserve width, sacrifice view panning
	c.Width = util.Min(c.Width, img.Bounds().Max.X)
	c.XOrigin = util.Min(c.XOrigin, img.Bounds().Max.X-c.Width)

	c.Height = util.Min(c.Height, img.Bounds().Max.Y)
	c.YOrigin = util.Min(c.YOrigin, img.Bounds().Max.Y-c.Height)

	return c
}

var Args Cli

type FocusView interface {
	GetYOrigin() int
	GetHeight() int
	GetXOrigin() int
	GetWidth() int
}

type ScaleFactors interface {
	GetScale() float64
	GetSquash() float64
}

type PathSpec interface {
	GetPath() string
	GetStdIn() bool
}

var (
	imgFile *os.File
)

// ArgsToJson serialises the passed CLI args.
// The output file will take its name from Args.Path
func ArgsToJson(c Cli) {
	file, _ := json.MarshalIndent(c, "", " ")
	fileName := fmt.Sprintf("%s.json", strings.Split(c.Path, ".")[0])

	_ = ioutil.WriteFile(fileName, file, 0644)
}

// loadImage first checks if an image is being passed via standard in.
// For example, curling an image and passing it through a pipe.
// If no file is passed this way, then os.Open will load the image
// indicated by Args.Path.
func loadImage(ps PathSpec) (img image.Image, err error) {
	if ps.GetStdIn() {
		imgFile = os.Stdin
	} else {
		imgFile, err = os.Open(ps.GetPath())
		if err != nil {
			return nil, err
		}
	}
	defer imgFile.Close()

	img, _, err = image.Decode(imgFile)
	if err != nil {
		return nil, err
	}

	return img, nil
}

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

func grayscale(c color.Color) int {
	r, g, b, _ := c.RGBA()
	return int(0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b))
}

func avgPixel(img image.Image, x, y, w, h int) int {
	cnt, sum, max := 0, 0, img.Bounds().Max
	for i := x; i < x+w && i < max.X; i++ {
		for j := y; j < y+h && j < max.Y; j++ {
			sum += grayscale(img.At(i, j))
			cnt++
		}
	}
	return sum / cnt
}

// PrintImg is the stdout of the program, i.e. Sick GFX.
func PrintImg(charset Charset, focusView FocusView, img image.Image) {
	glyphs := Charsets[charset]

	// img relative x, y pixel lower bounds
	top, left := focusView.GetYOrigin(), focusView.GetXOrigin()

	// img relative x, y pixel upper bounds (right, top)
	right := focusView.GetXOrigin() + focusView.GetWidth()
	btm := focusView.GetYOrigin() + focusView.GetHeight()

	// go row by row in the scaled image.Image and...
	for y := top; y < btm; y++ {

		// print cells from left to right
		for x := left; x < right; x++ {

			// get brightness of cell
			c := color.GrayModel.Convert(img.At(x, y)).(color.Gray)

			level := c.Y / 51 // 51 * 5 = 255
			if level == 5 {   // clipping
				level--
			}

			// get the rgba colour
			rgb := rotato.RotateHue(color.RGBAModel.Convert(img.At(x, y)).(color.RGBA), Args.HueAngle)

			// get the colour and glyph corresponding to the brightness
			ink := RGB(rgb.R, rgb.G, rgb.B, Foreground)
			glyph := glyphs[level]

			// and print the cell
			fmt.Print(ink + glyph)
		}

		fmt.Println(Normalizer)
	}
}

func tdPalette(img image.Image, focusView FocusView, sf ScaleFactors) {
	// pal := make([]byte, 256)

	// // setup a palette mapping
	// // palette: ABCDEFGHIJKLMNOP
	// var chunkIdx byte = 0
	// for len(pal) < 256 {
	// 	//chunk := bytes.Repeat([]byte{0x41+chunkIdx}, 16)
	// 	for i := 0; i < 16; i++ {
	// 		chunkOffset := int(chunkIdx)*16

	// 		idx := chunkOffset + i
	// 		if idx > 255 {
	// 			log.Fatalln("too big: ", idx)
	// 		}
	// 		pal[idx] = 0x41+chunkIdx
	// 	}
	// 	chunkIdx++
	// }

	// fmt.Printf("palette length: %d\n", len(pal))
	// if len(pal) > 256 {
	// 	log.Fatalf("Palette was too long!\n")
	// }
	// fmt.Println(pal)
}

func PaletteTesting(charset Charset, focusView FocusView, img image.Image) {
	glyphs := Ramps[charset]

	// img relative x, y pixel lower bounds
	top, left := focusView.GetYOrigin(), focusView.GetXOrigin()

	// img relative x, y pixel upper bounds (right, top)
	right := focusView.GetXOrigin() + focusView.GetWidth()
	btm := focusView.GetYOrigin() + focusView.GetHeight()

	scaleY := 1

	// go row by row in the scaled image.Image and...
	for y := top; y < btm; y += int(Args.Squash) {

		// print cells from left to right
		for x := left; x < right; x += scaleY {
			// get brightness of cell
			c := avgPixel(img, x, y, int(Args.Squash), scaleY)

			// get the rgba colour
			rgb := rotato.RotateHue(color.RGBAModel.Convert(img.At(x, y)).(color.RGBA), Args.HueAngle)

			// get the colour and glyph corresponding to the brightness
			ink := RGB(rgb.R, rgb.G, rgb.B, Foreground)

			fmt.Print(ink + string(glyphs[len(glyphs)*c/65536]))
		}
		fmt.Println()
	}
}

func main() {
	arg.MustParse(&Args)
	ArgsToJson(Args)

	img, err := loadImage(Args)
	if err != nil {
		log.Fatal(err)
	}

	img = ScaleImg(img, Args)
	focusView := Args.GetFocusView(img)

	switch {
	case Args.Mode == "A":
		PrintImg(Args.Charset, focusView, img)

	case Args.Mode == "B":
		PaletteTesting(Args.Charset, focusView, img)
	}
}
