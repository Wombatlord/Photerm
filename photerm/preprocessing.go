package photerm

import (
	"log"
	"os/exec"
	"strings"
)

// Proxy to ffmpeg CLI for generating sequential jpgs from an mp4.
func Mp4ToFrames(p PathSpec) {
	// Split path into constituent strings
	dest := strings.Split(p.GetPath(), "/")
	// re-join all but the final element to construct the path minus the target mp4
	destDir := strings.Join(dest[0:len(dest)-1], "/")

	// construct the ffmpeg command & run it to convert mp4 to indvidual images saved in destDir
	// images are named in ascending order, starting at 00000.jpg
	c := exec.Command("ffmpeg", "-i", p.GetPath(), "-vf", "fps=24", destDir+"/%05d.jpg")
	err := c.Run()
	// catch any errors from the ffmpeg call.
	if err != nil {
		log.Fatal(err)
	}
}
