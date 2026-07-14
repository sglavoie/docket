package storage

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/transaction"
	"github.com/sglavoie/tt/internal/tterrors"
)

// This file plans project archive-location, trash, restore, and purge as sets of
// contained transaction operations that the caller commits atomically through
// CommitService.CommitOps. Planning is deliberately execution-free: it validates
// identity and destinations, allocates trash entry identity, and returns the raw
// journal ops (a directory move, a tombstone write, a recursive delete) without
// touching the filesystem, so the whole choreography — directory move, tombstone,
// config rewrite, and cross-reference repairs — commits as one journaled
// transaction or not at all.

// TrashPlan carries the journaled operations that move a project's directory into
// a new versioned trash entry and write its tombstone, plus the allocated entry ID
// and the reconstructed destination path. The caller appends its own config and
// cross-reference write ops to Ops before committing the whole set atomically.
type TrashPlan struct {
	Ops     []transaction.Op
	EntryID string
	// EntryPath is the trash entry directory the move and tombstone target.
	EntryPath string
}

// PlanTrash validates a project's identity and allocates a fresh versioned trash
// entry, returning the journal operations that move the project's data directory
// into <trash>/<entryID>/data and write the entry's tombstone.json. It performs no
// filesystem mutation beyond reserving the entry directory name (allocateTrashEntry
// only stats candidates); the returned ops are committed by the caller together
// with the config rewrite so the whole delete is one atomic transaction. The
// tombstone captures the project metadata, original path, deletion reason, and the
// opaque caller-supplied configuration-reference snapshot for a faithful restore.
func PlanTrash(project *models.Project, dataDir, reason string, configRefs json.RawMessage) (TrashPlan, error) {
	if project == nil {
		return TrashPlan{}, fmt.Errorf("project cannot be nil")
	}
	if project.ID == "" {
		return TrashPlan{}, fmt.Errorf("project ID cannot be empty")
	}
	// The slug derives the restore destination path, so guard it (and the ID, which
	// keys the entry directory) against traversal or separators before either
	// reaches the filesystem.
	if err := SafeChildName(project.Slug); err != nil {
		return TrashPlan{}, fmt.Errorf("invalid project slug %q: %w", project.Slug, err)
	}
	if err := SafeChildName(project.ID); err != nil {
		return TrashPlan{}, fmt.Errorf("invalid project id %q: %w", project.ID, err)
	}

	deletedAt := time.Now()
	trashDir := getTrashDir(dataDir)
	entryID, entryPath, err := allocateTrashEntry(trashDir, project.ID, deletedAt)
	if err != nil {
		return TrashPlan{}, err
	}

	dataPath, err := SafeJoin(entryPath, trashDataDirName)
	if err != nil {
		return TrashPlan{}, err
	}
	tombPath, err := SafeJoin(entryPath, trashTombstoneFile)
	if err != nil {
		return TrashPlan{}, err
	}

	tomb := &TrashTombstone{
		SchemaVersion:  trashTombstoneSchemaVersion,
		EntryID:        entryID,
		Project:        project,
		OriginalPath:   project.Path,
		DeletedAt:      deletedAt,
		DeletionReason: reason,
		ConfigRefs:     configRefs,
	}
	tombData, err := marshalTombstone(tomb)
	if err != nil {
		return TrashPlan{}, err
	}

	ops := []transaction.Op{
		{Kind: transaction.OpMove, Source: project.Path, Target: dataPath},
		{Kind: transaction.OpWrite, Target: tombPath, Data: tombData, Mode: FilePermissions},
	}
	return TrashPlan{Ops: ops, EntryID: entryID, EntryPath: entryPath}, nil
}

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

// RestorePlan carries the journaled operations that move a trash entry's data back
// to its project directory and remove the drained entry, plus the faithfully
// reconstructed project (from the tombstone) and the tombstone itself so the caller
// can re-establish configuration references. Task data is reloaded from disk by the
// caller AFTER the commit places the files, mirroring RestoreTrashEntry.
type RestorePlan struct {
	Ops         []transaction.Op
	Project     *models.Project
	Tombstone   *TrashTombstone
	ProjectPath string
}

