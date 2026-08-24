package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// CommentResult is the presentation-neutral outcome of a comment mutation. It
// wraps the standard task result (the comment lives on a task, so the commit is
// a task commit) with the comment-specific detail adapters render.
type CommentResult struct {
	// Task is the standard single-task mutation result.
	Task TaskResult
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
	res, err := s.mutateRichRecordSelection(taskID, locator, "Add Comment", func(ref TaskRef) error {
		s.addComment(ref.Task, trimmed)
		return nil
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

// resolveCapturedComments normalizes every ;comment; text on one creation
// request before the request mutates anything, so a rejected comment aborts with
// nothing written. It is the comment sibling of resolveCapturedAttachments and
// runs the same normalizeCommentText an interactively added comment goes
// through, which is what keeps the two entry points from disagreeing.
func resolveCapturedComments(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	resolved := make([]string, 0, len(values))
	for _, value := range values {
		trimmed, err := normalizeCommentText(value)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, trimmed)
	}
	return resolved, nil
}

// applyCapturedComments records each normalized comment on the newly created
// task. Unlike attachments it stages no side effect: a comment is task document
// content, so it commits with the task write itself.
func applyCapturedComments(task *models.Task, texts []string, now time.Time) {
	for _, text := range texts {
		task.AddCommentAt(text, now)
	}
}

func (s *Service) addComment(task *models.Task, text string) {
	now := s.clock.Now()
	task.AddCommentAt(text, now)
	task.UpdatedAt = now
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
	res, err := s.mutateRichRecordSelection(taskID, locator, "Edit Comment", func(ref TaskRef) error {
		if err := validCommentIndex(index, len(ref.Task.Comments)); err != nil {
			return err
		}
		ref.Task.Comments[index].Text = text
		ref.Task.UpdatedAt = s.clock.Now()
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
	res, err := s.mutateRichRecordSelection(taskID, locator, "Remove Comment", func(ref TaskRef) error {
		return s.removeComment(ref.Task, index)
	})
	if err != nil {
		return CommentResult{}, err
	}
	out.Task = res
	return out, nil
}

func (s *Service) removeComment(task *models.Task, index int) error {
	if err := validCommentIndex(index, len(task.Comments)); err != nil {
		return err
	}
	if !task.RemoveComment(index) {
		return invalidInput("comment", fmt.Sprintf("failed to remove comment at index %d", index))
	}
	task.UpdatedAt = s.clock.Now()
	return nil
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
	res, err := s.mutateRichRecordSelection(taskID, locator, "Clear Comments", func(ref TaskRef) error {
		s.clearComments(ref.Task)
		return nil
	})
	if err != nil {
		return CommentResult{}, err
	}
	out.Task = res
	return out, nil
}

func (s *Service) clearComments(task *models.Task) {
	task.ClearComments()
	task.UpdatedAt = s.clock.Now()
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
