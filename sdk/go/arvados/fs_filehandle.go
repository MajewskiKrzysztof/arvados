// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package arvados

import (
	"context"
	"io"
	"io/fs"
	"os"
)

type filehandle struct {
	inode
	ptr        filenodePtr
	append     bool
	readable   bool
	writable   bool
	unreaddirs []os.FileInfo
}

func (f *filehandle) Read(p []byte) (n int, err error) {
	if !f.readable {
		return 0, ErrWriteOnlyMode
	}
	f.inode.RLock()
	defer f.inode.RUnlock()
	n, f.ptr, err = f.inode.Read(p, f.ptr)
	return
}

func (f *filehandle) Seek(off int64, whence int) (pos int64, err error) {
	size := f.inode.Size()
	ptr := f.ptr
	switch whence {
	case io.SeekStart:
		ptr.off = off
	case io.SeekCurrent:
		ptr.off += off
	case io.SeekEnd:
		ptr.off = size + off
	}
	if ptr.off < 0 {
		return f.ptr.off, ErrNegativeOffset
	}
	if ptr.off != f.ptr.off {
		f.ptr = ptr
		// force filenode to recompute f.ptr fields on next
		// use
		f.ptr.repacked = -1
	}
	return f.ptr.off, nil
}

func (f *filehandle) Truncate(size int64) error {
	return f.inode.Truncate(size)
}

func (f *filehandle) Write(p []byte) (n int, err error) {
	if !f.writable {
		return 0, ErrReadOnlyFile
	}
	f.inode.Lock()
	defer f.inode.Unlock()
	if fn, ok := f.inode.(*filenode); ok && f.append {
		f.ptr = filenodePtr{
			off:        fn.fileinfo.size,
			segmentIdx: len(fn.segments),
			segmentOff: 0,
			repacked:   fn.repacked,
		}
	}
	n, f.ptr, err = f.inode.Write(p, f.ptr)
	return
}

func (f *filehandle) Repack(ctx context.Context, opts RepackOptions) (int, error) {
	dn, ok := f.inode.(*dirnode)
	if !ok {
		return 0, ErrNotADirectory
	}
	dn.Lock()
	defer dn.Unlock()
	return dn.fs.repackTree(ctx, opts, dn)
}

// dirEntry implements fs.DirEntry, see (*filehandle)ReadDir().
type dirEntry struct {
	os.FileInfo
}

func (ent dirEntry) Type() fs.FileMode {
	return ent.Mode().Type()
}
func (ent dirEntry) Info() (fs.FileInfo, error) {
	return ent, nil
}

// ReadDir implements fs.ReadDirFile.
func (f *filehandle) ReadDir(count int) ([]fs.DirEntry, error) {
	fis, err := f.Readdir(count)
	if len(fis) == 0 {
		return nil, err
	}
	ents := make([]fs.DirEntry, len(fis))
	for i, fi := range fis {
		ents[i] = dirEntry{fi}
	}
	return ents, err
}

func (f *filehandle) Readdir(count int) ([]os.FileInfo, error) {
	if !f.inode.IsDir() {
		return nil, ErrInvalidOperation
	}
	if count <= 0 {
		return f.inode.Readdir()
	}
	if f.unreaddirs == nil {
		var err error
		f.unreaddirs, err = f.inode.Readdir()
		if err != nil {
			return nil, err
		}
	}
	if len(f.unreaddirs) == 0 {
		return nil, io.EOF
	}
	if count > len(f.unreaddirs) {
		count = len(f.unreaddirs)
	}
	ret := f.unreaddirs[:count]
	f.unreaddirs = f.unreaddirs[count:]
	return ret, nil
}

func (f *filehandle) Stat() (os.FileInfo, error) {
	return f.inode.FileInfo(), nil
}

func (f *filehandle) Close() error {
	return nil
}

func (f *filehandle) Sync() error {
	// Sync the containing filesystem.
	return f.FS().Sync()
}

func (f *filehandle) Snapshot() (*Subtree, error) {
	if !f.readable {
		return nil, ErrInvalidOperation
	}
	node, err := f.inode.Snapshot()
	return &Subtree{inode: node}, err
}

func (f *filehandle) Splice(r *Subtree) error {
	if !f.writable {
		return ErrReadOnlyFile
	}
	return f.inode.Splice(r.inode)
}
