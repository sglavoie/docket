package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	preSimplificationV5SnapshotPrefix = "pre-simplification-v5-"
	preSimplificationV5Timestamp      = "20060102T150405Z"
	preSimplificationV5ArchiveSuffix  = ".tar.gz"
	preSimplificationV5AuditSuffix    = ".audit.md"
	preSimplificationV5AuditEntry     = "pre-simplification-v5-audit.md"

	// Snapshot artifacts are application-owned rollback sources. Keeping them
	// owner-readable but not writable makes accidental ordinary-file replacement
	// fail even outside the higher-level immutable-path guard below.
	preSimplificationV5Permissions fs.FileMode = 0o400
)

// ErrImmutablePreSimplificationV5Snapshot classifies an application request to
// overwrite or delete a published pre-simplification snapshot artifact.
var ErrImmutablePreSimplificationV5Snapshot = errors.New("config: pre-simplification v5 snapshots are immutable")

// PreSimplificationV5SnapshotFile is one byte-exact source recorded by a
// pre-simplification snapshot. Path is the portable path used in the archive
// and audit report rather than an absolute machine path.
type PreSimplificationV5SnapshotFile struct {
	Path   string
	Kind   string
	Size   int64
	SHA256 string
}

// PreSimplificationV5Snapshot describes one atomically published rollback
// snapshot and its attachment inventory.
type PreSimplificationV5Snapshot struct {
	CreatedAt     time.Time
	ArchivePath   string
	AuditPath     string
	ArchiveSHA256 string
	ArchivedFiles []PreSimplificationV5SnapshotFile
	Attachments   []PreSimplificationV5SnapshotFile
}

// PreSimplificationV5SnapshotPair is one canonical rollback archive and its
// matching human-readable migration audit.
type PreSimplificationV5SnapshotPair struct {
	CreatedAt   time.Time
	ArchivePath string
	AuditPath   string
}

// PreSimplificationV5SnapshotInspection is the read-only result used by
// diagnostics. HasArtifacts becomes true for any direct backup-directory entry
// that looks application-owned by prefix, including malformed or orphaned
// names. Pairs contains only matching canonical archive/audit names.
type PreSimplificationV5SnapshotInspection struct {
	HasArtifacts bool
	Pairs        []PreSimplificationV5SnapshotPair
	Issues       []string
}

type preSimplificationV5SnapshotCallbacks interface {
	snapshotCreatedAt() time.Time
	didInventorySnapshot()
	willPublishSnapshot(archivePath, auditPath string)
}

type preSimplificationV5Source struct {
	PreSimplificationV5SnapshotFile
	sourcePath string
	data       []byte
}

type preSimplificationV5Inventory struct {
	archived    []preSimplificationV5Source
	attachments []preSimplificationV5Source
}

// PreSimplificationV5BackupsDir returns the application-owned directory that
// holds immutable v5 rollback snapshots.
func PreSimplificationV5BackupsDir(dataDir string) string {
	return filepath.Join(InternalDir(filepath.Clean(dataDir)), "backups")
}

// IsPreSimplificationV5SnapshotPath reports whether path names one direct,
// timestamped archive or audit artifact in the workspace's protected backup
// directory. It recognizes only the canonical naming grammar.
func IsPreSimplificationV5SnapshotPath(dataDir, path string) bool {
	backupsDir := filepath.Clean(PreSimplificationV5BackupsDir(dataDir))
	candidate := filepath.Clean(path)
	if filepath.Dir(candidate) != backupsDir {
		return false
	}
	name := filepath.Base(candidate)
	if !strings.HasPrefix(name, preSimplificationV5SnapshotPrefix) {
		return false
	}
	stampAndSuffix := strings.TrimPrefix(name, preSimplificationV5SnapshotPrefix)
	var stamp string
	switch {
	case strings.HasSuffix(stampAndSuffix, preSimplificationV5ArchiveSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preSimplificationV5ArchiveSuffix)
	case strings.HasSuffix(stampAndSuffix, preSimplificationV5AuditSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preSimplificationV5AuditSuffix)
	default:
		return false
	}
	_, err := time.Parse(preSimplificationV5Timestamp, stamp)
	return err == nil
}

