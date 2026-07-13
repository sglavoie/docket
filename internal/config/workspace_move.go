package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/util"
)

// MoveReport summarizes the outcome of a workspace move so the caller can print
// an accurate, non-destructive summary.
type MoveReport struct {
	Source         string
	Destination    string
	FilesCopied    int
	FilesSkipped   int // already present at the destination and byte-identical
	DirsCreated    int
	BytesCopied    int64
	SourceRetained bool
}

// workspaceEntry is one durable source path collected during inventory.
type workspaceEntry struct {
	src  string
	rel  string
	dir  bool
	perm fs.FileMode
}

// MoveWorkspace relocates a workspace's durable data directory from srcDataDir
// to dstDataDir using a staged, verified, resumable flow:
//
//  1. Inventory the durable source tree, excluding machine-local runtime state
//     and refusing symlinks / non-regular files.
//  2. Pre-check every destination file: absent, or byte-identical (an
//     interrupted move re-run is idempotent). A divergent existing file aborts
//     the whole move before anything is written.
//  3. Copy each missing file through a temp file with fsync and a sha256 read-
//     back verification, then atomically rename it into place.
//  4. Rewrite the destination config's data_dir and legacy project paths.
//  5. Switch the machine-local active-workspace pointer to the destination
//     (the atomic "the workspace now lives here" commit).
//
// The source is retained by default (SourceRetained is always true here); it is
// never deleted, so the operation is safe to interrupt and re-run. Callers that
// want to reclaim the old space remove the source themselves after verifying.
func MoveWorkspace(srcDataDir, dstDataDir string) (*MoveReport, error) {
	src := filepath.Clean(srcDataDir)
	dst := filepath.Clean(dstDataDir)
	if src == "" || src == "." || dst == "" || dst == "." {
		return nil, fmt.Errorf("workspace move requires both a source and a destination data dir")
	}
	if src == dst {
		return nil, fmt.Errorf("workspace move: source and destination are the same directory: %s", src)
	}

	srcInfo, statErr := os.Stat(src)
	if statErr != nil {
		return nil, fmt.Errorf("workspace move: source data dir %s is not accessible: %w", src, statErr)
	}
	if !srcInfo.IsDir() {
		return nil, fmt.Errorf("workspace move: source data dir %s is not a directory", src)
	}
	if err := ensureNotNested(src, dst); err != nil {
		return nil, err
	}

	entries, invErr := inventoryWorkspace(src)
	if invErr != nil {
		return nil, invErr
	}
	if err := precheckDestination(entries, dst); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dst, util.DirPermissions); err != nil {
		return nil, fmt.Errorf("workspace move: failed to create destination %s: %w", dst, err)
	}

	report := &MoveReport{Source: src, Destination: dst, SourceRetained: true}
	if err := createDestinationDirs(entries, dst, report); err != nil {
		return nil, err
	}
	if err := copyDestinationFiles(entries, dst, report); err != nil {
		return nil, err
	}

	// Rewrite the destination config so data_dir and legacy project paths point
	// at the new root. Only legacy default paths are rewritten; custom paths are
	// preserved. Absent config (fresh workspace) is a no-op.
	dstConfig := ConfigPathForDataDir(dst)
	if _, cfgStatErr := os.Stat(dstConfig); cfgStatErr == nil {
		if err := WriteMigratedConfig(dstConfig, src, dst); err != nil {
			return nil, fmt.Errorf("workspace move: failed to rewrite destination config: %w", err)
		}
	}

	// Atomic pointer switch: from here on this machine opens the destination.
	if err := WriteActiveWorkspacePointer(dst, readWorkspaceID(dst)); err != nil {
		return nil, fmt.Errorf("workspace move: failed to switch active workspace pointer: %w", err)
	}

	return report, nil
}

// precheckDestination verifies every destination file is absent or byte-
// identical, so a move is all-or-nothing and re-running after an interruption is
// idempotent. A divergent existing file — or a symlink or directory blocking a
// file's path — aborts before anything is written.
func precheckDestination(entries []workspaceEntry, dst string) error {
	for _, e := range entries {
		if e.dir {
			continue
		}
		target := filepath.Join(dst, e.rel)
		info, statErr := os.Lstat(target)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("workspace move: cannot inspect destination %s: %w", target, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("workspace move aborted: destination %s is a symlink; refusing to overwrite", target)
		}
		if info.IsDir() {
			return fmt.Errorf("workspace move aborted: destination %s is a directory but the source has a file there", target)
		}
		same, cmpErr := filesIdentical(e.src, target)
		if cmpErr != nil {
			return cmpErr
		}
		if !same {
			return fmt.Errorf("workspace move aborted: destination file already exists with different contents, refusing to overwrite: %s", target)
		}
	}
	return nil
}

// createDestinationDirs recreates each source directory under dst. WalkDir
// yielded parents before children, so directories are created before their
// contents.
func createDestinationDirs(entries []workspaceEntry, dst string, report *MoveReport) error {
	for _, e := range entries {
		if !e.dir {
			continue
		}
		target := filepath.Join(dst, e.rel)
		if _, statErr := os.Stat(target); errors.Is(statErr, os.ErrNotExist) {
			report.DirsCreated++
		}
		if err := os.MkdirAll(target, e.perm); err != nil {
			return fmt.Errorf("workspace move: failed to create dir %s: %w", target, err)
		}
	}
	return nil
}

