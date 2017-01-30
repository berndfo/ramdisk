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
	"runtime"
	"log"
)

var atomicInode uint64 = 1

func CreateRamFS() *ramdiskFS {
	filesys := &ramdiskFS{
		backendEvents: NewFSEvents(),
		addListenerChan: make(chan *FSEvents),
	}

	eventQueueMutex := sync.Mutex{}
	eventQueue := make([]interface{}, 0)

	// fetch backend events, and queue them. decoupling from listeners
	go func(fsevents FSEvents) {
		// TODO go routine termination
		for {
			var event interface{}
			select {
			case event = <-fsevents.FileCreated:
			case event = <-fsevents.FileOpened:
			case event = <-fsevents.FileRead:
			case event = <-fsevents.FileWritten:
			case event = <-fsevents.FileClosed:
			case event = <-fsevents.Unmount:
			}
			_ = event
			eventQueueMutex.Lock()
			eventQueue = append(eventQueue, event)
			eventQueueMutex.Unlock()
		}
		return
	} (filesys.backendEvents)

	// propagate queued events to listeners
	go func() {
		// TODO go routine termination
		listenerEvents := make([]*FSEvents, 0)

		for {
			select {
			case newListener:=<-filesys.addListenerChan:
				listenerEvents = append(listenerEvents, newListener)
			default:
				// fall through
			}

			var event interface{}

			eventQueueMutex.Lock()
			if len(eventQueue) > 0 {
				event = eventQueue[0]
				eventQueue = eventQueue[1:]
			}
			eventQueueMutex.Unlock()

			if event != nil {
				// this part relies on cooperation of listeners
				for _, listener := range listenerEvents {
					switch event.(type) {
					case EventFileCreated:
						listener.FileCreated <- event.(EventFileCreated)
					case EventFileOpened:
						listener.FileOpened <- event.(EventFileOpened)
					case EventFileWritten:
						listener.FileWritten <- event.(EventFileWritten)
					case EventFileRead:
						listener.FileRead <- event.(EventFileRead)
					case EventFileClosed:
						listener.FileClosed <- event.(EventFileClosed)
					case bool:
						listener.Unmount <- event.(bool)
					default:
						log.Panicf("unknown and unhandled FS event %T", event)
					}
				}
			} else {
				// be a good go citizen
				runtime.Gosched()
			}
		}
	}()

	return filesys
}

func MountAndServe(mountpoint string, optionalListener *FSEvents) error {
	c, err := fuse.Mount(mountpoint)
	if err != nil {
		log.Printf("failed to MountAndServe %q", mountpoint)
		return err
	}
	log.Printf("successfully mounted %q", mountpoint)

	defer c.Close()

	filesys := CreateRamFS()

	if optionalListener != nil {
		filesys.AddListener(optionalListener)
	}

	if err := fs.Serve(c, filesys); err != nil {
		log.Printf("failed to serve  a filesystem at MountAndServe %q", mountpoint)
		return err
	}

	// check if the MountAndServe process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Printf("failure mounting a filesystem at MountAndServe %q", mountpoint)
		return err
	}

	//fuse.Unmount(mountpoint)

	return nil
}
func nextInode() uint64 {
	return atomic.AddUint64(&atomicInode, 1)
}

// implements FSInodeGenerator
type ramdiskFS struct {
	backendEvents FSEvents
	addListenerChan chan *FSEvents
}

func (f *ramdiskFS) Root() (fs.Node, error) {
	return &Dir{fs: f}, nil
}

func (f *ramdiskFS) GenerateInode(parentInode uint64, name string) uint64 {
	return nextInode()
}

func (f *ramdiskFS) AddListener(newListener *FSEvents) {
	f.addListenerChan <- newListener
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
	return &entry.Meta, nil
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

	handle := Handle{inode: newEntry.Meta.inode}

	d.fs.backendEvents.FileCreated<-EventFileCreated{FSEvent{File: newEntry}}

	return &newEntry.Meta, handle, nil
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

	entry.fs.backendEvents.FileOpened<-EventFileOpened{FSEvent{File: entry}}

	return handle, nil
}

func (f *RamFile) Inode() uint64 {
	return f.inode
}

func (f *RamFile) Name() string {
	return f.name
}

func (f *RamFile) Size() uint64 {
	return f.size
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

	fuseutil.HandleRead(req, resp, entry.Data)

	entry.fs.backendEvents.FileRead <-EventFileRead{FSEvent{File: entry}}

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

	currentDataLength := len(entry.Data)
	offsetPos := int(req.Offset)
	if (offsetPos == currentDataLength) {
		// new data is added at the end
		entry.Data = append(entry.Data, newBytes...)
	} else if (offsetPos < currentDataLength) {
		// data is partially overwritten
		endPos := int(offsetPos) + len(newBytes)
		if (endPos > currentDataLength) {
			missingBytes := endPos - currentDataLength
			// extend slice by missing byte count
			entry.Data = append(entry.Data, make([]byte, missingBytes)...)
		}
		copy(entry.Data[offsetPos:endPos], newBytes[:])
	} else {
		// offset is beyond last byte
		newEndPos := int(offsetPos) + len(newBytes)
		missingBytes := newEndPos - currentDataLength
		entry.Data = append(entry.Data, make([]byte, missingBytes)...)
		copy(entry.Data[offsetPos:newEndPos], newBytes[:])
	}
	entry.Meta.size = uint64(len(entry.Data))

	entry.Meta.modified = time.Now()
	resp.Size = len(newBytes)
	//log.Printf("write: added: %d, new total: %d", resp.Size, entry.Meta.size)

	entry.fs.backendEvents.FileWritten<-EventFileWritten{FSEvent{File: entry}}

	return nil
}

func (h Handle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	inode := h.inode

	entry, found := findEntryByInode(inode)
	if !found {
		return fuse.Errno(syscall.ENOENT)
	}
	entry.fs.backendEvents.FileClosed<-EventFileClosed{FSEvent{File: entry}}

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
	fs       *ramdiskFS
	dirEntry fuse.Dirent
	Meta     RamFile
	Data     []byte
}

func createFileEntry(name string, fs *ramdiskFS) (entry *FileEntry) {
	inode := nextInode()
	emptyContent := make([]byte, 0)
	entry = &FileEntry{
		fs: fs,
		dirEntry: fuse.Dirent{Inode:inode, Name: name, Type: fuse.DT_File},
		Meta: RamFile{inode: inode, name: name, writable: true},
		Data: emptyContent,
	}
	return
}





