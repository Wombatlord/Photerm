package photerm

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"strings"
	"wombatlord/photerm/src/util"
)

const NotSet = 0

type Charset int

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
func (c Cli) GetRegion() Region {
	return Region{
		Left:  c.GetXOrigin(),
		Top:   c.GetYOrigin(),
		Right: c.GetXOrigin() + c.GetWidth(),
		Btm:   c.GetYOrigin() + c.GetHeight(),
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

// ArgsToJson serialises the passed CLI args.
// The output file will take its name from Args.Path
func ArgsToJson(c Cli) {
	file, _ := json.MarshalIndent(c, "", " ")
	fileName := fmt.Sprintf("%s.json", strings.Split(c.Path, ".")[0])

	_ = os.WriteFile(fileName, file, 0644)
}
