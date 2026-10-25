package tui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// syncedTree is one file-sync service's view of a workspace: relative path to
// bytes, with no directories of its own. A real sync copies files, not a live
// filesystem, which is why hazards are reproduced by capturing a tree and
// replaying it somewhere else rather than by sharing a directory.
type syncedTree map[string][]byte

// syncSnapshot captures everything under root the way a sync client uploads a
// workspace: every regular file, keyed by its slash-separated path relative to
// root. Symlinks and irregular entries are skipped, because a sync service does
// not reproduce them either.
func syncSnapshot(root string) (syncedTree, error) {
	return syncSnapshotExcept(root, nil)
}

// syncSnapshotExcept is syncSnapshot with an omission predicate, which is how a
// partially-transferred sync is expressed: the files skip returns true for never
// made it to the other side.
func syncSnapshotExcept(root string, skip func(rel string) bool) (syncedTree, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("sync snapshot needs a root directory")
	}
	tree := make(syncedTree)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return fmt.Errorf("relativize %s: %w", path, relErr)
		}
		rel = filepath.ToSlash(rel)
		if skip != nil && skip(rel) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", rel, readErr)
		}
		tree[rel] = data
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("snapshot workspace %s: %w", root, err)
	}
	return tree, nil
}

// syncTransactionsPrefix is the synced transaction directory, relative to the
// workspace root. It is the one part of a workspace whose partial arrival is
// recoverable rather than corrupt, which is why it gets its own helper.
const syncTransactionsPrefix = ".docket/transactions/"

// syncSnapshotPartialTransactions captures a workspace mid-transfer: everything
// arrived except the tail of the transaction directory. This is the shape a sync
// service actually leaves behind when it is interrupted — a workspace whose
// durable files are present and whose in-flight transaction is only half there —
// and it is the state crash recovery has to survive on the receiving machine.
//
// The tail is chosen deterministically (the last entry in path order) so a
// failure reproduces rather than flaking. A workspace with no transaction files
// yields the same tree syncSnapshot would.
func syncSnapshotPartialTransactions(root string) (syncedTree, error) {
	full, err := syncSnapshot(root)
	if err != nil {
		return nil, err
	}
	transactionFiles := make([]string, 0, len(full))
	for rel := range full {
		if strings.HasPrefix(rel, syncTransactionsPrefix) {
			transactionFiles = append(transactionFiles, rel)
		}
	}
	if len(transactionFiles) == 0 {
		return full, nil
	}
	sort.Strings(transactionFiles)
	delete(full, transactionFiles[len(transactionFiles)-1])
	return full, nil
}

// syncApply writes a captured tree into root the way a sync client downloads it:
// missing parents are created and existing files are overwritten. Files already
// under root that the tree does not carry are left alone, because a sync that
// delivered nothing about a path says nothing about it either.
func syncApply(root string, tree syncedTree) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("sync apply needs a root directory")
	}
	paths := make([]string, 0, len(tree))
	for rel := range tree {
		paths = append(paths, rel)
	}
	// Deterministic order so an interrupted apply in a future hazard test fails
	// the same way twice.
	sort.Strings(paths)
	for _, rel := range paths {
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", rel, err)
		}
		if err := os.WriteFile(target, tree[rel], 0o600); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
	}
	return nil
}
