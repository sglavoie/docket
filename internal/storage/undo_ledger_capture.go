package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

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
//   - an attachment copy: the destination file (absent → source bytes);
//   - an attachment delete: one image per file (bytes → absent);
//   - an attachment move: two images per file (source loses its bytes; the
//     destination gains them), so a reversal relocates the bytes back.
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
	// A copy fails closed on an existing destination, so its before state is
	// always absent: undo simply removes the ingested file.
	return []LedgerArtifact{{Path: destPath, AfterExists: true, After: data}}, nil
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
	var out []LedgerArtifact
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
	var out []LedgerArtifact
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("failed to read attachment for undo image: %w", err)
		}
		out = append(out, LedgerArtifact{Path: f, BeforeExists: true, Before: data})
	}
	return out, nil
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
