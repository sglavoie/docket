package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/transaction"
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
