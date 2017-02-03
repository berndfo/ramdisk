# ramdisk
a RAM disk written in Go. a RAM disk is a file system where data is held only in RAM.

An alpha-stage RAM disk implemented in Go.
The RAM disk can be mounted as a Linux file system in user space (FUSE), needing no elevated privileges.
Files can be created, read and written, but are not persisted to durable storage. Sufficient current must be flowing all the time.

The Go process creating the RAM disk has direct in-process access to file data, represented by a byte slice.

## prepare

```bash
 git clone https://github.com/berndfo/ramdisk.git
 cd ramdisk
 export GOPATH=`pwd` # use backticks here!
 go get bazil.org/fuse
 go get golang.org/x/net
 
 go run src/main.go
```

## how to mount

Mounting a RAM disk is very simple. Just prepare the mount point (here: `/mnt/myramdisk`)
and run:
```go
	ramdisk.MountAndServe("/mnt/myramdisk", nil)
```

A mounted RAM disk can be accessed like any other file system on Linux (cd, cp, echo, cat, etc.).
No byte will ever hit any disk. All data is lost after terminating the process.

## how to track changes to FS

to act on changes in the in-process RAM disk, you can listen on a number of channels:

```go
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
```

in this example, every file creation and close operation is logged.
Please make sure to listen on all channels, but feel free to ignore any event you're not interested in.

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

## accessing file data in-process

assume that `latest` is holding a recently written JPG image:
`var latest *ramdisk.FileEntry // last closed file entry`
this file might have been written by `ffmpeg` or any another out-of-process application.
a web request can directly render this image to the response by copying data

```go
func webHandler(response http.ResponseWriter, request *http.Request) {
    response.Header().Add("Content-type", "image/jpg")
    response.Write(latest.Data)
}
```

for a running, detailed example see `src/ramdisk/webserver/main.go`


    