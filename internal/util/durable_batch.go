package util

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// DurableBatch groups durability barriers for private transaction staging.
// Writes are NOT durable until Flush succeeds. Callers must flush before
// publishing a journal that references the files, and again after applying
// independent renames before marking that journal committed. It is not safe
// for concurrent use. Ordinary standalone writes use AtomicWriteFile instead.
type DurableBatch struct {
	dirs map[string]bool
}

func NewDurableBatch() *DurableBatch { return &DurableBatch{dirs: map[string]bool{}} }

// WriteFile performs the same checked atomic replacement as AtomicWriteFile,
// but groups its final durability barrier with the other writes in the batch.
func (b *DurableBatch) WriteFile(path string, data []byte, mode os.FileMode) error {
	return atomicWriteFile(path, data, mode, syncBatchFile, b.SyncDir)
}

// MkdirAll also remembers the parents of newly created directories: syncing
// just the leaf directory cannot make the entries naming its ancestors durable.
func (b *DurableBatch) MkdirAll(dir string, mode os.FileMode) error {
	var parents []string
	for path := filepath.Clean(dir); ; path = filepath.Dir(path) {
		_, err := os.Stat(path)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(path)
		parents = append(parents, parent)
		if parent == path {
			break
		}
	}
	if err := os.MkdirAll(dir, mode); err != nil {
		return err
	}
	for _, parent := range parents {
		b.dirs[parent] = true
	}
	return nil
}

// SyncDir queues directory metadata for the next durability barrier.
func (b *DurableBatch) SyncDir(dir string) error {
	b.dirs[filepath.Clean(dir)] = true
	return nil
}

// Flush makes every queued file and directory durable. A failed flush keeps
// the batch pending; no caller may publish its prepared/committed marker.
func (b *DurableBatch) Flush() error {
	dirs := make([]string, 0, len(b.dirs))
	for dir := range b.dirs {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	if err := syncBatchDirectories(dirs); err != nil {
		return fmt.Errorf("flushing transaction files: %w", err)
	}
	clear(b.dirs)
	return nil
}
