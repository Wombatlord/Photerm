package rotato

import (
	"image/color"
	"math"

	"github.com/ungerik/go3d/quaternion"
	"github.com/ungerik/go3d/vec3"
)

func RotateHue(rgb color.RGBA, hueAngle float32) color.RGBA {
    mag := float32(math.Cbrt(3))
    axis := vec3.T{
        1.0/mag,
        1.0/mag,
        1.0/mag,
    }

    rot := quaternion.FromAxisAngle(&axis, hueAngle)

    rgbVec := vec3.T{
        float32(rgb.R),
        float32(rgb.G),
        float32(rgb.B),
    }

    res := rot.RotatedVec3(&rgbVec)

    return color.RGBA{
        R: uint8(res[0]),
        G: uint8(res[1]),
        B: uint8(res[2]),
        A: rgb.A,
    }
}
