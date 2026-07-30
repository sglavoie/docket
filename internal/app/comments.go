package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
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
}

// AddComment appends a timestamped comment to a task and commits it as one
// ChangeSet. Empty or whitespace-only text is rejected before any mutation.
func (s *Service) AddComment(taskID, text string) (CommentResult, error) {
	return s.addCommentSelection(taskID, TaskLocator{}, text)
}

func (s *Service) AddCommentByLocator(locator TaskLocator, text string) (CommentResult, error) {
	return s.addCommentSelection("", locator, text)
}

func (s *Service) addCommentSelection(taskID string, locator TaskLocator, text string) (CommentResult, error) {
	trimmed, err := normalizeCommentText(text)
	if err != nil {
		return CommentResult{}, err
	}
	var out CommentResult
	res, err := s.mutateRichRecordSelection(taskID, locator, "Add Comment", "", func(ref TaskRef) error {
		var mutationErr error
		out, mutationErr = s.addComment(ref.Task, trimmed)
		return mutationErr
	})
	if err != nil {
		return CommentResult{}, err
	}
	out.Task = res
	return out, nil
}

func normalizeCommentText(text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", invalidInput("comment", "text is required")
	}
	return trimmed, nil
}

func (s *Service) addComment(task *models.Task, text string) (CommentResult, error) {
	now := s.clock.Now()
	task.AddCommentAt(text, now)
	task.UpdatedAt = now
	return CommentResult{Index: len(task.Comments) - 1, Count: len(task.Comments)}, nil
}

// EditComment replaces the text of the comment at the given index, preserving
// its original timestamp. Empty replacement text and out-of-range indices are
// rejected before any mutation.
func (s *Service) EditComment(taskID string, index int, text string) (CommentResult, error) {
	return s.editCommentSelection(taskID, TaskLocator{}, index, text)
}

func (s *Service) EditCommentByLocator(locator TaskLocator, index int, text string) (CommentResult, error) {
	return s.editCommentSelection("", locator, index, text)
}

func (s *Service) editCommentSelection(taskID string, locator TaskLocator, index int, text string) (CommentResult, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return CommentResult{}, invalidInput("comment", "text is required")
	}
	var out CommentResult
	res, err := s.mutateRichRecordSelection(taskID, locator, "Edit Comment", "", func(ref TaskRef) error {
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
	return s.removeCommentSelection(taskID, TaskLocator{}, index)
}

func (s *Service) RemoveCommentByLocator(locator TaskLocator, index int) (CommentResult, error) {
	return s.removeCommentSelection("", locator, index)
}

func (s *Service) removeCommentSelection(taskID string, locator TaskLocator, index int) (CommentResult, error) {
	var out CommentResult
	res, err := s.mutateRichRecordSelection(taskID, locator, "Remove Comment", "", func(ref TaskRef) error {
		var mutationErr error
		out, mutationErr = s.removeComment(ref.Task, index)
		return mutationErr
	})
	if err != nil {
		return CommentResult{}, err
	}
	out.Task = res
	return out, nil
}

func (s *Service) removeComment(task *models.Task, index int) (CommentResult, error) {
	if err := validCommentIndex(index, len(task.Comments)); err != nil {
		return CommentResult{}, err
	}
	if !task.RemoveComment(index) {
		return CommentResult{}, invalidInput("comment", fmt.Sprintf("failed to remove comment at index %d", index))
	}
	task.UpdatedAt = s.clock.Now()
	return CommentResult{Index: index, Count: len(task.Comments)}, nil
}

// ClearComments removes every comment from a task. Clearing a task with no
// comments still commits the (unchanged) project, matching the other idempotent
// mutations; adapters that want a no-op fast path check before calling.
func (s *Service) ClearComments(taskID string) (CommentResult, error) {
	return s.clearCommentsSelection(taskID, TaskLocator{})
}

func (s *Service) ClearCommentsByLocator(locator TaskLocator) (CommentResult, error) {
	return s.clearCommentsSelection("", locator)
}

func (s *Service) clearCommentsSelection(taskID string, locator TaskLocator) (CommentResult, error) {
	var out CommentResult
	res, err := s.mutateRichRecordSelection(taskID, locator, "Clear Comments", "", func(ref TaskRef) error {
		out = s.clearComments(ref.Task)
		return nil
	})
	if err != nil {
		return CommentResult{}, err
	}
	out.Task = res
	return out, nil
}

func (s *Service) clearComments(task *models.Task) CommentResult {
	out := CommentResult{Index: -1, Count: len(task.Comments)}
	task.ClearComments()
	task.UpdatedAt = s.clock.Now()
	return out
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
