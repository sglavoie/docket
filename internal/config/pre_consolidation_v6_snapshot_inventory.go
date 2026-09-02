package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/storage"
)

//nolint:govet // Walk callbacks use scoped errors independently of the outer traversal result.
func inventoryPreConsolidationV6(
	root string,
	machinePath string,
	captureData bool,
) (*preConsolidationV6Inventory, error) {
	inventory := &preConsolidationV6Inventory{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
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
		if err := validatePreSimplificationV5RelativePath(rel); err != nil {
			return fmt.Errorf("unsafe pre-consolidation v6 source path %s: %w", path, err)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("pre-consolidation v6 snapshot refuses symlink source: %s", path)
		}
		if isPreConsolidationV6Excluded(rel) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect pre-consolidation v6 source %s: %w", path, err)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("pre-consolidation v6 snapshot refuses non-regular source: %s", path)
		}
		source, err := readPreConsolidationV6Source(
			path,
			filepath.ToSlash(filepath.Join("workspace", rel)),
			preConsolidationV6Kind(rel),
			captureData,
		)
		if err != nil {
			return err
		}
		inventory.files = append(inventory.files, source)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inventory workspace for pre-consolidation v6 snapshot: %w", err)
	}

	workspaceID, err := preSimplificationV5WorkspaceID(root)
	if err != nil {
		return nil, err
	}
	var machineCandidates []string
	if strings.TrimSpace(machinePath) != "" {
		machineCandidates = []string{machinePath, machinePath + ".bak"}
	}
	for _, candidate := range machineCandidates {
		info, statErr := os.Lstat(candidate)
		if errors.Is(statErr, fs.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return nil, fmt.Errorf("inspect pre-consolidation v6 machine source %s: %w", candidate, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("pre-consolidation v6 machine source is not a regular file: %s", candidate)
		}
		archiveName := machineStateFileName
		if strings.HasSuffix(candidate, ".bak") {
			archiveName += ".bak"
		}
		source, err := readPreConsolidationV6Source(
			candidate,
			filepath.ToSlash(filepath.Join("machine", runtimeDirName, workspaceID, archiveName)),
			"machine state",
			captureData,
		)
		if err != nil {
			return nil, err
		}
		inventory.files = append(inventory.files, source)
	}

	sort.Slice(inventory.files, func(left, right int) bool {
		if inventory.files[left].Path != inventory.files[right].Path {
			return inventory.files[left].Path < inventory.files[right].Path
		}
		return inventory.files[left].sourcePath < inventory.files[right].sourcePath
	})
	for index := 1; index < len(inventory.files); index++ {
		if inventory.files[index-1].Path == inventory.files[index].Path {
			return nil, fmt.Errorf(
				"pre-consolidation v6 archive path collision %q from %s and %s",
				inventory.files[index].Path,
				inventory.files[index-1].sourcePath,
				inventory.files[index].sourcePath,
			)
		}
	}
	return inventory, nil
}

func isPreConsolidationV6Excluded(rel string) bool {
	clean := filepath.Clean(rel)
	if isMigrationRootExcluded(clean) {
		return true
	}
	if clean == filepath.Join(dataDirName, "backups") {
		return true
	}
	base := filepath.Base(clean)
	return strings.HasSuffix(base, ".lock") ||
		strings.Contains(base, ".tmp.") ||
		strings.HasPrefix(base, ".tt-move-")
}

func preConsolidationV6Kind(rel string) string {
	clean := filepath.ToSlash(filepath.Clean(rel))
	base := filepath.Base(clean)
	switch {
	case clean == ".tt/workspace.json", clean == ".tt/planning.json":
		return "synced domain"
	case strings.HasPrefix(clean, ".tt/transactions/"):
		return "transaction state"
	case strings.HasPrefix(clean, ".tt/undo/"):
		return "undo ledger"
	case strings.HasPrefix(clean, ".tt/global-notes/"), strings.Contains(clean, "/notes/"):
		return "note"
	case strings.Contains(clean, "/.attachments/"):
		return "attachment"
	case base == "tasks.md":
		return "project document"
	default:
		return "workspace data"
	}
}

func readPreConsolidationV6Source(
	path string,
	archivePath string,
	kind string,
	captureData bool,
) (preConsolidationV6Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return preConsolidationV6Source{}, fmt.Errorf("read pre-consolidation v6 source %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	source := preConsolidationV6Source{
		PreConsolidationV6SnapshotFile: PreConsolidationV6SnapshotFile{
			Path: archivePath, Kind: kind, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]),
		},
		sourcePath: filepath.Clean(path),
	}
	if captureData {
		source.data = data
	}
	return source, nil
}

func comparePreConsolidationV6Inventories(
	expected *preConsolidationV6Inventory,
	current *preConsolidationV6Inventory,
) error {
	if len(expected.files) != len(current.files) {
		return fmt.Errorf("pre-consolidation v6 snapshot aborted: source inventory changed while snapshotting")
	}
	for index := range expected.files {
		left, right := expected.files[index], current.files[index]
		if left.Path != right.Path || left.sourcePath != right.sourcePath ||
			left.Size != right.Size || left.SHA256 != right.SHA256 {
			return fmt.Errorf("pre-consolidation v6 snapshot aborted: source changed while snapshotting: %s", left.sourcePath)
		}
	}
	return nil
}

func collectPreConsolidationV6TaskFacts(
	plan V6WorkspaceConversionPlan,
	inventory *preConsolidationV6Inventory,
) ([]preConsolidationV6TaskFact, error) {
	bySourcePath := make(map[string][]byte, len(inventory.files))
	for _, source := range inventory.files {
		bySourcePath[source.sourcePath] = source.data
	}
	var facts []preConsolidationV6TaskFact
	for _, project := range plan.Projects {
		if !project.SourceExists {
			continue
		}
		data, exists := bySourcePath[filepath.Clean(project.SourcePath)]
		if !exists {
			return nil, fmt.Errorf("validated project %q is missing from the captured snapshot inventory", project.ProjectID)
		}
		document, err := storage.ParseV5ProjectDocument(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse captured v5 project %q for migration audit: %w", project.ProjectID, err)
		}
		for _, task := range document.Tasks {
			facts = append(facts, preConsolidationV6TaskFact{
				projectID: project.ProjectID,
				taskID:    task.ID,
				state:     task.State,
				priority:  task.Priority,
				decisions: len(task.Decisions),
			})
		}
	}
	return facts, nil
}
