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

// This file holds the shared bulk-mutation builders used by the flat list views
// (Daily Review, Due Date Overview, Backlog/Focus, Priority, Weekly Review).
// Each builder resolves the selected task IDs, builds the application-layer
// BulkCoordinator command, and returns finalizeBatchResult(cmd, ids, summary) so
// the shell owns reload, selection-clear, recent-visit repair, and rule-trigger
// persistence — exactly the reference shape proven on All Tasks. The view-facing
// wording ("Changed %d tasks to %s", etc.) matches the previous synchronous
// path so migrated toasts read identically. A nil return means nothing was
// selected and the caller should no-op.

// bulkListStateCmd transitions every selected task to the given canonical
// WorkflowState, running the workflow-rule cascade and dropping tasks that
// cannot transition (SkipInvalid), matching the legacy per-task guard.
func bulkListStateCmd(bulk BulkCoordinator, rules []workflow.WorkflowRule, items []TaskProjectRef, state models.WorkflowState) tea.Cmd {
	taskIDs := taskIDsFromTaskRefs(items)
	if len(taskIDs) == 0 {
		return nil
	}
	cmd := bulk.TransitionWithWorkflow(
		app.BulkTransitionRequest{
			TaskIDs:     taskIDs,
			Target:      state,
			Rules:       rules,
			UndoAction:  StateVerb(state) + " Task",
			UndoContext: fmt.Sprintf("%d tasks", len(taskIDs)),
			SkipInvalid: true,
		},
		func(changed int) string {
			if changed == 0 {
				return "No tasks were changed"
			}
			return fmt.Sprintf("Changed %d tasks to %s", changed, state)
		},
	)
	return finalizeBatchResult(cmd, taskIDs, "")
}

// bulkListPriorityCmd sets one priority on every selected task.
func bulkListPriorityCmd(bulk BulkCoordinator, items []TaskProjectRef, priority int) tea.Cmd {
	taskIDs := taskIDsFromTaskRefs(items)
	if len(taskIDs) == 0 {
		return nil
	}
	cmd := bulk.SetPriority(taskIDs, priority)
	return finalizeBatchResult(cmd, taskIDs, fmt.Sprintf("Set priority %d on %d tasks", priority, len(taskIDs)))
}

// bulkListDueDateCmd sets a due date daysFromNow on every selected task.
func bulkListDueDateCmd(bulk BulkCoordinator, items []TaskProjectRef, daysFromNow int) tea.Cmd {
	taskIDs := taskIDsFromTaskRefs(items)
	if len(taskIDs) == 0 {
		return nil
	}
	dueDate := util.StartOfDay(time.Now().AddDate(0, 0, daysFromNow))
	cmd := bulk.SetDueDate(taskIDs, dueDate, false)
	return finalizeBatchResult(cmd, taskIDs, fmt.Sprintf("Due date set on %d tasks", len(taskIDs)))
}

// bulkListClearDueDateCmd clears the due date on every selected task.
func bulkListClearDueDateCmd(bulk BulkCoordinator, items []TaskProjectRef) tea.Cmd {
	taskIDs := taskIDsFromTaskRefs(items)
	if len(taskIDs) == 0 {
		return nil
	}
	cmd := bulk.ClearDueDate(taskIDs)
	return finalizeBatchResult(cmd, taskIDs, fmt.Sprintf("Cleared due date on %d tasks", len(taskIDs)))
}

// bulkListSnoozeCmd snoozes every selected task until the given time (or clears
// the snooze when until is nil).
func bulkListSnoozeCmd(bulk BulkCoordinator, items []TaskProjectRef, until *time.Time) tea.Cmd {
	taskIDs := taskIDsFromTaskRefs(items)
	if len(taskIDs) == 0 {
		return nil
	}
	cmd := bulk.Snooze(taskIDs, until)
	return finalizeBatchResult(cmd, taskIDs, fmt.Sprintf("Snoozed %d tasks", len(taskIDs)))
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
	taskIDs := taskIDsFromTaskRefs(items)
	if len(taskIDs) == 0 {
		return nil, true
	}
	cmd := bulk.TransitionWithWorkflow(
		app.BulkTransitionRequest{
			TaskIDs:     taskIDs,
			Target:      target,
			Rules:       rules,
			UndoAction:  "Move to Section",
			UndoContext: fmt.Sprintf("%d tasks", len(taskIDs)),
			SkipInvalid: true,
		},
		func(changed int) string {
			if changed == 0 {
				return "No tasks were moved"
			}
			return fmt.Sprintf("Moved %d tasks to %s", changed, section)
		},
	)
	return finalizeBatchResult(cmd, taskIDs, ""), true
}
