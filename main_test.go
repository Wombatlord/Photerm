package main

import (
	"image"
	"image/color"
	"math/rand"
	"os"
	"testing"
	"time"
	"wombatlord/imagestuff/photerm"
)

// a random number generator
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// this is a stub implementation of a frame
type stubImg struct {
	r photerm.Region
}

// Bounds just translates the photerm.Region in stubImg.r into an image.Rectangle
// and returns it
func (s stubImg) Bounds() image.Rectangle {
	return image.Rect(s.r.Left, s.r.Top, s.r.Right, s.r.Btm)
}

// Generates random noise. Not a sensible implementation for any other use case
func (s stubImg) At(x, y int) color.Color {
	return color.RGBA{
		R: uint8(rng.Intn(256)),
		G: uint8(rng.Intn(256)),
		B: uint8(rng.Intn(256)),
	}
}

// ColorModel is part of the image.Image interface. Just returns color.RGBAModel
func (s stubImg) ColorModel() color.Model {
	return color.RGBAModel
}

func BenchmarkRenderFrame(b *testing.B) {
	charset := MakeCharPalette("#")
	var img image.Image

	r := photerm.Region{
		Right: 160,
		Btm:   90,
	}
	img = stubImg{r}
	for n := 0; n < b.N; n++ {
		RenderFrame(img, charset, r)
	}
}

func BenchmarkFoutFromBuf(b *testing.B) {
	r := photerm.Region{Right: 160, Btm: 90}

	imageBuf := make(chan image.Image, b.N+1)

	go func(n int, res chan<- image.Image) {
		for i := 0; i < n; i++ {
			res <- stubImg{r}
		}
		close(res)
	}(b.N, imageBuf)

	out, err := os.OpenFile(os.DevNull, os.O_APPEND|os.O_RDWR, 0o744)
	if err != nil {
		b.Fatalf("%s", err)
	}
	FOutFromBuf(out, imageBuf, "#", frameEndHooks.Animate)
}
