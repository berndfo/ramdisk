package ramdisk

type FSEvent struct {
	File *FileEntry
}

type EventFileCreated struct {
	FSEvent
}
type EventFileOpened struct {
	FSEvent
}
type EventFileRead struct {
	FSEvent
}
type EventFileWritten struct {
	FSEvent
}
type EventFileClosed struct {
	FSEvent
}

type FSEvents struct {
	FileCreated chan EventFileCreated
	FileOpened  chan EventFileOpened
	FileRead    chan EventFileRead
	FileWritten chan EventFileWritten
	FileClosed  chan EventFileClosed
	Unmount     chan bool
}

func NewFSEvents() (fsevents FSEvents) {
	fsevents = FSEvents{
		FileCreated: make(chan EventFileCreated),
		FileOpened: make(chan EventFileOpened),
		FileRead: make(chan EventFileRead),
		FileWritten: make(chan EventFileWritten),
		FileClosed: make(chan EventFileClosed),
		Unmount: make(chan bool),
	}
	return
}
