package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sglavoie/docket/internal/util"
)

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
		targetHash, hashErr := hashFile(target)
		if hashErr != nil {
			return fmt.Errorf("workspace move: failed to hash destination %s: %w", target, hashErr)
		}
		if targetHash != e.hash {
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
		if info, statErr := os.Lstat(target); statErr == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("workspace move aborted: destination path became occupied while copying, refusing to overwrite: %s", target)
			}
			targetHash, hashErr := hashFile(target)
			if hashErr != nil {
				return fmt.Errorf("workspace move: failed to hash destination %s: %w", target, hashErr)
			}
			if targetHash != e.hash {
				return fmt.Errorf("workspace move aborted: destination file changed while copying, refusing to overwrite: %s", target)
			}
			report.FilesSkipped++
			continue
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("workspace move: cannot inspect destination %s: %w", target, statErr)
		}
		copied, err := copyFileVerified(e.src, target, e.perm, e.hash)
		if err != nil {
			return err
		}
		if copied {
			report.FilesCopied++
		} else {
			report.FilesSkipped++
		}
	}
	return nil
}

// validateSourceSnapshot proves every inventoried source file still has the
// hash captured under the workspace locks. Ordinary Docket commits cannot reach
// this point concurrently; this guard catches direct filesystem writers.
func validateSourceSnapshot(entries []workspaceEntry) error {
	for _, e := range entries {
		if e.dir {
			continue
		}
		current, err := hashFile(e.src)
		if err != nil {
			return fmt.Errorf("workspace move aborted: source changed while copying (%s): %w", e.src, err)
		}
		if current != e.hash {
			return fmt.Errorf("workspace move aborted: source changed while copying, refusing to publish a mixed snapshot: %s", e.src)
		}
	}
	return nil
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
// matches the inventoried source hash, atomically links it into place without
// replacement, and fsyncs the destination directory. The bool reports whether
// this call published the file; an identical destination race is reusable.
func copyFileVerified(src, dst string, perm fs.FileMode, expectedHash string) (bool, error) {
	in, openErr := os.Open(src)
	if openErr != nil {
		return false, fmt.Errorf("workspace move: failed to open source %s: %w", src, openErr)
	}
	defer func() { _ = in.Close() }()

	dir := filepath.Dir(dst)
	tmp, tmpErr := os.CreateTemp(dir, ".tt-move-*")
	if tmpErr != nil {
		return false, fmt.Errorf("workspace move: failed to create temp file in %s: %w", dir, tmpErr)
	}
	tmpName := tmp.Name()

	srcHash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(tmp, srcHash), in)
	if copyErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("workspace move: failed to copy %s: %w", src, copyErr)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("workspace move: failed to set permissions on %s: %w", tmpName, err)
	}
	if err := util.SyncFile(tmp); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("workspace move: failed to fsync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("workspace move: failed to close %s: %w", tmpName, err)
	}

	// Read-back verification: the published file must match the source exactly.
	dstHash, hashErr := hashFile(tmpName)
	if hashErr != nil {
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("workspace move: failed to verify copy of %s: %w", src, hashErr)
	}
	copiedSourceHash := hex.EncodeToString(srcHash.Sum(nil))
	if copiedSourceHash != expectedHash {
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("workspace move aborted: source changed while copying, refusing to publish mixed bytes: %s", src)
	}
	if dstHash != copiedSourceHash {
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("workspace move: hash mismatch after copying %s (corruption detected)", src)
	}

	// Publish without replacement. A destination writer racing the move must
	// never be overwritten between the occupancy check and publication.
	if err := os.Link(tmpName, dst); err != nil {
		if errors.Is(err, fs.ErrExist) {
			existingHash, hashErr := hashFile(dst)
			_ = os.Remove(tmpName)
			if hashErr != nil {
				return false, fmt.Errorf("workspace move: failed to inspect destination race at %s: %w", dst, hashErr)
			}
			if existingHash == expectedHash {
				return false, nil
			}
			return false, fmt.Errorf("workspace move aborted: destination file appeared while copying, refusing to overwrite: %s", dst)
		}
		_ = os.Remove(tmpName)
		return false, fmt.Errorf("workspace move: failed to publish %s without overwrite: %w", dst, err)
	}
	if err := os.Remove(tmpName); err != nil {
		return false, fmt.Errorf("workspace move: failed to remove linked temp file %s: %w", tmpName, err)
	}
	_ = util.SyncDir(dir)
	return true, nil
}
