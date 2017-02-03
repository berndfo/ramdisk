package main

import (
	"ramdisk"
	"log"
)

func main() {

	fsevents := ramdisk.NewFSEvents()

	go func() {
		for {
			var event interface{}
			select {
			case event = <-fsevents.FileCreated:
				log.Printf("file create: %q", event.(ramdisk.EventFileCreated).File.Meta.Name())
			case event = <-fsevents.FileOpened:
			case event = <-fsevents.FileWritten:
			case event = <-fsevents.FileClosed:
				file := event.(ramdisk.EventFileClosed)
				log.Printf("file closed: %q, size = %d", file.File.Meta.Name(), file.File.Meta.Size())
			case event = <-fsevents.Unmount:
			}
		}
	} ()

	ramdisk.MountAndServe("/mnt/myramdisk", &fsevents)
}

