package config

import (
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

// projectsDirName is the subdirectory under a data directory that holds
// per-project task data. Only project paths under the legacy data dir's
// projects/ subtree are re-rooted during migration.
const projectsDirName = "projects"

// migrationRootExcludedNames are top-level entry names that must never be
// copied out of the legacy ~/.tt root during migration. These hold
// machine-local runtime state (logs and TUI PID locks) that must stay local and
// never sync to iCloud Drive; matching names below projects/ and other durable
// subtrees are copied normally.
var migrationRootExcludedNames = map[string]bool{
	"logs":     true,
	"pid.lock": true,
	"runtime":  true,
	".runtime": true,
}

func isMigrationRootExcluded(rel string) bool {
	clean := filepath.Clean(rel)
	if filepath.Dir(clean) != "." {
		return false
	}
	return migrationRootExcludedNames[clean]
}

// MigrateCopyTree recursively copies the durable contents of srcRoot into
// dstRoot without overwriting any existing destination file. Runtime-only
// entries (logs directories, TUI PID locks, runtime dirs) are skipped so they
// stay machine-local. Conflicts are refused: if any regular file that would be
// written already exists at the destination, no files are copied and an
// actionable error is returned so the caller can abort before mutating state.
//
// This helper implements the copy half of the copy-then-switch migration. It
// is intentionally not wired into the manager load path yet.
func MigrateCopyTree(srcRoot, dstRoot string) error {
	type entry struct {
		src  string
		dst  string
		dir  bool
		perm fs.FileMode
	}

	var entries []entry
	walkErr := filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(srcRoot, path)
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
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		entries = append(entries, entry{
			src:  path,
			dst:  filepath.Join(dstRoot, rel),
			dir:  d.IsDir(),
			perm: info.Mode().Perm(),
		})
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("failed to scan legacy data dir %s: %w", srcRoot, walkErr)
	}

	// Conflict pre-check: refuse to overwrite any existing destination file.
	// Doing this before copying anything keeps the operation all-or-nothing.
	for _, e := range entries {
		if e.dir {
			continue
		}
		if _, err := os.Lstat(e.dst); err == nil {
			return fmt.Errorf("migration aborted: destination file already exists, refusing to overwrite: %s", e.dst)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to check destination %s: %w", e.dst, err)
		}
	}

	if err := os.MkdirAll(dstRoot, util.DirPermissions); err != nil {
		return fmt.Errorf("failed to create destination data dir %s: %w", dstRoot, err)
	}

	// WalkDir yields parents before children in lexical order, so directories
	// are created ahead of the files they contain.
	for _, e := range entries {
		if e.dir {
			if err := os.MkdirAll(e.dst, e.perm); err != nil {
				return fmt.Errorf("failed to create dir %s: %w", e.dst, err)
			}
			continue
		}
		if err := copyFileNoOverwrite(e.src, e.dst, e.perm); err != nil {
			return err
		}
	}
	return nil
}

// copyFileNoOverwrite copies src to dst, failing if dst already exists. O_EXCL
// enforces the no-overwrite guarantee even against a concurrent writer, so the
// pre-check in MigrateCopyTree and this call together never clobber data.
func copyFileNoOverwrite(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("failed to copy %s to %s: %w", src, dst, err)
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return fmt.Errorf("failed to sync destination file %s: %w", dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("failed to close destination file %s: %w", dst, err)
	}
	return nil
}

// RewriteMigratedConfig rewrites data_dir and legacy project paths in raw config
// JSON so a config file copied from the legacy ~/.tt root points at the new data
// directory. Only legacy default values are rewritten:
//
//   - data_dir is set to newDataDir only when it is missing, empty, or exactly
//     the legacy data dir. A custom (non-legacy) data_dir is preserved.
//   - each project whose path is under legacyDataDir/projects/ is re-rooted under
//     newDataDir/projects/ with its relative subpath preserved. Project paths
//     outside that subtree (custom locations) are left untouched.
//
// Unknown top-level JSON keys and unknown per-project keys are preserved. The
// returned bytes are indented to match the manager's Save output.
func RewriteMigratedConfig(data []byte, legacyDataDir, newDataDir string) ([]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config for migration rewrite: %w", err)
	}
	if raw == nil {
		return nil, fmt.Errorf("failed to parse config for migration rewrite: unexpected null document")
	}

	if err := rewriteDataDir(raw, legacyDataDir, newDataDir); err != nil {
		return nil, err
	}
	if err := rewriteProjectPaths(raw, legacyDataDir, newDataDir); err != nil {
		return nil, err
	}

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal migrated config: %w", err)
	}
	return out, nil
}

