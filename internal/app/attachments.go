package app

import (
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// AttachURL records a URL reference on a task. URLs are stored as metadata only —
// no file is copied — so the mutation commits exactly one project write. The
// caller is expected to have validated the URL's shape; the model still rejects a
// value carrying the reserved attachment delimiters.
func (s *Service) AttachURL(locator TaskLocator, url string) (TaskResult, error) {
	url, err := validateAttachmentURL(url)
	if err != nil {
		return TaskResult{}, err
	}
	return s.mutateAttachmentSelection(locator, "Attach URL", url, func(ref TaskRef) error {
		return ref.Task.AddAttachmentAt(url, true, url, s.clock.Now())
	})
}

// messageURLPrefix is Mail's message link, the one URL scheme besides http(s)
// an attachment admits. A real link reads message://%3C<message-id>%3E, and
// url.Parse refuses it outright: the percent-encoded angle brackets land in what
// it reads as the host, where an escape is invalid. So the scheme and a
// non-empty remainder decide it instead, and the link is stored exactly as
// given, to be opened by the system the way an http(s) link is.
const messageURLPrefix = "message://"

// IsMessageURL reports whether raw names Mail's message scheme, complete or not.
// A value that does is a URL attachment candidate, never a file path, so an
// incomplete one is refused as a link rather than as a missing file.
func IsMessageURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	return len(raw) >= len("message:") && strings.EqualFold(raw[:len("message:")], "message:")
}

func validateAttachmentURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if IsMessageURL(raw) {
		if len(raw) <= len(messageURLPrefix) || !strings.EqualFold(raw[:len(messageURLPrefix)], messageURLPrefix) ||
			strings.ContainsAny(raw, " \t\n") {
			return "", invalidInput("attachment", "a message: link must name a message after message://")
		}
		return raw, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", invalidInput("attachment", "URL must be an absolute http or https URL")
	}
	return raw, nil
}

// AttachCodeReference records a code reference (file:line) on a task. Code
// references are stored as metadata only — the referenced file is never copied —
// so the mutation commits exactly one project write.
func (s *Service) AttachCodeReference(locator TaskLocator, path string, line int) (TaskResult, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.ContainsRune(path, '\x00') {
		return TaskResult{}, invalidInput("attachment", "code reference path is required")
	}
	return s.mutateAttachmentSelection(locator, "Attach Code Ref", fmt.Sprintf("%s:%d", path, line), func(ref TaskRef) error {
		return ref.Task.AddCodeReferenceAt(path, line, s.clock.Now())
	})
}

// AttachFileResult reports a file attachment: the standard task outcome plus the
// normalized destination filename the copy used, which the adapter surfaces to
// the user.
type AttachFileResult struct {
	// Task is the standard task mutation result.
	Task TaskResult
}

// AttachFile ingests an external file into a task's attachments directory as ONE
// transaction: the metadata is validated and appended first, then the source
// bytes ride the same ChangeSet as an AttachmentCopy side effect. Because the copy
// is journaled with the metadata write, a validation failure aborts before any
// byte is written and a commit failure leaves no orphaned copy — replacing the
// legacy copy-then-compensate dance. A destination filename that already exists is
// rejected by the copy resolver, matching the legacy CopyAttachment contract.
func (s *Service) AttachFile(locator TaskLocator, sourcePath string) (AttachFileResult, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	return s.attachFile(locator, sourcePath, filepath.Base(sourcePath), sourcePath)
}

// ImportAttachment ingests bytes a frontend staged at sourcePath under the
// given filename, through the same single transaction AttachFile uses. The
// staged copy is temporary — a phone copies a Files or Photos item there while
// its provider grant is valid — so its path is never recorded: the attachment
// keeps no original path, and the workspace bytes are its only storage.
func (s *Service) ImportAttachment(locator TaskLocator, sourcePath, filename string) (AttachFileResult, error) {
	return s.attachFile(locator, strings.TrimSpace(sourcePath), strings.TrimSpace(filename), "")
}

