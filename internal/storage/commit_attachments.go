package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/transaction"
)

// resolveAttachmentOps translates a changeset's attachment side effects into
// concrete transaction operations. Copies stage the source bytes for the
// destination path; a move renames a non-empty task attachment directory (or
// deletes an empty source directory); a delete removes every file and then the
// directory. The returned dirsToCreate name destination .attachments parents
// that a move rename needs in place before commit. Any error aborts the caller's
// whole commit.
func resolveAttachmentOps(ops []AttachmentOp) (txnOps []transaction.Op, dirsToCreate []string, err error) {
	for i := range ops {
		op := ops[i]
		switch op.Kind {
		case AttachmentCopy:
			copyOp, copyErr := resolveAttachmentCopy(op)
			if copyErr != nil {
				return nil, nil, copyErr
			}
			txnOps = append(txnOps, copyOp)
		case AttachmentMove:
			moveOps, destParent, moveErr := resolveAttachmentMove(op)
			if moveErr != nil {
				return nil, nil, moveErr
			}
			if destParent != "" {
				dirsToCreate = append(dirsToCreate, destParent)
			}
			txnOps = append(txnOps, moveOps...)
		case AttachmentDelete:
			delOps, delErr := resolveAttachmentDelete(op)
			if delErr != nil {
				return nil, nil, delErr
			}
			txnOps = append(txnOps, delOps...)
		default:
			return nil, nil, fmt.Errorf("storage: unknown attachment op kind %d", op.Kind)
		}
	}
	return txnOps, dirsToCreate, nil
}

// resolveAttachmentCopy stages an external file's bytes for the task's destination
// attachment path. It rejects an existing destination (duplicate filename) so the
// copy contract matches CopyAttachment: a name already present is an error rather
// than a silent overwrite.
func resolveAttachmentCopy(op AttachmentOp) (transaction.Op, error) {
	if op.DestProject == nil {
		return transaction.Op{}, fmt.Errorf("storage: attachment copy requires a destination project")
	}
	if op.SourcePath == "" {
		return transaction.Op{}, fmt.Errorf("storage: attachment copy requires a source path")
	}
	filename := op.Filename
	if filename == "" {
		filename = filepath.Base(op.SourcePath)
	}
	destPath, err := resolveAttachmentPath(op.DestProject, op.Task, filename)
	if err != nil {
		return transaction.Op{}, err
	}
	if _, statErr := os.Stat(destPath); statErr == nil {
		return transaction.Op{}, fmt.Errorf("attachment %q already exists", filename)
	} else if !os.IsNotExist(statErr) {
		return transaction.Op{}, fmt.Errorf("failed to stat attachment destination: %w", statErr)
	}
	data, err := os.ReadFile(op.SourcePath)
	if err != nil {
		return transaction.Op{}, fmt.Errorf("failed to read attachment source: %w", err)
	}
	return transaction.Op{Kind: transaction.OpWrite, Target: destPath, Data: data, Mode: FilePermissions}, nil
}

// resolveAttachmentMove builds the transaction op that renames a task's whole
// attachments directory to the destination project. It is a no-op when the task
// has no attachment directory, deletes an empty source without creating a
// destination, and fails closed when the destination directory already exists.
// The returned destParent is the destination .attachments directory a non-empty
// rename target needs in place.
func resolveAttachmentMove(op AttachmentOp) (ops []transaction.Op, destParent string, err error) {
	if op.SourceProject == nil || op.DestProject == nil {
		return nil, "", fmt.Errorf("storage: attachment move requires source and destination projects")
	}
	srcDir, err := resolveTaskAttachmentsDir(op.SourceProject, op.Task)
	if err != nil {
		return nil, "", err
	}
	info, statErr := os.Stat(srcDir)
	if os.IsNotExist(statErr) {
		return nil, "", nil
	}
	if statErr != nil {
		return nil, "", fmt.Errorf("failed to stat source attachments directory: %w", statErr)
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("source attachments path is not a directory: %s", srcDir)
	}
	destDir, err := resolveTaskAttachmentsDir(op.DestProject, op.Task)
	if err != nil {
		return nil, "", err
	}
	if _, destStatErr := os.Stat(destDir); destStatErr == nil {
		return nil, "", fmt.Errorf("destination task attachments directory already exists: %s", destDir)
	} else if !os.IsNotExist(destStatErr) {
		return nil, "", fmt.Errorf("failed to stat destination attachments directory: %w", destStatErr)
	}
	files, _, err := listTaskAttachmentFiles(srcDir)
	if err != nil {
		return nil, "", err
	}
	if len(files) == 0 {
		return []transaction.Op{{Kind: transaction.OpDelete, Target: srcDir}}, "", nil
	}
	return []transaction.Op{{Kind: transaction.OpMove, Source: srcDir, Target: destDir}}, filepath.Dir(destDir), nil
}

// resolveAttachmentDelete builds the delete ops for an attachment removal. When
// op.Filename is set it removes just that one file, leaving the directory and any
// sibling attachments intact (the single-attachment `tt attachment remove` case);
// otherwise it removes every file in the task's attachments directory plus the
// directory itself (the whole-task deletion case). It is a no-op when the target
// is absent — a missing single file is tolerated the same way the legacy
// RemoveAttachmentFile ignored os.IsNotExist. Deleting each file before the
// directory keeps every op a non-recursive removal, matching the transaction
// journal's delete primitive.
func resolveAttachmentDelete(op AttachmentOp) ([]transaction.Op, error) {
	project := op.SourceProject
	if project == nil {
		project = op.DestProject
	}
	if project == nil {
		return nil, fmt.Errorf("storage: attachment delete requires a project")
	}
	if op.Filename != "" {
		return resolveSingleAttachmentDelete(project, op)
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
	ops := make([]transaction.Op, 0, len(files)+1)
	for _, f := range files {
		ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: f})
	}
	ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: dir})
	return ops, nil
}

// resolveSingleAttachmentDelete builds the delete op for one named attachment
// file, leaving the attachments directory and any siblings untouched. A file that
// is already gone yields no op — matching the legacy RemoveAttachmentFile, which
// ignored os.IsNotExist so a metadata removal was never blocked by missing bytes.
func resolveSingleAttachmentDelete(project *models.Project, op AttachmentOp) ([]transaction.Op, error) {
	path, err := resolveAttachmentPath(project, op.Task, op.Filename)
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return nil, nil
	} else if statErr != nil {
		return nil, fmt.Errorf("failed to stat attachment file: %w", statErr)
	}
	return []transaction.Op{{Kind: transaction.OpDelete, Target: path}}, nil
}
