package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sglavoie/docket/internal/transaction"
)

// captureOpsLedgerImages records the net topology transition of a raw
// lifecycle transaction plus its typed artifact rewrites. Raw operations are
// captured first and artifacts overlay their final bytes, which is important
// for a rename that moves an old tasks.md and then rewrites it under the new
// project directory.
func captureOpsLedgerImages(ops []transaction.Op, artifacts []ArtifactChange) ([]LedgerArtifact, error) {
	images := make(map[string]LedgerArtifact)
	merge := func(next LedgerArtifact) error {
		path := filepath.Clean(next.Path)
		next.Path = path
		current, ok := images[path]
		if !ok {
			images[path] = next
			return nil
		}
		currentKind := current.Kind
		if currentKind == "" {
			currentKind = ArtifactImageKindFile
		}
		nextKind := next.Kind
		if nextKind == "" {
			nextKind = ArtifactImageKindFile
		}
		if currentKind != nextKind {
			return fmt.Errorf("storage: lifecycle undo target %q changes image kind", path)
		}
		current.AfterExists = next.AfterExists
		current.After = next.After
		images[path] = current
		return nil
	}

	for _, op := range ops {
		captured, err := captureRawOpLedgerImages(op)
		if err != nil {
			return nil, err
		}
		for _, image := range captured {
			if err := merge(image); err != nil {
				return nil, err
			}
		}
	}
	artifactImages, err := captureArtifactLedgerImages(artifacts)
	if err != nil {
		return nil, err
	}
	for _, image := range artifactImages {
		if err := merge(image); err != nil {
			return nil, err
		}
	}

	paths := make([]string, 0, len(images))
	for path := range images {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := make([]LedgerArtifact, 0, len(paths))
	for _, path := range paths {
		out = append(out, images[path])
	}
	return out, nil
}

func captureRawOpLedgerImages(op transaction.Op) ([]LedgerArtifact, error) {
	switch op.Kind {
	case transaction.OpWrite:
		before, exists, err := readFileMaybe(op.Target)
		if err != nil {
			return nil, err
		}
		out := []LedgerArtifact{{
			Path:         op.Target,
			BeforeExists: exists,
			Before:       before,
			AfterExists:  true,
			After:        op.Data,
		}}
		parent := filepath.Dir(filepath.Clean(op.Target))
		parentExists, err := directoryExistsMaybe(parent)
		if err != nil {
			return nil, err
		}
		if !parentExists {
			out = append(out, LedgerArtifact{
				Kind: ArtifactImageKindDirectory, Path: parent, AfterExists: true,
			})
		}
		return out, nil
	case transaction.OpDelete:
		return capturePathTransition(op.Target, "", false)
	case transaction.OpMove:
		return capturePathTransition(op.Source, op.Target, true)
	default:
		return nil, fmt.Errorf("storage: unsupported lifecycle operation kind %q", op.Kind)
	}
}

// capturePathTransition flattens a file or directory tree into byte images.
// A move maps every source descendant to the corresponding target descendant;
// a delete records only the source before-images.
func capturePathTransition(source, target string, move bool) ([]LedgerArtifact, error) {
	source = filepath.Clean(source)
	target = filepath.Clean(target)
	var out []LedgerArtifact
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) && path == source {
				return nil
			}
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("storage: lifecycle undo does not support symlink %q", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := ""
		if move {
			destination = target
			if rel != "." {
				destination = filepath.Join(target, rel)
			}
		}
		if entry.IsDir() {
			out = append(out, LedgerArtifact{
				Kind: ArtifactImageKindDirectory, Path: path, BeforeExists: true,
			})
			if move {
				exists, existsErr := directoryExistsMaybe(destination)
				if existsErr != nil {
					return existsErr
				}
				if exists {
					return fmt.Errorf("storage: lifecycle move target already exists: %s", destination)
				}
				out = append(out, LedgerArtifact{
					Kind: ArtifactImageKindDirectory, Path: destination, AfterExists: true,
				})
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("storage: lifecycle undo does not support non-regular file %q", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out = append(out, LedgerArtifact{Path: path, BeforeExists: true, Before: data})
		if move {
			if _, exists, err := readFileMaybe(destination); err != nil {
				return err
			} else if exists {
				return fmt.Errorf("storage: lifecycle move target already exists: %s", destination)
			}
			out = append(out, LedgerArtifact{Path: destination, AfterExists: true, After: data})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func directoryExistsMaybe(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// captureArtifactLedgerImages captures the byte-exact state transition for an
// artifact-only transaction. It runs after every optimistic revision has been
// validated and while every artifact lock is held, so each before-image is the
// same state the journal is about to replace or delete. Writes carry their exact
// requested bytes as the after-image; deletes carry an absent after-state.
func captureArtifactLedgerImages(artifacts []ArtifactChange) ([]LedgerArtifact, error) {
	out := make([]LedgerArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		before, beforeExists, err := readFileMaybe(artifact.Path)
		if err != nil {
			return nil, err
		}
		image := LedgerArtifact{
			Path:         artifact.Path,
			BeforeExists: beforeExists,
			Before:       before,
		}
		if !artifact.Delete {
			image.AfterExists = true
			image.After = artifact.Data
		}
		out = append(out, image)
	}
	return out, nil
}

// captureLedgerArtifacts expands a ChangeSet into the flat list of per-file
// before/after images the workspace undo ledger retains. It runs under the
// commit locks BEFORE the transaction applies, so every before-image is the real
// pre-mutation content and an attachment delete or move can be reversed byte for
// byte. content is the tasks.md the commit is about to write.
//
// The mapping mirrors how the transaction resolves each side effect, but as pure
// file images so undo and redo are a uniform "make each path match its recorded
// state":
//   - the project tasks.md: before = current bytes (if any), after = content;
//   - each artifact: before = current bytes (if any), after = data (or absent
//     for a delete);
//   - an attachment copy: the destination file (absent → source bytes), plus
//     the task directory when this is its first file;
//   - an attachment delete: one image per file and the task directory;
//   - an attachment move: source/destination file images plus both task
//     directories, so reversal restores bytes and meaningful topology.
func captureLedgerArtifacts(cs ChangeSet, content string) ([]LedgerArtifact, error) {
	var out []LedgerArtifact

	tasksPath := tasksFilePath(cs.Project.Path)
	before, beforeExists, err := readFileMaybe(tasksPath)
	if err != nil {
		return nil, err
	}
	out = append(out, LedgerArtifact{
		Path:         tasksPath,
		BeforeExists: beforeExists,
		Before:       before,
		AfterExists:  true,
		After:        []byte(content),
	})

	for _, artifact := range cs.Artifacts {
		before, beforeExists, err := readFileMaybe(artifact.Path)
		if err != nil {
			return nil, err
		}
		img := LedgerArtifact{Path: artifact.Path, BeforeExists: beforeExists, Before: before}
		if !artifact.Delete {
			img.AfterExists = true
			img.After = artifact.Data
		}
		out = append(out, img)
	}

	for i := range cs.AttachmentOps {
		imgs, err := captureAttachmentOpImages(cs.AttachmentOps[i])
		if err != nil {
			return nil, err
		}
		out = append(out, imgs...)
	}

	return out, nil
}

// captureAttachmentOpImages turns one attachment side effect into its before/after
// file images, read under the commit locks before the operation applies.
func captureAttachmentOpImages(op AttachmentOp) ([]LedgerArtifact, error) {
	switch op.Kind {
	case AttachmentCopy:
		return captureAttachmentCopyImage(op)
	case AttachmentMove:
		return captureAttachmentMoveImages(op)
	case AttachmentDelete:
		return captureAttachmentDeleteImages(op)
	default:
		return nil, fmt.Errorf("storage: unknown attachment op kind %d", op.Kind)
	}
}

func captureAttachmentCopyImage(op AttachmentOp) ([]LedgerArtifact, error) {
	if op.DestProject == nil {
		return nil, fmt.Errorf("storage: attachment copy requires a destination project")
	}
	filename := op.Filename
	if filename == "" {
		filename = filepath.Base(op.SourcePath)
	}
	destPath, err := resolveAttachmentPath(op.DestProject, op.Task, filename)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(op.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read attachment source for undo image: %w", err)
	}
	destDir := filepath.Dir(destPath)
	dirExists, err := directoryExists(destDir)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect attachment directory for undo image: %w", err)
	}
	var out []LedgerArtifact
	if !dirExists {
		out = append(out, LedgerArtifact{
			Kind: ArtifactImageKindDirectory, Path: destDir, AfterExists: true,
		})
	}
	// A copy fails closed on an existing destination, so its before state is
	// always absent: undo removes the ingested file and, for a first copy, the
	// newly created task directory after the file delete.
	return append(out, LedgerArtifact{Path: destPath, AfterExists: true, After: data}), nil
}

func captureAttachmentMoveImages(op AttachmentOp) ([]LedgerArtifact, error) {
	if op.SourceProject == nil || op.DestProject == nil {
		return nil, fmt.Errorf("storage: attachment move requires source and destination projects")
	}
	srcDir, err := resolveTaskAttachmentsDir(op.SourceProject, op.Task)
	if err != nil {
		return nil, err
	}
	destDir, err := resolveTaskAttachmentsDir(op.DestProject, op.Task)
	if err != nil {
		return nil, err
	}
	files, exists, err := listTaskAttachmentFiles(srcDir)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	// Empty task attachment directories contain no user data. Moving one removes
	// the source but creates no destination, and undo deliberately does not
	// recreate the empty source.
	if len(files) == 0 {
		return nil, nil
	}
	out := []LedgerArtifact{
		{Kind: ArtifactImageKindDirectory, Path: srcDir, BeforeExists: true},
		{Kind: ArtifactImageKindDirectory, Path: destDir, AfterExists: true},
	}
	for _, srcFile := range files {
		data, err := os.ReadFile(srcFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read attachment for undo image: %w", err)
		}
		destFile := filepath.Join(destDir, filepath.Base(srcFile))
		// Source loses the bytes; destination gains them. Reversing both images
		// relocates the file back to the source project.
		out = append(out,
			LedgerArtifact{Path: srcFile, BeforeExists: true, Before: data},
			LedgerArtifact{Path: destFile, AfterExists: true, After: data},
		)
	}
	return out, nil
}

func captureAttachmentDeleteImages(op AttachmentOp) ([]LedgerArtifact, error) {
	project := op.SourceProject
	if project == nil {
		project = op.DestProject
	}
	if project == nil {
		return nil, fmt.Errorf("storage: attachment delete requires a project")
	}
	if op.Filename != "" {
		// Single-file removal: capture only that file's before-image so undo restores
		// exactly it and redo re-deletes exactly it, leaving sibling attachments in the
		// same directory untouched. Mirrors resolveSingleAttachmentDelete.
		path, err := resolveAttachmentPath(project, op.Task, op.Filename)
		if err != nil {
			return nil, err
		}
		data, exists, err := readFileMaybe(path)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, nil
		}
		return []LedgerArtifact{{Path: path, BeforeExists: true, Before: data}}, nil
	}
	dir, err := resolveTaskAttachmentsDir(project, op.Task)
	if err != nil {
		return nil, err
	}
	files, exists, err := listTaskAttachmentFiles(dir)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	// Empty task directories are non-semantic: the concrete delete removes them,
	// but history does not recreate them on undo.
	if len(files) == 0 {
		return nil, nil
	}
	out := []LedgerArtifact{{
		Kind: ArtifactImageKindDirectory, Path: dir, BeforeExists: true,
	}}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("failed to read attachment for undo image: %w", err)
		}
		out = append(out, LedgerArtifact{Path: f, BeforeExists: true, Before: data})
	}
	return out, nil
}

func directoryExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("path is not a directory: %s", path)
	}
	return true, nil
}

// readFileMaybe reads a file, reporting (nil, false, nil) when it does not exist
// so callers can capture the "absent" before-state without special-casing.
func readFileMaybe(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to read %q for undo image: %w", path, err)
	}
	return data, true, nil
}
