package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
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
func (s *Service) AddComment(locator TaskLocator, text string) (CommentResult, error) {
	trimmed, err := normalizeCommentText(text)
	if err != nil {
		return CommentResult{}, err
	}
	var out CommentResult
	res, err := s.mutateRichRecordSelection(locator, "Add Comment", func(ref TaskRef) error {
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
// nothing written. It is the comment sibling of ResolveCapturedAttachments and
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

// EditComment edits only when index still holds the comment the
// caller read. Text and creation time are checked in the mutation's snapshot,
// so a shifted row or a concurrent edit cannot redirect the replacement. The
// original creation timestamp is preserved.
func (s *Service) EditComment(locator TaskLocator, index int, expected models.Comment, text string) (CommentResult, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return CommentResult{}, invalidInput("comment", "text is required")
	}
	var out CommentResult
	res, err := s.mutateRichRecordSelection(locator, "Edit Comment", func(ref TaskRef) error {
		if err := validateCommentTarget(ref.Task, index, expected); err != nil {
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

// RemoveComment removes only the comment the caller confirmed, judged
// against the same snapshot the transaction commits.
func (s *Service) RemoveComment(locator TaskLocator, index int, expected models.Comment) (CommentResult, error) {
	var out CommentResult
	res, err := s.mutateRichRecordSelection(locator, "Remove Comment", func(ref TaskRef) error {
		if err := validateCommentTarget(ref.Task, index, expected); err != nil {
			return err
		}
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
	if !task.RemoveCommentAt(index, s.clock.Now()) {
		return invalidInput("comment", fmt.Sprintf("failed to remove comment at index %d", index))
	}
	task.UpdatedAt = s.clock.Now()
	return nil
}

func validateCommentTarget(task *models.Task, index int, expected models.Comment) error {
	if err := validCommentIndex(index, len(task.Comments)); err != nil {
		return err
	}
	comment := task.Comments[index]
	if comment == nil || comment.Text != expected.Text || !comment.CreatedAt.Equal(expected.CreatedAt) {
		return invalidInput("comment", "comment changed since it was read; reopen it and try again")
	}
	return nil
}

// validCommentIndex checks bounds; validateCommentTarget also checks the stored
// value the caller read.
func validCommentIndex(index, count int) error {
	if index < 0 || index >= count {
		return invalidInput("comment", fmt.Sprintf("comment index %d out of range (task has %d)", index, count))
	}
	return nil
}
