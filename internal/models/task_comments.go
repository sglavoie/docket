package models

import "time"

// AddComment adds a new timestamped comment to the task.
func (t *Task) AddComment(text string) {
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

// CommentCount returns the number of comments on the task.
func (t *Task) CommentCount() int {
	return len(t.Comments)
}
