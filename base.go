package rfs

import (
	"context"
	"strings"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)


type filesystem struct{}
// used for static api
type root struct{}

type fileHandle struct {
	Parent *File
	// needs to be pointer, so that it can be resized
	// because fileHandle is copied by value
	Contents *[]byte
	Writing bool
}


var (
	protocolAPI ProtocolAPI
	rootDir = root{}

	confContents = []DirNode{
		&File{
			Inode: 0,
			Name: "flush",
			OnWrite: func(data []byte) error {
				command := strings.TrimRight(string(data), "\r\n")
				if command == ":all" || command == ":a" {
					flushAll()
					protocolAPI.FlushAll()
				} else {
					flushName(command)
					protocolAPI.FlushResource(processPath(command))
				}
				return nil
			},
			OnRead: func() ([]byte, error) {
				return []byte("write stuff to flush cache\n"), nil
			},
		},
	}


	cDir  = &Dir{
		Inode: 0,
		Name: ":c",
		Contents: confContents,
	}

	configDir  = &Dir{
		Inode: 0,
		Name: ":config",
		Contents: confContents,
	}
)


func (filesystem) Root() (fs.Node, error) {
	return rootDir, nil
}


func (root) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 0
	a.Mode = os.ModeDir | 0o555
	return nil
}


func (root) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if name == ":c" || name == ":config"{
		return configDir, nil
	}
	return newPath(name), nil
}


func (root) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	return []fuse.Dirent{cDir.GetDirEnt(), configDir.GetDirEnt()}, nil
}
