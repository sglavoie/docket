//go:build !darwin

package util

import "os"

// Other platforms keep their ordinary per-file durability guarantee and
// coalesce only repeated syncs of the same directory.
func syncBatchFile(f *os.File) error { return SyncFile(f) }

func syncBatchDirectories(dirs []string) error {
	for _, dir := range dirs {
		if err := SyncDir(dir); err != nil {
			return err
		}
	}
	return nil
}
