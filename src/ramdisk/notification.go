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
type EventFileWritten struct {
	FSEvent
}
type EventFileClosed struct {
	FSEvent
}

type FSEvents struct {
	FileCreated chan EventFileCreated
	FileOpened chan EventFileOpened
	FileWritten chan EventFileWritten
	FileClosed chan EventFileClosed
	Unmount chan bool
}

func NewFSEvents() (fsevents FSEvents) {
	fsevents = FSEvents{
		FileCreated: make(chan EventFileCreated),
		FileOpened: make(chan EventFileOpened),
		FileWritten: make(chan EventFileWritten),
		FileClosed: make(chan EventFileClosed),
		Unmount: make(chan bool),
	}

	go func(fsevents FSEvents) {
		for {
			select {
			case <-fsevents.FileCreated:
			case <-fsevents.FileOpened:
			case <-fsevents.FileWritten:
			case <-fsevents.FileClosed:
			case <-fsevents.Unmount:
				return
			}
		}
		return
	} (fsevents)

	return
}
