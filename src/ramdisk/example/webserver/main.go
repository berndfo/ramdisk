package main

import (
	"ramdisk"
	"log"
	"net/http"
	"strings"
	"sync"
	"image"
	"bytes"
	"image/jpeg"
	"image/color"
	"image/draw"
)

const WEBSITE = `
<html>
<meta http-equiv="refresh" content="0">
<body style='background-color:grey;'>
  A pic.
  <img src="/pic.jpg">
  <img src="/pic_alt.jpg">
<body>
</html>
`

// webHandler either sends the JPG stored in the global variable 'latest'
func webHandler(response http.ResponseWriter, request *http.Request) {
	if strings.HasSuffix(request.RequestURI, ".jpg") {
		// send latest file, assuming it represents a jpg
		if latest == nil {
			response.WriteHeader(http.StatusNotFound)
			return
		}
		response.Header().Add("Cache-control", "no-cache")
		response.Header().Add("Content-type", "image/jpg")
		latestMutex.Lock()
		if strings.HasSuffix(request.RequestURI, "_alt.jpg") {
			stampOutPicture(latest.Data) // create new latestAlt from latest
			response.Write(latestAlt)
		} else {
			response.Write(latest.Data)
		}
		latestMutex.Unlock()
	} else {
		// send website for every URL not ending in .jpg
		response.Header().Add("Content-type", "text/html")
		response.Write([]byte(WEBSITE))
	}
}

var latest *ramdisk.FileEntry // last closed file entry
var latestAlt []byte // latest file transformed
var latestMutex sync.Mutex // safeguard access to latest

var maskedImage *image.RGBA

// main starts the sample application, which
// * opens a webserver on port 8080
// * mounts RAM disk at /mnt/myramdisk (this directory must have created in advance)
// when creating new files there, for example by running
//  ffmpeg -i MY_VIDEO.mp4  /mnt/myramdisk/%3d.jpg
// the website http://localhost:8080/ will constantly render the latest picture.
func main() {

	// prepare some static data
	maskedImage = image.NewRGBA(image.Rect(0, 0, 320, 200))
	draw.Draw(maskedImage, maskedImage.Bounds(), &image.Uniform{image.White}, image.ZP, draw.Src)

	// notifications from file system
	fsevents := ramdisk.NewFSEvents()

	// start webserver
	go func() {
		http.ListenAndServe("localhost:8080", http.HandlerFunc(webHandler))
	} ()

	// prepare receiving file change notifications
	// here, a pointer the latest closed file is maintained
	go func() {
		for {
			var event interface{}
			select {
			case event = <-fsevents.FileCreated:
				log.Printf("file create: %q", event.(ramdisk.EventFileCreated).File.Meta.Name())
			case event = <-fsevents.FileOpened:
			case event = <-fsevents.FileWritten:
			case event = <-fsevents.FileClosed:
				// as soon as file is closed, keep it in global variable
				file := event.(ramdisk.EventFileClosed)
				log.Printf("file closed: %q, size = %d", file.File.Meta.Name(), file.File.Meta.Size())
				latestMutex.Lock()
				latest = file.File
				latestMutex.Unlock()
			case event = <-fsevents.Unmount:
			}
		}
	} ()

	// mount ramdisk at "/mnt/fusemnt"
	// you can now copy files to it, like "cp mypic.jpg /mnt/fusemnt
	// it will appear as the
	ramdisk.MountAndServe("/mnt/myramdisk", &fsevents)
}

type circle struct {
	p image.Point
	r int
}

func (c *circle) ColorModel() color.Model {
	return color.AlphaModel
}

func (c *circle) Bounds() image.Rectangle {
	return image.Rect(c.p.X-c.r, c.p.Y-c.r, c.p.X+c.r, c.p.Y+c.r)
}

func (c *circle) At(x, y int) color.Color {
	xx, yy, rr := float64(x-c.p.X)+0.5, float64(y-c.p.Y)+0.5, float64(c.r)
	if xx*xx+yy*yy < rr*rr {
		return color.Alpha{255}
	}
	return color.Alpha{0}
}

func stampOutPicture(jpgData []byte) {
	img, err := jpeg.Decode(bytes.NewReader(jpgData))
	if err != nil {
		return
	}

	draw.DrawMask(maskedImage, maskedImage.Bounds(), img, image.ZP, &circle{image.Point{160, 100}, 100}, image.ZP, draw.Over)

	buffer := bytes.Buffer{}
	jpeg.Encode(&buffer, maskedImage, nil)
	latestAlt = buffer.Bytes()
}

