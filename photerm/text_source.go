package photerm

import (
	"fmt"
	"image"
	"image/draw"
	"io"
	"io/ioutil"
	"log"
	"strings"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

func NewCtx(f *truetype.Font, rgba draw.Image, pts float64) (ctx *freetype.Context) {
	// Initialize the ctx.
	fg, bg := image.White, image.Black
	draw.Draw(rgba, rgba.Bounds(), bg, image.ZP, draw.Src)
	ctx = freetype.NewContext()
	ctx.SetDPI(72)
	ctx.SetFont(f)
	ctx.SetFontSize(pts)
	ctx.SetClip(rgba.Bounds())
	ctx.SetDst(rgba)
	ctx.SetSrc(fg)
	ctx.SetHinting(font.HintingFull)
	return ctx
}

// TypeFace is the encapsulation of all of the information about the
// geometry & scale of the font.
type TypeFace struct {
	Font *truetype.Font
	Face font.Face
}

// ImageHeight returns the height bounding rectangle for any glyph in the font
func (tf *TypeFace) ImageHeight() int {
	return int(tf.Face.Metrics().Height)>>6 + int(tf.Face.Metrics().Descent)>>6
}

// ImageHeight returns the width bounding rectangle for any glyph in the font
func (tf *TypeFace) ImageWidth() int {
	w, ok := tf.Face.GlyphAdvance(' ')
	if !ok {
		log.Fatal("Could not get glyph width")
	}

	return int(w) >> 6
}

// Generators

// These functions return a string channel read handle
// that can be passed to GenImages

// GenWords takes text and sends each word down the pipe one by one.
func GenWords(text string) <-chan string {
	work := func(out chan<- string, txt string) {
		defer close(out)
		for found := true; found; {
			var word, rest string
			word, rest, found = strings.Cut(txt, " ")
			txt = string(rest)
			out <- string(word)
		}
	}

	res := make(chan string)
	go work(res, text)
	return res
}

// GenFixedWidth emulates a marquee by sending a fixed length substring
// down the pipe with it's starting position incremented by one
func GenFixedWidth(text string, width int) <-chan string {
	text = strings.ReplaceAll(text, "\t", "    ")
	work := func(out chan<- string, w int, t string) {
		defer close(out)
		glyphs := []rune(t)
		for i := 0; i < len(glyphs)-w; i++ {
			s := string(glyphs[i : i+w])
			out <- s
		}
	}

	results := make(chan string)
	go work(results, width, text)

	return results
}

// LoadTypeFace attempts to load the font at the path specified, and if successful, it returns the typeface, nil.
// In case of an error, it returns nil, err.
func LoadTypeFace(path string, size float64) (typeface *TypeFace, err error) {
	// everything is declared at this point, so returning if
	// err != nil will return nil, err
	var fontBytes []byte

	// Read the font data.
	fontBytes, err = ioutil.ReadFile(path)
	if err != nil {
		return
	}

	// then parse it
	f, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return
	}

	face := truetype.NewFace(f, &truetype.Options{Size: size})

	return &TypeFace{Font: f, Face: face}, nil
}

// LefToRightConcat concatenates the images passed with imgs[0] being the leftmost.
func LefToRightConcat(imgs ...draw.Image) (concat draw.Image, err error) {
	if len(imgs) == 0 {
		return nil, fmt.Errorf("LefToRightConcat: Must supply at least one image")
	}
	cellWidth := imgs[0].Bounds().Dx()

	width := 0
	for _, nxt := range imgs {
		width += nxt.Bounds().Dx()
	}
	concat = image.NewRGBA(
		image.Rectangle{
			image.Point{0, 0},
			image.Point{
				width,
				imgs[0].Bounds().Dy(),
			},
		},
	)

	for i, nxt := range imgs {
		draw.Draw(
			concat,
			nxt.Bounds().Add(image.Point{i * cellWidth, 0}),
			nxt,
			image.Point{0, 0},
			draw.Src,
		)
	}

	return
}

// GenImages takes a string generator and returns a image buffer, i.e. an instance of <-chan image.Image.
// This buffer transports image.Image instances that are the png rendered text using the TypeFace.
func GenImages(text <-chan string, pts float64, panningStep int) <-chan image.Image {
	work := func(res chan<- image.Image, txt <-chan string, tf *TypeFace) {
		defer close(res)
		const smoothing = 2
		imgs := []draw.Image{}
		w, h := tf.ImageWidth(), tf.ImageHeight()
		if panningStep == 0 {
			panningStep = w
		}
		for x := range text {
			for _, glyph := range x {
				// create the rgba image
				glyphImg := image.NewRGBA(image.Rect(0, 0, w, h))

				// Initialise the ctx.
				ctx := NewCtx(tf.Font, glyphImg, pts)
				m := tf.Face.Metrics()
				pixelHeight := int(m.Height) >> 6

				// set the positioning
				pt := freetype.Pt(0, pixelHeight)

				// draw
				ctx.DrawString(string(glyph), pt)
				imgs = append(imgs, glyphImg)
			}
			// concatenate the images and...
			concat, err := LefToRightConcat(imgs...)
			if err != nil {
				log.Fatal(err)
			}

			for i := 0; i < w; i += panningStep {
				dstBounds := image.Rect( // The dimensions of the cropped image:
					0,                      // left
					0,                      // top
					concat.Bounds().Dx()-w, // right
					h,                      // btm
				)
				crop := image.NewRGBA(dstBounds)
				croppingRect := dstBounds.Add(image.Point{i, 0})

				draw.Draw(crop, crop.Bounds(), concat, croppingRect.Min, draw.Src)
				res <- crop
			}
			// send concat!
			imgs = []draw.Image{}
		}

	}

	typeface, err := LoadTypeFace("./font.ttf", pts)
	if err != nil {
		log.Fatal(err)
	}

	out := make(chan image.Image)
	go work(out, text, typeface)

	return out
}

func Marquee(from io.Reader, fontPts float64, letterWidth int) (<-chan image.Image, error) {
	text, err := ioutil.ReadAll(from)
	if err != nil {
		return nil, err
	}
	words := GenFixedWidth(string(text), letterWidth)

	return GenImages(words, fontPts, 1), nil
}
