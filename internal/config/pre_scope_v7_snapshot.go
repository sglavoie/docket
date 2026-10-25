package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/storage"
)

const (
	preScopeV7SnapshotPrefix = "pre-scope-v7-"
	preScopeV7Timestamp      = "20060102T150405Z"
	preScopeV7ArchiveSuffix  = ".tar.gz"
	preScopeV7AuditSuffix    = ".audit.md"
	preScopeV7AuditEntry     = "pre-scope-v7-audit.md"

	preScopeV7Permissions fs.FileMode = 0o400
)

// ErrImmutablePreScopeV7Snapshot classifies attempts to mutate a canonical
// pre-v7 rollback artifact.
var ErrImmutablePreScopeV7Snapshot = errors.New("config: pre-scope v7 snapshots are immutable")

// PreScopeV7SnapshotFile is one byte-exact source in the pre-v7 archive. Path
// is portable and source-relative rather than machine-absolute.
type PreScopeV7SnapshotFile struct {
	Path   string
	Kind   string
	Size   int64
	SHA256 string
}

// PreScopeV7Snapshot describes one atomically published rollback archive and
// its matching migration audit.
type PreScopeV7Snapshot struct {
	CreatedAt     time.Time
	ArchivePath   string
	AuditPath     string
	ArchiveSHA256 string
	Files         []PreScopeV7SnapshotFile
}

// PreScopeV7SnapshotPair is one canonical rollback archive and its matching
// human-readable migration audit.
type PreScopeV7SnapshotPair struct {
	CreatedAt   time.Time
	ArchivePath string
	AuditPath   string
}

// PreScopeV7SnapshotInspection is the read-only diagnostics view of the
// protected backup directory. Pairs contains only complete canonical
// archive/audit pairs; Issues describes malformed, empty, or orphaned
// application-owned artifacts.
type PreScopeV7SnapshotInspection struct {
	HasArtifacts bool
	Pairs        []PreScopeV7SnapshotPair
	Issues       []string
}

// PreScopeV7SnapshotOptions identifies the schema-6 workspace being converted.
// Machine-local state carries only the theme and is untouched by schema 7.
type PreScopeV7SnapshotOptions struct {
	DataDir string
}

type preScopeV7SnapshotCallbacks interface {
	snapshotCreatedAt() time.Time
	didInventorySnapshot()
	willPublishSnapshot(archivePath, auditPath string) error
}

type preScopeV7Source struct {
	PreScopeV7SnapshotFile
	sourcePath string
	data       []byte
}

type preScopeV7Inventory struct {
	files []preScopeV7Source
}

// PreScopeV7BackupsDir returns the application-owned directory for immutable
// pre-v7 rollback artifacts.
func PreScopeV7BackupsDir(dataDir string) string {
	return filepath.Join(InternalDir(filepath.Clean(dataDir)), "backups")
}

// IsPreScopeV7SnapshotPath reports whether path is one canonical timestamped
// pre-v7 archive or audit sidecar.
func IsPreScopeV7SnapshotPath(dataDir, path string) bool {
	backupsDir := filepath.Clean(PreScopeV7BackupsDir(dataDir))
	candidate := filepath.Clean(path)
	if filepath.Dir(candidate) != backupsDir {
		return false
	}
	name := filepath.Base(candidate)
	if !strings.HasPrefix(name, preScopeV7SnapshotPrefix) {
		return false
	}
	stampAndSuffix := strings.TrimPrefix(name, preScopeV7SnapshotPrefix)
	var stamp string
	switch {
	case strings.HasSuffix(stampAndSuffix, preScopeV7ArchiveSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preScopeV7ArchiveSuffix)
	case strings.HasSuffix(stampAndSuffix, preScopeV7AuditSuffix):
		stamp = strings.TrimSuffix(stampAndSuffix, preScopeV7AuditSuffix)
	default:
		return false
	}
	_, err := time.Parse(preScopeV7Timestamp, stamp)
	return err == nil
}

