package util

import "fmt"

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
