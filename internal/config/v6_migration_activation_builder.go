package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

type v6ActivationBuilder struct {
	dataDir          string
	projectSourceDir string
	projectTargetDir string
	artifacts        []storage.ArtifactChange
	attachmentMoves  []transaction.Op
	cleanupFiles     map[string]bool
	cleanupDirs      map[string]bool
	artifactTargets  map[string]bool
	moveSources      map[string]bool
	moveTargets      map[string]bool
	projectMove      *transaction.Op
}

func newV6ActivationBuilder(dataDir, projectSourceDir, projectTargetDir string) *v6ActivationBuilder {
	return &v6ActivationBuilder{
		dataDir:          filepath.Clean(dataDir),
		projectSourceDir: filepath.Clean(projectSourceDir),
		projectTargetDir: filepath.Clean(projectTargetDir),
		cleanupFiles:     make(map[string]bool),
		cleanupDirs:      make(map[string]bool),
		artifactTargets:  make(map[string]bool),
		moveSources:      make(map[string]bool),
		moveTargets:      make(map[string]bool),
	}
}

func (builder *v6ActivationBuilder) addArtifact(candidate V6WorkspaceArtifactCandidate) error {
	sourcePath := filepath.Clean(candidate.SourcePath)
	targetPath := filepath.Clean(candidate.Path)
	effectiveTarget, carried := rebaseV6ActivationPath(
		targetPath,
		builder.projectTargetDir,
		builder.projectSourceDir,
	)
	if sourcePath != targetPath && (!carried || effectiveTarget != sourcePath) {
		return fmt.Errorf(
			"v6 activation cannot publish %s relocation %s -> %s outside the protected project move",
			candidate.Kind,
			sourcePath,
			targetPath,
		)
	}
	if !carried {
		effectiveTarget = targetPath
	}
	if builder.artifactTargets[effectiveTarget] || builder.cleanupFiles[effectiveTarget] || builder.moveTargets[effectiveTarget] {
		return fmt.Errorf("v6 migration repeats artifact target %s", effectiveTarget)
	}
	if err := RefusePreConsolidationV6SnapshotMutation(builder.dataDir, effectiveTarget); err != nil {
		return err
	}

	expectedRevision := ""
	if candidate.SourceExists {
		source, err := readV5DryRunSource(sourcePath)
		if err != nil {
			return err
		}
		expectedRevision = storage.RawArtifactRevision(source)
	} else if _, found, err := readOptionalV5DryRunSource(sourcePath); err != nil {
		return err
	} else if found {
		return fmt.Errorf("v6 migration expected source to be absent: %s", sourcePath)
	}

	change := storage.ArtifactChange{
		Path:             effectiveTarget,
		Data:             append([]byte(nil), candidate.Data...),
		Mode:             FilePermissions,
		ExpectedRevision: &expectedRevision,
	}
	if sourcePath != effectiveTarget {
		change.ExpectedRevisionPath = sourcePath
	}
	builder.artifactTargets[effectiveTarget] = true
	builder.artifacts = append(builder.artifacts, change)
	return nil
}

func (builder *v6ActivationBuilder) addAttachmentMove(candidate V6RetainedAttachmentCandidate) error {
	sourcePath := filepath.Clean(candidate.SourcePath)
	targetPath, carried := rebaseV6ActivationPath(
		candidate.Path,
		builder.projectTargetDir,
		builder.projectSourceDir,
	)
	if !carried {
		targetPath = filepath.Clean(candidate.Path)
	}
	if sourcePath == targetPath {
		return nil
	}
	if builder.moveSources[sourcePath] || builder.moveTargets[targetPath] ||
		builder.artifactTargets[sourcePath] || builder.artifactTargets[targetPath] ||
		builder.cleanupFiles[sourcePath] || builder.cleanupFiles[targetPath] {
		return fmt.Errorf("v6 migration attachment path collision %s -> %s", sourcePath, targetPath)
	}
	if _, err := os.Lstat(targetPath); err == nil {
		return fmt.Errorf("v6 migration attachment target already exists: %s", targetPath)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect v6 attachment target %s: %w", targetPath, err)
	}
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("inspect v6 attachment source %s: %w", sourcePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("v6 attachment source is not a regular file: %s", sourcePath)
	}
	builder.moveSources[sourcePath] = true
	builder.moveTargets[targetPath] = true
	builder.attachmentMoves = append(builder.attachmentMoves, transaction.Op{
		Kind: transaction.OpMove, Source: sourcePath, Target: targetPath,
	})
	builder.cleanupDirs[filepath.Dir(sourcePath)] = true
	return nil
}