// PlanRestore resolves a trash entry and returns the journal operations that move
// its data back to <data>/projects/<slug>, delete its tombstone, and remove the
// now-empty entry directory. It fails closed when the entry is missing, when the
// restored slug is unsafe, or when an active project already occupies the
// destination. Legacy pre-versioned entries (the entry directory itself holds the
// project files, with no tombstone) are handled by moving the entry directory
// directly and synthesizing minimal metadata, so an old-layout deletion is not
// stranded. The returned Project carries the reconstructed identity with Path set
// to the destination; the caller reloads its computed task fields after the commit.
func PlanRestore(dataDir, entryID string) (RestorePlan, error) {
	if entryID == "" {
		return RestorePlan{}, fmt.Errorf("trash entry id cannot be empty")
	}
	trashDir := getTrashDir(dataDir)
	entryPath, err := SafeJoin(trashDir, entryID)
	if err != nil {
		return RestorePlan{}, fmt.Errorf("invalid trash entry id %q: %w", entryID, err)
	}
	if _, statErr := os.Stat(entryPath); os.IsNotExist(statErr) {
		return RestorePlan{}, fmt.Errorf("%w: '%s' not in trash", tterrors.ErrProjectNotFound, entryID)
	}

	tomb, dataSource, err := loadTrashEntryForRestore(entryPath, entryID)
	if err != nil {
		return RestorePlan{}, err
	}

	slug := tomb.Project.Slug
	if nameErr := SafeChildName(slug); nameErr != nil {
		return RestorePlan{}, fmt.Errorf("invalid restored project slug %q: %w", slug, nameErr)
	}
	projectPath, err := SafeJoin(filepath.Join(dataDir, "projects"), slug)
	if err != nil {
		return RestorePlan{}, fmt.Errorf("invalid restored project slug %q: %w", slug, err)
	}
	if _, statErr := os.Stat(projectPath); statErr == nil {
		return RestorePlan{}, fmt.Errorf("%w: '%s' already exists; rename or delete it first", tterrors.ErrProjectExists, slug)
	}

	ops := []transaction.Op{
		{Kind: transaction.OpMove, Source: dataSource, Target: projectPath},
	}
	// For a versioned entry the data lived under <entry>/data, leaving the tombstone
	// and the now-empty entry directory to remove. A legacy entry IS the moved
	// directory, so there is nothing left to clean.
	if dataSource != entryPath {
		tombPath, joinErr := SafeJoin(entryPath, trashTombstoneFile)
		if joinErr != nil {
			return RestorePlan{}, joinErr
		}
		ops = append(ops,
			transaction.Op{Kind: transaction.OpDelete, Target: tombPath},
			transaction.Op{Kind: transaction.OpDelete, Target: entryPath},
		)
	}

	project := tomb.Project
	project.Path = projectPath
	return RestorePlan{Ops: ops, Project: project, Tombstone: tomb, ProjectPath: projectPath}, nil
}

// ReloadRestoredProject loads a just-restored project's task data into its computed
// fields (Tasks, sections, tags, NextShortID) without disturbing its config-level
// identity, mirroring the reload RestoreTrashEntry performed inline. The caller runs
// it after the restore transaction commits, once the project files are in place.
func ReloadRestoredProject(project *models.Project) error {
	return reloadProjectComputedFields(project)
}

// PlanPurge returns the journal operations that permanently and recursively delete
// a trash entry. It enumerates every file and directory under the entry and emits a
// contained OpDelete for each — files first, then directories deepest-first — so the
// removal rides the same journal that guards every other mutation and never bypasses
// its root-containment checks. A missing entry yields no operations (a no-op purge),
// matching PurgeTrashEntry.
func PlanPurge(dataDir, entryID string) ([]transaction.Op, error) {
	if entryID == "" {
		return nil, fmt.Errorf("trash entry id cannot be empty")
	}
	entryPath, err := SafeJoin(getTrashDir(dataDir), entryID)
	if err != nil {
		return nil, fmt.Errorf("invalid trash entry id %q: %w", entryID, err)
	}
	if _, statErr := os.Stat(entryPath); os.IsNotExist(statErr) {
		return nil, nil
	}
	return planRecursiveDelete(entryPath)
}

// planRecursiveDelete walks root and returns OpDelete operations that remove every
// file and directory it contains, ordered so no non-empty directory is removed
// before its contents: all files first, then directories sorted by descending path
// length (a child path is always longer than its parent, so children precede
// parents). The journal's delete primitive is a non-recursive os.Remove, so this
// ordering is what makes a recursive removal safe through the journal.
func planRecursiveDelete(root string) ([]transaction.Op, error) {
	var files, dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			dirs = append(dirs, path)
		} else {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("enumerating trash entry for purge: %w", err)
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })

	ops := make([]transaction.Op, 0, len(files)+len(dirs))
	for _, f := range files {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: f})
	}
	for _, dir := range dirs {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: dir})
	}
	return ops, nil
}
