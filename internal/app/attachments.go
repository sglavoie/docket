package app

import (
	"fmt"
	"path/filepath"

	"github.com/sglavoie/tt/internal/storage"
)

// AttachURL records a URL reference on a task. URLs are stored as metadata only —
// no file is copied — so the mutation commits exactly one project write. The
// caller is expected to have validated the URL's shape; the model still rejects a
// value carrying the reserved attachment delimiters.
func (s *Service) AttachURL(taskID, url string) (TaskResult, error) {
	return s.mutateTask(taskID, "Attach URL", url, func(ref TaskRef) error {
		return ref.Task.AddAttachment(url, true, url)
	})
}

// AttachCodeReference records a code reference (file:line) on a task. Code
// references are stored as metadata only — the referenced file is never copied —
// so the mutation commits exactly one project write.
func (s *Service) AttachCodeReference(taskID, path string, line int) (TaskResult, error) {
	return s.mutateTask(taskID, "Attach Code Ref", fmt.Sprintf("%s:%d", path, line), func(ref TaskRef) error {
		return ref.Task.AddCodeReference(path, line)
	})
}

// AttachFileResult reports a file attachment: the standard task outcome plus the
// normalized destination filename the copy used, which the adapter surfaces to
// the user.
type AttachFileResult struct {
	// Task is the standard task mutation result.
	Task TaskResult
	// Filename is the destination name the copied file was stored under.
	Filename string
}

// AttachFile ingests an external file into a task's attachments directory as ONE
// transaction: the metadata is validated and appended first, then the source
// bytes ride the same ChangeSet as an AttachmentCopy side effect. Because the copy
// is journaled with the metadata write, a validation failure aborts before any
// byte is written and a commit failure leaves no orphaned copy — replacing the
// legacy copy-then-compensate dance. A destination filename that already exists is
// rejected by the copy resolver, matching the legacy CopyAttachment contract.
func (s *Service) AttachFile(taskID, sourcePath string) (AttachFileResult, error) {
	if sourcePath == "" {
		return AttachFileResult{}, invalidInput("attachment", "source path is required")
	}
	ref, revision, err := s.resolveTask(taskID)
	if err != nil {
		return AttachFileResult{}, err
	}
	filename := filepath.Base(sourcePath)
	// Validate and record the metadata BEFORE staging the copy. A rejected filename
	// aborts here, so the AttachmentCopy op — and therefore any byte write — never runs.
	if err := ref.Task.AddAttachment(filename, false, sourcePath); err != nil {
		return AttachFileResult{}, err
	}
	op := storage.AttachmentOp{
		Kind:        storage.AttachmentCopy,
		Task:        ref.Task,
		DestProject: ref.Project,
		SourcePath:  sourcePath,
		Filename:    filename,
	}
	res, err := s.commitTaskWithSideEffects(ref, revision, []storage.AttachmentOp{op}, "Attach File", filename)
	if err != nil {
		return AttachFileResult{}, err
	}
	return AttachFileResult{Task: res, Filename: filename}, nil
}

// RemoveAttachmentResult reports an attachment removal: the standard task outcome,
// the removed attachment's identifying fields, and whether the on-disk bytes were
// deleted as part of the same transaction.
type RemoveAttachmentResult struct {
	// Task is the standard task mutation result.
	Task TaskResult
	// Filename is the removed attachment's filename (or URL / code reference).
	Filename string
	// IsURL reports whether the removed attachment was a URL reference.
	IsURL bool
	// IsCodeRef reports whether the removed attachment was a code reference.
	IsCodeRef bool
	// FileDeleted reports whether the attachment's local bytes were deleted in the
	// same transaction as the metadata removal.
	FileDeleted bool
}

// RemoveAttachment removes the attachment at index from a task's metadata and,
// when deleteFile is set and the attachment is a copied local file (not a URL or
// code reference), deletes its bytes in the SAME transaction. Committing the
// metadata removal and the byte delete together closes the legacy window where a
// crash between the two left tasks.md referencing a file that was already gone (or
// vice versa). URL and code-reference removals, and removals with deleteFile
// false, carry no file side effect.
func (s *Service) RemoveAttachment(taskID string, index int, deleteFile bool) (RemoveAttachmentResult, error) {
	ref, revision, err := s.resolveTask(taskID)
	if err != nil {
		return RemoveAttachmentResult{}, err
	}
	if index < 0 || index >= len(ref.Task.Attachments) {
		return RemoveAttachmentResult{}, invalidInput("attachment", fmt.Sprintf("index %d out of range", index))
	}
	removed, ok := ref.Task.RemoveAttachment(index)
	if !ok {
		return RemoveAttachmentResult{}, invalidInput("attachment", fmt.Sprintf("index %d out of range", index))
	}

	deleteBytes := deleteFile && !removed.IsURL && !removed.IsCodeRef
	var ops []storage.AttachmentOp
	if deleteBytes {
		ops = []storage.AttachmentOp{{
			Kind:          storage.AttachmentDelete,
			Task:          ref.Task,
			SourceProject: ref.Project,
			Filename:      removed.Filename,
		}}
	}
	res, err := s.commitTaskWithSideEffects(ref, revision, ops, "Remove Attachment", removed.Filename)
	if err != nil {
		return RemoveAttachmentResult{}, err
	}
	return RemoveAttachmentResult{
		Task:        res,
		Filename:    removed.Filename,
		IsURL:       removed.IsURL,
		IsCodeRef:   removed.IsCodeRef,
		FileDeleted: deleteBytes,
	}, nil
}

// ReorderAttachment swaps the attachments at from and to, matching the
// attachment panel's single-step move semantics. Reordering touches metadata
// only — no bytes move — so the mutation commits exactly one project write.
func (s *Service) ReorderAttachment(taskID string, from, to int) (TaskResult, error) {
	return s.mutateTask(taskID, "Reorder Attachments", "", func(ref TaskRef) error {
		count := len(ref.Task.Attachments)
		if from < 0 || from >= count || to < 0 || to >= count {
			return invalidInput("attachment", fmt.Sprintf("reorder %d -> %d out of range (task has %d)", from, to, count))
		}
		ref.Task.Attachments[from], ref.Task.Attachments[to] = ref.Task.Attachments[to], ref.Task.Attachments[from]
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

// commitTaskWithSideEffects runs the post-mutation hooks and commits a single
// project write plus zero or more attachment file side effects (copy/delete) as
// ONE ChangeSet, so a task's attachment bytes and the metadata that references
// them are staged, committed, and crash-recovered together. It is the attachment
// analogue of commitProjectWithArtifacts.
func (s *Service) commitTaskWithSideEffects(ref TaskRef, expectedRevision string, ops []storage.AttachmentOp, undoAction, undoContext string) (TaskResult, error) {
	generated := applyPostMutationHooks(ref.Project)
	results, err := s.repo.Commit(storage.ChangeSet{
		Project:          ref.Project,
		ExpectedRevision: expectedRevision,
		AttachmentOps:    ops,
		UndoAction:       undoAction,
		UndoContext:      undoContext,
	})
	if err != nil {
		return TaskResult{}, err
	}
	revision := ""
	if len(results) > 0 && results[0] != nil {
		revision = results[0].Revision
	}
	res := newTaskResult(ref, revision)
	res.RecurringGenerated = generated
	return res, nil
}
