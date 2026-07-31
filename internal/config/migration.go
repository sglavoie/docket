package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/util"
)

// projectsDirName is the subdirectory under a data directory that holds
// per-project task data. Only project paths under the legacy data dir's
// projects/ subtree are re-rooted during migration.
const projectsDirName = "projects"

// migrationRootExcludedNames are machine-local entries that must not be copied
// into durable workspace snapshots or moves.
var migrationRootExcludedNames = map[string]bool{
	"logs":               true,
	"pid.lock":           true,
	machineStateFileName: true,
	"runtime":            true,
	".runtime":           true,
}

func isMigrationRootExcluded(rel string) bool {
	clean := filepath.Clean(rel)
	if filepath.Dir(clean) != "." {
		return false
	}
	return migrationRootExcludedNames[clean]
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