// InspectPreSimplificationV5Snapshots validates the names, file types, sizes,
// and pairing of application-owned rollback artifacts without creating,
// changing, or deleting anything. Ordinary files in the backups directory are
// ignored; a pre-simplification-v5-* entry is always treated as migration
// evidence and reported when it is malformed or incomplete.
func InspectPreSimplificationV5Snapshots(dataDir string) (PreSimplificationV5SnapshotInspection, error) {
	var result PreSimplificationV5SnapshotInspection
	backupsDir := PreSimplificationV5BackupsDir(dataDir)
	dirInfo, err := os.Lstat(backupsDir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("inspect pre-simplification v5 backups directory: %w", err)
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return result, fmt.Errorf("pre-simplification v5 backups path is not a regular directory: %s", backupsDir)
	}
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return result, fmt.Errorf("read pre-simplification v5 backups directory: %w", err)
	}

	type pairState struct {
		createdAt   time.Time
		archivePath string
		auditPath   string
	}
	states := make(map[string]*pairState)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, preSimplificationV5SnapshotPrefix) {
			continue
		}
		// Artifact commits serialize publication with an adjacent advisory lock.
		// A lock is coordination state, not rollback evidence, but only ignore it
		// when its target is itself a canonical snapshot artifact.
		if strings.HasSuffix(name, ".lock") {
			target := strings.TrimSuffix(filepath.Join(backupsDir, name), ".lock")
			if IsPreSimplificationV5SnapshotPath(dataDir, target) {
				continue
			}
		}
		result.HasArtifacts = true
		path := filepath.Join(backupsDir, name)
		if !IsPreSimplificationV5SnapshotPath(dataDir, path) {
			result.Issues = append(result.Issues, fmt.Sprintf("malformed pre-simplification snapshot artifact %q", name))
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return result, fmt.Errorf("inspect pre-simplification snapshot artifact %q: %w", name, infoErr)
		}
		if !info.Mode().IsRegular() {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-simplification snapshot artifact %q is not a regular file", name))
		} else if info.Size() == 0 {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-simplification snapshot artifact %q is empty", name))
		}

		var stem string
		isArchive := strings.HasSuffix(name, preSimplificationV5ArchiveSuffix)
		if isArchive {
			stem = strings.TrimSuffix(name, preSimplificationV5ArchiveSuffix)
		} else {
			stem = strings.TrimSuffix(name, preSimplificationV5AuditSuffix)
		}
		state := states[stem]
		if state == nil {
			stamp := strings.TrimPrefix(stem, preSimplificationV5SnapshotPrefix)
			createdAt, parseErr := time.Parse(preSimplificationV5Timestamp, stamp)
			if parseErr != nil {
				// IsPreSimplificationV5SnapshotPath already checked this, so
				// reaching this branch would indicate an internal grammar drift.
				return result, fmt.Errorf("parse canonical pre-simplification snapshot timestamp %q: %w", stamp, parseErr)
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
			result.Issues = append(result.Issues, fmt.Sprintf("pre-simplification snapshot %q is missing its archive", stem))
		case state.auditPath == "":
			result.Issues = append(result.Issues, fmt.Sprintf("pre-simplification snapshot %q is missing its audit report", stem))
		default:
			result.Pairs = append(result.Pairs, PreSimplificationV5SnapshotPair{
				CreatedAt:   state.createdAt,
				ArchivePath: state.archivePath,
				AuditPath:   state.auditPath,
			})
		}
	}
	sort.Strings(result.Issues)
	return result, nil
}

// RefusePreSimplificationV5SnapshotMutation is the shared application guard for
// any future cleanup or migration code that accepts a filesystem target.
func RefusePreSimplificationV5SnapshotMutation(dataDir, path string) error {
	if !IsPreSimplificationV5SnapshotPath(dataDir, path) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrImmutablePreSimplificationV5Snapshot, filepath.Clean(path))
}
