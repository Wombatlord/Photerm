package main

import (

	"bufio"
	"encoding/json"

	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
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

var numeralCache = func() [][]byte {
	val := make([][]byte, 256)
	for i := range val {
		val[i] = []byte(fmt.Sprint(i))
	}
	return val
}()

// RGB paints the string with a true color rgb painter
func RGB(r, g, b byte, p Painter) []byte {
	// the apalling code below is way faster than the fmt.Sprintf version
	// but for readability the fmt.Sprintf version is below
	// return fmt.Sprintf("%s2;%d;%d;%dm", p, r, g, b)
	return append(append(append(append(append(append(append(
		[]byte(p), []byte("2;")...), []byte(numeralCache[r])...), ';'), numeralCache[g]...), ';'), numeralCache[b]...), 'm',
	)
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

var Args photerm.Cli
var fc photerm.FrameCache

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

// FOutFromBuf consumes the image.Image files sent into imageBuffer by BufferImages()
// This function prints the buffer to the passed io.WriteCloser sequentially.
// Essentially, this is lo-fi in-terminal video playback via UTF-8 / ASCII encoded pixels.
// For now, use ffmpeg cli to generate frames from a video file.
func FOutFromBuf(writer io.WriteCloser, imageBuffer <-chan image.Image, glyphs string, frameEndHook FrameEndHook) (err error) {
	palette := MakeCharPalette(glyphs)

	// Use a buffered writer bc it's probably faster
	bufWriter := bufio.NewWriter(writer)
	for img := range imageBuffer {
		r := Args.GetFocusView(img).GetRegion()

		// render and print the frame
		frame := RenderFrame(img, palette, r)
		printedHeight := len(frame)
		_, err := fmt.Fprint(bufWriter, strings.Join(frame, "\n"))
		if err != nil {
			return err
		}

		if err = frameEndHook(bufWriter, printedHeight); err != nil {
			return err
		}

		// flushing the bufWriter here will actually do the write to the output writer
		// think of it as some ghetto ass vsync
		bufWriter.Flush()
	}
	return nil
}

// RenderFrame returns the printable representation of a single frame as a string. Each frame is a slice of strings
// each string representing a horizontal line of pixels
func RenderFrame(img image.Image, palette CharPalette, r photerm.Region) (frameLines []string) {
	frameLines = []string{}

	// go row by row in the Scaled image.Image and...

	for y := r.Top; y < r.Btm; y++ {
		line := []byte{}
		// print cells from left to right
		for x := r.Left; x < r.Right; x++ {
			rgb := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)

			// get brightness of cell
			c := color.GrayModel.Convert(rgb).(color.Gray).Y

			// rotate hue
			rotato.RotateHue(&rgb, Args.HueAngle)

			// get the colour and glyph corresponding to the brightness
			ink := RGB(rgb.R, rgb.G, rgb.B, Foreground)

			line = append(append(line, ink...), []byte(string(palette[c]))...)
		}
		frameLines = append(frameLines, string(line))
	}
	return frameLines
}

func main() {
	arg.MustParse(&Args)
	photerm.ArgsToJson(Args)

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

		fc.LoadImageFiles(Args)
	
		// load image files in a goroutine
		// ensures playback is not blocked by io.
		imageBuffer := fc.BufferImageDir(Args)

		// Consumes image.Image from imageBuffer
		// Prints each to the terminal.
		util.Must(PlayFromBuff(imageBuffer, charset, Args.FrameRate))

	case "I":
		// Provide a full path to Mode A for individual image display.
		imageBuffer := make(chan image.Image, 1)
		fc.BufferImagePath(imageBuffer, Args, Args)
		PrintFromBuf(imageBuffer, charset)

		util.Must(BufferImagePath(imageBuffer))
		util.Must(PrintFromBuf(imageBuffer, charset))

	case "S":
		// S is the streaming mode!
		// the async stream is represented by a <-chan []byte
		stream, err := photerm.StreamMp4ToFrames(Args)
		if err != nil {
			log.Fatal(err)
		}
		buf := make(chan image.Image)

		// here the stream is transformed into a <-chan image.Image
		go photerm.Stream2Buf(buf, stream, Args)

		// and the played out to the terminal
		util.Must(PlayFromBuff(buf, charset, Args.FrameRate))

	// T stands for Text mode. Text mode expects string data from the stdin.
	// This can be provided by pipe:
	// 			$ echo 'foo' | photerm [ARGS]
	case "T":
		// here we create the marquee image source to read text from the stdin
		buf, err := photerm.Marquee(os.Stdin, 100, 8)
		if err != nil {
			log.Fatal(err)
		}
		buf = photerm.AppendScalingStep(buf, Args)
		util.Must(PlayFromBuff(buf, charset, Args.FrameRate))
	}
}
