package ramdisk

import (
	"testing"
	"bazil.org/fuse/fs/fstestutil"
	"os"
	"io/ioutil"
	"log"
	"io"
)

func init() {
	fstestutil.DebugByDefault()
}

func TestWriteOnce(t *testing.T) {
	mnt, mntErr := fstestutil.MountedT(t, CreateRamFS(), nil)
	defer mnt.Close()

	writer, createErr := os.Create(mnt.Dir + "/" + "a1.txt")
	writtenBytes, writeErr:= writer.WriteString("testtesttest")
	defer writer.Close()

	if (mntErr != nil || createErr != nil || writeErr != nil) {
		t.Error("mount or create or write failed.")
	}

	if (writtenBytes != 12) {
		t.Error("not 12 bytes written")
	}

	writer.Close()
	log.Print("file closed.")

	fileInfo, errStat := os.Stat(mnt.Dir + "/" + "a1.txt")
	if errStat != nil {
		t.Fatal("no stat on written file")
	}
	if fileInfo.Size() != 12 {
		t.Fatalf("stat reports wrong file size %d for file %q", fileInfo.Size(), fileInfo.Name())
	}

}

func TestWriteMultiple(t *testing.T) {
	mnt, mntErr := fstestutil.MountedT(t, CreateRamFS(), nil)
	defer mnt.Close()

	writer, createErr := os.Create(mnt.Dir + "/" + "a2.txt")
	defer writer.Close()

	_, writeErr1 := writer.WriteString("testtesttest")
	if (mntErr != nil || createErr != nil || writeErr1 != nil) {
		t.Fatal("first write failed")
	}

	writtenBytes, writeErr2 := writer.WriteString("aaaabbbb")
	if (writeErr2 != nil) {
		t.Fatal("second write failed")
	}

	if writtenBytes != 8 {
		t.Fatal("not written 8 bytes")
	}
	writer.Close()

	fileInfo, errStat := os.Stat(mnt.Dir + "/" + "a2.txt")
	if errStat != nil {
		t.Fatal("no stat on written file")
	}
	if fileInfo.Size() != (3*4 + 8) {
		t.Fatal("stat reports wrong file size", fileInfo.Size())
	}
}

func TestReadMultiwrite(t *testing.T) {
	mnt, mntErr := fstestutil.MountedT(t, CreateRamFS(), nil)
	defer mnt.Close()

	writer, createErr := os.Create(mnt.Dir + "/" + "a3.txt")
	defer writer.Close()

	_, writeErr1 := writer.WriteString("testtesttest")
	writtenBytes, writeErr2 := writer.WriteString("aaaabbbb")

	if (mntErr != nil || createErr != nil || writeErr1 != nil || writeErr2 != nil) {
		t.Fail()
	}

	writer.Close()

	_, errStat := os.Stat(mnt.Dir + "/" + "a3.txt")
	if errStat != nil {
		t.Fatal("no stat on written file")
	}

	reader, err := os.OpenFile(mnt.Dir + "/" + "a3.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatal("not opened, " + err.Error())
	}
	defer reader.Close()

	byts, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatal("not read")
	}

	bytsToString := string(byts)
	log.Printf("read: %q", bytsToString)
	if bytsToString != "testtesttestaaaabbbb" {
		t.Fail()
	}

	if (writtenBytes != 8) {
		t.Fail()
	}


}

func TestRandomRead(t *testing.T) {
	mnt, mntErr := fstestutil.MountedT(t, CreateRamFS(), nil)
	defer mnt.Close()

	writer, createErr := os.Create(mnt.Dir + "/" + "a4.txt")
	defer writer.Close()

	_, writeErr1 := writer.WriteString("testabctest")

	if (mntErr != nil || createErr != nil || writeErr1 != nil) {
		t.Fail()
	}

	writer.Close()

	reader, err := os.OpenFile(mnt.Dir + "/" + "a4.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatal("not opened, " + err.Error())
	}
	defer reader.Close()

	threeBytes := make([]byte, 3)
	readCount, errRead := reader.ReadAt(threeBytes, 4)
	if errRead != nil {
		t.Fatal("not read")
	}
	if readCount != 3 {
		t.Fatalf("instad of 3, read %d", readCount)
	}

	bytsToString := string(threeBytes)
	log.Printf("read: %q", bytsToString)
	if bytsToString != "abc" {
		t.Fail()
	}
}

func TestRandomReadIncomplete(t *testing.T) {
	mnt, mntErr := fstestutil.MountedT(t, CreateRamFS(), nil)
	defer mnt.Close()

	writer, createErr := os.Create(mnt.Dir + "/" + "a5.txt")
	defer writer.Close()

	_, writeErr1 := writer.WriteString("testtestab")

	if mntErr != nil || createErr != nil || writeErr1 != nil {
		t.Fail()
	}

	writer.Close()

	reader, err := os.OpenFile(mnt.Dir + "/" + "a5.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatal("not opened, " + err.Error())
	}
	defer reader.Close()

	threeBytes := make([]byte, 3)
	readCount, errRead := reader.ReadAt(threeBytes, 8) // only 2 bytes left in file
	if errRead != io.EOF {
		t.Fatal("not EOF", errRead.Error())
	}
	if readCount != 2 {
		t.Fatalf("instad of 3, read %d", readCount)
	}

	bytsToString := string(threeBytes[:readCount])
	log.Printf("read: %q", bytsToString)
	if bytsToString != "ab" {
		t.Fail()
	}
}

func TestRandomSeek(t *testing.T) {
	mnt, mntErr := fstestutil.MountedT(t, CreateRamFS(), nil)
	defer mnt.Close()

	writer, createErr := os.Create(mnt.Dir + "/" + "a6.txt")
	defer writer.Close()

	_, writeErr1 := writer.WriteString("testabatesttesttbabesttesttesttestcbctest")

	if (mntErr != nil || createErr != nil || writeErr1 != nil) {
		t.Fail()
	}

	writer.Close()

	reader, err := os.OpenFile(mnt.Dir + "/" + "a6.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatal("not opened, " + err.Error())
	}
	defer reader.Close()

	threeBytes := make([]byte, 3)

	reader.Seek(4, 0) // seek from start
	_, _ = reader.Read(threeBytes)
	if "aba" != string(threeBytes) {
		t.Fatal("not seeked to pos 4")
	}

	reader.Seek(16, 0)
	_, _ = reader.Read(threeBytes)
	if "bab" != string(threeBytes) {
		t.Fatal("not seeked to pos 16")
	}

	threeBytes = []byte("___") // neutralizes

	reader.Seek(-7, 2) // seek 7 backwards from end
	actuallyRead, seekErr := reader.Read(threeBytes)
	if seekErr != nil {
		t.Fatal(seekErr.Error())
	}
	if "cbc" != string(threeBytes) {
		t.Fatalf("not seeked to pos 16: %d %q", actuallyRead, threeBytes)
	}

	reader.Seek(10, 0) // seek from start...
	reader.Seek(6, 1) // ... then seek relative
	_, _ = reader.Read(threeBytes)
	if "bab" != string(threeBytes) {
		t.Fatal("not seeked to pos 10+6")
	}
}