// WriteMigratedConfig reads the config at path, rewrites its legacy default
// paths via RewriteMigratedConfig, and writes the result back atomically.
func WriteMigratedConfig(path, legacyDataDir, newDataDir string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read migrated config %s: %w", path, err)
	}
	rewritten, err := RewriteMigratedConfig(data, legacyDataDir, newDataDir)
	if err != nil {
		return err
	}
	if err := util.AtomicWriteFile(path, rewritten, FilePermissions); err != nil {
		return fmt.Errorf("failed to write migrated config %s: %w", path, err)
	}
	return nil
}

// rewriteDataDir updates the top-level data_dir key in place when it is a legacy
// default value (missing, empty, or exactly the legacy data dir).
func rewriteDataDir(raw map[string]json.RawMessage, legacyDataDir, newDataDir string) error {
	var current string
	if msg, ok := raw["data_dir"]; ok {
		if err := json.Unmarshal(msg, &current); err != nil {
			return fmt.Errorf("failed to parse data_dir for migration: %w", err)
		}
	}
	if current != "" && filepath.Clean(current) != filepath.Clean(legacyDataDir) {
		// Custom data_dir: leave untouched.
		return nil
	}
	encoded, err := json.Marshal(newDataDir)
	if err != nil {
		return fmt.Errorf("failed to encode migrated data_dir: %w", err)
	}
	raw["data_dir"] = encoded
	return nil
}

// rewriteProjectPaths re-roots each project path under the legacy projects
// subtree onto the new projects subtree, preserving every other project field.
func rewriteProjectPaths(raw map[string]json.RawMessage, legacyDataDir, newDataDir string) error {
	msg, ok := raw["projects"]
	if !ok {
		return nil
	}
	var projects []json.RawMessage
	if err := json.Unmarshal(msg, &projects); err != nil {
		// Not an array (e.g. null) - nothing to rewrite.
		return nil //nolint:nilerr // a non-array projects value is a no-op, not a failure
	}

	legacyProjectsRoot := filepath.Join(legacyDataDir, projectsDirName)
	newProjectsRoot := filepath.Join(newDataDir, projectsDirName)

	changed := false
	for i, projRaw := range projects {
		var proj map[string]json.RawMessage
		if err := json.Unmarshal(projRaw, &proj); err != nil || proj == nil {
			continue
		}
		pathMsg, ok := proj["path"]
		if !ok {
			continue
		}
		var path string
		if err := json.Unmarshal(pathMsg, &path); err != nil || path == "" {
			continue
		}
		newPath, ok := rerootLegacyProjectPath(path, legacyProjectsRoot, newProjectsRoot)
		if !ok {
			continue
		}
		encoded, err := json.Marshal(newPath)
		if err != nil {
			return fmt.Errorf("failed to encode migrated project path: %w", err)
		}
		proj["path"] = encoded
		rebuilt, err := json.Marshal(proj)
		if err != nil {
			return fmt.Errorf("failed to re-marshal migrated project: %w", err)
		}
		projects[i] = rebuilt
		changed = true
	}

	if !changed {
		return nil
	}
	encoded, err := json.Marshal(projects)
	if err != nil {
		return fmt.Errorf("failed to re-marshal migrated projects: %w", err)
	}
	raw["projects"] = encoded
	return nil
}

// rerootLegacyProjectPath returns the re-rooted path and true when path lives
// under legacyProjectsRoot; otherwise it returns false so custom project paths
// are left untouched.
func rerootLegacyProjectPath(path, legacyProjectsRoot, newProjectsRoot string) (string, bool) {
	rel, err := filepath.Rel(legacyProjectsRoot, filepath.Clean(path))
	if err != nil {
		return "", false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return filepath.Join(newProjectsRoot, rel), true
}
