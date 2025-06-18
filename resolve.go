package rfs

import (
	"fmt"
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
	Status int
	Err error
}


const (
	entryStatusOK = iota
	entryStatusFailed
	entryStatusProcessing
)


var (
	entries = map[string]*pathEntry{}
)


func (p path) Attr(ctx context.Context, a *fuse.Attr) error {
	ent, ok := entries[p.FullPath]
	// File
	if ok { 

		err := processStatus(ent)
		if err != nil {
			return err
		}

		a.Inode = 1
		a.Mode = os.ModeIrregular | 0o770
		a.Size = uint64(len(ent.Contents))

		ent.TTL = DefaultTTL

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
	ent, ok := entries[p.FullPath]
	if !ok {
		return nil, fuse.ENOENT
	}

	err := processStatus(ent)
	if err != nil {
		return nil, err
	}

	original := []byte{}
	// Truncate sometimes
	if !((req.Flags&fuse.OpenWriteOnly != 0 && req.Flags&fuse.OpenAppend == 0) || req.Flags&fuse.OpenTruncate != 0) {
		original = ent.Contents
	}

	var contents []byte
	if req.Flags.IsReadOnly() {
		contents = original
	} else {
		contents = make([]byte, len(original))
		copy(contents, original)
	}

	ent.TTL = DefaultTTL

	return pathHandle{
		Parent:   ent,
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
		name, mods := processPath(h.Name)
		resp, err := protocolAPI.Write(name, mods, *h.Contents)
		fmt.Println(string(resp))
		if err != nil {
			return err
		}
	}
	return nil
}


func processStatus(ent *pathEntry) error {
	for ent.Status == entryStatusProcessing {
		time.Sleep(10 * time.Millisecond)
	}

	if ent.Status == entryStatusFailed {
		return ent.Err
	}

	return nil
}


func newPath(name string) path {
	if name[len(name)-1] == ':' {
		handleEntry(name)
	}

	return path{FullPath: name}
}


func handleEntry(name string) {
	ent, ok := entries[name]

	if !ok {
		entries[name] = &pathEntry{
			Contents: []byte{},
			TTL: DefaultTTL,
			Status: entryStatusProcessing,
		}

		go fillEntry(entries[name], name)

	} else {
		ent.TTL = DefaultTTL
	}
}


func fillEntry(ent *pathEntry, name string) {
	data, err := protocolAPI.Read(processPath(name))
	if err != nil {
		ent.Status = entryStatusFailed
		ent.Err = err
	} else {
		ent.Status = entryStatusOK
	}

	ent.Contents = data
}


func processPath(path string) (string, []string) {
	mods := []string{}
	for path[0] == ':' {
		splits := strings.SplitN(path, "/", 2)
		if len(splits) == 1 {
			break
		}

		mods = append(mods, splits[0])
		path = splits[1]
	}

	return path[:len(path)-1], mods
}
