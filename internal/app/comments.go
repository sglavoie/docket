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
	// Resolved is the comment's resolved state after the mutation (meaningful
	// for ToggleCommentResolved only).
	Resolved bool
}

// CommentWorkflowResult preserves comment-specific output alongside the common
// workflow cascade effects returned by every workflow-aware mutation.
type CommentWorkflowResult struct {
	Comment CommentResult
	Cascade BulkResult
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
	res, err := s.mutateTaskSelection(taskID, locator, "Add Comment", "", func(ref TaskRef) error {
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
	task.AddComment(text)
	task.UpdatedAt = s.clock.Now()
	return CommentResult{Index: len(task.Comments) - 1, Count: len(task.Comments)}, nil
}

// AddCommentWithWorkflow appends a comment through the same validation as
// AddComment and runs the workflow cascade atomically with the task write.
func (s *Service) AddCommentWithWorkflow(taskID, text string, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	return s.addCommentWithWorkflowSelection(taskID, TaskLocator{}, text, opts)
}

func (s *Service) AddCommentWithWorkflowByLocator(locator TaskLocator, text string, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	return s.addCommentWithWorkflowSelection("", locator, text, opts)
}

func (s *Service) addCommentWithWorkflowSelection(taskID string, locator TaskLocator, text string, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	trimmed, err := normalizeCommentText(text)
	if err != nil {
		return CommentWorkflowResult{}, err
	}
	opts = withDefaultWorkflowUndoAction(opts, "Add Comment")
	var comment CommentResult
	cascade, err := s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, _ *WorkspaceResolver) error {
		var mutationErr error
		comment, mutationErr = s.addComment(ref.Task, trimmed)
		return mutationErr
	})
	if err != nil {
		return CommentWorkflowResult{}, err
	}
	comment.Task = cascade.Tasks[0]
	return CommentWorkflowResult{Comment: comment, Cascade: cascade}, nil
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
	res, err := s.mutateTaskSelection(taskID, locator, "Edit Comment", "", func(ref TaskRef) error {
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
	res, err := s.mutateTaskSelection(taskID, locator, "Remove Comment", "", func(ref TaskRef) error {
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

// RemoveCommentWithWorkflow removes the fresh-snapshot comment index and
// preserves its post-mutation count alongside workflow effects.
func (s *Service) RemoveCommentWithWorkflow(taskID string, index int, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	return s.removeCommentWithWorkflowSelection(taskID, TaskLocator{}, index, opts)
}

func (s *Service) RemoveCommentWithWorkflowByLocator(locator TaskLocator, index int, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	return s.removeCommentWithWorkflowSelection("", locator, index, opts)
}

func (s *Service) removeCommentWithWorkflowSelection(taskID string, locator TaskLocator, index int, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Remove Comment")
	var comment CommentResult
	cascade, err := s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, _ *WorkspaceResolver) error {
		var mutationErr error
		comment, mutationErr = s.removeComment(ref.Task, index)
		return mutationErr
	})
	if err != nil {
		return CommentWorkflowResult{}, err
	}
	comment.Task = cascade.Tasks[0]
	return CommentWorkflowResult{Comment: comment, Cascade: cascade}, nil
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
	res, err := s.mutateTaskSelection(taskID, locator, "Clear Comments", "", func(ref TaskRef) error {
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

// ClearCommentsWithWorkflow clears comments and returns the removed count. An
// already-empty comment list is a true no-op matching the legacy CLI command.
func (s *Service) ClearCommentsWithWorkflow(taskID string, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	return s.clearCommentsWithWorkflowSelection(taskID, TaskLocator{}, opts)
}

func (s *Service) ClearCommentsWithWorkflowByLocator(locator TaskLocator, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	return s.clearCommentsWithWorkflowSelection("", locator, opts)
}

func (s *Service) clearCommentsWithWorkflowSelection(taskID string, locator TaskLocator, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Clear Comments")
	comment := CommentResult{Index: -1}
	cascade, err := s.mutateWithWorkflowSelection(taskID, locator, opts, func(ref TaskRef) bool {
		comment.Count = len(ref.Task.Comments)
		return comment.Count == 0
	}, func(ref TaskRef, _ *WorkspaceResolver) error {
		comment = s.clearComments(ref.Task)
		return nil
	})
	if err != nil {
		return CommentWorkflowResult{}, err
	}
	comment.Task = cascade.Tasks[0]
	return CommentWorkflowResult{Comment: comment, Cascade: cascade}, nil
}

// ToggleCommentResolved flips the resolved state of the comment at the given
// index and reports the post-toggle state.
func (s *Service) ToggleCommentResolved(taskID string, index int) (CommentResult, error) {
	return s.toggleCommentResolvedSelection(taskID, TaskLocator{}, index)
}

func (s *Service) ToggleCommentResolvedByLocator(locator TaskLocator, index int) (CommentResult, error) {
	return s.toggleCommentResolvedSelection("", locator, index)
}

func (s *Service) toggleCommentResolvedSelection(taskID string, locator TaskLocator, index int) (CommentResult, error) {
	var out CommentResult
	res, err := s.mutateTaskSelection(taskID, locator, "Resolve Comment", "", func(ref TaskRef) error {
		var mutationErr error
		out, mutationErr = s.toggleCommentResolved(ref.Task, index)
		return mutationErr
	})
	if err != nil {
		return CommentResult{}, err
	}
	out.Task = res
	return out, nil
}

func (s *Service) toggleCommentResolved(task *models.Task, index int) (CommentResult, error) {
	if err := validCommentIndex(index, len(task.Comments)); err != nil {
		return CommentResult{}, err
	}
	if !task.ToggleCommentResolved(index) {
		return CommentResult{}, invalidInput("comment", fmt.Sprintf("failed to toggle comment at index %d", index))
	}
	task.UpdatedAt = s.clock.Now()
	return CommentResult{Index: index, Count: len(task.Comments), Resolved: task.Comments[index].Resolved}, nil
}

// ToggleCommentResolvedWithWorkflow toggles the fresh-snapshot comment index
// and returns the resulting resolved state with the workflow cascade.
func (s *Service) ToggleCommentResolvedWithWorkflow(taskID string, index int, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	return s.toggleCommentResolvedWithWorkflowSelection(taskID, TaskLocator{}, index, opts)
}

func (s *Service) ToggleCommentResolvedWithWorkflowByLocator(locator TaskLocator, index int, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	return s.toggleCommentResolvedWithWorkflowSelection("", locator, index, opts)
}

func (s *Service) toggleCommentResolvedWithWorkflowSelection(taskID string, locator TaskLocator, index int, opts SingleTaskWorkflowOptions) (CommentWorkflowResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Resolve Comment")
	var comment CommentResult
	cascade, err := s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, _ *WorkspaceResolver) error {
		var mutationErr error
		comment, mutationErr = s.toggleCommentResolved(ref.Task, index)
		return mutationErr
	})
	if err != nil {
		return CommentWorkflowResult{}, err
	}
	comment.Task = cascade.Tasks[0]
	return CommentWorkflowResult{Comment: comment, Cascade: cascade}, nil
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
