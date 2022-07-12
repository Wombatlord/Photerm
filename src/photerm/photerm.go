package photerm

import (
	"fmt"
	"github.com/nfnt/resize"
	"image"
	"image/color"
	"io"
	"log"
	"strconv"
	"strings"
	"wombatlord/imagestuff/src/rotato"
	"wombatlord/imagestuff/src/util"
)

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

type Styled interface {
	IsBold() bool
	GetHueAngle() float32
	GetBG() RGBSpec
}

type ImageControls interface {
	FocusView
	ScaleFactors
	PathSpec
	Styled
}

type Region struct{ Left, Top, Right, Btm int }

// OutputDimsOf consumes a Cli value and returns pixel width, height tuple
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
	return resize.Resize(w, h, img, resize.Bicubic)
}

// GlyphPrint implements Printout and depends on the image.Image, the PaletteMap and
// the command line args
type GlyphPrint struct {
	Img   image.Image
	Chars PaletteMap
	Cli   ImageControls
}

type Animation struct {
	*GlyphPrint
	Buff <-chan image.Image
}

func (a *Animation) PrintBuff(writer io.Writer, buff <-chan image.Image, r Region) {
	for img := range buff {
		a.Img = img
		a.PrintRegion(writer, r)
	}
}

// ColorModel is just here to fill out the Printout interface
func (g *GlyphPrint) ColorModel() color.Model {
	return color.RGBAModel
}

// Chroma is where all the colour transformation logic goes
func (g *GlyphPrint) Chroma(c color.Color) (string, color.RGBA) {
	fg, rgbFg := g.FGChroma(c)
	bg := g.BGChroma(c)
	return bg + fg, rgbFg
}

func (g *GlyphPrint) FGChroma(c color.Color) (string, color.RGBA) {
	rgbOut := rotato.RotateHue(color.RGBAModel.Convert(c).(color.RGBA), g.Cli.GetHueAngle())
	return util.RGB(rgbOut.R, rgbOut.G, rgbOut.B, util.Foreground), rgbOut
}

func (g *GlyphPrint) BGChroma(_ color.Color) string {
	painter := string(util.NoPaint)
	if g.Cli.GetBG() != "" {
		rgb := g.Cli.GetBG().RGBA()
		painter += util.RGB(rgb.R, rgb.G, rgb.B, util.Background)
	}
	return painter
}

// Typeset is where the printable glyph is chosen
func (g *GlyphPrint) Typeset(c color.Color) string {
	glyphIdx := color.GrayModel.Convert(c).(color.Gray).Y
	return string(g.Chars[glyphIdx])
}

func (g *GlyphPrint) At(x, y int) CellSpec {
	// set up display attributes like bold or italics
	displayAttributes := util.Opt[util.Painter]{true: util.Bold}[g.Cli.IsBold()]

	// derive the cell
	pix := g.Img.At(x, y)

	// Create the painter
	painter, rgbOut := g.Chroma(pix)

	// select the printable glyph
	glyph := g.Typeset(pix)

	// return the CellSpec
	return CellSpec{string(displayAttributes), painter, glyph, rgbOut}
}

// PaletteMap is a convenient alias for a length 256 rune array
type PaletteMap [256]rune

// CellSpec is the terminal printout equivalent of a pixel, however it has the extra
// parameter of the text to be displayed in the terminal
type CellSpec struct {
	Display, Ink, Glyph string
	rgba                color.RGBA
}

// String generates the terminal output
func (c CellSpec) String() string { return c.Display + c.Ink + c.Glyph }

// RGBA is the interface of an image.Color
func (c *CellSpec) RGBA() (r, g, b, a uint8) {
	return c.rgba.R, c.rgba.G, c.rgba.B, c.rgba.A
}

// Bounds is the size of the GlyphPrint
func (g *GlyphPrint) Bounds() image.Rectangle {
	return g.Img.Bounds()
}

func (g *GlyphPrint) SetPalette(p string) {
	glyphs := PaletteMap{}
	copy(glyphs[:], []rune(util.Stretch(p, 255)))
	g.Chars = glyphs
}

func may(_ any, err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// PrintRegion actually prints out the region to whatever output its given
func (g *GlyphPrint) PrintRegion(writer io.Writer, region Region) {
	// go row by row in the scaled image.Image and...
	for y := region.Top; y < region.Btm; y++ {
		// then cell by cell...
		for x := region.Left; x < region.Right; x++ {
			cell := g.At(x, y)
			may(fmt.Fprint(writer, cell))
		}
		may(fmt.Fprintln(writer, util.Normalizer))
	}
}

type RGBSpec string

func (r RGBSpec) RGBA() color.RGBA {
	rgbs := strings.SplitN(string(r), ":", 3)
	rgb := [3]uint8{}
	for i := range rgb {
		c, err := strconv.Atoi(rgbs[i])
		if err != nil {
			log.Fatal(err)
		}
		rgb[i] = uint8(c)
	}
	return color.RGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: 255}
}
