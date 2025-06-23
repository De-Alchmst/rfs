package rfs

import (
	"os"
	"os/signal"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)


type Dir struct{
	Inode uint64
	Name string

	Contents []DirNode
}

type File struct{
	Inode uint64
	Name string

	OnWrite func([]byte) error
	OnRead  func() ([]byte, error)
}

type DirNode interface {
	fs.Node
	GetDirEnt() fuse.Dirent
	GetName() string
}


type ProtocolAPI interface {
	Read(address string, modifiers []string) ([]byte, error)
	Write(address string, modifiers []string, data []byte) ([]byte, error)
	FlushAll()
	FlushResource(address string, modifiers []string)
}


func MountFS(mountpoint, fsName, fsSubtype string, confs []DirNode, api ProtocolAPI) error {
	protocolAPI = api
	for _, node := range confs {
		confDir.Contents = append(confDir.Contents, node)
	}

	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName(fsName),
		fuse.Subtype(fsSubtype),
	)

	if err != nil {
		return err
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	// Start serving in a goroutine
	serveDone := make(chan error, 1)
	go func() {
		server := fs.New(c, &fs.Config{
			Debug: fuse.Debug, // do nothing
			WithContext: withContext, // store PID in context
		})
		serveDone <- server.Serve(filesystem{})
	}()

	// start cache flushing in a goroutine
	go cacheFlushing()

	select {
		case <-serveDone:
		case <-sigChan:
	}
	
	err = fuse.Unmount(mountpoint)
	c.Close()

	return nil
}