// copyDestinationFiles copies each source file to dst, skipping files already
// present (precheckDestination proved them identical) so a resumed move only
// does the outstanding work.
func copyDestinationFiles(entries []workspaceEntry, dst string, report *MoveReport) error {
	for _, e := range entries {
		if e.dir {
			continue
		}
		target := filepath.Join(dst, e.rel)
		if info, statErr := os.Lstat(target); statErr == nil && info.Mode().IsRegular() {
			report.FilesSkipped++
			continue
		}
		n, err := copyFileVerified(e.src, target, e.perm)
		if err != nil {
			return err
		}
		report.FilesCopied++
		report.BytesCopied += n
	}
	return nil
}

// inventoryWorkspace walks a data directory and returns its durable entries.
// Machine-local runtime state is excluded; symlinks and other non-regular files
// are refused so a move never follows a link out of the workspace root or
// silently drops special files.
func inventoryWorkspace(root string) ([]workspaceEntry, error) {
	var entries []workspaceEntry
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if isMigrationRootExcluded(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("workspace move aborted: source contains a symlink, resolve it before moving: %s", path)
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		if !d.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("workspace move aborted: source contains a non-regular file, resolve it before moving: %s", path)
		}
		entries = append(entries, workspaceEntry{src: path, rel: rel, dir: d.IsDir(), perm: info.Mode().Perm()})
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("workspace move: failed to inventory source %s: %w", root, walkErr)
	}
	return entries, nil
}

// ensureNotNested refuses a move where source and destination overlap, which a
// recursive copy could otherwise loop on or corrupt. Both the lexical paths and
// their symlink-resolved forms (where they exist) are checked.
func ensureNotNested(src, dst string) error {
	if isSubpath(src, dst) {
		return fmt.Errorf("workspace move aborted: destination %s is inside the source %s", dst, src)
	}
	if isSubpath(dst, src) {
		return fmt.Errorf("workspace move aborted: source %s is inside the destination %s", src, dst)
	}
	rs, err1 := filepath.EvalSymlinks(src)
	rd, err2 := filepath.EvalSymlinks(dst)
	if err1 == nil && err2 == nil && (isSubpath(rs, rd) || isSubpath(rd, rs)) {
		return fmt.Errorf("workspace move aborted: source and destination overlap after resolving symlinks (%s, %s)", rs, rd)
	}
	return nil
}

// isSubpath reports whether child is parent or lives beneath it.
func isSubpath(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if parent == child {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// filesIdentical reports whether two files have identical sha256 content hashes.
func filesIdentical(a, b string) (bool, error) {
	ha, err := hashFile(a)
	if err != nil {
		return false, fmt.Errorf("workspace move: failed to hash %s: %w", a, err)
	}
	hb, err := hashFile(b)
	if err != nil {
		return false, fmt.Errorf("workspace move: failed to hash %s: %w", b, err)
	}
	return ha == hb, nil
}

// hashFile returns the hex sha256 of a file's contents.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyFileVerified copies src to dst durably: it streams into a temp file in the
// destination directory while hashing, fsyncs it, verifies the on-disk copy
// matches the source hash, atomically renames it into place, and fsyncs the
// destination directory. The number of bytes copied is returned.
func copyFileVerified(src, dst string, perm fs.FileMode) (int64, error) {
	in, openErr := os.Open(src)
	if openErr != nil {
		return 0, fmt.Errorf("workspace move: failed to open source %s: %w", src, openErr)
	}
	defer func() { _ = in.Close() }()

	dir := filepath.Dir(dst)
	tmp, tmpErr := os.CreateTemp(dir, ".tt-move-*")
	if tmpErr != nil {
		return 0, fmt.Errorf("workspace move: failed to create temp file in %s: %w", dir, tmpErr)
	}
	tmpName := tmp.Name()

	srcHash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, srcHash), in)
	if copyErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("workspace move: failed to copy %s: %w", src, copyErr)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("workspace move: failed to fsync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("workspace move: failed to close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("workspace move: failed to set permissions on %s: %w", tmpName, err)
	}

	// Read-back verification: the published file must match the source exactly.
	dstHash, hashErr := hashFile(tmpName)
	if hashErr != nil {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("workspace move: failed to verify copy of %s: %w", src, hashErr)
	}
	if dstHash != hex.EncodeToString(srcHash.Sum(nil)) {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("workspace move: hash mismatch after copying %s (corruption detected)", src)
	}

	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("workspace move: failed to publish %s: %w", dst, err)
	}
	fsyncDir(dir)
	return written, nil
}

// fsyncDir best-effort fsyncs a directory so a rename into it is durable.
func fsyncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

// readWorkspaceID best-effort reads the workspace ID from a data dir's synced
// workspace catalog so the active-workspace pointer can carry it. A missing or
// unreadable catalog yields an empty ID (the pointer's authoritative field is
// its data dir).
func readWorkspaceID(dataDir string) string {
	data, err := os.ReadFile(WorkspaceCatalogPath(dataDir))
	if err != nil {
		return ""
	}
	var env struct {
		WorkspaceID string `json:"workspace_id"`
		Data        struct {
			WorkspaceID string `json:"workspace_id"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &env) != nil {
		return ""
	}
	if env.WorkspaceID != "" {
		return env.WorkspaceID
	}
	return env.Data.WorkspaceID
}

// StorageLocationLabel returns a human-readable label for where a data dir
// lives: an iCloud Drive path is labeled as synced, everything else as local.
func StorageLocationLabel(dataDir string) string {
	if root, ok := iCloudDataRoot(); ok && isSubpath(root, filepath.Clean(dataDir)) {
		return "iCloud Drive (synced)"
	}
	return "local"
}
