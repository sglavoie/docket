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
	preConsolidationV6SnapshotPrefix = "pre-consolidation-v6-"
	preConsolidationV6Timestamp      = "20060102T150405Z"
	preConsolidationV6ArchiveSuffix  = ".tar.gz"
	preConsolidationV6AuditSuffix    = ".audit.md"
	preConsolidationV6AuditEntry     = "pre-consolidation-v6-audit.md"

	preConsolidationV6Permissions fs.FileMode = 0o400
)

// ErrImmutablePreConsolidationV6Snapshot classifies attempts to mutate a
// canonical pre-v6 rollback artifact.
var ErrImmutablePreConsolidationV6Snapshot = errors.New("config: pre-consolidation v6 snapshots are immutable")

// PreConsolidationV6SnapshotFile is one byte-exact source in the pre-v6
// archive. Path is portable and source-relative rather than machine-absolute.
type PreConsolidationV6SnapshotFile struct {
	Path   string
	Kind   string
	Size   int64
	SHA256 string
}

// PreConsolidationV6Snapshot describes one atomically published rollback
// archive and its matching migration audit.
type PreConsolidationV6Snapshot struct {
	CreatedAt     time.Time
	ArchivePath   string
	AuditPath     string
	ArchiveSHA256 string
	Files         []PreConsolidationV6SnapshotFile
}

// PreConsolidationV6SnapshotPair is one canonical rollback archive and its
// matching human-readable migration audit.
type PreConsolidationV6SnapshotPair struct {
	CreatedAt   time.Time
	ArchivePath string
	AuditPath   string
}

// PreConsolidationV6SnapshotInspection is the read-only diagnostics view of
// the protected backup directory. Pairs contains only complete canonical
// archive/audit pairs; Issues describes malformed, empty, or orphaned
// application-owned artifacts.
type PreConsolidationV6SnapshotInspection struct {
	HasArtifacts bool
	Pairs        []PreConsolidationV6SnapshotPair
	Issues       []string
}

// PreConsolidationV6SnapshotOptions identifies the schema-5 workspace and an
// optional isolated machine-state source. It has the same source semantics as
// V6WorkspaceDryRunOptions.
type PreConsolidationV6SnapshotOptions struct {
	DataDir          string
	MachineStatePath string
}

type preConsolidationV6SnapshotCallbacks interface {
	snapshotCreatedAt() time.Time
	didInventorySnapshot()
	willPublishSnapshot(archivePath, auditPath string) error
}

type preConsolidationV6Source struct {
	PreConsolidationV6SnapshotFile
	sourcePath string
	data       []byte
}

type preConsolidationV6Inventory struct {
	files []preConsolidationV6Source
}

type preConsolidationV6TaskFact struct {
	projectID string
	taskID    string
	state     string
	priority  int
	decisions int
}

// PreConsolidationV6BackupsDir returns the application-owned directory for
// immutable pre-v6 rollback artifacts.
func PreConsolidationV6BackupsDir(dataDir string) string {
	return filepath.Join(InternalDir(filepath.Clean(dataDir)), "backups")
}

// IsPreConsolidationV6SnapshotPath reports whether path is one canonical
// timestamped pre-v6 archive or audit sidecar.
func IsPreConsolidationV6SnapshotPath(dataDir, path string) bool {
	backupsDir := filepath.Clean(PreConsolidationV6BackupsDir(dataDir))
	candidate := filepath.Clean(path)
	if filepath.Dir(candidate) != backupsDir {
		return false
	}
	name := filepath.Base(candidate)
	if !strings.HasPrefix(name, preConsolidationV6SnapshotPrefix) {
		return false
	}
	stampAndSuffix := strings.TrimPrefix(name, preConsolidationV6SnapshotPrefix)
	var stamp string
	switch {
	case strings.HasSuffix(stampAndSuffix, preConsolidationV6ArchiveSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preConsolidationV6ArchiveSuffix)
	case strings.HasSuffix(stampAndSuffix, preConsolidationV6AuditSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preConsolidationV6AuditSuffix)
	default:
		return false
	}
	_, err := time.Parse(preConsolidationV6Timestamp, stamp)
	return err == nil
}

// InspectPreConsolidationV6Snapshots validates immutable pre-v6 rollback
// artifacts without changing them. Unrelated files in the shared backups
// directory are ignored.
func InspectPreConsolidationV6Snapshots(dataDir string) (PreConsolidationV6SnapshotInspection, error) {
	var result PreConsolidationV6SnapshotInspection
	backupsDir := PreConsolidationV6BackupsDir(dataDir)
	dirInfo, err := os.Lstat(backupsDir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("inspect pre-consolidation v6 backups directory: %w", err)
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return result, fmt.Errorf("pre-consolidation v6 backups path is not a regular directory: %s", backupsDir)
	}
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return result, fmt.Errorf("read pre-consolidation v6 backups directory: %w", err)
	}

	type pairState struct {
		createdAt   time.Time
		archivePath string
		auditPath   string
	}
	states := make(map[string]*pairState)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, preConsolidationV6SnapshotPrefix) {
			continue
		}
		if strings.HasSuffix(name, ".lock") {
			target := strings.TrimSuffix(filepath.Join(backupsDir, name), ".lock")
			if IsPreConsolidationV6SnapshotPath(dataDir, target) {
				continue
			}
		}
		result.HasArtifacts = true
		path := filepath.Join(backupsDir, name)
		if !IsPreConsolidationV6SnapshotPath(dataDir, path) {
			result.Issues = append(result.Issues, fmt.Sprintf("malformed pre-consolidation v6 snapshot artifact %q", name))
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return result, fmt.Errorf("inspect pre-consolidation v6 snapshot artifact %q: %w", name, infoErr)
		}
		if !info.Mode().IsRegular() {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-consolidation v6 snapshot artifact %q is not a regular file", name))
		} else if info.Size() == 0 {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-consolidation v6 snapshot artifact %q is empty", name))
		}

		isArchive := strings.HasSuffix(name, preConsolidationV6ArchiveSuffix)
		stem := strings.TrimSuffix(name, preConsolidationV6AuditSuffix)
		if isArchive {
			stem = strings.TrimSuffix(name, preConsolidationV6ArchiveSuffix)
		}
		state := states[stem]
		if state == nil {
			stamp := strings.TrimPrefix(stem, preConsolidationV6SnapshotPrefix)
			createdAt, parseErr := time.Parse(preConsolidationV6Timestamp, stamp)
			if parseErr != nil {
				return result, fmt.Errorf("parse canonical pre-consolidation v6 snapshot timestamp %q: %w", stamp, parseErr)
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
			result.Issues = append(result.Issues, fmt.Sprintf("pre-consolidation v6 snapshot %q is missing its archive", stem))
		case state.auditPath == "":
			result.Issues = append(result.Issues, fmt.Sprintf("pre-consolidation v6 snapshot %q is missing its audit report", stem))
		default:
			result.Pairs = append(result.Pairs, PreConsolidationV6SnapshotPair{
				CreatedAt: state.createdAt, ArchivePath: state.archivePath, AuditPath: state.auditPath,
			})
		}
	}
	sort.Strings(result.Issues)
	return result, nil
}

// RefusePreConsolidationV6SnapshotMutation is the application-level immutable
// path guard for future migration and cleanup code.
func RefusePreConsolidationV6SnapshotMutation(dataDir, path string) error {
	if !IsPreConsolidationV6SnapshotPath(dataDir, path) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrImmutablePreConsolidationV6Snapshot, filepath.Clean(path))
}
