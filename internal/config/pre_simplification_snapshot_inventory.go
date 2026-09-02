package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func inventoryPreSimplificationV5(root string, captureData bool) (*preSimplificationV5Inventory, error) {
	inventory := &preSimplificationV5Inventory{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if validateErr := validatePreSimplificationV5RelativePath(rel); validateErr != nil {
			return fmt.Errorf("unsafe workspace path %s: %w", path, validateErr)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("pre-simplification v5 snapshot refuses symlink source: %s", path)
		}
		if isPreSimplificationV5Excluded(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("inspect snapshot source %s: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("pre-simplification v5 snapshot refuses non-regular source: %s", path)
		}
		archivePath := filepath.ToSlash(filepath.Join("workspace", rel))
		source, err := readPreSimplificationV5Source(path, archivePath, preSimplificationV5Kind(rel), captureData)
		if err != nil {
			return err
		}
		if isPreSimplificationV5Attachment(rel) {
			inventory.attachments = append(inventory.attachments, source)
		} else {
			inventory.archived = append(inventory.archived, source)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inventory workspace for pre-simplification v5 snapshot: %w", err)
	}

	machineSources, err := inventoryPreSimplificationV5MachineState(root, captureData)
	if err != nil {
		return nil, err
	}
	inventory.archived = append(inventory.archived, machineSources...)
	sortPreSimplificationV5Sources(inventory.archived)
	sortPreSimplificationV5Sources(inventory.attachments)
	if err := rejectDuplicatePreSimplificationV5Paths(inventory); err != nil {
		return nil, err
	}
	return inventory, nil
}

func isPreSimplificationV5Excluded(rel string) bool {
	clean := filepath.Clean(rel)
	if isMigrationRootExcluded(clean) {
		return true
	}
	switch clean {
	case filepath.Join(dataDirName, "backups"),
		filepath.Join(dataDirName, "transactions"):
		return true
	}
	// Advisory locks and interrupted atomic-write temporaries are coordination
	// state, not rollback data. Excluding them also keeps repeated snapshots
	// stable after ordinary tt operations.
	base := filepath.Base(clean)
	return strings.HasSuffix(base, ".lock") ||
		strings.Contains(base, ".tmp.") ||
		strings.HasPrefix(base, ".tt-move-")
}

func isPreSimplificationV5Attachment(rel string) bool {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(rel)), "/")
	if len(parts) < 4 || parts[0] != projectsDirName {
		return false
	}
	for _, part := range parts[2:] {
		if part == ".attachments" {
			return true
		}
	}
	return false
}

func preSimplificationV5Kind(rel string) string {
	clean := filepath.ToSlash(filepath.Clean(rel))
	base := filepath.Base(clean)
	switch {
	case clean == configFileName,
		clean == ".tt/workspace.json",
		clean == ".tt/planning.json",
		clean == ".tt/settings.json",
		clean == ".tt/automation.json",
		strings.HasPrefix(clean, ".tt/themes/"),
		strings.HasPrefix(clean, "themes/"):
		return "portable configuration"
	case strings.HasPrefix(clean, ".tt/global-notes/"),
		strings.Contains(clean, "/notes/"):
		return "note"
	case strings.HasPrefix(clean, ".tt/trash/"):
		return "project trash"
	case strings.HasPrefix(base, "completed") && strings.HasSuffix(base, ".md"):
		return "archived task Markdown"
	case base == "tasks.md":
		return "active task Markdown"
	case isPreSimplificationV5Attachment(rel):
		return "attachment retained in place"
	case base == "blockers.md" || base == ".completed.index.json":
		return "project metadata"
	default:
		return "workspace data"
	}
}

func readPreSimplificationV5Source(path, archivePath, kind string, captureData bool) (preSimplificationV5Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return preSimplificationV5Source{}, fmt.Errorf("read snapshot source %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	source := preSimplificationV5Source{
		PreSimplificationV5SnapshotFile: PreSimplificationV5SnapshotFile{
			Path:   archivePath,
			Kind:   kind,
			Size:   int64(len(data)),
			SHA256: hex.EncodeToString(sum[:]),
		},
		sourcePath: path,
	}
	if captureData {
		source.data = data
	}
	return source, nil
}

func inventoryPreSimplificationV5MachineState(dataDir string, captureData bool) ([]preSimplificationV5Source, error) {
	workspaceID, err := preSimplificationV5WorkspaceID(dataDir)
	if err != nil {
		return nil, err
	}
	candidates := legacyMachineStatePaths(dataDir)
	if workspaceID != "" {
		machinePath, pathErr := MachineStatePath(workspaceID)
		if pathErr != nil {
			return nil, pathErr
		}
		candidates = append(candidates, machinePath)
	}

	localRoot, err := filepath.Abs(LocalDataRoot())
	if err != nil {
		return nil, fmt.Errorf("resolve machine-state root: %w", err)
	}
	seen := make(map[string]bool)
	var sources []preSimplificationV5Source
	for _, candidate := range candidates {
		for _, path := range []string{candidate, candidate + ".bak"} {
			path, err = filepath.Abs(filepath.Clean(path))
			if err != nil {
				return nil, fmt.Errorf("resolve machine-state source %s: %w", path, err)
			}
			if seen[path] {
				continue
			}
			seen[path] = true
			info, statErr := os.Lstat(path)
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			if statErr != nil {
				return nil, fmt.Errorf("inspect machine-state source %s: %w", path, statErr)
			}
			if err := validatePreSimplificationV5ContainedFile(localRoot, path, info); err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(localRoot, path)
			if err != nil {
				return nil, fmt.Errorf("relativize machine-state source %s: %w", path, err)
			}
			if validateErr := validatePreSimplificationV5RelativePath(rel); validateErr != nil {
				return nil, fmt.Errorf("unsafe machine-state path %s: %w", path, validateErr)
			}
			source, err := readPreSimplificationV5Source(
				path,
				filepath.ToSlash(filepath.Join("machine", rel)),
				"machine state",
				captureData,
			)
			if err != nil {
				return nil, err
			}
			sources = append(sources, source)
		}
	}
	return sources, nil
}

func preSimplificationV5WorkspaceID(dataDir string) (string, error) {
	path := WorkspaceCatalogPath(dataDir)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read workspace identity for snapshot: %w", err)
	}
	var envelope domainEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", fmt.Errorf("decode workspace identity for snapshot: %w", err)
	}
	if envelope.Kind != domainCatalog.name() {
		return "", fmt.Errorf("workspace identity artifact kind is %q, want %q", envelope.Kind, domainCatalog.name())
	}
	if err := validateWorkspaceID(envelope.WorkspaceID); err != nil {
		return "", fmt.Errorf("workspace identity for snapshot: %w", err)
	}
	return envelope.WorkspaceID, nil
}
