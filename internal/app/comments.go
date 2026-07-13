package app

import (
	"fmt"
	"strings"
)

// CommentResult is the presentation-neutral outcome of a comment mutation. It
// wraps the standard task result (the comment lives on a task, so the commit is
// a task commit) with the comment-specific detail adapters render.
type CommentResult struct {
	// Task is the standard single-task mutation result.
	Task TaskResult
	// Index is the affected comment's index (for add, the new comment's index).
	Index int
	// Count is the number of comments on the task after the mutation.
	Count int
	// Resolved is the comment's resolved state after the mutation (meaningful
	// for ToggleCommentResolved only).
	Resolved bool
}

// AddComment appends a timestamped comment to a task and commits it as one
// ChangeSet. Empty or whitespace-only text is rejected before any mutation.
func (s *Service) AddComment(taskID, text string) (CommentResult, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return CommentResult{}, invalidInput("comment", "text is required")
	}
	var out CommentResult
	res, err := s.mutateTask(taskID, "Add Comment", "", func(ref TaskRef) error {
		ref.Task.AddComment(trimmed)
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = len(ref.Task.Comments) - 1
		out.Count = len(ref.Task.Comments)
		return nil
	})
	if err != nil {
		return CommentResult{}, err
	}
	out.Task = res
	return out, nil
}

// EditComment replaces the text of the comment at the given index, preserving
// its original timestamp. Empty replacement text and out-of-range indices are
// rejected before any mutation.
func (s *Service) EditComment(taskID string, index int, text string) (CommentResult, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return CommentResult{}, invalidInput("comment", "text is required")
	}
	var out CommentResult
	res, err := s.mutateTask(taskID, "Edit Comment", "", func(ref TaskRef) error {
		if err := validCommentIndex(index, len(ref.Task.Comments)); err != nil {
			return err
		}
		ref.Task.Comments[index].Text = text
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = index
		out.Count = len(ref.Task.Comments)
		return nil
	})
	if err != nil {
		return CommentResult{}, err
	}
	out.Task = res
	return out, nil
}

// RemoveComment removes the comment at the given index.
func (s *Service) RemoveComment(taskID string, index int) (CommentResult, error) {
	var out CommentResult
	res, err := s.mutateTask(taskID, "Remove Comment", "", func(ref TaskRef) error {
		if err := validCommentIndex(index, len(ref.Task.Comments)); err != nil {
			return err
		}
		if !ref.Task.RemoveComment(index) {
			return invalidInput("comment", fmt.Sprintf("failed to remove comment at index %d", index))
		}
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = index
		out.Count = len(ref.Task.Comments)
		return nil
	})
	if err != nil {
		return CommentResult{}, err
	}
	out.Task = res
	return out, nil
}

// ClearComments removes every comment from a task. Clearing a task with no
// comments still commits the (unchanged) project, matching the other idempotent
// mutations; adapters that want a no-op fast path check before calling.
func (s *Service) ClearComments(taskID string) (CommentResult, error) {
	var out CommentResult
	res, err := s.mutateTask(taskID, "Clear Comments", "", func(ref TaskRef) error {
		out.Index = -1
		out.Count = len(ref.Task.Comments)
		ref.Task.ClearComments()
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
	if err != nil {
		return CommentResult{}, err
	}
	out.Task = res
	return out, nil
}

// ToggleCommentResolved flips the resolved state of the comment at the given
// index and reports the post-toggle state.
func (s *Service) ToggleCommentResolved(taskID string, index int) (CommentResult, error) {
	var out CommentResult
	res, err := s.mutateTask(taskID, "Resolve Comment", "", func(ref TaskRef) error {
		if err := validCommentIndex(index, len(ref.Task.Comments)); err != nil {
			return err
		}
		if !ref.Task.ToggleCommentResolved(index) {
			return invalidInput("comment", fmt.Sprintf("failed to toggle comment at index %d", index))
		}
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = index
		out.Count = len(ref.Task.Comments)
		out.Resolved = ref.Task.Comments[index].Resolved
		return nil
	})
	if err != nil {
		return CommentResult{}, err
	}
	out.Task = res
	return out, nil
}

// validCommentIndex rejects an out-of-range comment index against the current
// comment count, so a stale adapter-side index cannot silently target the wrong
// comment after a concurrent change.
func validCommentIndex(index, count int) error {
	if index < 0 || index >= count {
		return invalidInput("comment", fmt.Sprintf("comment index %d out of range (task has %d)", index, count))
	}
	return nil
}
