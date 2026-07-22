package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
	"github.com/sglavoie/tt/internal/workflow"
)

// This file holds shared bulk-mutation builders used by task-row views
// (Daily Review, Due Date Overview, Backlog/Focus, Priority, Weekly Review).
// Each builder resolves the selected task IDs, builds the application-layer
// BulkCoordinator command, and returns finalizeBatchResult(cmd, ids, summary) so
// the shell owns reload, selection-clear, recent-visit repair, and rule-trigger
// persistence. The view-facing
// wording ("Changed %d tasks to %s", etc.) matches the previous synchronous
// path so migrated toasts read identically. A nil return means nothing was
// selected and the caller should no-op.

// bulkListStateCmd transitions every selected task to the given canonical
// WorkflowState, running the workflow-rule cascade and dropping tasks that
// cannot transition (SkipInvalid), matching the legacy per-task guard.
func bulkListStateCmd(bulk BulkCoordinator, rules []workflow.WorkflowRule, items []TaskProjectRef, state models.WorkflowState) tea.Cmd {
	locators := taskLocatorsFromTaskRefs(items)
	if len(locators) == 0 {
		return nil
	}
	cmd := bulk.TransitionWithWorkflow(
		app.BulkTransitionRequest{
			TaskLocators: locators,
			Target:       state,
			Rules:        rules,
			UndoAction:   StateVerb(state) + " Task",
			UndoContext:  fmt.Sprintf("%d tasks", len(locators)),
			SkipInvalid:  true,
		},
		func(changed int) string {
			if changed == 0 {
				return "No tasks were changed"
			}
			return fmt.Sprintf("Changed %d tasks to %s", changed, state)
		},
	)
	return finalizeLocatedBatchResult(cmd, locators, "")
}

// bulkListPriorityCmd sets one priority on every selected task.
func bulkListPriorityCmd(bulk BulkCoordinator, items []TaskProjectRef, priority int) tea.Cmd {
	locators := taskLocatorsFromTaskRefs(items)
	if len(locators) == 0 {
		return nil
	}
	cmd := bulk.SetPriorityLocators(locators, priority)
	return finalizeLocatedBatchResult(cmd, locators, fmt.Sprintf("Set priority %d on %d tasks", priority, len(locators)))
}

// bulkListDueDateCmd sets a due date daysFromNow on every selected task.
func bulkListDueDateCmd(bulk BulkCoordinator, items []TaskProjectRef, daysFromNow int) tea.Cmd {
	locators := taskLocatorsFromTaskRefs(items)
	if len(locators) == 0 {
		return nil
	}
	dueDate := util.StartOfDay(time.Now().AddDate(0, 0, daysFromNow))
	cmd := bulk.SetDueDateLocators(locators, dueDate, false)
	return finalizeLocatedBatchResult(cmd, locators, fmt.Sprintf("Due date set on %d tasks", len(locators)))
}

// bulkListClearDueDateCmd clears the due date on every selected task.
func bulkListClearDueDateCmd(bulk BulkCoordinator, items []TaskProjectRef) tea.Cmd {
	locators := taskLocatorsFromTaskRefs(items)
	if len(locators) == 0 {
		return nil
	}
	cmd := bulk.ClearDueDateLocators(locators)
	return finalizeLocatedBatchResult(cmd, locators, fmt.Sprintf("Cleared due date on %d tasks", len(locators)))
}

// bulkListSnoozeCmd snoozes every selected task until the given time (or clears
// the snooze when until is nil).
func bulkListSnoozeCmd(bulk BulkCoordinator, items []TaskProjectRef, until *time.Time) tea.Cmd {
	locators := taskLocatorsFromTaskRefs(items)
	if len(locators) == 0 {
		return nil
	}
	cmd := bulk.SnoozeLocators(locators, until)
	return finalizeLocatedBatchResult(cmd, locators, fmt.Sprintf("Snoozed %d tasks", len(locators)))
}

// bulkListSectionMoveCmd moves every selected task to the workflow state the
// target GTD section maps onto (sections are retired; a "send to section X" bulk
// action is a state transition). The bool reports whether section was a
// recognized name; false means the caller should surface an "unknown section"
// message. A nil cmd with ok==true means nothing was selected.
func bulkListSectionMoveCmd(bulk BulkCoordinator, rules []workflow.WorkflowRule, items []TaskProjectRef, section string) (tea.Cmd, bool) {
	target, ok := stateForSectionName(section)
	if !ok {
		return nil, false
	}
	locators := taskLocatorsFromTaskRefs(items)
	if len(locators) == 0 {
		return nil, true
	}
	cmd := bulk.TransitionWithWorkflow(
		app.BulkTransitionRequest{
			TaskLocators: locators,
			Target:       target,
			Rules:        rules,
			UndoAction:   "Move to Section",
			UndoContext:  fmt.Sprintf("%d tasks", len(locators)),
			SkipInvalid:  true,
		},
		func(changed int) string {
			if changed == 0 {
				return "No tasks were moved"
			}
			return fmt.Sprintf("Moved %d tasks to %s", changed, section)
		},
	)
	return finalizeLocatedBatchResult(cmd, locators, ""), true
}
