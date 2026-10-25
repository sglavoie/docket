package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/util"
)

func validatePreSimplificationV5ContainedFile(root, path string, info fs.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("pre-simplification v5 snapshot refuses symlink source: %s", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("pre-simplification v5 snapshot refuses non-regular source: %s", path)
	}
	if !util.PathWithinRoot(root, path) {
		return fmt.Errorf("machine-state source escapes the local runtime root: %s", path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	current := root
	for _, part := range strings.Split(filepath.Clean(rel), string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		partInfo, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("inspect machine-state path component %s: %w", current, err)
		}
		if partInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("pre-simplification v5 snapshot refuses symlink source: %s", current)
		}
	}
	return nil
}

func validatePreSimplificationV5RelativePath(rel string) error {
	clean := filepath.Clean(rel)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path is not a safe relative child: %q", rel)
	}
	if strings.ContainsAny(clean, "\x00\r\n") {
		return fmt.Errorf("path contains a control character: %q", rel)
	}
	return nil
}

func sortPreSimplificationV5Sources(sources []preSimplificationV5Source) {
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Path != sources[j].Path {
			return sources[i].Path < sources[j].Path
		}
		return sources[i].sourcePath < sources[j].sourcePath
	})
}

func rejectDuplicatePreSimplificationV5Paths(inventory *preSimplificationV5Inventory) error {
	seen := make(map[string]string, len(inventory.archived)+len(inventory.attachments))
	for _, sources := range [][]preSimplificationV5Source{inventory.archived, inventory.attachments} {
		for _, source := range sources {
			if previous, ok := seen[source.Path]; ok && previous != source.sourcePath {
				return fmt.Errorf("snapshot archive path collision %q from %s and %s", source.Path, previous, source.sourcePath)
			}
			seen[source.Path] = source.sourcePath
		}
	}
	return nil
}

func comparePreSimplificationV5Inventories(expected, current *preSimplificationV5Inventory) error {
	if err := comparePreSimplificationV5SourceSet("archived", expected.archived, current.archived); err != nil {
		return err
	}
	return comparePreSimplificationV5SourceSet("attachment", expected.attachments, current.attachments)
}

func comparePreSimplificationV5SourceSet(kind string, expected, current []preSimplificationV5Source) error {
	if len(expected) != len(current) {
		return fmt.Errorf("pre-simplification v5 snapshot aborted: %s inventory changed while snapshotting", kind)
	}
	for i := range expected {
		if expected[i].Path != current[i].Path ||
			expected[i].sourcePath != current[i].sourcePath ||
			expected[i].Size != current[i].Size ||
			expected[i].SHA256 != current[i].SHA256 {
			return fmt.Errorf("pre-simplification v5 snapshot aborted: source changed while snapshotting: %s", expected[i].sourcePath)
		}
	}
	return nil
}
