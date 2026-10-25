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

	"github.com/sglavoie/docket/internal/util"
)

// snapshotOriginal copies every durable file of the original, verified by
// hash, into a new snapshot directory under root, and records what it holds
// last. The directory is assembled under a partial name and renamed into place
// once complete, so a snapshot that exists is a whole one; interrupted
// enrollment partials are removed before the next enrollment snapshot.
func snapshotOriginal(entries []workspaceEntry, original, root string, at time.Time) (string, error) {
	return snapshotWorkspace(entries, original, root, at, enrollmentSnapshotFormat, enrollmentFormatVersion)
}

func snapshotWorkspace(entries []workspaceEntry, original, root string, at time.Time, format string, version int) (string, error) {
	for _, entry := range entries {
		if entry.rel == snapshotManifest {
			return "", fmt.Errorf("snapshot cannot use %s as a source: it is reserved for the snapshot manifest", entry.src)
		}
	}
	if err := os.MkdirAll(root, DirPermissions); err != nil {
		return "", fmt.Errorf("snapshot: creating the root %s: %w", root, err)
	}
	if format == enrollmentSnapshotFormat {
		stale, err := filepath.Glob(filepath.Join(root, "*"+snapshotPartialExt))
		if err != nil {
			return "", err
		}
		for _, partial := range stale {
			if removeErr := os.RemoveAll(partial); removeErr != nil {
				return "", fmt.Errorf("snapshot: removing an interrupted copy %s: %w", partial, removeErr)
			}
		}
	}
	// A snapshot is never replaced: an earlier run's complete snapshot of the
	// same instant keeps its name and this one takes the next.
	stamp := at.UTC().Format("20060102T150405.000000000Z")
	final := filepath.Join(root, stamp)
	for n := 2; ; n++ {
		if _, statErr := os.Lstat(final); errors.Is(statErr, fs.ErrNotExist) {
			break
		}
		final = filepath.Join(root, fmt.Sprintf("%s-%d", stamp, n))
	}
	partial := final + snapshotPartialExt
	report := &MoveReport{}
	if dirsErr := createDestinationDirs(entries, partial, report); dirsErr != nil {
		return "", dirsErr
	}
	if copyErr := copyDestinationFiles(entries, partial, report); copyErr != nil {
		return "", copyErr
	}
	type snapshotFile struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	manifest := struct {
		Format        string         `json:"format"`
		FormatVersion int            `json:"format_version"`
		Source        string         `json:"source"`
		CreatedAt     time.Time      `json:"created_at"`
		Files         []snapshotFile `json:"files"`
	}{Format: format, FormatVersion: version, Source: original, CreatedAt: at.UTC(), Files: []snapshotFile{}}
	for _, entry := range entries {
		if !entry.dir {
			manifest.Files = append(manifest.Files, snapshotFile{Path: filepath.ToSlash(entry.rel), SHA256: entry.hash})
		}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := util.AtomicWriteFile(filepath.Join(partial, snapshotManifest), append(data, '\n'), FilePermissions); err != nil {
		return "", fmt.Errorf("snapshot: recording the manifest: %w", err)
	}
	if err := validateSourceSnapshot(entries); err != nil {
		return "", err
	}
	if err := os.Rename(partial, final); err != nil {
		return "", fmt.Errorf("snapshot: publishing the copy: %w", err)
	}
	return final, nil
}

// workingCopyExcluded is what enrollment leaves behind in the original: the
// device-local history, undo ledger, journal, and migration snapshots, which
// describe the original's past rather than the enrolled workspace's, and the
// lock files and sidecars no workspace carries across a copy. Conflict bundles
// stay with the workspace whose files they are about.
func workingCopyExcluded(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if parts[0] == legacyInternalDirName {
		// The tombstone is per working copy: each one gets its own.
		return true
	}
	if len(parts) >= 2 && parts[0] == internalDirName {
		switch parts[1] {
		case "sync", "undo", "transactions", "backups":
			return true
		}
	}
	base := parts[len(parts)-1]
	return strings.HasSuffix(base, ".lock") || strings.HasSuffix(base, ".bak") || strings.HasPrefix(base, ".tt-stage-")
}

// stageWorkingCopy copies the original's durable files into a fresh staging
// directory beside the working copy, marked as enrollment's own.
func stageWorkingCopy(entries []workspaceEntry, workingCopy string) (string, int, error) {
	staging := workingCopy + enrollStagingSuffix
	if _, err := os.Lstat(staging); err == nil {
		if _, markerErr := os.Stat(filepath.Join(staging, enrollStagingMarker)); markerErr != nil {
			return "", 0, fmt.Errorf("enroll: %s exists and is not an interrupted enrollment's staging directory; remove it or choose another working copy", staging)
		}
		if err := os.RemoveAll(staging); err != nil {
			return "", 0, fmt.Errorf("enroll: removing an interrupted enrollment's staging directory: %w", err)
		}
	}
	if err := os.MkdirAll(staging, DirPermissions); err != nil {
		return "", 0, fmt.Errorf("enroll: creating the staging directory: %w", err)
	}
	if err := util.AtomicWriteFile(filepath.Join(staging, enrollStagingMarker), []byte("tt enrollment staging\n"), FilePermissions); err != nil {
		return "", 0, fmt.Errorf("enroll: marking the staging directory: %w", err)
	}
	kept := make([]workspaceEntry, 0, len(entries))
	for _, entry := range entries {
		if !workingCopyExcluded(entry.rel) {
			kept = append(kept, entry)
		}
	}
	report := &MoveReport{}
	if err := createDestinationDirs(kept, staging, report); err != nil {
		return "", 0, err
	}
	if err := copyDestinationFiles(kept, staging, report); err != nil {
		return "", 0, err
	}
	return staging, report.FilesCopied, nil
}

// enrollStagedCatalog advances the staged catalog to the enrolled schema with
// its sync block, through the checked domain store, and proves the staged
// domains load as the enrolled workspace.
func enrollStagedCatalog(staging string, enrolledAt time.Time) error {
	store := NewDomainFileStore(staging)
	catalog, rev, err := store.LoadCatalogChecked()
	if err != nil {
		return fmt.Errorf("enroll: reading the staged catalog: %w", err)
	}
	catalog.SchemaVersion = SyncEnrolledConfigVersion
	catalog.Sync = &CatalogSync{EnrolledAt: enrolledAt, Exchange: SyncExchangeFormat}
	if err := store.SaveCatalogChecked(catalog, rev); err != nil {
		return fmt.Errorf("enroll: writing the enrolled catalog: %w", err)
	}
	// The checked save leaves a sidecar of the original catalog; the working
	// copy starts without one, exactly as a copy excludes the original's.
	if err := os.Remove(WorkspaceCatalogPath(staging) + ".bak"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("enroll: removing the staged catalog sidecar: %w", err)
	}
	if _, err := NewDomainFileStore(staging).LoadCatalogReadOnly(); err != nil {
		return fmt.Errorf("enroll: the enrolled catalog does not load: %w", err)
	}
	if _, err := NewDomainFileStore(staging).LoadPlanning(); err != nil {
		return fmt.Errorf("enroll: the staged planning does not load: %w", err)
	}
	return nil
}

// publishWorkingCopy makes the staged working copy the working copy in one
// rename, after dropping the staging marker and an empty placeholder directory
// the user may have created for it.
func publishWorkingCopy(staging, workingCopy string) error {
	if err := os.Remove(filepath.Join(staging, enrollStagingMarker)); err != nil {
		return fmt.Errorf("enroll: unmarking the staging directory: %w", err)
	}
	if err := os.Remove(workingCopy); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("enroll: the working copy path %s is not empty: %w", workingCopy, err)
	}
	if err := os.MkdirAll(filepath.Dir(workingCopy), DirPermissions); err != nil {
		return fmt.Errorf("enroll: creating the working copy's parent: %w", err)
	}
	if err := os.Rename(staging, workingCopy); err != nil {
		return fmt.Errorf("enroll: publishing the working copy: %w", err)
	}
	return nil
}
