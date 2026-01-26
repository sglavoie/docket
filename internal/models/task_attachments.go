package models

import "time"

// AddAttachment adds a file attachment to the task.
func (t *Task) AddAttachment(filename string, isURL bool) {
	attachment := Attachment{
		Filename: filename,
		IsURL:    isURL,
		AddedAt:  time.Now(),
	}
	t.Attachments = append(t.Attachments, attachment)
	t.UpdatedAt = time.Now()
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