func (builder *v6ActivationBuilder) addOptionalFileDelete(path string) error {
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect optional v6 migration cleanup file %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("v6 migration cleanup target is not a regular file: %s", path)
	}
	if builder.artifactTargets[path] || builder.moveSources[path] || builder.moveTargets[path] {
		return fmt.Errorf("v6 migration cannot both retain and delete %s", path)
	}
	if err := RefusePreConsolidationV6SnapshotMutation(builder.dataDir, path); err != nil {
		return err
	}
	builder.cleanupFiles[path] = true
	return nil
}

func (builder *v6ActivationBuilder) addCleanupTree(root string) error {
	root = filepath.Clean(root)
	info, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect v6 migration cleanup tree %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("v6 migration refuses symlink cleanup tree: %s", root)
	}
	if !info.IsDir() {
		return builder.addOptionalFileDelete(root)
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		path = filepath.Clean(path)
		if err := RefusePreConsolidationV6SnapshotMutation(builder.dataDir, path); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("v6 migration refuses symlink cleanup target: %s", path)
		}
		if entry.IsDir() {
			builder.cleanupDirs[path] = true
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("v6 migration cleanup target is not regular: %s", path)
		}
		if builder.artifactTargets[path] || builder.moveSources[path] || builder.moveTargets[path] {
			return fmt.Errorf("v6 migration cannot both retain and delete %s", path)
		}
		builder.cleanupFiles[path] = true
		return nil
	})
}

func (builder *v6ActivationBuilder) addProtectedProjectMove() error {
	if builder.projectSourceDir == builder.projectTargetDir {
		return nil
	}
	info, err := os.Lstat(builder.projectSourceDir)
	if err != nil {
		return fmt.Errorf("inspect protected v6 project source %s: %w", builder.projectSourceDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("protected v6 project source is not a regular directory: %s", builder.projectSourceDir)
	}
	if _, err := os.Lstat(builder.projectTargetDir); err == nil {
		return fmt.Errorf("schema-6 Unfiled filesystem target already exists: %s", builder.projectTargetDir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect schema-6 Unfiled filesystem target %s: %w", builder.projectTargetDir, err)
	}
	move := transaction.Op{
		Kind: transaction.OpMove, Source: builder.projectSourceDir, Target: builder.projectTargetDir,
	}
	builder.projectMove = &move
	return nil
}

func (builder *v6ActivationBuilder) finish() (v6MigrationActivationPlan, error) {
	if len(builder.artifacts) == 0 {
		return v6MigrationActivationPlan{}, fmt.Errorf("v6 migration produced no canonical artifacts")
	}
	sort.Slice(builder.artifacts, func(i, j int) bool {
		return builder.artifacts[i].Path < builder.artifacts[j].Path
	})
	sort.Slice(builder.attachmentMoves, func(i, j int) bool {
		if builder.attachmentMoves[i].Source != builder.attachmentMoves[j].Source {
			return builder.attachmentMoves[i].Source < builder.attachmentMoves[j].Source
		}
		return builder.attachmentMoves[i].Target < builder.attachmentMoves[j].Target
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

	ops := append([]transaction.Op(nil), builder.attachmentMoves...)
	for _, path := range files {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: path})
	}
	for _, path := range dirs {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: path})
	}
	if builder.projectMove != nil {
		ops = append(ops, *builder.projectMove)
	}
	return v6MigrationActivationPlan{artifacts: builder.artifacts, cleanupOps: ops}, nil
}

func rebaseV6ActivationPath(path, fromDir, toDir string) (string, bool) {
	path = filepath.Clean(path)
	fromDir = filepath.Clean(fromDir)
	rel, err := filepath.Rel(fromDir, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.Join(filepath.Clean(toDir), rel), true
}
