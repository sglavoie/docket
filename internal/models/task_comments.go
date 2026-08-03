package models

import (
	"strings"
	"time"
)

// AddComment adds a new timestamped comment to the task. Comments with
// empty/whitespace-only text are silently dropped because they cannot be
// round-tripped through the markdown comments block (the parser treats empty
// text as a flush sentinel and discards the entry).
func (t *Task) AddComment(text string) {
	t.AddCommentAt(text, time.Now())
}

// AddCommentAt adds a comment using the caller-owned mutation timestamp.
func (t *Task) AddCommentAt(text string, now time.Time) {
	if strings.TrimSpace(text) == "" {
		return
	}
	comment := &Comment{
		Text:      text,
		CreatedAt: now,
	}
	t.Comments = append(t.Comments, comment)
	t.UpdatedAt = now
}

// RemoveComment removes the comment at the given index.
// Returns true if a comment was removed.
func (t *Task) RemoveComment(index int) bool {
	if index < 0 || index >= len(t.Comments) {
		return false
	}
	copy(t.Comments[index:], t.Comments[index+1:])
	t.Comments[len(t.Comments)-1] = nil
	t.Comments = t.Comments[:len(t.Comments)-1]
	t.UpdatedAt = time.Now()
	return true
}

// ClearComments removes all comments from the task.
func (t *Task) ClearComments() {
	if len(t.Comments) > 0 {
		t.Comments = []*Comment{}
		t.UpdatedAt = time.Now()
	}
}
