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


type entry struct {
	Contents []byte
	TTL int64
	Status int
	Using int
	Err error
}


const (
	entryStatusOK = iota
	entryStatusFailed
	entryStatusProcessing
)


var (
	DefaultTTL int64 = 60 * 60
	entries = map[string]*entry{}
)


func (p path) Attr(ctx context.Context, a *fuse.Attr) error {
	ent, ok := entries[p.FullPath]
	// File
	if ok { 

		ent.Using += 1
		for ent.Status == entryStatusProcessing {
			time.Sleep(10 * time.Millisecond)
		}

		if ent.Status == entryStatusFailed {
			ent.Using -= 1
			return ent.Err
		}

		a.Inode = 1
		a.Mode = os.ModeIrregular | 0o770
		a.Size = uint64(len(ent.Contents))

		ent.TTL = DefaultTTL
		ent.Using -= 1

	// Directory
	} else {
		a.Inode = 1
		a.Mode = os.ModeDir | 0o777
	}

	return nil
}


func (p path) ReadAll(ctx context.Context) ([]byte, error) {
	ent, ok := entries[p.FullPath]
	if !ok {
		return nil, fuse.ENOENT
	}

	ent.Using += 1
	for ent.Status == entryStatusProcessing {
		time.Sleep(10 * time.Millisecond)
	}

	ent.TTL = DefaultTTL
	ent.Using -= 1
	if ent.Status == entryStatusOK {
		return ent.Contents, nil
	}
	return nil, ent.Err
}


func (p path) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	ent, ok := entries[p.FullPath]
	if !ok {
		return fuse.ENOENT
	}

	ent.Using += 1
	for ent.Status == entryStatusProcessing {
		time.Sleep(10 * time.Millisecond)
	}

	if ent.Status == entryStatusFailed {
		return ent.Err
	}

	fuseutil.HandleRead(req, resp, ent.Contents)

	ent.TTL = DefaultTTL
	ent.Using -= 1

	return nil
}


func (path) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	return []fuse.Dirent{}, nil
}


func (p path) Lookup(ctx context.Context, name string) (fs.Node, error) {
	return newPath(filepath.Join(p.FullPath, name)), nil
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
		entries[name] = &entry{
			Contents: []byte{},
			TTL: DefaultTTL,
			Status: entryStatusProcessing,
			Using: 0,
		}

		go fillEntry(entries[name], name)

	} else {
		ent.TTL = DefaultTTL
	}
}


func fillEntry(ent *entry, name string) {
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
