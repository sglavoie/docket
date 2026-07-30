package models

import (
	"fmt"
	"strings"
	"time"
)

// attachmentDelimiter is used to separate the filename from the original path
// in serialized attachment metadata. Filenames and paths must not contain this
// sequence — see validateAttachmentPath.
const attachmentDelimiter = "||"

// validateAttachmentPath returns an error if the path contains characters
// reserved by the storage format: the attachment delimiter ("||") or
// semicolons (";"), which delimit multiple attachments in metadata.
func validateAttachmentPath(path string) error {
	if strings.Contains(path, attachmentDelimiter) {
		return fmt.Errorf("path %q contains reserved delimiter %q", path, attachmentDelimiter)
	}
	if strings.Contains(path, ";") {
		return fmt.Errorf("path %q contains semicolons which are reserved as attachment delimiters", path)
	}
	return nil
}

// AddAttachment adds a file attachment to the task.
func (t *Task) AddAttachment(filename string, isURL bool, originalPath string) error {
	return t.AddAttachmentAt(filename, isURL, originalPath, time.Now())
}

// AddAttachmentAt adds a file or URL attachment using the caller-owned
// mutation timestamp.
func (t *Task) AddAttachmentAt(filename string, isURL bool, originalPath string, now time.Time) error {
	if filename == "" {
		return fmt.Errorf("attachment filename must not be empty")
	}
	if err := validateAttachmentPath(filename); err != nil {
		return fmt.Errorf("adding attachment: %w", err)
	}
	if err := validateAttachmentPath(originalPath); err != nil {
		return fmt.Errorf("adding attachment: %w", err)
	}
	if now.IsZero() {
		return fmt.Errorf("adding attachment: timestamp must not be zero")
	}
	attachment := Attachment{
		Filename:     filename,
		IsURL:        isURL,
		OriginalPath: originalPath,
		AddedAt:      now,
	}
	t.Attachments = append(t.Attachments, attachment)
	t.UpdatedAt = now
	return nil
}

// AddCodeReference adds a code reference attachment to the task.
// Code references are stored without copying the file.
func (t *Task) AddCodeReference(path string, lineNumber int) error {
	return t.AddCodeReferenceAt(path, lineNumber, time.Now())
}

// AddCodeReferenceAt adds a code reference using the caller-owned mutation
// timestamp.
func (t *Task) AddCodeReferenceAt(path string, lineNumber int, now time.Time) error {
	if path == "" {
		return fmt.Errorf("code reference path must not be empty")
	}
	if lineNumber < 1 {
		return fmt.Errorf("code reference line number must be positive")
	}
	if err := validateAttachmentPath(path); err != nil {
		return fmt.Errorf("adding code reference: %w", err)
	}
	if now.IsZero() {
		return fmt.Errorf("adding code reference: timestamp must not be zero")
	}
	attachment := Attachment{
		Filename:     path,
		IsURL:        false,
		IsCodeRef:    true,
		LineNumber:   lineNumber,
		OriginalPath: path,
		AddedAt:      now,
	}
	t.Attachments = append(t.Attachments, attachment)
	t.UpdatedAt = now
	return nil
}

// RemoveAttachment removes the attachment at the given index.
// Returns the removed attachment and true if successful.
func (t *Task) RemoveAttachment(index int) (Attachment, bool) {
	if index < 0 || index >= len(t.Attachments) {
		return Attachment{}, false
	}
	removed := t.Attachments[index]
	t.Attachments = append(t.Attachments[:index], t.Attachments[index+1:]...)
	t.UpdatedAt = time.Now()
	return removed, true
}

// AttachmentCount returns the number of attachments on the task.
func (t *Task) AttachmentCount() int {
	return len(t.Attachments)
}

// HasAttachments returns true if the task has any attachments.
func (t *Task) HasAttachments() bool {
	return len(t.Attachments) > 0
}

// GetAttachment returns the attachment at the given index.
func (t *Task) GetAttachment(index int) *Attachment {
	if index < 0 || index >= len(t.Attachments) {
		return nil
	}
	return &t.Attachments[index]
}
