# ramdisk
RAM disk FUSE implementation in Go

A alpha-stage RAM disk implemented in Go.
The RAM disk can be mounted as a Linux file system in user space (FUSE), needing no elevated privileges.
Files can be created, read and written, but are not persisted to durable storage. Sufficient current must be flowing all the time.

TODO: how to mount.

TODO: how to unmount.
