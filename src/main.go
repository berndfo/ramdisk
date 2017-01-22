package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"ramdisk"
	"log"
)

func main() {
	mount("/mnt/fusemnt")
}


func mount(mountpoint string) error {
	c, err := fuse.Mount(mountpoint)
	if err != nil {
		log.Printf("failed to mount %q", mountpoint)
		return err
	}
	log.Printf("successfully mounted %q", mountpoint)

	defer c.Close()

	filesys := ramdisk.CreateRamFS()
	if err := fs.Serve(c, filesys); err != nil {
		log.Printf("failed to serve  a filesystem at mount %q", mountpoint)
		return err
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Printf("failure mounting a filesystem at mount %q", mountpoint)
		return err
	}

	//fuse.Unmount(mountpoint)

	return nil
}