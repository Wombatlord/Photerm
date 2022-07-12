package rotato

import (
	"image/color"
	"math"
	"wombatlord/imagestuff/src/util"

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
    // the min/max fuckery is to prevent artifacting due to rounding errors causing integer overflows
    return color.RGBA{
        R: uint8(util.Max(util.Min(int(res[0]), 255), 0)),
        G: uint8(util.Max(util.Min(int(res[1]), 255), 0)),
        B: uint8(util.Max(util.Min(int(res[2]), 255), 0)),
        A: rgb.A,
    }
}
