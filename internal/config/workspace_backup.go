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

	"github.com/sglavoie/tt/internal/storage"
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
	var report *WorkspaceBackupReport
	commits := storage.NewCommitService(TransactionsDir(source), []string{source})
	err = storage.WithWorkspaceCommitLocks([]*storage.CommitService{commits}, func() error {
		catalog, readErr := NewDomainFileStore(source).LoadCatalogReadOnly()
		if readErr != nil {
			return fmt.Errorf("reading workspace catalog: %w", readErr)
		}
		if catalog.WorkspaceID == "" {
			return errors.New("workspace catalog has no identity")
		}
		entries, inventoryErr := inventoryWorkspace(source)
		if inventoryErr != nil {
			return inventoryErr
		}
		path, snapshotErr := snapshotWorkspace(entries, source, filepath.Join(root, catalog.WorkspaceID), time.Now(), workspaceBackupFormat, workspaceBackupFormatVersion)
		if snapshotErr != nil {
			return snapshotErr
		}
		report, snapshotErr = VerifyWorkspaceBackup(path)
		return snapshotErr
	})
	return report, err
}

// VerifyWorkspaceBackup reads every file and checks that the backup contains
// exactly the regular files named by its manifest, with matching SHA-256 bytes.
// It never opens or changes the live source workspace.
func VerifyWorkspaceBackup(path string) (*WorkspaceBackupReport, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(path, snapshotManifest)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading backup manifest: %w", err)
	}
	var manifest workspaceBackupManifest
	if err = json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decoding backup manifest: %w", err)
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
