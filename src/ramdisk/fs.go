package ramdisk

import (
	"bazil.org/fuse/fs"
	"bazil.org/fuse"
	"os"
	"golang.org/x/net/context"
	"sync/atomic"
	"syscall"
	"time"
	"bazil.org/fuse/fuseutil"
	"sync"
)

var atomicInode uint64 = 1

func CreateRamFS() *ramdiskFS {
	filesys := &ramdiskFS{
		Events: NewFSEvents(),
	}
	return filesys
}

func nextInode() uint64 {
	return atomic.AddUint64(&atomicInode, 1)
}

// implements FSInodeGenerator
type ramdiskFS struct {
	Events FSEvents
}

func (f ramdiskFS) Root() (fs.Node, error) {
	return &Dir{fs: &f}, nil
}

func (f ramdiskFS) GenerateInode(parentInode uint64, name string) uint64 {
	return nextInode()
}

type Dir struct {
	mutex sync.RWMutex
	fs *ramdiskFS
}

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	entry, found := findEntryByName(name)
	if !found {
		return nil, fuse.ENOENT
	}
	return &entry.file, nil
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Mode = os.ModeDir | 0555
	return nil
}

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	entries := make([]fuse.Dirent, 0)
	for _, entry := range rootEntries {
		entries = append(entries, entry.dirEntry)
	}
	return entries, nil
}

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	requestedName := req.Name
	if requestedName == "" {
		// no file has no name
		return nil, nil, fuse.EPERM
	}

	_, alreadyExits := findEntryByName(requestedName)
	if alreadyExits {
		// already exists
		return nil, nil, fuse.EPERM
	}

	newEntry := createFileEntry(requestedName, d.fs)

	d.mutex.Lock()
	rootEntries = append(rootEntries, newEntry)
	d.mutex.Unlock()

	handle := Handle{inode: newEntry.file.inode}

	d.fs.Events.FileCreated<-EventFileCreated{FSEvent{File: newEntry}}

	return &newEntry.file, handle, nil
}

// implements fs.Node
type RamFile struct {
	fuse    *fs.Server
	inode   uint64
	name string
	size   uint64
	created time.Time
	modified time.Time
	writable bool
}

func (f *RamFile) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = f.inode
	if f.writable {
		a.Mode = 0666
	} else {
		a.Mode = 0555
	}
	a.Size = f.size
	a.Ctime = f.created
	a.Mtime = f.modified
	return nil
}

func (f *RamFile) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	if !f.writable && !req.Flags.IsReadOnly() {
		return nil, fuse.Errno(syscall.EACCES)
	}
	resp.Flags |= fuse.OpenDirectIO

	entry, found := findEntryByInode(f.inode)
	if !found {
		return nil, fuse.Errno(syscall.ENOENT)
	}

	handle := Handle{inode: f.inode}

	entry.fs.Events.FileOpened<-EventFileOpened{FSEvent{File: entry}}

	return handle, nil
}

// implements fs.Handle, fs.HandleWriter, fs.HandleReader
type Handle struct {
	inode   uint64
}

func (h Handle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	entry, found := findEntryByInode(h.inode)
	if !found {
		return fuse.Errno(syscall.ENOENT)
	}

	fuseutil.HandleRead(req, resp, entry.data)

	return nil
}

func (h Handle) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	//log.Printf("try to write %s", req.ID)
	//n, err := w.buf.Write(req.Data)
	newBytes := req.Data

	inode := h.inode

	entry, found := findEntryByInode(inode)
	if !found {
		return fuse.Errno(syscall.ENOENT)
	}

	currentDataLength := len(entry.data)
	offsetPos := int(req.Offset)
	if (offsetPos == currentDataLength) {
		// new data is added at the end
		entry.data = append(entry.data, newBytes...)
	} else if (offsetPos < currentDataLength) {
		// data is partially overwritten
		endPos := int(offsetPos) + len(newBytes)
		if (endPos > currentDataLength) {
			missingBytes := endPos - currentDataLength
			// extend slice by missing byte count
			entry.data = append(entry.data, make([]byte, missingBytes)...)
		}
		copy(entry.data[offsetPos:endPos], newBytes[:])
	} else {
		// offset is beyond last byte
		newEndPos := int(offsetPos) + len(newBytes)
		missingBytes := newEndPos - currentDataLength
		entry.data = append(entry.data, make([]byte, missingBytes)...)
		copy(entry.data[offsetPos:newEndPos], newBytes[:])
	}
	entry.file.size = uint64(len(entry.data))

	entry.file.modified = time.Now()
	resp.Size = len(newBytes)
	//log.Printf("write: added: %d, new total: %d", resp.Size, entry.file.size)

	entry.fs.Events.FileWritten<-EventFileWritten{}

	return nil
}

func (h Handle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	inode := h.inode

	entry, found := findEntryByInode(inode)
	if !found {
		return fuse.Errno(syscall.ENOENT)
	}
	entry.fs.Events.FileClosed<-EventFileClosed{}

	return nil
}


var rootEntries = []*FileEntry{
}

func findEntryByName(name string) (*FileEntry, bool) {
	for _, fileEntry := range rootEntries {
		if fileEntry.dirEntry.Name == name {
			return fileEntry, true
		}
	}
	return nil, false
}

func findEntryByInode(inode uint64) (*FileEntry, bool) {
	for _, fileEntry := range rootEntries {
		if fileEntry.dirEntry.Inode == inode {
			return fileEntry, true
		}
	}
	return nil, false
}

type FileEntry struct {
	fs *ramdiskFS
	dirEntry fuse.Dirent
	file     RamFile
	data     []byte
}

func createFileEntry(name string, fs *ramdiskFS) (entry *FileEntry) {
	inode := nextInode()
	emptyContent := make([]byte, 0)
	entry = &FileEntry{
		fs: fs,
		dirEntry: fuse.Dirent{Inode:inode, Name: name, Type: fuse.DT_File},
		file: RamFile{inode: inode, name: name, writable: true},
		data: emptyContent,
	}
	return
}