func (s *Service) attachFile(locator TaskLocator, sourcePath, filename, originalPath string) (AttachFileResult, error) {
	if sourcePath == "" || strings.ContainsRune(sourcePath, '\x00') {
		return AttachFileResult{}, invalidInput("attachment", "source path is required")
	}
	if err := storage.SafeChildName(filename); err != nil {
		return AttachFileResult{}, invalidInput("attachment", err.Error())
	}
	ref, snap, revision, err := s.loadTaskSelection("", locator)
	if err != nil {
		return AttachFileResult{}, err
	}
	before := cloneSnapshot(snap)
	keepMutation := false
	defer func() {
		if !keepMutation {
			restoreSnapshotProjects(snap, before)
		}
	}()
	// Validate and record the metadata BEFORE staging the copy. A rejected filename
	// aborts here, so the AttachmentCopy op — and therefore any byte write — never runs.
	if addErr := ref.Task.AddAttachmentAt(filename, false, originalPath, s.clock.Now()); addErr != nil {
		return AttachFileResult{}, addErr
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
	keepMutation = true
	return AttachFileResult{Task: res}, nil
}

// RemoveAttachmentResult reports an attachment removal: the standard task outcome,
// the removed attachment's identifying fields, and whether the on-disk bytes were
// deleted as part of the same transaction.
type RemoveAttachmentResult struct {
	// Task is the standard task mutation result.
	Task TaskResult
}

// RemoveAttachment removes the attachment at index from a task's metadata and,
// when deleteFile is set and the attachment is a copied local file (not a URL or
// code reference), deletes its bytes in the SAME transaction. Committing the
// metadata removal and the byte delete together closes the legacy window where a
// crash between the two left tasks.md referencing a file that was already gone (or
// vice versa). URL and code-reference removals, and removals with deleteFile
// false, carry no file side effect.
func (s *Service) RemoveAttachment(locator TaskLocator, index int, deleteFile bool) (RemoveAttachmentResult, error) {
	return s.removeAttachment(locator, index, nil, deleteFile)
}

// RemoveAttachmentExpecting is RemoveAttachment for a caller that read the
// attachment before asking: the attachment at index in the fresh snapshot must
// still be expected, or nothing is removed. A position is not an identity — a
// reload between the question and the answer can shift the list — so a
// confirmed removal names what it confirmed rather than where it was.
func (s *Service) RemoveAttachmentExpecting(locator TaskLocator, index int, expected models.Attachment, deleteFile bool) (RemoveAttachmentResult, error) {
	return s.removeAttachment(locator, index, &expected, deleteFile)
}

// SameAttachment reports whether a and b are the same stored attachment. It
// compares every stored field, instants by Equal, so a re-read of an untouched
// attachment always matches and a replaced one never does.
func SameAttachment(a, b models.Attachment) bool {
	return a.Filename == b.Filename &&
		a.IsURL == b.IsURL &&
		a.IsCodeRef == b.IsCodeRef &&
		a.LineNumber == b.LineNumber &&
		a.OriginalPath == b.OriginalPath &&
		a.AddedAt.Equal(b.AddedAt)
}

func (s *Service) removeAttachment(locator TaskLocator, index int, expected *models.Attachment, deleteFile bool) (RemoveAttachmentResult, error) {
	ref, snap, revision, err := s.loadTaskSelection("", locator)
	if err != nil {
		return RemoveAttachmentResult{}, err
	}
	if index < 0 || index >= len(ref.Task.Attachments) {
		if expected != nil {
			return RemoveAttachmentResult{}, invalidInput("attachment", "attachment is no longer on this task; nothing was removed")
		}
		return RemoveAttachmentResult{}, invalidInput("attachment", fmt.Sprintf("index %d out of range", index))
	}
	if expected != nil && !SameAttachment(ref.Task.Attachments[index], *expected) {
		return RemoveAttachmentResult{}, invalidInput("attachment", "attachments changed since you asked; nothing was removed")
	}
	before := cloneSnapshot(snap)
	keepMutation := false
	defer func() {
		if !keepMutation {
			restoreSnapshotProjects(snap, before)
		}
	}()
	removed, ok := ref.Task.RemoveAttachmentAt(index, s.clock.Now())
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
	keepMutation = true
	return RemoveAttachmentResult{Task: res}, nil
}

// ReorderAttachment swaps the attachments at from and to, matching the
// attachment panel's single-step move semantics. Reordering touches metadata
// only — no bytes move — so the mutation commits exactly one project write.
// The complete ordered list must still match the caller's read; stale indexes
// must never redirect a reorder to different attachments.
func (s *Service) ReorderAttachment(locator TaskLocator, from, to int, expected []models.Attachment) (TaskResult, error) {
	return s.mutateAttachmentSelection(locator, "Reorder Attachments", "", func(ref TaskRef) error {
		if !slices.EqualFunc(ref.Task.Attachments, expected, SameAttachment) {
			return invalidInput("attachment", "attachments changed since they were read; nothing was reordered")
		}
		count := len(ref.Task.Attachments)
		if from < 0 || from >= count || to < 0 || to >= count {
			return invalidInput("attachment", fmt.Sprintf("reorder %d -> %d out of range (task has %d)", from, to, count))
		}
		ref.Task.Attachments[from], ref.Task.Attachments[to] = ref.Task.Attachments[to], ref.Task.Attachments[from]
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

// mutateAttachmentSelection gives metadata-only attachment mutations the same
// rollback guarantee as copy/delete mutations. Repository snapshots can retain
// caller-visible pointers, so a failed commit must restore the pre-mutation
// object graph rather than merely relying on storage atomicity.
func (s *Service) mutateAttachmentSelection(locator TaskLocator, undoAction, undoContext string, mutate func(TaskRef) error) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection("", locator)
	if err != nil {
		return TaskResult{}, err
	}
	before := cloneSnapshot(snap)
	keepMutation := false
	defer func() {
		if !keepMutation {
			restoreSnapshotProjects(snap, before)
		}
	}()
	if mutateErr := mutate(ref); mutateErr != nil {
		return TaskResult{}, mutateErr
	}
	result, err := s.commitTaskWithSideEffects(ref, revision, nil, undoAction, undoContext)
	if err != nil {
		return TaskResult{}, err
	}
	keepMutation = true
	return result, nil
}

// commitTaskWithSideEffects is the attachment-ops-only spelling of
// commitProjectWithSideEffects, kept for the attachment mutations that carry no
// auxiliary artifact: a task's attachment bytes and the metadata that references
// them are staged, committed, and crash-recovered together.
func (s *Service) commitTaskWithSideEffects(ref TaskRef, expectedRevision string, ops []storage.AttachmentOp, undoAction, undoContext string) (TaskResult, error) {
	return s.commitProjectWithSideEffects(ref, expectedRevision, nil, ops, undoAction, undoContext)
}
