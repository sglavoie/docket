package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/storage"
)

const (
	workspaceBackupFormat        = "tt-workspace-backup"
	workspaceBackupFormatVersion = 1
)

type WorkspaceBackupReport struct {
	Path      string
	Source    string
	CreatedAt time.Time
	Files     int
}

type workspaceBackupManifest struct {
	Format        string    `json:"format"`
	FormatVersion int       `json:"format_version"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
	Files         []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
}

// CreateWorkspaceBackup publishes a new hash-verified copy under root.
// It holds the workspace commit lock throughout the inventory and copy, and
// verifies source bytes again before the staged copy becomes visible.
func CreateWorkspaceBackup(source, root string) (*WorkspaceBackupReport, error) {
	source, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = ensureNotNested(source, root); err != nil {
		return nil, fmt.Errorf("backup placement: %w; choose a backup directory outside the workspace", err)
	}
	if info, statErr := os.Lstat(source); statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("backup source must be an existing, non-symlink directory: %s", source)
	}
	// Resolve a newly created destination too: a symlink in its parent must not
	// turn an apparently separate backup into recursion through the source.
	if err = os.MkdirAll(root, DirPermissions); err != nil {
		return nil, fmt.Errorf("creating backup root: %w", err)
	}
	resolvedSource, err := filepath.EvalSymlinks(source)
	if err != nil {
		return nil, err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	if err = ensureNotNested(resolvedSource, resolvedRoot); err != nil {
		return nil, fmt.Errorf("backup placement: %w; choose a backup directory outside the workspace", err)
	}
	legacy, err := backupReadsLegacyLayout(source)
	if err != nil {
		return nil, err
	}
	// A pre-v24 workspace is copied as it stands, under the lock its own
	// builds took, so backing up before the upgrade never starts one.
	transactions := TransactionsDir(source)
	if legacy {
		transactions = filepath.Join(LegacyInternalDir(source), "transactions")
	}
	var report *WorkspaceBackupReport
	commits := storage.NewCommitService(transactions, []string{source})
	err = storage.WithWorkspaceCommitLocks([]*storage.CommitService{commits}, func() error {
		workspaceID, readErr := backupWorkspaceID(source, legacy)
		if readErr != nil {
			return readErr
		}
		entries, inventoryErr := inventoryWorkspaceExcluding(source, backupExcludes(legacy))
		if inventoryErr != nil {
			return inventoryErr
		}
		path, snapshotErr := snapshotWorkspace(entries, source, filepath.Join(root, workspaceID), time.Now(), workspaceBackupFormat, workspaceBackupFormatVersion)
		if snapshotErr != nil {
			return snapshotErr
		}
		report, snapshotErr = VerifyWorkspaceBackup(path)
		return snapshotErr
	})
	return report, err
}

// backupReadsLegacyLayout reports whether source still keeps its internal
// state in the pre-v24 .tt/, which a backup copies without migrating. A lone
// tombstone is a workspace whose state has not synced here yet, so there is
// nothing to back up.
func backupReadsLegacyLayout(source string) (bool, error) {
	if dirExists(InternalDir(source)) {
		return false, nil
	}
	state, err := inspectV24Legacy(source)
	if err != nil {
		return false, err
	}
	switch state {
	case v24LegacyTombstone:
		return false, &WorkspaceMovedElsewhereError{DataDir: source}
	case v24LegacyLive:
		return true, nil
	default:
		return false, nil
	}
}

// backupExcludes is what a backup leaves out: what a move does, except that a
// pre-v24 workspace's .tt/ is its live state rather than a tombstone, so only
// its commit lock stays behind.
func backupExcludes(legacy bool) func(rel string) bool {
	if !legacy {
		return isWorkspaceMoveExcluded
	}
	return func(rel string) bool {
		clean := filepath.Clean(rel)
		if clean == legacyInternalDirName {
			return false
		}
		return clean == filepath.Join(legacyInternalDirName, "transactions", "commit.lock") || isWorkspaceMoveExcluded(rel)
	}
}

// backupWorkspaceID is the identity a backup is filed under: the current
// catalog's, or the envelope identity a pre-v24 catalog declares, read without
// decoding a payload this build no longer loads.
func backupWorkspaceID(source string, legacy bool) (string, error) {
	if !legacy {
		catalog, err := NewDomainFileStore(source).LoadCatalogReadOnly()
		if err != nil {
			return "", fmt.Errorf("reading workspace catalog: %w", err)
		}
		if catalog.WorkspaceID == "" {
			return "", errors.New("workspace catalog has no identity")
		}
		return catalog.WorkspaceID, nil
	}
	path := filepath.Join(LegacyInternalDir(source), "workspace.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("workspace %s predates the split workspace catalog and has no identity to file a backup under; "+
			"copy the folder by hand, or open it with Docket once so it upgrades", DisplayPath(source))
	}
	if err != nil {
		return "", fmt.Errorf("reading workspace catalog: %w", err)
	}
	env, err := decodeEnvelope(data)
	if err != nil {
		return "", fmt.Errorf("decode workspace catalog envelope: %w", err)
	}
	if err = validateWorkspaceID(env.WorkspaceID); err != nil {
		return "", fmt.Errorf("workspace catalog identity: %w", err)
	}
	return env.WorkspaceID, nil
}

// missingBackupError names a directory holding no backup manifest in plain
// words, while still unwrapping to the underlying fs.ErrNotExist.
type missingBackupError struct {
	dir string
	err error
}

func (e *missingBackupError) Error() string {
	return fmt.Sprintf("no backup at %s (%s is missing)", DisplayPath(e.dir), snapshotManifest)
}

func (e *missingBackupError) Unwrap() error { return e.err }

// readWorkspaceBackupManifest decodes the manifest at the backup root, naming a
// directory that holds none in plain words.
func readWorkspaceBackupManifest(path string) (workspaceBackupManifest, error) {
	var manifest workspaceBackupManifest
	data, err := os.ReadFile(filepath.Join(path, snapshotManifest))
	if errors.Is(err, fs.ErrNotExist) {
		return manifest, &missingBackupError{dir: path, err: err}
	}
	if err != nil {
		return manifest, fmt.Errorf("reading backup manifest: %w", err)
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("decoding backup manifest: %w", err)
	}
	return manifest, nil
}

// VerifyWorkspaceBackup reads every file and checks that the backup contains
// exactly the regular files named by its manifest, with matching SHA-256 bytes.
// It never opens or changes the live source workspace.
func VerifyWorkspaceBackup(path string) (*WorkspaceBackupReport, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	manifest, err := readWorkspaceBackupManifest(path)
	if err != nil {
		return nil, err
	}
	if manifest.Format != workspaceBackupFormat || manifest.FormatVersion != workspaceBackupFormatVersion ||
		manifest.Source == "" || manifest.CreatedAt.IsZero() || len(manifest.Files) == 0 {
		return nil, errors.New("backup manifest has an unsupported or incomplete format")
	}
	want := make(map[string]string, len(manifest.Files))
	for _, file := range manifest.Files {
		rel := filepath.FromSlash(file.Path)
		if file.Path == "" || filepath.IsAbs(rel) || filepath.Clean(rel) != rel ||
			rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) ||
			len(file.SHA256) != 64 {
			return nil, fmt.Errorf("unsafe backup manifest entry %q", file.Path)
		}
		if _, exists := want[rel]; exists {
			return nil, fmt.Errorf("duplicate backup manifest entry %q", file.Path)
		}
		want[rel] = file.SHA256
	}
	seen := make(map[string]bool, len(want))
	err = filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(path, current)
		if relErr != nil || rel == "." {
			return relErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup contains a symlink: %s", rel)
		}
		if entry.IsDir() {
			return nil
		}
		if rel == snapshotManifest {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("backup contains a non-regular file: %s", rel)
		}
		expected, ok := want[rel]
		if !ok {
			return fmt.Errorf("backup contains an unlisted file: %s", rel)
		}
		actual, hashErr := hashFile(current)
		if hashErr != nil {
			return hashErr
		}
		if actual != expected {
			return fmt.Errorf("backup hash mismatch: %s", rel)
		}
		seen[rel] = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	for rel := range want {
		if !seen[rel] {
			return nil, fmt.Errorf("backup is missing %s", rel)
		}
	}
	return &WorkspaceBackupReport{Path: path, Source: manifest.Source, CreatedAt: manifest.CreatedAt, Files: len(want)}, nil
}
