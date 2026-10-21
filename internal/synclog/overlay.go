package synclog

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/sglavoie/tt/internal/transaction"
)

// vfile is one file as a transaction under construction sees it: the on-disk
// bytes until an earlier operation in the same transaction replaced, moved, or
// removed them.
type vfile struct {
	exists bool
	digest string
	data   []byte
}

// overlay is the workspace as it will stand after the operations planned so
// far, read through to disk for every path they have not touched. It also
// collects the blob writes that retain every digest an effect names.
type overlay struct {
	h       *History
	files   map[string]*vfile
	planned map[string]bool
	blobOps []transaction.Op
	// keepIdentical makes a rewrite of identical bytes an effect.
	keepIdentical bool
}

func newOverlay(h *History) *overlay {
	return &overlay{h: h, files: map[string]*vfile{}, planned: map[string]bool{}}
}

// lookup returns the file at abs and whether abs is a directory on disk.
func (o *overlay) lookup(abs string) (*vfile, bool, error) {
	if f, ok := o.files[abs]; ok {
		return f, false, nil
	}
	info, err := os.Stat(abs)
	if isNotExist(err) {
		return &vfile{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.IsDir() {
		return nil, true, nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, false, fmt.Errorf("reading %s for sync history: %w", abs, err)
	}
	f := &vfile{exists: true, digest: Digest(data), data: data}
	o.files[abs] = f
	return f, false, nil
}

func (o *overlay) set(abs string, f *vfile) { o.files[abs] = f }

// filesUnder lists every file a move of source carries: source itself when it
// is a file, or each file below it when it is a directory, as the overlay sees
// them.
func (o *overlay) filesUnder(source string) ([]string, error) {
	f, isDir, err := o.lookup(source)
	if err != nil {
		return nil, err
	}
	if !isDir && f.exists {
		return []string{source}, nil
	}
	seen := map[string]bool{}
	var out []string
	if isDir {
		walkErr := filepath.WalkDir(source, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			seen[p] = true
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("listing %s for sync history: %w", source, walkErr)
		}
	}
	for p := range o.files {
		if hasPathPrefix(p, source) && p != source {
			seen[p] = true
		}
	}
	for p := range seen {
		file, dir, err := o.lookup(p)
		if err != nil {
			return nil, err
		}
		if !dir && file.exists {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

// retain stages f's bytes in the blob store unless they are already there.
func (o *overlay) retain(f *vfile) {
	o.retainBytes(f.digest, f.data)
}

func (o *overlay) retainBytes(digest string, data []byte) {
	if o.planned[digest] {
		return
	}
	o.planned[digest] = true
	path := o.h.BlobPath(digest)
	if _, err := os.Lstat(path); err == nil {
		return
	}
	o.blobOps = append(o.blobOps, writeOp(path, data))
}
