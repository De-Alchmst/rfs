package rfs

import (
	"path/filepath"
	"context"
	"strings"
	"time"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fuseutil"
)


// used for resolving
type path struct {
	FullPath string
}

type pathHandle struct {
	Parent *pathEntry
	// needs to be pointer, so that it can be resized
	// because fileHandle is copied by value
	Contents *[]byte
	Writing bool
	Name string
}

type pathEntry struct {
	Contents []byte
	TTL int64
	Status entryStatus
	Err error
}

type pidKey struct {
	PID uint32
	Name string
}

type entryStatus int

const (
	entryStatusOK entryStatus = iota
	entryStatusFailed
	entryStatusProcessing
)


var (
	entries = map[string]*pathEntry{}
	pidEntries = map[pidKey]*pathEntry{}
)


func withContext(ctx context.Context, req fuse.Request) context.Context {
	return context.WithValue(ctx, "PID", req.Hdr().Pid)
}


func (p path) Attr(ctx context.Context, a *fuse.Attr) error {
	entry, ok := getEntry(p.FullPath, ctx.Value("PID").(uint32), false)
	// File
	if ok { 
		err := processStatus(entry)
		if err != nil {
			return err
		}

		a.Inode = 1
		a.Mode = os.ModeIrregular | 0o770
		a.Size = uint64(len(entry.Contents))

		entry.TTL = DefaultTTL

	// Directory
	} else {
		a.Inode = 1
		a.Mode = os.ModeDir | 0o777
	}

	return nil
}


func (path) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	return []fuse.Dirent{}, nil
}


func (p path) Lookup(ctx context.Context, name string) (fs.Node, error) {
	return newPath(filepath.Join(p.FullPath, name)), nil
}


func (p path) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	entry, ok := getEntry(p.FullPath, ctx.Value("PID").(uint32), true)
	if !ok {
		return nil, fuse.ENOENT
	}

	err := processStatus(entry)
	if err != nil {
		return nil, err
	}

	original := []byte{}
	// Truncate sometimes
	if !((req.Flags&fuse.OpenWriteOnly != 0 && req.Flags&fuse.OpenAppend == 0) || req.Flags&fuse.OpenTruncate != 0) {
		original = entry.Contents
	}

	var contents []byte
	if req.Flags.IsReadOnly() {
		contents = original
	} else {
		contents = make([]byte, len(original))
		copy(contents, original)
	}

	entry.TTL = DefaultTTL

	return pathHandle{
		Parent:   entry,
		Name :    p.FullPath,
		Contents: &contents,
		Writing:  !req.Flags.IsReadOnly(),
	}, nil
}


func (h pathHandle) ReadAll(ctx context.Context) ([]byte, error) {
	h.Parent.TTL = DefaultTTL
	return *h.Contents, nil
}


func (h pathHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	fuseutil.HandleRead(req, resp, *h.Contents)
	h.Parent.TTL = DefaultTTL
	return nil
}


func (h pathHandle) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	fileLen := len(*h.Contents)
	reqLen  := len(req.Data)
	spaceDelta := reqLen - fileLen + int(req.Offset) 
	
	if spaceDelta > 0 {
		oldContents := *h.Contents
		*h.Contents = make([]byte, fileLen + spaceDelta)
		copy(*h.Contents, oldContents)
	}

	copy((*h.Contents)[req.Offset:], req.Data)
	resp.Size = len(req.Data)
	return nil
}


func (h pathHandle) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	if h.Writing && len(*h.Contents) != 0 {
		// no need to wait here til communication finishes...
		entry := createPIDEntry(h.Name, ctx.Value("PID").(uint32))

		go func() {
			name, mods := processPath(h.Name)
			resp, err := protocolAPI.Write(name, mods, *h.Contents)
			if err != nil {
				entry.Status = entryStatusFailed
				entry.Err = err
			} else {
				entry.Status = entryStatusOK
				entry.Contents = resp
			}
		}()
	}
	return nil
}


func getEntry(name string, pid uint32, removePid bool) (*pathEntry, bool) {
	var (
		entry *pathEntry
		ok  bool
		key pidKey
	)

	key = pidKey{PID: pid, Name: name}

	entry, ok = pidEntries[key]
	if !ok {
		entry, ok = entries[name]
		if !ok {
			return nil, false
		}

	} else if removePid {
		delete(pidEntries, key)
	}

	return entry, true
}


func processStatus(entry *pathEntry) error {
	for entry.Status == entryStatusProcessing {
		time.Sleep(10 * time.Millisecond)
	}

	if entry.Status == entryStatusFailed {
		return entry.Err
	}

	return nil
}


func newPath(name string) path {
	if len(name) > 1 && name[len(name)-1] == ':' {
		handleEntry(name)
	}

	return path{FullPath: name}
}


func handleEntry(name string) {
	entry, ok := entries[name]

	if !ok {
		entries[name] = &pathEntry{
			Contents: []byte{},
			TTL: DefaultTTL,
			Status: entryStatusProcessing,
		}

		go fillEntry(entries[name], name)

	} else {
		entry.TTL = DefaultTTL
	}
}


func createPIDEntry(name string, pid uint32) *pathEntry {
	entry := &pathEntry {
		Contents: []byte{},
		Status: entryStatusProcessing,
		TTL: DefaultTTL,
	}
	pidEntries[pidKey{PID: pid, Name: name}] = entry
	return entry
}


func fillEntry(entry *pathEntry, name string) {
	data, err := protocolAPI.Read(processPath(name))
	if err != nil {
		entry.Status = entryStatusFailed
		entry.Err = err
	} else {
		entry.Status = entryStatusOK
	}

	entry.Contents = data
}


func processPath(path string) (string, []string) {
	mods := []string{}
	for path[0] == ':' {
		before, after, found := strings.Cut(path, "/")
		if !found {
			break
		}

		// no need to send :nop further
		if before != ":nop" {
			mods = append(mods, before)
		}
		path = after
	}

	return path[:len(path)-1], mods
}
