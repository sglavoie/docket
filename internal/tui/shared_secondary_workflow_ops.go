package tui

import (
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// workflowOpsView is the seam the migrated secondary views (all-tasks, weekly
// review, priority, due-date, focus, focus-board, project-focus) satisfy so their
// single-task keybindings commit through the application-layer TaskItemCoordinator
// while still running the legacy workflow-rule cascade. It is the async,
// coordinator-backed replacement for the synchronous taskOps* + workflowPersist
// path: every wf* helper returns a tea.Cmd resolving to a BatchTransactionResultMsg
// (or TransactionErrorMsg), which the shell handles by reloading every touched
// project, persisting fired rule-trigger counts, and refreshing the active view.
//
// The views supply no prior-state snapshot, so evaluation is stateless — exactly
// what the legacy secondary-view save path did (it built its WorkflowSaveOptions
// with a nil PrevStates). The primary views, which captured prior states, are
// deliberately out of scope for this migration.
type workflowOpsView interface {
	getTaskRef() (task *models.Task, project *models.Project)
	taskItemCoordinator() TaskItemCoordinator
	workflowRules() []workflow.WorkflowRule
}

// wfOpts builds the stateless workflow options for a single-task mutation on the
// cursor task, labeling the ledger entry with action and the task context.
func wfOpts(v workflowOpsView, task *models.Task, action string) app.SingleTaskWorkflowOptions {
	return app.SingleTaskWorkflowOptions{
		Rules:       v.workflowRules(),
		Contextual:  false,
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

// wfUnblock returns the cursor task from the waiting state to the Next queue
// (todo + Next section), running the workflow cascade. The focus-board caller
// guards that the task is actually blocked before invoking this.
func wfUnblock(v workflowOpsView) tea.Cmd {
	task, _ := v.getTaskRef()
	if task == nil {
		return nil
	}
	opts := wfOpts(v, task, "Unblock Task")
	return v.taskItemCoordinator().TransitionWorkflow(task.ID, models.StateNext, opts,
		func(res app.TaskResult) string {
			return fmt.Sprintf("Task #%d unblocked", res.ShortID)
		})
}

// wfDeleteTask deletes the cursor task's subtree through the coordinator, folding
// the durable planning-ref cleanup (Focus Queue, Daily Plan, dismissals) into the
// same transaction as a config artifact. Machine-local recent visits are repaired
// by the shell after the commit (see handleBatchTransactionResult). cfg/mgr are the
// shell's authoritative config so the in-memory and on-disk views agree once
// committed.
func wfDeleteTask(v workflowOpsView, cfg *config.Config, mgr *config.Manager) tea.Cmd {
	task, project := v.getTaskRef()
	if task == nil || project == nil {
		return nil
	}
	shortID := task.ShortID

	var artifacts []storage.ArtifactChange
	if cfg != nil && mgr != nil {
		idsToDelete := collectDescendantIDs(project.Tasks, map[string]bool{task.ID: true})
		if cfg.RemoveDurableTaskRefs(idsToDelete) > 0 {
			arts, err := configDomainArtifacts(mgr, cfg)
			if err != nil {
				return func() tea.Msg { return ErrorMsg{Err: err} }
			}
			artifacts = arts
		}
	}

	return v.taskItemCoordinator().DeleteTask(task.ID, artifacts,
		func(app.DeleteResult) string {
			return fmt.Sprintf("Deleted task #%d", shortID)
		})
}
