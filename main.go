package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"strconv"

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

// Opt is for optional values
type Opt[T any] map[bool]T

type Painter string

const (
    NoPaint    Painter = ""
	Foreground         = "\u001b[38;"
	Background         = "\u001b[48;"
    Bold               = "\u001b[1m"
	Normalizer         = "\u001b[0m"
)

func (p Painter) Apply(cells ...rune) Painter { return Painter(fmt.Sprint(p, string(cells))) }

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
	ASCIIFY:  {" ", "∙", "*", "$", "@"},
}

var Ramps = []string{
	Normal: "$@B%8&WM#*oahkbdpqwmZO0QLCJUYXzcvunxrjft/\\|()1{}[]?-_+~<>i!lI;:,^`'. ",
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
    return color.RGBA{rgb[0], rgb[1], rgb[2], 255}
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
    Bold     bool    `arg:"--bold" help:"Render the glyphs in bold" default:"true"`
    CustomPalette string `arg:"--custom" help:"Supply a string of characters to be used to indicate brightness" default:"#"`
    BG RGBSpec `arg:"--bg" help:"Set the background RGB as "`
}

func (c *Cli) GetPath() string    { return c.Path }
func (c *Cli) GetStdIn() bool     { return c.StdInput }
func (c *Cli) GetScale() float64  { return c.Scale }
func (c *Cli) GetSquash() float64 { return c.Squash }
func (c *Cli) GetYOrigin() int    { return c.YOrigin }
func (c *Cli) GetHeight() int     { return c.Height }
func (c *Cli) GetXOrigin() int    { return c.XOrigin }
func (c *Cli) GetWidth() int      { return c.Width }
func (c *Cli) GetCustomPalette() string { return c.CustomPalette }
func (c *Cli) GetHueAngle() string { return c.CustomPalette }
func (c *Cli) GetRegion() Region {
    return Region{
        Left: c.GetXOrigin(), 
        Top: c.GetYOrigin(), 
        Right: c.GetXOrigin() + c.GetWidth(), 
        Btm: c.GetYOrigin() + c.GetHeight(),
    }
}

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
	return resize.Resize(w, h, img, resize.Bicubic)
}

// Printout is an interface designed to mimic the parts of the image.Image that
// are applicable to a text based printout of an image. Notable differences are the
// return value of At is a CellSpec.
type Printout interface {
    ColorModel() color.Model
    Bounds() image.Rectangle
    At(x, y int) CellSpec
}

// GlyphPrint implements Printout and depends on the image.Image, the PaletteMap and
// the command line args
type GlyphPrint struct{
    Img image.Image
    Chars PaletteMap
    Cli Cli
    YStep int
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
    rgbOut := rotato.RotateHue(color.RGBAModel.Convert(c).(color.RGBA), g.Cli.HueAngle)
    return  RGB(rgbOut.R, rgbOut.G, rgbOut.B, Foreground), rgbOut
}

func (g *GlyphPrint) BGChroma(c color.Color) string {
    painter := string(NoPaint)
    if g.Cli.BG != "" {
        rgb := g.Cli.BG.RGBA()
        painter += RGB(rgb.R, rgb.G, rgb.B, Background)
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
    displayAttributes := Opt[Painter]{true: Bold}[g.Cli.Bold]
    
    // derive the cell
    pix := g.Img.At(x, y)
    
    // Create the painter
    painter, rgbOut := g.Chroma(pix) 

    // select the printable glyph
    glyph := g.Typeset(pix) 
    
    // return the CellSpec 
    return CellSpec{string(displayAttributes), string(painter), glyph, rgbOut}
}

// PaletteMap is a conveinient alias for a length 256 rune array
type PaletteMap [256]rune
type Region struct {Left, Top, Right, Btm int}

// CellSpec is the terminal printout eqivalent of a pixel, however it has the extra
// parameter of the text to be displayed in the terminal
type CellSpec struct { Display, Ink, Glyph string; rgba color.RGBA }

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

// PrintRegion actually prints out the region to whatever output it's given
func (g *GlyphPrint) PrintRegion(writer io.Writer, region Region) {
	// go row by row in the scaled image.Image and...
	for y := region.Top; y < region.Btm; y += g.YStep  {
		// then cell by cell...
		for x := region.Left; x < region.Right; x ++ {
            cell := g.At(x, y)
            fmt.Fprint(writer, cell)
		}
		fmt.Fprintln(writer)
	}
}

func main() {
	arg.MustParse(&Args)
	ArgsToJson(Args)

	img, err := loadImage(&Args)
	if err != nil {
		log.Fatal(err)
	}

	img = ScaleImg(img, &Args)
	focusView := Args.GetFocusView(img)
    printOut := &GlyphPrint{Img: img, Cli: Args, YStep: 1}
    charset := "#"
	switch Args.Mode {
	case "A":
        charset = strings.Join(Charsets[Args.Charset][:], "")
	case "B":
        charset = Ramps[Normal]
	case "C":
        charset = Args.GetCustomPalette()
    }
    printOut.SetPalette(charset)
    printOut.PrintRegion(os.Stdout, focusView.GetRegion())
}
