package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

type v5ActivationBuilder struct {
	dataDir      string
	artifacts    []storage.ArtifactChange
	artifactKind map[string]bool
	cleanupFiles map[string]bool
	cleanupDirs  map[string]bool
}

func newV5ActivationBuilder(dataDir string) *v5ActivationBuilder {
	return &v5ActivationBuilder{
		dataDir:      dataDir,
		artifactKind: make(map[string]bool),
		cleanupFiles: make(map[string]bool),
		cleanupDirs:  make(map[string]bool),
	}
}

func (builder *v5ActivationBuilder) addWrite(path string, data []byte) error {
	return builder.addWriteCandidate(path, data, false)
}

func (builder *v5ActivationBuilder) addMachineWrite(path string, data []byte) error {
	return builder.addWriteCandidate(path, data, true)
}

func (builder *v5ActivationBuilder) addWriteCandidate(path string, data []byte, allowCreate bool) error {
	path = filepath.Clean(path)
	if builder.artifactKind[path] || builder.cleanupFiles[path] {
		return fmt.Errorf("v5 migration repeats artifact target %s", path)
	}
	if err := RefusePreSimplificationV5SnapshotMutation(builder.dataDir, path); err != nil {
		return err
	}
	source, found, err := readOptionalV5DryRunSource(path)
	if err != nil {
		return err
	}
	if !found && !allowCreate {
		return fmt.Errorf("inspect v5 conversion source %s: %w", path, fs.ErrNotExist)
	}
	expectedRevision := ""
	if found {
		expectedRevision = storage.RawArtifactRevision(source)
	}
	builder.artifactKind[path] = true
	builder.artifacts = append(builder.artifacts, storage.ArtifactChange{
		Path:             path,
		Data:             append([]byte(nil), data...),
		Mode:             FilePermissions,
		ExpectedRevision: &expectedRevision,
	})
	return nil
}

func (builder *v5ActivationBuilder) addFileDelete(path string) error {
	path = filepath.Clean(path)
	if builder.artifactKind[path] {
		return fmt.Errorf("v5 migration cannot write and delete %s", path)
	}
	if builder.cleanupFiles[path] {
		return nil
	}
	if err := RefusePreSimplificationV5SnapshotMutation(builder.dataDir, path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect v5 migration cleanup file %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("v5 migration cleanup target is not a regular file: %s", path)
	}
	builder.cleanupFiles[path] = true
	return nil
}

func (builder *v5ActivationBuilder) addOptionalFileDelete(path string) error {
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect optional v5 migration cleanup file %s: %w", path, err)
	}
	return builder.addFileDelete(path)
}

func (builder *v5ActivationBuilder) addCleanupTree(root string) error {
	root = filepath.Clean(root)
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect v5 migration cleanup tree %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("v5 migration refuses symlink cleanup tree: %s", root)
	}
	if !info.IsDir() {
		return builder.addFileDelete(root)
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		path = filepath.Clean(path)
		if err := RefusePreSimplificationV5SnapshotMutation(builder.dataDir, path); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("v5 migration refuses symlink cleanup target: %s", path)
		}
		if entry.IsDir() {
			builder.cleanupDirs[path] = true
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("v5 migration cleanup target is not regular: %s", path)
		}
		if builder.artifactKind[path] {
			return fmt.Errorf("v5 migration cannot write and delete %s", path)
		}
		builder.cleanupFiles[path] = true
		return nil
	})
}

func (builder *v5ActivationBuilder) finish() (v5MigrationActivationPlan, error) {
	sort.Slice(builder.artifacts, func(i, j int) bool {
		return builder.artifacts[i].Path < builder.artifacts[j].Path
	})
	files := make([]string, 0, len(builder.cleanupFiles))
	for path := range builder.cleanupFiles {
		files = append(files, path)
	}
	sort.Strings(files)
	dirs := make([]string, 0, len(builder.cleanupDirs))
	for path := range builder.cleanupDirs {
		dirs = append(dirs, path)
	}
	sort.Slice(dirs, func(i, j int) bool {
		leftDepth := strings.Count(filepath.Clean(dirs[i]), string(os.PathSeparator))
		rightDepth := strings.Count(filepath.Clean(dirs[j]), string(os.PathSeparator))
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return dirs[i] < dirs[j]
	})

	ops := make([]transaction.Op, 0, len(files)+len(dirs))
	for _, path := range files {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: path})
	}
	for _, path := range dirs {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: path})
	}
	if len(builder.artifacts) == 0 {
		return v5MigrationActivationPlan{}, fmt.Errorf("v5 migration produced no canonical artifacts")
	}
	return v5MigrationActivationPlan{artifacts: builder.artifacts, cleanupOps: ops}, nil
}
