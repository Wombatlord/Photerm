<!-- markdownlint-disable MD004 MD033 MD034 -->

<div align="center">

# Photerm

</div>
<p align="center">
	<img alt="GitHub" src="https://img.shields.io/github/license/Wombatlord/Photerm?logo=Github&logoColor=green">
	
	<img alt="GitHub" src="https://img.shields.io/github/go-mod/go-version/Wombatlord/photerm?logo=go"></p>

## Introduction
Photerm is a command line interface tool for interacting with images from within the terminal. 

## Features
- Render images in the terminal.
- Rescale images.
- Rotate the hue across the image.
- Render the image with different character sets.

## Installation
To use Photerm, clone the repo and run `go run main.go` in the project root, along with the path to an image you wish to render.

## Usage
Photerm has various CLI arguments for altering the output.

To save the output, simply redirect it to a txt file. The image can then be rerendered any time by printing the file in the terminal.

```
Usage: main.exe [--scale SCALE] [--wide-boyz WIDE-BOYZ] [--in] [--mode MODE] [--y-org Y-ORG] [--height HEIGHT] [--x-org X-ORG] [--width WIDTH] [--hue HUE] [PATH]

Positional arguments:
  PATH                   file path for an image

Options:
  --scale SCALE, -s SCALE
                         overall image scale [default: 1.0]
  --wide-boyz WIDE-BOYZ, -w WIDE-BOYZ
                         How wide you want it guv? (Widens the image) [default: 1.0]
  --in, -i               read from stdin
  --mode MODE, -m MODE   mode selection determines characters used for rendering [default: 0]
  --y-org Y-ORG          minimum Y, top of focus [default: 0]
  --height HEIGHT        height, vertical size of focus [default: 0]
  --x-org X-ORG          minimum X, left edge of focus [default: 0]
  --width WIDTH          width, width of focus [default: 0]
  --hue HUE              hue rotation angle in radians [default: 0.0]
  --help, -h             display this help and exit
```
