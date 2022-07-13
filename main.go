package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"os/exec"
	"strings"

	"wombatlord/imagestuff/photerm"
	"wombatlord/imagestuff/src/rotato"
	"wombatlord/imagestuff/src/util"

	"github.com/alexflint/go-arg"
)

type Painter string
type ImageBuff []image.Image

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
	ASCIIFY2
)

// Charsets is the mapping of glyph to brightness levels
// indexed by Charset Arg.
var Charsets = [4][]string{
	Normal:   {"█", "█", "█", "█", "█"},
	TurboGFX: {" ", "░", "▒", "▓", "█"},
	ASCIIFY:  {" ", ".", "*", "$", "@"},
	ASCIIFY2: {"$", "@", "B", "%", "8", "&", "W", "M", "#", "*", "o", "a", "h", "k", "b", "d", "p", "q", "w", "m", "Z", "O", "0", "Q", "L", "C", "J",
		"U", "Y", "X", "z", "c", "v", "u", "n", "x", "r", "j", "f", "t", "/", "\\", "|", "(", ")", "1", "{", "}", "[", "]", "?", "-", "_", "+", "~", "<", ">",
		"i", "!", "l", "I", ";", ":", ",", "^", "`", "'", ".", " ", "\"", ","},
}

const NotSet = 0

type Cli struct {
	Path     string  `arg:"positional" help:"file path for an image" default:"liljeffrey.jpg"`
	Scale    float64 `arg:"-s, --scale" help:"overall image scale" default:"1.0"`
	Squash   float64 `arg:"-w, --wide-boyz" help:"How wide you want it guv? (Widens the image)" default:"1.0"`
	StdInput bool    `arg:"-i, --in" help:"read from stdin"`
	Mode     string  `arg:"-m, --mode" help:"mode selection determines renderer" default:"A"`
	Charset  Charset `arg:"-c, --Charset" help:"Charset selection determines the character set used by the renderer" default:"0"`
	Custom   string  `arg:"--custom" help:"provide a custom string to render with" default:"█"`
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
func (c Cli) GetRegion() photerm.Region {
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

// BufferImageDir runs asynchronously to load files into memory
// Each file is sent into imageBuffer to be consumed elsewhere.
// This is an example of a generator pattern in golang.
func BufferImageDir(fs []os.DirEntry) <-chan image.Image {

	// Define the asynchronous work that will process data and populate the generator
	// Anonymous func takes the write side of a channel
	work := func(results chan<- image.Image) {
		for _, file := range fs {

			// Ignore serialised args file and proceed with iteration
			ext := strings.Split(file.Name(), ".")[1]

			switch ext {
			case "json":
				continue
			case "mp4":
				continue
			}

			imgFile, err := os.Open(fmt.Sprintf("%s%s", Args.GetPath(), file.Name()))

			if err != nil {
				log.Fatalf("LOAD ERR: %s", err)
			}

			img, _, err := image.Decode(imgFile)
			_ = imgFile.Close()

			if err != nil {
				log.Fatalf("DECODE ERR: %s", err)
			}
			// Scale image, then read image.Image into channel.
			img = photerm.ScaleImg(img, Args)
			results <- img
		}
		// Close the channel once all files have been read into it.
		close(results)
	}

	// blocking code
	imageBuffer := make(chan image.Image, len(fs))
	// non-blocking
	go work(imageBuffer)

	// Return the read side of the channel
	return imageBuffer
}

// Buffer a single image for non-sequential display
// from a full provided path.
func BufferImagePath(imageBuffer chan image.Image) (err error) {
	imgFile, err = os.Open(Args.GetPath())

	if err != nil {
		log.Fatalf("LOAD ERR: %s", err)
	}

	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)

	if err != nil {
		log.Fatalf("DECODE ERR: %s", err)
	}
	// Scale image, then read image.Image into channel.
	img = photerm.ScaleImg(img, Args)
	imageBuffer <- img
	close(imageBuffer)
	return nil
}

// PrintFromBuff consumes the image.Image files sent into imageBuffer by BufferImages()
// This function prints the buffer sequentially.
// Essentially, this is lo-fi in-terminal video playback via UTF-8 / ASCII encoded pixels.
// For now, use ffmpeg cli to generate frames from a video file.
func PrintFromBuff(imageBuffer <-chan image.Image, glyphs string) (err error) {
	var frame string

	palette := [256]rune{}
	copy(palette[:], []rune(util.Stretch(glyphs, 255)))

	for img := range imageBuffer {

		scaleY := 1

		focusView := Args.GetFocusView(img)
		r := focusView.GetRegion()
		//top, left, right, btm := FocusArea(focusView)
		// go row by row in the scaled image.Image and...
		for y := r.Top; y < r.Btm; y += int(Args.Squash) {
			frame = ""
			// print cells from left to right
			for x := r.Left; x < r.Right; x += scaleY {
				// get brightness of cell
				//c := AvgPixel(img, x, y, int(Args.Squash), scaleY)
				c := color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y

				// get the rgba colour
				rgb := rotato.RotateHue(color.RGBAModel.Convert(img.At(x, y)).(color.RGBA), Args.HueAngle)

				// get the colour and glyph corresponding to the brightness
				ink := RGB(rgb.R, rgb.G, rgb.B, Foreground)

				frame += ink + string(palette[c])
			}
			frame += Normalizer + "\n"
			fmt.Print(frame)
		}

		if len(imageBuffer) == 0 {
			fmt.Println(Normalizer) // leave the final frame in the terminal. Allows for single image render.
		} else {
			fmt.Printf("\033[%sA", fmt.Sprint(r.Btm)) // reset cursor to original position before drawing next image.
		}
	}
	return nil
}

func mp4ToFrames() {
	// Split path into constituent strings
	dest := strings.Split(Args.Path, "/")
	// re-join all but the final element to construct the path minus the target mp4
	destDir := strings.Join(dest[0:len(dest)-1], "/")

	// construct the ffmpeg command & run it to convert mp4 to indvidual images saved in destDir
	// images are named in ascending order, starting at 00000.jpg
	c := exec.Command("ffmpeg", "-i", Args.Path, "-vf", "fps=24", destDir+"/%05d.jpg")
	err := c.Run()
	// catch any errors from the ffmpeg call.
	if err != nil {
		log.Fatal(err)
	}
}

func main() {

	arg.MustParse(&Args)
	ArgsToJson(Args)

	charset := strings.Join(Charsets[Args.Charset], "")
	if Args.Custom != "" {
		charset = Args.Custom
	}

	switch Args.Mode {
	case "L":
		// Provide a path to an mp4.
		// it will be converted to individual jpgs.
		// jpgs will be saved in the same directory as the mp4.
		mp4ToFrames()
		fallthrough

	case "R":
		// Provide a DIRECTORY to Mode B for sequential play of all images inside.

		fs, err := os.ReadDir(Args.GetPath())
		if err != nil {
			log.Fatal(err)
		}

		// load image files in a goroutine
		// ensures playback is not blocked by io.
		imageBuffer := BufferImageDir(fs)

		// Consumes image.Image from imageBuffer
		// Prints each to the terminal.
		PrintFromBuff(imageBuffer, charset)

	case "I":
		// Provide a full path to Mode A for individual image display.

		imageBuffer := make(chan image.Image, 1)
		BufferImagePath(imageBuffer)
		PrintFromBuff(imageBuffer, charset)
	}
}
