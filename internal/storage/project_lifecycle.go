package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/docket/internal/transaction"
)

// RenamePlan carries the journaled operation that moves a project's data directory
// to its new slug-derived location, plus the reconstructed destination path. The
// caller appends its own tasks.md heading rewrite, config rewrite, and
// cross-reference repair ops to the returned Ops before committing the whole set
// atomically. Ops is empty when the slug is unchanged or the source directory does
// not yet exist (a project with no persisted tasks.md), in which case only NewPath
// is meaningful — the caller still commits its config/tasks writes against it.
type RenamePlan struct {
	Ops     []transaction.Op
	NewPath string
}

// PlanProjectRename validates a project's slug change and returns the journal
// operation that renames <data>/projects/<oldSlug> to <data>/projects/<newSlug>.
// It performs no filesystem mutation: the returned move op is committed by the
// caller together with the tasks.md heading rewrite, the config rewrite, and any
// cross-project reference repairs, so the whole rename is one atomic transaction.
// It fails closed on empty or unsafe slugs and refuses to overwrite an existing
// destination directory, mirroring the eager RenameProjectDir it replaces. When
// the source directory does not exist (a project whose tasks.md has never been
// written) it returns no move op and only the retargeted NewPath.
func PlanProjectRename(dataDir, oldSlug, newSlug string) (RenamePlan, error) {
	if oldSlug == "" || newSlug == "" {
		return RenamePlan{}, fmt.Errorf("project slug cannot be empty (old=%q, new=%q)", oldSlug, newSlug)
	}
	projectsDir := filepath.Join(dataDir, "projects")
	newPath, err := SafeJoin(projectsDir, newSlug)
	if err != nil {
		return RenamePlan{}, fmt.Errorf("invalid project slug %q: %w", newSlug, err)
	}
	if oldSlug == newSlug {
		return RenamePlan{NewPath: newPath}, nil
	}
	oldPath, err := SafeJoin(projectsDir, oldSlug)
	if err != nil {
		return RenamePlan{}, fmt.Errorf("invalid project slug %q: %w", oldSlug, err)
	}
	if _, statErr := os.Stat(oldPath); os.IsNotExist(statErr) {
		return RenamePlan{NewPath: newPath}, nil
	}
	if _, statErr := os.Stat(newPath); statErr == nil {
		return RenamePlan{}, fmt.Errorf("directory '%s' already exists", newSlug)
	}
	return RenamePlan{
		Ops:     []transaction.Op{{Kind: transaction.OpMove, Source: oldPath, Target: newPath}},
		NewPath: newPath,
	}, nil
}

// PlanProjectDelete validates a project slug and returns the journal operations
// that remove <data>/projects/<slug> in full — tasks.md, notes, and attachments
// together. It performs no filesystem mutation: the caller appends its own
// catalog and planning rewrites and commits the set atomically, so removing a
// project is one transaction whose byte images the workspace undo ledger
// captures beforehand. It fails closed on an empty or unsafe slug, and returns
// no op when the directory does not exist, so a project whose tasks.md was never
// written still deletes cleanly from the catalog.
//
// The tree is enumerated into one op per path rather than a single op on the
// directory because transaction.OpDelete is os.Remove: it removes one entry and
// refuses a non-empty directory. Enumerating keeps the delete inside the
// existing primitive instead of widening every delete in the workspace into a
// recursive one.
func PlanProjectDelete(dataDir, slug string) ([]transaction.Op, error) {
	if slug == "" {
		return nil, fmt.Errorf("project slug cannot be empty")
	}
	path, err := SafeJoin(filepath.Join(dataDir, "projects"), slug)
	if err != nil {
		return nil, fmt.Errorf("invalid project slug %q: %w", slug, err)
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return nil, nil
	}
	return planTreeDeleteOps(path)
}

// planTreeDeleteOps orders one delete per path so that no directory is removed
// before its contents. WalkDir is pre-order and lexical, so reversing its visit
// order is exactly post-order: deterministic, and children always precede the
// parent the journal applies next.
func planTreeDeleteOps(root string) ([]transaction.Op, error) {
	var paths []string
	if err := filepath.WalkDir(root, func(path string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("enumerating project directory %q: %w", root, err)
	}
	ops := make([]transaction.Op, 0, len(paths))
	for index := len(paths) - 1; index >= 0; index-- {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: paths[index]})
	}
	return ops, nil
}
