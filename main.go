package main

import (
	"encoding/json"
	"fmt"
	"image"
	"wombatlord/imagestuff/src/photerm"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"wombatlord/imagestuff/src/util"

	"github.com/alexflint/go-arg"
)

type Charset int

const (
	Normal Charset = iota
	TurboGFX
)

// Charsets is the mapping of glyph to brightness levels
// indexed by Charset Arg.
var Charsets = [3][5]string{
	Normal:   {"█", "█", "█", "█", "█"},
	TurboGFX: {" ", "░", "▒", "▓", "█"},
}

var Ramps = []string{
	Normal: "$@B%8&WM#*oahkbdpqwmZO0QLCJUYXzcvunxrjft/\\|()1{}[]?-_+~<>i!lI;:,^`'. ",
}

const NotSet = 0

type Cli struct {
	Path          string          `arg:"positional" help:"file path for an image" default:"liljeffrey.jpg"`
	Scale         float64         `arg:"-s, --scale" help:"overall image scale" default:"1.0"`
	Squash        float64         `arg:"-w, --wide-boyz" help:"How wide you want it guv? (Widens the image)" default:"1.0"`
	StdInput      bool            `arg:"-i, --in" help:"read from stdin"`
	Mode          string          `arg:"-m, --mode" help:"mode selection determines renderer" default:"A"`
	Charset       Charset         `arg:"-c, --Charset" help:"Charset selection determines the character set used by the renderer" default:"0"`
	YOrigin       int             `arg:"--y-org" help:"minimum Y, top of focus" default:"0"`
	Height        int             `arg:"--height" help:"height, vertical size of focus" default:"0"`
	XOrigin       int             `arg:"--x-org" help:"minimum X, left edge of focus" default:"0"`
	Width         int             `arg:"--width" help:"width, width of focus" default:"0"`
	HueAngle      float32         `arg:"--hue" help:"hue rotation angle in radians" default:"0.0"`
	Bold          bool            `arg:"--bold" help:"Render the glyphs in bold" default:"true"`
	CustomPalette string          `arg:"--custom" help:"Supply a string of characters to be used to indicate brightness" default:"#"`
	BG            photerm.RGBSpec `arg:"--bg" help:"Set the background RGB as "`
}

func (c *Cli) GetPath() string          { return c.Path }
func (c *Cli) GetStdIn() bool           { return c.StdInput }
func (c *Cli) GetScale() float64        { return c.Scale }
func (c *Cli) GetSquash() float64       { return c.Squash }
func (c *Cli) GetYOrigin() int          { return c.YOrigin }
func (c *Cli) GetHeight() int           { return c.Height }
func (c *Cli) GetXOrigin() int          { return c.XOrigin }
func (c *Cli) GetWidth() int            { return c.Width }
func (c *Cli) GetCustomPalette() string { return c.CustomPalette }
func (c *Cli) GetHueAngle() float32     { return c.HueAngle }
func (c *Cli) IsBold() bool             { return c.Bold }
func (c *Cli) GetBG() photerm.RGBSpec   { return c.BG }
func (c *Cli) GetRegion() photerm.Region {
	return photerm.Region{
		Left:  c.GetXOrigin(),
		Top:   c.GetYOrigin(),
		Right: c.GetXOrigin() + c.GetWidth(),
		Btm:   c.GetYOrigin() + c.GetHeight(),
	}
}

func (c *Cli) GetFocusView(img image.Image) photerm.FocusView {
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

var (
	imgFile *os.File
)

// ArgsToJson serialises the passed CLI args.
// The output file will take its name from Args.Path
func ArgsToJson(c Cli) {
	file, _ := json.MarshalIndent(c, "", " ")
	fileName := fmt.Sprintf("%s.json", strings.Split(c.Path, ".")[0])

	_ = os.WriteFile(fileName, file, 0644)
}

// loadImage first checks if an image is being passed via standard in.
// For example, curling an image and passing it through a pipe.
// If no file is passed this way, then os.Open will load the image
// indicated by Args.Path.
func loadImage(ps photerm.PathSpec) (img image.Image, err error) {
	if ps.GetStdIn() {
		imgFile = os.Stdin
	} else {
		log.Println(ps.GetPath())
		imgFile, err = os.Open(ps.GetPath())
		if err != nil {
			return nil, err
		}
	}

	img, _, err = image.Decode(imgFile)
	util.Try(imgFile.Close())
	if err != nil {
		return nil, err
	}

	return img, nil
}

// BufferImages runs asynchronously to load files into memory
// Each file is sent into imageBuffer to be consumed elsewhere.
func BufferImages(dirPath string, sf photerm.ScaleFactors) <-chan image.Image {
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		log.Fatal(err)
	}
	work := func(imageBuffer chan<- image.Image) {
		for _, fileInfo := range files {
            if !strings.HasSuffix(fileInfo.Name(), ".jpg") { continue }

			file, err := os.Open(fmt.Sprintf("./tmp/%s", fileInfo.Name()))
			if err != nil {
				log.Fatalf("LOAD ERR: %s", err)
			}

			img, _, err := image.Decode(imgFile)

			// blocking close after decode instead of keeping all files
			// open until end of goroutine
			util.Try(file.Close())

			if err != nil {
				log.Fatalf("DECODE ERR: %s", err)
			}

			// Scale image, then read image.Image into channel.
			img = photerm.ScaleImg(img, sf)
			imageBuffer <- img
		}
		// Close the channel once all files have been read into it.
		close(imageBuffer)
	}

	frames := make(chan image.Image, len(files))
	go work(frames)
	return frames
}

func main() {
	arg.MustParse(&Args)
	// ArgsToJson(Args)

	img, err := loadImage(&Args)
	if err != nil {
		log.Fatal(err)
	}

	img = photerm.ScaleImg(img, &Args)
	focusView := Args.GetFocusView(img)
	printOut := &photerm.GlyphPrint{Img: img, Cli: &Args}
	charset := "#"
	switch Args.Mode {
	case "A":
		charset = strings.Join(Charsets[Args.Charset][:], "")
	case "B":
		charset = Ramps[Normal]
	case "C":
		charset = Ramps[Normal]
		animation := photerm.Animation{GlyphPrint: printOut}
		buffer := BufferImages("./tmp", &Args)
		animation.PrintBuff(os.Stdout, buffer, focusView.GetRegion())
		return
	case "D":
		charset = Args.GetCustomPalette()
	}
	printOut.SetPalette(charset)
	printOut.PrintRegion(os.Stdout, focusView.GetRegion())
}
