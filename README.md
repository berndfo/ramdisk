# ramdisk
RAM disk FUSE implementation in Go

A alpha-stage RAM disk implemented in Go.
The RAM disk can be mounted as a Linux file system in user space (FUSE), needing no elevated privileges.
Files can be created, read and written, but are not persisted to durable storage. Sufficient current must be flowing all the time.

## how to mount

```go

mount("/mnt/myfs")

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

	return nil
}
```

## how to unmount
```bash
# on the Linux shell
fusermount -u /mnt/fusemnt
```

or 

```go
// in Go code
fuse.Unmount(mountpoint)
```

TODO: how to track changes to FS