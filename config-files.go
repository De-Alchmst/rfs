package rfs

import (
	"context"
	"os"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fuseutil"
)


// FUSE stuff...
func (d Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = d.Inode
	a.Mode = os.ModeDir | 0o777
	return nil
}


// FUSE stuff...
func (d Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	for _, ent := range d.Contents {
		if ent.GetName() == name {
			return ent, nil
		}
	}

	return nil, syscall.ENOENT
}


// FUSE stuff...
func (d Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	dirents := make([]fuse.Dirent, len(d.Contents))
	for i, ent := range d.Contents {
		dirents[i] = ent.GetDirEnt()
	}

	return dirents, nil
}


// FUSE stuff...
func (d Dir) GetDirEnt() fuse.Dirent {
	return fuse.Dirent{
		Inode: d.Inode,
		Name:  d.Name,
		Type:  fuse.DT_Dir,
	}
}


// FUSE stuff...
func (d Dir) GetName() string {
	return d.Name
}


// FUSE stuff...
func (f File) GetDirEnt() fuse.Dirent {
	return fuse.Dirent{
		Inode: f.Inode,
		Name:  f.Name,
		Type:  fuse.DT_File,
	}
}


// FUSE stuff...
func (f File) GetName() string {
	return f.Name
}


// FUSE stuff...
func (f File) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 2
	a.Mode = 0o666

	contents, err := f.OnRead()
	var length uint64
	if err == nil {
		length = uint64(len(contents))
	} else {
		length = 0
	}

	a.Size = length
	return nil
}


// FUSE stuff...
func (f File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	original := []byte{}

	// Truncate sometimes
	if !((req.Flags&fuse.OpenWriteOnly != 0 && req.Flags&fuse.OpenAppend == 0) || req.Flags&fuse.OpenTruncate != 0) {
		original, _ = f.OnRead()
	}

	var contents []byte
	if req.Flags.IsReadOnly() {
		contents = original
	} else {
		contents = make([]byte, len(original))
		copy(contents, original)
	}

	return fileHandle{
		Parent:	  &f,
		Contents: &contents,
		Writing:  !req.Flags.IsReadOnly(),
	}, nil
}


// FUSE stuff...
func (h fileHandle) ReadAll(ctx context.Context) ([]byte, error) {
	return *h.Contents, nil
}


// FUSE stuff...
func (h fileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	fuseutil.HandleRead(req, resp, *h.Contents)
	return nil
}


// FUSE stuff...
func (h fileHandle) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
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


// FUSE stuff...
func (h fileHandle) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	if h.Writing && len(*h.Contents) != 0 {
		h.Parent.OnWrite(*h.Contents)
	}
	return nil
}
