package models

import (
	"fmt"
	"strings"
	"time"
)

// attachmentDelimiter is used to separate the filename from the original path
// in serialized attachment metadata. Filenames and paths must not contain this
// sequence — see ValidateAttachmentPath.
const attachmentDelimiter = "||"

// ValidateAttachmentPath returns an error if the path contains characters
// reserved by the storage format: the attachment delimiter ("||") or
// semicolons (";"), which delimit multiple attachments in metadata.
func ValidateAttachmentPath(path string) error {
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
	if filename == "" {
		return fmt.Errorf("attachment filename must not be empty")
	}
	if err := ValidateAttachmentPath(filename); err != nil {
		return fmt.Errorf("adding attachment: %w", err)
	}
	if err := ValidateAttachmentPath(originalPath); err != nil {
		return fmt.Errorf("adding attachment: %w", err)
	}
	attachment := Attachment{
		Filename:     filename,
		IsURL:        isURL,
		OriginalPath: originalPath,
		AddedAt:      time.Now(),
	}
	t.Attachments = append(t.Attachments, attachment)
	t.UpdatedAt = time.Now()
	return nil
}

// AddCodeReference adds a code reference attachment to the task.
// Code references are stored without copying the file.
func (t *Task) AddCodeReference(path string, lineNumber int) error {
	if path == "" {
		return fmt.Errorf("code reference path must not be empty")
	}
	if err := ValidateAttachmentPath(path); err != nil {
		return fmt.Errorf("adding code reference: %w", err)
	}
	attachment := Attachment{
		Filename:     path,
		IsURL:        false,
		IsCodeRef:    true,
		LineNumber:   lineNumber,
		OriginalPath: path,
		AddedAt:      time.Now(),
	}
	t.Attachments = append(t.Attachments, attachment)
	t.UpdatedAt = time.Now()
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
