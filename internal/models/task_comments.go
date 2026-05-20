package models

import (
	"strings"
	"time"
)

// AddComment adds a new timestamped comment to the task. Comments with
// empty/whitespace-only text are silently dropped because they cannot be
// round-tripped through the markdown comments block (the parser treats
// empty text as a flush sentinel and discards the entry, mirroring
// AddDecision).
func (t *Task) AddComment(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	comment := &Comment{
		Text:      text,
		CreatedAt: time.Now(),
	}
	t.Comments = append(t.Comments, comment)
	t.UpdatedAt = time.Now()
}

// RemoveComment removes the comment at the given index.
// Returns true if a comment was removed.
func (t *Task) RemoveComment(index int) bool {
	if index < 0 || index >= len(t.Comments) {
		return false
	}
	t.Comments = append(t.Comments[:index], t.Comments[index+1:]...)
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

// ToggleCommentResolved flips the Resolved state of the comment at the given index.
// Returns false if the index is out of range.
func (t *Task) ToggleCommentResolved(index int) bool {
	if index < 0 || index >= len(t.Comments) {
		return false
	}
	t.Comments[index].Resolved = !t.Comments[index].Resolved
	t.UpdatedAt = time.Now()
	return true
}

// UnresolvedCommentCount returns the number of comments where Resolved is false.
func (t *Task) UnresolvedCommentCount() int {
	count := 0
	for _, c := range t.Comments {
		if !c.Resolved {
			count++
		}
	}
	return count
}
