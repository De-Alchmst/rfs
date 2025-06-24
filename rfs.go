// Package rfs provides a simple way to implement resolving file systems with FUSE.
// See: https://github.com/De-Alchmst/resolving-file-system-spec/
//
// for example programs using rfs, see:
// 	- https://github.com/De-Alchmst/webrfs
// 	- https://github.com/De-Alchmst/gopherrfs
package rfs

import (
	"os"
	"os/signal"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// Dir is a structure used for adding custom directories into the '/:c/' and '/:config' directories.
type Dir struct{
	Inode uint64 // 0 to assign random value
	Name string

	Contents []DirNode
}

// File is a structure used for adding custom files into  the '/:c/' and '/:config' directories.
type File struct{
	Inode uint64 // 0 to assign random value
	Name string

	OnWrite func([]byte) error
	OnRead  func() ([]byte, error)
}

// DirNode is used for contents of '/:c/' and '/:config'.
// See File and Dir.
type DirNode interface {
	fs.Node
	GetDirEnt() fuse.Dirent
	GetName() string
}

// ProtocolAPI is used to export protocol-specific functoinality into the file system.
type ProtocolAPI interface {
	Read(address string, modifiers []string) ([]byte, error)
	Write(address string, modifiers []string, data []byte) ([]byte, error)

	// In case you implement separate caching
	FlushAll()
	FlushResource(address string, modifiers []string)
}


// MountFS is used to start the filesystem server.
// Mountpoint is an existing directory ot which filesystem will be mounted.
// FsName and fsSubtype are taken by FUSE and don't matter all that much.
// Confs contains extra files and directories to be included in '/:c/' and '/:config/'.
// Api contains all the functions implementing given protocol.
func MountFS(mountpoint, fsName, fsSubtype string, confs []DirNode, api ProtocolAPI) error {
	protocolAPI = api
	for _, node := range confs {
		configDir.Contents = append(configDir.Contents, node)
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
