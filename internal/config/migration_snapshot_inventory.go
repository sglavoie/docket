package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Snapshot prefixes for every boundary from schema 8 on. They live here, in one
// table, because two readers need the same spelling: the activation that
// publishes the archive/audit pair and the diagnostic that inspects it
// afterwards. A boundary whose prefix were spelled twice could drift into a
// snapshot the doctor cannot see.
//
// Schemas 5, 6, and 7 keep their own dedicated inspection functions: each one
// validates a structural conversion's specific artifacts, so their prefixes stay
// beside those checks.
const (
	preTagV8SnapshotPrefix             = "pre-tag-v8-"
	preStateClockV9SnapshotPrefix      = "pre-state-clock-v9-"
	preDeferRenameV10SnapshotPrefix    = "pre-defer-rename-v10-"
	preDeferInstantV11SnapshotPrefix   = "pre-defer-instant-v11-"
	preContextV12SnapshotPrefix        = "pre-context-v12-"
	preDeferCountV13SnapshotPrefix     = "pre-defer-count-v13-"
	preDueTimeV14SnapshotPrefix        = "pre-due-time-v14-"
	preDependenciesV15SnapshotPrefix   = "pre-dependencies-v15-"
	prePortableLensesV16SnapshotPrefix = "pre-portable-lenses-v16-"
	preStoredReviewV17SnapshotPrefix   = "pre-stored-review-v17-"
	preResumeNoteV18SnapshotPrefix     = "pre-resume-note-v18-"
)

// MigrationSnapshotBoundary is one schema boundary that publishes an immutable
// archive/audit pair, named by the schema it activates and the artifact prefix it
// writes.
type MigrationSnapshotBoundary struct {
	Schema int
	Prefix string
}

// MigrationSnapshotBoundaries returns every boundary from schema 8 through the
// current schema, in ascending order. Adding a boundary means adding a row here,
// which is what keeps the diagnostic's coverage advancing with the chain instead
// of freezing at the schema it was written for.
func MigrationSnapshotBoundaries() []MigrationSnapshotBoundary {
	return []MigrationSnapshotBoundary{
		{Schema: 8, Prefix: preTagV8SnapshotPrefix},
		{Schema: 9, Prefix: preStateClockV9SnapshotPrefix},
		{Schema: 10, Prefix: preDeferRenameV10SnapshotPrefix},
		{Schema: 11, Prefix: preDeferInstantV11SnapshotPrefix},
		{Schema: 12, Prefix: preContextV12SnapshotPrefix},
		{Schema: 13, Prefix: preDeferCountV13SnapshotPrefix},
		{Schema: 14, Prefix: preDueTimeV14SnapshotPrefix},
		{Schema: 15, Prefix: preDependenciesV15SnapshotPrefix},
		{Schema: 16, Prefix: prePortableLensesV16SnapshotPrefix},
		{Schema: 17, Prefix: preStoredReviewV17SnapshotPrefix},
		{Schema: 18, Prefix: preResumeNoteV18SnapshotPrefix},
	}
}

// MigrationSnapshotPair is one complete rollback archive and its audit sidecar.
type MigrationSnapshotPair struct {
	Schema      int
	CreatedAt   time.Time
	ArchivePath string
	AuditPath   string
}

// MigrationSnapshotInspection is the read-only view of the published snapshots
// for one boundary: the complete pairs, and whatever is malformed, empty, or
// half-published. Issues are defects a human resolves; nothing here is repaired,
// because a migration snapshot is immutable by contract.
type MigrationSnapshotInspection struct {
	Schema       int
	HasArtifacts bool
	Pairs        []MigrationSnapshotPair
	Issues       []string
}

// InspectMigrationSnapshots validates the published archive/audit pairs of every
// boundary from schema 8 on, returning one inspection per boundary in ascending
// schema order. It never modifies anything, and a missing backups directory is a
// clean result rather than an error: a workspace created at the current schema
// crossed no boundary and so published no snapshot.
func InspectMigrationSnapshots(dataDir string) ([]MigrationSnapshotInspection, error) {
	names, err := readSnapshotBackupNames(dataDir)
	if err != nil {
		return nil, err
	}
	boundaries := MigrationSnapshotBoundaries()
	out := make([]MigrationSnapshotInspection, 0, len(boundaries))
	for _, boundary := range boundaries {
		inspection, err := inspectBoundarySnapshots(dataDir, boundary, names)
		if err != nil {
			return nil, err
		}
		out = append(out, inspection)
	}
	return out, nil
}

