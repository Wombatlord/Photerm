package photerm

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"io"
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

func StreamMp4ToFrames(p PathSpec) (<-chan []byte, error) {
	// construct the ffmpeg command & run it to convert mp4 to indvidual images saved in destDir
	// images are named in ascending order, starting at 00000.jpg
	//
	// Useful for debugging:
	// c := exec.Command("/bin/cat", "tmp/my_test_data")
	//
	c := exec.Command("ffmpeg", "-i", p.GetPath(), "-vf", "fps=24", "-vcodec", "png", "-f", "image2pipe", "-")

	out, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stream := out

	logs, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}
	logStream := bufio.NewScanner(logs)

	if err = c.Start(); err != nil {
		return nil, err
	}

	// copy the logs to the actual logs
	go func() {
		subprocessLogs := []string{}
		for logStream.Scan() {
			subprocessLogs = append(subprocessLogs, string(logStream.Bytes()))
			if err != nil {
				fmt.Println(strings.Join(subprocessLogs, "\n"))
				log.Fatal(err)
			}
		}
	}()

	// Tidies up after itself
	go func(p *exec.Cmd) {
		p.Wait()
	}(c)

	frames := make(chan []byte)
	go CutPNGsFromStream(frames, stream)

	return frames, nil
}

var PNGHead = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

// Lookahead is an exact match lookahead search function.
// In this context it's being used to detect PNG file headers
// in an undifferentiated stream of bytes.
func Lookahead(needle, haystack []byte) (imgOffset int) {

	// Search is the loop over incoming bytes
	for i := range haystack {

		// we need to be able to look ahead
		// by the length of the needle
		if i >= len(haystack)-len(needle) {
			break
		}

		// The data should start with a header but
		// we don't want to treat the zero preceding
		// bytes as an image
		if i < len(needle) {
			continue
		}

		// lookahead search at each byte
		for offset, patternByte := range needle {
			// if the pattern matches at the current offset
			if haystack[i+offset] == patternByte {
				// and the current offset is the end of the pattern
				if offset == len(needle)-1 {
					// return the imgOffset
					imgOffset = i
					return imgOffset
				}
			} else {
				// if the pattern does not match,
				// move to the next byte
				break
			}
		}
	}

	return imgOffset
}

// CutPNGsFromStream uses the Lookahead search to aggregate an image from the
// stream and send the completed PNG bytes down its return channel.
// It will stop either when the number of bytes read is zero or an
// error occurs.
func CutPNGsFromStream(out chan<- []byte, r io.ReadCloser) {
	defer close(out)
	defer r.Close()
	var (
		imgData []byte
	)

	for {
		buff := make([]byte, 4096)
		n, err := r.Read(buff[:])
		if n == 0 {
			break
		}
		if err != nil {
			break
		}

		// append the buffer to the imageData
		imgData = append(imgData, buff[:n]...)

		// search for the next
		offset := Lookahead(PNGHead, imgData)

		if offset != 0 {
			// make a nextData variable to hold the data
			// beginning with the header of the next image
			nextData := make([]byte, len(imgData)-offset)

			// copy the image data starting at the header
			copy(nextData, imgData[offset:])

			// make a result to hold the complete first image
			result := make([]byte, offset)
			copy(result, imgData[:offset])

			// send the result down the pipe
			out <- result

			// overwrite the assignment of imgData with nextData
			imgData = nextData
		}
	}
}

// Stream2Buf is a transformation step in the pipeline.
func Stream2Buf(buf chan<- image.Image, s <-chan []byte, sf ScaleFactors) {
	defer close(buf)
	for r := range s {
		br := bytes.NewReader(r)
		img, _, err := image.Decode(br)
		if err != nil {
			log.Fatal("stream2Buff: ", err)
		}
		buf <- ScaleImg(img, sf)
	}
}