// InspectPreScopeV7Snapshots validates immutable pre-v7 rollback artifacts
// without changing them. Unrelated files in the shared backups directory are
// ignored.
func InspectPreScopeV7Snapshots(dataDir string) (PreScopeV7SnapshotInspection, error) {
	var result PreScopeV7SnapshotInspection
	backupsDir := PreScopeV7BackupsDir(dataDir)
	dirInfo, err := os.Lstat(backupsDir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("inspect pre-scope v7 backups directory: %w", err)
	}
	if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		return result, fmt.Errorf("pre-scope v7 backups path is not a regular directory: %s", backupsDir)
	}
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return result, fmt.Errorf("read pre-scope v7 backups directory: %w", err)
	}

	type pairState struct {
		createdAt   time.Time
		archivePath string
		auditPath   string
	}
	states := make(map[string]*pairState)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, preScopeV7SnapshotPrefix) {
			continue
		}
		if strings.HasSuffix(name, ".lock") {
			target := strings.TrimSuffix(filepath.Join(backupsDir, name), ".lock")
			if IsPreScopeV7SnapshotPath(dataDir, target) {
				continue
			}
		}
		result.HasArtifacts = true
		path := filepath.Join(backupsDir, name)
		if !IsPreScopeV7SnapshotPath(dataDir, path) {
			result.Issues = append(result.Issues, fmt.Sprintf("malformed pre-scope v7 snapshot artifact %q", name))
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return result, fmt.Errorf("inspect pre-scope v7 snapshot artifact %q: %w", name, infoErr)
		}
		if !info.Mode().IsRegular() {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-scope v7 snapshot artifact %q is not a regular file", name))
		} else if info.Size() == 0 {
			result.Issues = append(result.Issues, fmt.Sprintf("pre-scope v7 snapshot artifact %q is empty", name))
		}

		isArchive := strings.HasSuffix(name, preScopeV7ArchiveSuffix)
		stem := strings.TrimSuffix(name, preScopeV7AuditSuffix)
		if isArchive {
			stem = strings.TrimSuffix(name, preScopeV7ArchiveSuffix)
		}
		state := states[stem]
		if state == nil {
			stamp := strings.TrimPrefix(stem, preScopeV7SnapshotPrefix)
			createdAt, parseErr := time.Parse(preScopeV7Timestamp, stamp)
			if parseErr != nil {
				return result, fmt.Errorf("parse canonical pre-scope v7 snapshot timestamp %q: %w", stamp, parseErr)
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
			result.Issues = append(result.Issues, fmt.Sprintf("pre-scope v7 snapshot %q is missing its archive", stem))
		case state.auditPath == "":
			result.Issues = append(result.Issues, fmt.Sprintf("pre-scope v7 snapshot %q is missing its audit report", stem))
		default:
			result.Pairs = append(result.Pairs, PreScopeV7SnapshotPair{
				CreatedAt: state.createdAt, ArchivePath: state.archivePath, AuditPath: state.auditPath,
			})
		}
	}
	sort.Strings(result.Issues)
	return result, nil
}

// RefusePreScopeV7SnapshotMutation is the application-level immutable path
// guard for future migration and cleanup code.
func RefusePreScopeV7SnapshotMutation(dataDir, path string) error {
	if !IsPreScopeV7SnapshotPath(dataDir, path) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrImmutablePreScopeV7Snapshot, filepath.Clean(path))
}