// readSnapshotBackupNames lists the backups directory once, so inspecting ten
// boundaries reads the directory once rather than ten times.
func readSnapshotBackupNames(dataDir string) ([]string, error) {
	backupsDir := PreScopeV7BackupsDir(dataDir)
	info, err := os.Lstat(backupsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect migration backups directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("migration backups path is not a regular directory: %s", backupsDir)
	}
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return nil, fmt.Errorf("read migration backups directory: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

func inspectBoundarySnapshots(dataDir string, boundary MigrationSnapshotBoundary, names []string) (MigrationSnapshotInspection, error) {
	result := MigrationSnapshotInspection{Schema: boundary.Schema}
	backupsDir := PreScopeV7BackupsDir(dataDir)

	type pairState struct {
		createdAt   time.Time
		archivePath string
		auditPath   string
	}
	states := map[string]*pairState{}
	for _, name := range names {
		if !strings.HasPrefix(name, boundary.Prefix) {
			continue
		}
		// A lock file beside a canonical artifact is publication machinery, not an
		// artifact, exactly as the v5 through v7 inspections treat it.
		if strings.HasSuffix(name, ".lock") {
			continue
		}
		result.HasArtifacts = true
		path := filepath.Join(backupsDir, name)
		stem, isArchive, ok := splitSnapshotArtifactName(name)
		if !ok {
			result.Issues = append(result.Issues, fmt.Sprintf("malformed schema-%d snapshot artifact %q", boundary.Schema, name))
			continue
		}
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return result, fmt.Errorf("inspect schema-%d snapshot artifact %q: %w", boundary.Schema, name, statErr)
		}
		if !info.Mode().IsRegular() {
			result.Issues = append(result.Issues, fmt.Sprintf("schema-%d snapshot artifact %q is not a regular file", boundary.Schema, name))
		} else if info.Size() == 0 {
			result.Issues = append(result.Issues, fmt.Sprintf("schema-%d snapshot artifact %q is empty", boundary.Schema, name))
		}
		state := states[stem]
		if state == nil {
			createdAt, parseErr := time.Parse(preScopeV7Timestamp, strings.TrimPrefix(stem, boundary.Prefix))
			if parseErr != nil {
				result.Issues = append(result.Issues, fmt.Sprintf("malformed schema-%d snapshot artifact %q", boundary.Schema, name))
				continue
			}
			state = &pairState{createdAt: createdAt}
			states[stem] = state
		}
		if isArchive {
			state.archivePath = path
		} else {
			state.auditPath = path
		}
	}

	stems := make([]string, 0, len(states))
	for stem := range states {
		stems = append(stems, stem)
	}
	sort.Strings(stems)
	for _, stem := range stems {
		state := states[stem]
		switch {
		case state.archivePath == "":
			result.Issues = append(result.Issues, fmt.Sprintf("schema-%d snapshot %q is missing its archive", boundary.Schema, stem))
		case state.auditPath == "":
			result.Issues = append(result.Issues, fmt.Sprintf("schema-%d snapshot %q is missing its audit report", boundary.Schema, stem))
		default:
			result.Pairs = append(result.Pairs, MigrationSnapshotPair{
				Schema:      boundary.Schema,
				CreatedAt:   state.createdAt,
				ArchivePath: state.archivePath,
				AuditPath:   state.auditPath,
			})
		}
	}
	sort.Strings(result.Issues)
	return result, nil
}

// splitSnapshotArtifactName separates a canonical artifact name into its
// timestamped stem and whether it is the archive or the audit sidecar. The audit
// entry published inside the archive shares the prefix but carries no timestamp,
// so a name that is neither suffix is malformed rather than ignored.
func splitSnapshotArtifactName(name string) (stem string, isArchive bool, ok bool) {
	switch {
	case strings.HasSuffix(name, preScopeV7ArchiveSuffix):
		return strings.TrimSuffix(name, preScopeV7ArchiveSuffix), true, true
	case strings.HasSuffix(name, preScopeV7AuditSuffix):
		return strings.TrimSuffix(name, preScopeV7AuditSuffix), false, true
	default:
		return "", false, false
	}
}
