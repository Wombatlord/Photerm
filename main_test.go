package main

import (
	"image"
	"image/color"
	"math/rand"
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
        Btm: 90,
    }
    img = stubImg{r}
    for n := 0; n < b.N; n++ {
        RenderFrame(img, charset, r)
    }
}
