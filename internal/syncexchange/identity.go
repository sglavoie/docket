package syncexchange

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// checkAuthorSequences reads directory entries alone, including placeholders.
// A reused sequence is a fork, even when its number is below the received
// watermark. Refuse it before publishing or importing anything: choosing one
// of its manifests would hide the other working copy's changes.
func checkAuthorSequences(root string, outgoing ...string) error {
	type authorSequence struct {
		device string
		seq    uint64
	}
	seen := make(map[authorSequence]string)
	check := func(rel string) error {
		device, seq, txnID, ok := ParseManifestPath(rel)
		if !ok {
			return nil
		}
		key := authorSequence{device, seq}
		if previous, exists := seen[key]; exists && previous != txnID {
			return fmt.Errorf("sync identity %s reused sequence %d; preserve both working copies and repair their history before exchanging again", device, seq)
		}
		seen[key] = txnID
		return nil
	}
	for _, rel := range outgoing {
		if err := check(rel); err != nil {
			return err
		}
	}
	txns := filepath.Join(root, "txns")
	return filepath.WalkDir(txns, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == txns && os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			if cloudDataless(path) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if target, ok := placeholderTarget(rel); ok {
			rel = target
		}
		return check(rel)
	})
}
