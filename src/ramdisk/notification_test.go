package ramdisk

import (
	"testing"
	"bazil.org/fuse/fs/fstestutil"
	"os"
	"time"
)

func TestNotification(t *testing.T) {
	fs := CreateRamFS()

	mnt, _ := fstestutil.MountedT(t, fs, nil)
	defer mnt.Close()

	notification := NewFSEvents()
	fs.AddListener(&notification)

	writer, _ := os.Create(mnt.Dir + "/" + "b1.txt")
	defer writer.Close()

	select {
	case <-notification.FileCreated:
		// success
	case <-time.After(1*time.Minute):
		t.Fatal("missing FileCreated")
	}

	writer.Write([]byte("test"))
	select {
	case <-notification.FileWritten:
	// success
	case <-time.After(1*time.Minute):
		t.Fatal("missing FileWritten")
	}

	writer.Seek(0, 0)
	writer.Read(make([]byte, 4))
	select {
	case <-notification.FileRead:
	// success
	case <-time.After(1*time.Minute):
		t.Fatal("missing FileRead")
	}

	writer.Close()
	select {
	case <-notification.FileClosed:
	// success
	case <-time.After(1*time.Minute):
		t.Fatal("missing FileClosed")
	}
}

