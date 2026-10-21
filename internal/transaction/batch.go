package transaction

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type fileBatch interface {
	MkdirAll(string, os.FileMode) error
	WriteFile(string, []byte, os.FileMode) error
	SyncDir(string) error
	Flush() error
}

// WithBatchedWrites groups durability waits for independent writes. Every
// staged file is durable before the prepared manifest, and every applied
// rename is durable before the committed manifest. Moves, deletes and writes
// to overlapping paths retain the ordinary per-operation apply barriers.
// Recovery needs no format flag: it can replay the same journal serially.
func (j *Journal) WithBatchedWrites() *Journal {
	j.batchWrites = true
	return j
}

func onlyWrites(ops []Op) bool {
	for _, op := range ops {
		if op.Kind != OpWrite {
			return false
		}
	}
	return true
}

// Independent files may share a durability barrier without depending on the
// persistence order of their renames. Resolve parents (which staging created)
// so symlink aliases cannot hide two writes to the same physical target.
func independentWrites(ops []FileOp) bool {
	parents := map[string]string{}
	paths := make([]string, 0, len(ops)*2)
	for _, op := range ops {
		if op.Kind != OpWrite {
			return false
		}
		for _, path := range []string{op.Target, op.Staged} {
			if path == "" {
				return false
			}
			parent := filepath.Dir(path)
			resolved, ok := parents[parent]
			if !ok {
				var err error
				resolved, err = filepath.EvalSymlinks(parent)
				if err != nil {
					return false
				}
				parents[parent] = resolved
			}
			physical := filepath.Join(resolved, filepath.Base(path))
			// Case-insensitive filesystems can alias names even without
			// symlinks. Fold ASCII conservatively on every platform; Unicode
			// filesystem normalization keeps the ordinary apply barriers.
			for _, r := range physical {
				if r > 127 {
					return false
				}
			}
			paths = append(paths, strings.ToLower(physical))
		}
	}
	sort.Strings(paths)
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			return false
		}
		for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
			if seen[parent] {
				return false
			}
			if filepath.Dir(parent) == parent {
				break
			}
		}
		seen[path] = true
	}
	return true
}
