package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"os"
	"strings"
	"time"

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
	CSI                = "\u001b["
)

// RGB paints the string with a true color rgb painter
func RGB(r, g, b byte, p Painter) string {
	return fmt.Sprintf("%s2;%d;%d;%dm", p, r, g, b)
}

func MoveCursorUp(n int) string {
	return fmt.Sprintf("\n%s%dA", CSI, n)
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

type CharPalette [256]rune

// MakeCharPalette takes an arbitrary number of string arguments and concatenates (and stretches, if necessary)
// them together into a CharPalette
func MakeCharPalette(glyphs ...string) CharPalette {
	concat := strings.Join(glyphs, "")
	pal := CharPalette{}
	copy(pal[:], []rune(util.Stretch(concat, 255)))
	return pal
}

const NotSet = 0

type Cli struct {
	Path      string  `arg:"positional" help:"file path for an image" default:"liljeffrey.jpg"`
	Scale     float64 `arg:"-s, --scale" help:"overall image scale" default:"1.0"`
	Squash    float64 `arg:"-w, --wide-boyz" help:"How wide you want it guv? (Widens the image)" default:"1.0"`
	StdInput  bool    `arg:"-i, --in" help:"read from stdin"`
	Mode      string  `arg:"-m, --mode" help:"mode selection determines renderer" default:"A"`
	Charset   Charset `arg:"-c, --Charset" help:"Charset selection determines the character set used by the renderer" default:"0"`
	Custom    string  `arg:"--custom" help:"provide a custom string to render with" default:"█"`
	YOrigin   int     `arg:"--y-org" help:"minimum Y, top of focus" default:"0"`
	Height    int     `arg:"--height" help:"height, vertical size of focus" default:"0"`
	XOrigin   int     `arg:"--x-org" help:"minimum X, left edge of focus" default:"0"`
	Width     int     `arg:"--width" help:"width, width of focus" default:"0"`
	HueAngle  float32 `arg:"--hue" help:"hue rotation angle in radians" default:"0.0"`
	FrameRate int     `arg:"--fps" help:"Provide an integer number of frames per second as an upper limit to the playback speed"`
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
	imgFile, err := os.Open(Args.GetPath())

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

// GetFpsLimiter returns an adaptor locked to the provided FPS
// that takes the original unlimited buffer and a new buffer to be populated with one
// frame per tick where a tick is 1/fps seconds.
func GetFpsLimiter(fps int) func(in <-chan image.Image, out chan<- image.Image) {
	fpsLimiter := func(in <-chan image.Image, out chan<- image.Image) {
		ticker := time.NewTicker(time.Second / time.Duration(fps))
		for range ticker.C {
			frame, frameOk := <-in
			if !frameOk {
				break
			} else {
				out <- frame
			}
		}
		close(out)
	}
	return fpsLimiter
}

// FrameEndHook is a function that decides what happens between frames.
// this is used to switch from animation mode to printout mode
type FrameEndHook func(writer io.Writer, seekBack int) error

// FrameEndHooks is just a named collection of the above
type FrameEndHooks struct {
	Print, Animate FrameEndHook
}

var frameEndHooks = FrameEndHooks{
	// Print is the appropriate frame end hook to use when the output is intended to be a still image.
	Print: func(writer io.Writer, _ int) error {
		_, err := fmt.Fprintln(writer, Normalizer)
		return err
	},
	// Animate is the appropriate frame end hook when subsquent images are to be treated as frames in an
	// animation.
	Animate: func(writer io.Writer, seekBack int) error {
		_, err := fmt.Fprintln(writer, util.MoveUp(seekBack))
		return err
	},
}

// PlayFromBuff consumes the image.Image files sent into imageBuffer by BufferImages()
// This function prints the buffer sequentially.
// Essentially, this is lo-fi in-terminal video playback via UTF-8 / ASCII encoded pixels.
// For now, use ffmpeg cli to generate frames from a video file.
func PlayFromBuff(imageBuffer <-chan image.Image, glyphs string, fps int) (err error) {
	if fps != 0 {
		fpsLimiter := GetFpsLimiter(fps)
		fpsLimitedBuffer := make(chan image.Image, len(imageBuffer))
		go fpsLimiter(imageBuffer, fpsLimitedBuffer)
		return FOutFromBuf(os.Stdout, fpsLimitedBuffer, glyphs, frameEndHooks.Animate)
	} else {
		return FOutFromBuf(os.Stdout, imageBuffer, glyphs, frameEndHooks.Animate)
	}
}

// PrintFromBuf is designed to print an image or sequence of images to file or stdout.
// so it uses the print frame end hook
func PrintFromBuf(imageBuffer <-chan image.Image, glyphs string) (err error) {
	return FOutFromBuf(os.Stdout, imageBuffer, glyphs, frameEndHooks.Print)
}

// FprintFromBuff consumes the image.Image files sent into imageBuffer by BufferImages()
// This function prints the buffer to the passed io.WriteCloser sequentially.
// Essentially, this is lo-fi in-terminal video playback via UTF-8 / ASCII encoded pixels.
// For now, use ffmpeg cli to generate frames from a video file.
func FOutFromBuf(writer io.WriteCloser, imageBuffer <-chan image.Image, glyphs string, frameEndHook FrameEndHook) (err error) {
	palette := MakeCharPalette(glyphs)

	for img := range imageBuffer {
		r := Args.GetFocusView(img).GetRegion()

		// render and print the frame
		frame := RenderFrame(img, palette, r)
		printedHeight := len(frame)
		_, err := fmt.Fprint(writer, strings.Join(frame, "\n"))
		if err != nil {
			return err
		}

		if err = frameEndHook(writer, printedHeight); err != nil {
			return err
		}
	}
	return nil
}

// RenderFrame returns the printable representation of a single frame as a string. Each frame is a slice of strings
// each string representing a horizontal line of pixels
func RenderFrame(img image.Image, palette CharPalette, r photerm.Region) (frameLines []string) {
    frameLines = []string{}
	// go row by row in the scaled image.Image and...
	for y := r.Top; y < r.Btm; y++ {
		line := ""
		// print cells from left to right
		for x := r.Left; x < r.Right; x++ {
			// get brightness of cell
			//c := AvgPixel(img, x, y, int(Args.Squash), scaleY)
			c := color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y

			// get the rgba colour
			rgb := rotato.RotateHue(color.RGBAModel.Convert(img.At(x, y)).(color.RGBA), Args.HueAngle)

			// get the colour and glyph corresponding to the brightness
			ink := RGB(rgb.R, rgb.G, rgb.B, Foreground)

			line += fmt.Sprint(ink, string(palette[c]))
		}
		frameLines = append(frameLines, line)
	}
	//frameLines = append(frameLines, Normalizer)
	return frameLines
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
		photerm.Mp4ToFrames(Args)
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
		PlayFromBuff(imageBuffer, charset, Args.FrameRate)

	case "I":
		// Provide a full path to Mode A for individual image display.

		imageBuffer := make(chan image.Image, 1)
		BufferImagePath(imageBuffer)
		PrintFromBuf(imageBuffer, charset)
	}
}
