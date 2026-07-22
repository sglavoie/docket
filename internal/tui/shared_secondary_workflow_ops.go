package tui

import (
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// workflowOpsView is the seam used by migrated secondary views (weekly
// review, priority, due-date, focus, focus-board, project-focus) satisfy so their
// single-task keybindings commit through the application-layer TaskItemCoordinator
// while still running the legacy workflow-rule cascade. It is the async,
// coordinator-backed replacement for the synchronous taskOps* + workflowPersist
// path: every wf* helper returns a tea.Cmd resolving to a BatchTransactionResultMsg
// (or TransactionErrorMsg), which the shell handles by reloading every touched
// project, persisting fired rule-trigger counts, and refreshing the active view.
//
// The application service captures the repository snapshot before every
// mutation, so all views share contextual workflow semantics.
type workflowOpsView interface {
	getTaskRef() (task *models.Task, project *models.Project)
	taskItemCoordinator() TaskItemCoordinator
	workflowRules() []workflow.WorkflowRule
}

// wfOpts builds workflow options for a single-task mutation on the
// cursor task, labeling the ledger entry with action and the task context.
func wfOpts(v workflowOpsView, task *models.Task, action string) app.SingleTaskWorkflowOptions {
	return app.SingleTaskWorkflowOptions{
		Rules:       v.workflowRules(),
		UndoAction:  action,
		UndoContext: fmt.Sprintf("#%d %s", task.ShortID, task.Title),
	}
}

// wfChangeState transitions the cursor task to newState, running the workflow
// cascade. It is the coordinator-backed replacement for taskOpsChangeStatus and
// takes a canonical WorkflowState directly rather than a legacy TaskStatus.
func wfChangeState(v workflowOpsView, newState models.WorkflowState) tea.Cmd {
	task, _ := v.getTaskRef()
	if task == nil {
		return nil
	}
	opts := wfOpts(v, task, StateVerb(newState)+" Task")
	return v.taskItemCoordinator().TransitionWorkflow(task.ID, newState, opts,
		func(res app.TaskResult) string {
			return fmt.Sprintf("Task #%d -> %s", res.ShortID, newState)
		})
}

// wfChangePriority sets the cursor task's priority from a numeric key string.
func wfChangePriority(v workflowOpsView, key string) tea.Cmd {
	task, _ := v.getTaskRef()
	if task == nil {
		return nil
	}
	priority, _ := strconv.Atoi(key)
	// Preserve the legacy no-op: an unchanged priority does not commit (matching
	// applyPriorityChange's early return).
	if task.Priority == priority {
		return nil
	}
	opts := wfOpts(v, task, "Set Priority")
	return v.taskItemCoordinator().SetPriorityWorkflow(task.ID, priority, opts,
		func(app.TaskResult) string {
			return fmt.Sprintf("Priority set to %d", priority)
		})
}

// wfSetQuickDueDate sets the cursor task's due date to a relative day offset.
func wfSetQuickDueDate(v workflowOpsView, daysFromNow int) tea.Cmd {
	task, _ := v.getTaskRef()
	if task == nil {
		return nil
	}
	due, label := dueDateFromDays(daysFromNow, false)
	opts := wfOpts(v, task, "Set Due Date")
	return v.taskItemCoordinator().SetDueDateWorkflow(task.ID, due, false, opts,
		func(app.TaskResult) string {
			return fmt.Sprintf("Due date set to %s", label)
		})
}

// wfClearDueDate clears the cursor task's due date.
func wfClearDueDate(v workflowOpsView) tea.Cmd {
	task, _ := v.getTaskRef()
	if task == nil {
		return nil
	}
	opts := wfOpts(v, task, "Clear Due Date")
	return v.taskItemCoordinator().SetDueDateWorkflow(task.ID, nil, false, opts,
		func(app.TaskResult) string { return "Due date cleared" })
}

// wfQuickSnooze sets or clears (nil snoozeDate) the cursor task's snooze date.
func wfQuickSnooze(v workflowOpsView, snoozeDate *time.Time) tea.Cmd {
	task, _ := v.getTaskRef()
	if task == nil {
		return nil
	}
	opts := wfOpts(v, task, "Snooze")
	return v.taskItemCoordinator().SnoozeWorkflow(task.ID, snoozeDate, opts,
		func(app.TaskResult) string {
			if snoozeDate == nil {
				return "Snooze cleared"
			}
			return fmt.Sprintf("Snoozed until %s", snoozeDate.Format("Mon Jan 2"))
		})
}

// wfSectionMove moves the cursor task to the given GTD section.
func wfSectionMove(v workflowOpsView, section string) tea.Cmd {
	task, _ := v.getTaskRef()
	if task == nil {
		return nil
	}
	opts := wfOpts(v, task, "Move to Section")
	return v.taskItemCoordinator().SetSectionWorkflow(task.ID, section, opts,
		func(app.TaskResult) string {
			name := section
			if name == "" {
				name = "Active"
			}
			return fmt.Sprintf("Moved to %s", name)
		})
}

// wfTogglePin flips the cursor task's Pinned flag, running the workflow cascade.
// It is the coordinator-backed replacement for taskOpsTogglePinned.
func wfTogglePin(v workflowOpsView) tea.Cmd {
	task, _ := v.getTaskRef()
	if task == nil {
		return nil
	}
	opts := wfOpts(v, task, "Toggle Pin")
	return v.taskItemCoordinator().TogglePinnedWorkflow(task.ID, nil, opts,
		func(res app.TaskResult) string {
			return fmt.Sprintf("Toggled pin on #%d", res.ShortID)
		})
}

// wfToggleDim flips the cursor task's Dimmed flag, running the workflow cascade.
// It is the coordinator-backed replacement for taskOpsToggleDimmed.
func wfToggleDim(v workflowOpsView) tea.Cmd {
	task, _ := v.getTaskRef()
	if task == nil {
		return nil
	}
	opts := wfOpts(v, task, "Toggle Dim")
	return v.taskItemCoordinator().ToggleDimmedWorkflow(task.ID, opts,
		func(res app.TaskResult) string {
			return fmt.Sprintf("Toggled dim on #%d", res.ShortID)
		})
}