//nolint:govet // Scoped errors keep each snapshot validation and transaction stage local.
func createPreScopeV7Snapshot(
	options PreScopeV7SnapshotOptions,
	callbacks preScopeV7SnapshotCallbacks,
) (*PreScopeV7Snapshot, error) {
	root, err := validateV5DryRunRoot(options.DataDir)
	if err != nil {
		return nil, err
	}
	dryRunOptions := V7WorkspaceDryRunOptions{DataDir: root}

	// This validation happens before the commit service can create a lock or
	// transaction artifact. Invalid candidates therefore leave no snapshot state.
	validatedPlan, err := DryRunV7WorkspaceConversion(dryRunOptions)
	if err != nil {
		return nil, fmt.Errorf("pre-scope v7 candidate validation failed; workspace was not changed: %w", err)
	}
	if err := ValidateV7WorkspaceConversionPlan(validatedPlan); err != nil {
		return nil, fmt.Errorf("pre-scope v7 candidate validation failed; workspace was not changed: %w", err)
	}

	createdAt := time.Now().UTC().Truncate(time.Second)
	if callbacks != nil {
		if override := callbacks.snapshotCreatedAt(); !override.IsZero() {
			createdAt = override.UTC().Truncate(time.Second)
		}
	}
	stem := preScopeV7SnapshotPrefix + createdAt.Format(preScopeV7Timestamp)
	archivePath := filepath.Join(PreScopeV7BackupsDir(root), stem+preScopeV7ArchiveSuffix)
	auditPath := filepath.Join(PreScopeV7BackupsDir(root), stem+preScopeV7AuditSuffix)
	if err := precheckPreScopeV7Targets(archivePath, auditPath); err != nil {
		return nil, err
	}
	// Validate the complete snapshot topology before taking the workspace lock.
	// The locked inventory below remains authoritative.
	if _, err := inventoryPreScopeV7(root, false); err != nil {
		return nil, err
	}

	var result *PreScopeV7Snapshot
	commit := storage.NewCommitService(TransactionsDir(root), []string{root})
	_, err = commit.CommitPlannedArtifacts(func() ([]storage.ArtifactChange, error) {
		if err := precheckPreScopeV7Targets(archivePath, auditPath); err != nil {
			return nil, err
		}
		// Recovery, if any, has completed under the workspace lock. Rebuild the
		// candidate from that exact coherent source state before capturing bytes.
		plan, err := DryRunV7WorkspaceConversion(dryRunOptions)
		if err != nil {
			return nil, fmt.Errorf("pre-scope v7 candidate changed before snapshot publication: %w", err)
		}
		if err := ValidateV7WorkspaceConversionPlan(plan); err != nil {
			return nil, err
		}
		inventory, err := inventoryPreScopeV7(root, true)
		if err != nil {
			return nil, err
		}
		if callbacks != nil {
			callbacks.didInventorySnapshot()
		}
		confirmedPlan, err := DryRunV7WorkspaceConversion(dryRunOptions)
		if err != nil {
			return nil, fmt.Errorf("pre-scope v7 candidate changed while snapshotting: %w", err)
		}
		if !reflect.DeepEqual(plan, confirmedPlan) {
			return nil, fmt.Errorf("pre-scope v7 candidate changed while snapshotting")
		}
		current, err := inventoryPreScopeV7(root, false)
		if err != nil {
			return nil, err
		}
		if err := comparePreScopeV7Inventories(inventory, current); err != nil {
			return nil, err
		}
		audit := renderPreScopeV7Audit(createdAt, plan, inventory)
		archive, err := buildPreScopeV7Archive(inventory.files, audit)
		if err != nil {
			return nil, err
		}
		if callbacks != nil {
			if err := callbacks.willPublishSnapshot(archivePath, auditPath); err != nil {
				return nil, err
			}
		}
		archiveSum := sha256.Sum256(archive)
		absent := ""
		result = &PreScopeV7Snapshot{
			CreatedAt: createdAt, ArchivePath: archivePath, AuditPath: auditPath,
			ArchiveSHA256: hex.EncodeToString(archiveSum[:]),
			Files:         publicPreScopeV7Files(inventory.files),
		}
		return []storage.ArtifactChange{
			{Path: archivePath, Data: archive, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
			{Path: auditPath, Data: audit, Mode: preScopeV7Permissions, ExpectedRevision: &absent},
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("pre-scope v7 snapshot failed: %w", err)
	}
	return result, nil
}
