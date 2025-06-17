package rfs

import (
	"context"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)


type filesystem struct{}
// used for static api
type root struct{}
// used for resolving
type path struct {
	FullPath string
}

type fileHandle struct {
	Parent *File
	Inode uint64
	// needs to be pointer, so that it can be resized
	// because fileHandle is copied by value
	Contents *[]byte
}


var (
	protocolAPI ProtocolAPI
	rootDir = root{}
	confDir  = &Dir{
		Inode: 0,
		Name: ":c",
		Contents: []DirNode{
			&File{
				Inode: 1,
				Name: "flush",
				OnWrite: func(data []byte) error {
					flushAll()
					return nil
				},
				OnRead: func() ([]byte, error) {
					return []byte("write stuff to flush cache\n"), nil
				},
			},
		},
	}
)


func (filesystem) Root() (fs.Node, error) {
	return rootDir, nil
}


func (root) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Mode = os.ModeDir | 0o555
	return nil
}


func (root) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if name == ":c" {
		return confDir, nil
	}
	return newPath(name), nil
}


func (root) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	return []fuse.Dirent{confDir.GetDirEnt()}, nil
}
