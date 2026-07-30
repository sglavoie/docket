package app

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// Deliberately absent: BulkSetSection and BulkSetStage.
//
// The locked design retires task sections and custom stages in favor of one
// WorkflowState (see the migration precedence in the workspace design). A
// view's legacy "move selection to section X" bulk action is therefore not a
// new attribute use case — it is a state transition, and its target section
// maps onto a WorkflowState (Next->next, Waiting->waiting, Someday->someday,
// Inbox->inbox, Completed->done, ...). Route those through BulkTransition,
// which validates the transition and runs the same atomic single-commit path
// these attribute operations use.
//
// Custom-stage bulk assignment has no WorkflowState equivalent and is retired
// outright: when a view that offered a bulk "set stage" action migrates, that
// action is dropped rather than reimplemented here. This is why no
// BulkSetStage exists to call.

// bulkAttribute is the shared runner for every bulk attribute mutation. It
// mirrors the BulkTransition path: load a
// revision-consistent snapshot, resolve the distinct target tasks (duplicates
// collapsed, an unresolved id aborts the batch), validate every resolved task
// before touching any, apply mutate to each, and commit every distinct project
// as ONE transaction. validate runs against all refs first, so an invalid input
// aborts the whole batch before any mutation persists — the batch is
// all-or-nothing, matching the atomic guarantee every other bulk use case gives.
// A nil validate skips the pre-check; a nil mutate is a programming error.
func (s *Service) bulkAttribute(taskIDs []string, locators []TaskLocator, undoAction, undoContext string, validate func(TaskRef) error, mutate func(*models.Task)) (BulkResult, error) {
	if len(taskIDs) == 0 && len(locators) == 0 {
		return BulkResult{}, invalidInput("tasks", "at least one task id is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkResult{}, err
	}
	refs, err := resolveRequestedTasks(snap, taskIDs, locators)
	if err != nil {
		return BulkResult{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return BulkResult{}, fmt.Errorf("capture bulk attribute rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()
	if validate != nil {
		for _, r := range refs {
			if validationErr := validate(r); validationErr != nil {
				return BulkResult{}, validationErr
			}
		}
	}
	for _, r := range refs {
		mutate(r.Task)
	}

	commits, idx := distinctProjectCommits(refs, snap)
	results, err := s.commitProjects(commits, undoAction, undoContext)
	if err != nil {
		return BulkResult{}, err
	}
	committed = true

	out := BulkResult{Tasks: make([]TaskResult, 0, len(refs))}
	for _, r := range refs {
		out.Tasks = append(out.Tasks, newTaskResult(r, revisionAt(results.results, idx[r.Project.ID])))
	}
	for _, c := range commits {
		out.TouchedProjectIDs = append(out.TouchedProjectIDs, c.project.ID)
	}
	return out, nil
}

// BulkSetPriority sets one priority on every selected task, committing every
// touched project as ONE transaction. The priority is validated once up front —
// it is uniform across the batch — so an out-of-range value aborts before any
// snapshot is mutated. Tasks may span projects.
func (s *Service) BulkSetPriority(taskIDs []string, priority int) (BulkResult, error) {
	return s.bulkSetPriority(taskIDs, nil, priority)
}

func (s *Service) BulkSetPriorityByLocators(locators []TaskLocator, priority int) (BulkResult, error) {
	return s.bulkSetPriority(nil, locators, priority)
}

func (s *Service) bulkSetPriority(taskIDs []string, locators []TaskLocator, priority int) (BulkResult, error) {
	if err := models.ValidatePriority(priority, true); err != nil {
		return BulkResult{}, invalidInput("priority", err.Error())
	}
	return s.bulkAttribute(taskIDs, locators, "Set Priority", fmt.Sprintf("priority %d", priority), nil, func(t *models.Task) {
		t.Priority = priority
		t.UpdatedAt = s.clock.Now()
	})
}

// BulkSetDueDate sets one calendar due date on every selected task, committing
// every touched project as ONE transaction. Use BulkClearDueDate to remove a
// due date. Tasks may span projects.
func (s *Service) BulkSetDueDate(taskIDs []string, due time.Time) (BulkResult, error) {
	return s.bulkSetDueDate(taskIDs, nil, due)
}

func (s *Service) BulkSetDueDateByLocators(locators []TaskLocator, due time.Time) (BulkResult, error) {
	return s.bulkSetDueDate(nil, locators, due)
}

func (s *Service) bulkSetDueDate(taskIDs []string, locators []TaskLocator, due time.Time) (BulkResult, error) {
	return s.bulkAttribute(taskIDs, locators, "Set Due Date", "due date", nil, func(t *models.Task) {
		t.DueDate = cloneCalendarDatePtr(&due)
		t.UpdatedAt = s.clock.Now()
	})
}

// BulkClearDueDate removes the due date from every
// selected task, committing every touched project as ONE transaction. Clearing a
// task that has no due date is a no-op that still commits the (unchanged)
// project, matching the idempotent single-task attribute mutations. Tasks may
// span projects.
func (s *Service) BulkClearDueDate(taskIDs []string) (BulkResult, error) {
	return s.bulkClearDueDate(taskIDs, nil)
}

func (s *Service) BulkClearDueDateByLocators(locators []TaskLocator) (BulkResult, error) {
	return s.bulkClearDueDate(nil, locators)
}

func (s *Service) bulkClearDueDate(taskIDs []string, locators []TaskLocator) (BulkResult, error) {
	return s.bulkAttribute(taskIDs, locators, "Clear Due Date", "due date", nil, func(t *models.Task) {
		t.DueDate = nil
		t.UpdatedAt = s.clock.Now()
	})
}

// BulkSnooze hides every selected task until the given date, or clears the snooze
// on every task when until is nil, committing every touched project as ONE
// transaction. When snoozing (until non-nil) the same guards single-task Snooze
// enforces apply to every task first: snoozing a terminal or in-progress task is
// rejected, and because the batch is all-or-nothing one invalid task aborts the
// whole batch before any commit. Clearing a snooze carries no guards. Tasks may
// span projects.
func (s *Service) BulkSnooze(taskIDs []string, until *time.Time) (BulkResult, error) {
	return s.bulkSnooze(taskIDs, nil, until)
}

func (s *Service) BulkSnoozeByLocators(locators []TaskLocator, until *time.Time) (BulkResult, error) {
	return s.bulkSnooze(nil, locators, until)
}

func (s *Service) bulkSnooze(taskIDs []string, locators []TaskLocator, until *time.Time) (BulkResult, error) {
	if until == nil {
		return s.bulkAttribute(taskIDs, locators, "Snooze Tasks", "clear", nil, func(t *models.Task) {
			t.ClearSnooze()
		})
	}
	validate := func(r TaskRef) error {
		if r.Task.IsTerminalState() {
			return invalidInput("snooze", fmt.Sprintf("cannot snooze a %s task", r.Task.State()))
		}
		if r.Task.IsInState(models.StateInProgress) {
			return invalidInput("snooze", "cannot snooze an in-progress task (change state first)")
		}
		return nil
	}
	return s.bulkAttribute(taskIDs, locators, "Snooze Tasks", "snooze", validate, func(t *models.Task) {
		u := *until
		t.SnoozedUntil = &u
		t.UpdatedAt = s.clock.Now()
	})
}

// BulkSetTags replaces the tag set on every selected task with tags, committing
// every touched project as ONE transaction. The tags are normalized, validated,
// and de-duplicated once up front — they are uniform across the batch — so a
// malformed tag aborts before any snapshot is mutated. An empty tags slice clears
// every task's tags. Tasks may span projects.
func (s *Service) BulkSetTags(taskIDs []string, tags []string) (BulkResult, error) {
	return s.bulkSetTags(taskIDs, nil, tags)
}

func (s *Service) BulkSetTagsByLocators(locators []TaskLocator, tags []string) (BulkResult, error) {
	return s.bulkSetTags(nil, locators, tags)
}

func (s *Service) bulkSetTags(taskIDs []string, locators []TaskLocator, tags []string) (BulkResult, error) {
	normalized, err := normalizeBulkTags(tags)
	if err != nil {
		return BulkResult{}, err
	}
	return s.bulkAttribute(taskIDs, locators, "Set Tags", "tags", nil, func(t *models.Task) {
		t.Tags = append([]string{}, normalized...)
		t.UpdatedAt = s.clock.Now()
	})
}

// BulkPatchTagsByLocators applies a tri-state tag edit to every selected task.
// Tags in add are made present everywhere, tags in remove are made absent
// everywhere, and all other memberships are preserved independently. The two
// sets are validated before any task is touched and every project commits in
// the same transaction.
func (s *Service) BulkPatchTagsByLocators(
	locators []TaskLocator,
	add []string,
	remove []string,
) (BulkResult, error) {
	normalizedAdd, err := normalizeBulkTags(add)
	if err != nil {
		return BulkResult{}, err
	}
	normalizedRemove, err := normalizeBulkTags(remove)
	if err != nil {
		return BulkResult{}, err
	}
	removeSet := make(map[string]bool, len(normalizedRemove))
	for _, tag := range normalizedRemove {
		removeSet[tag] = true
	}
	for _, tag := range normalizedAdd {
		if removeSet[tag] {
			return BulkResult{}, invalidInput("tag", fmt.Sprintf("%q cannot be added and removed", tag))
		}
	}
	addSet := make(map[string]bool, len(normalizedAdd))
	for _, tag := range normalizedAdd {
		addSet[tag] = true
	}
	return s.bulkAttribute(nil, locators, "Set Tags", "tags", nil, func(t *models.Task) {
		next := make([]string, 0, len(t.Tags)+len(normalizedAdd))
		present := make(map[string]bool, len(t.Tags)+len(normalizedAdd))
		for _, existing := range models.DedupeTags(t.Tags) {
			if removeSet[existing] || present[existing] {
				continue
			}
			present[existing] = true
			next = append(next, existing)
		}
		for _, tag := range normalizedAdd {
			if present[tag] {
				continue
			}
			present[tag] = true
			next = append(next, tag)
		}
		t.Tags = next
		t.UpdatedAt = s.clock.Now()
	})
}

func normalizeBulkTags(tags []string) ([]string, error) {
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, raw := range tags {
		tag := models.NormalizeTag(raw)
		if err := models.ValidateTag(tag); err != nil {
			return nil, invalidInput("tag", err.Error())
		}
		if seen[tag] {
			continue
		}
		seen[tag] = true
		normalized = append(normalized, tag)
	}
	return normalized, nil
}

// BulkSetPinned pins or unpins every selected task, committing every touched
// project as one transaction. Each newly pinned task receives the same operation
// timestamp; unpinning clears both the flag and ordering timestamp.
func (s *Service) BulkSetPinned(taskIDs []string, pinned bool) (BulkResult, error) {
	return s.bulkSetPinned(taskIDs, nil, pinned)
}

func (s *Service) BulkSetPinnedByLocators(locators []TaskLocator, pinned bool) (BulkResult, error) {
	return s.bulkSetPinned(nil, locators, pinned)
}

func (s *Service) bulkSetPinned(taskIDs []string, locators []TaskLocator, pinned bool) (BulkResult, error) {
	now := s.clock.Now()
	action := "Unpin Tasks"
	if pinned {
		action = "Pin Tasks"
	}
	return s.bulkAttribute(taskIDs, locators, action, "pin", nil, func(t *models.Task) {
		t.Pinned = pinned
		if pinned {
			pinnedAt := now
			t.PinnedAt = &pinnedAt
		} else {
			t.PinnedAt = nil
		}
		t.UpdatedAt = now
	})
}
