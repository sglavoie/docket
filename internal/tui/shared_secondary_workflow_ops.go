package tui

import (
	"fmt"
	"strconv"

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

// wfChangePriority sets the cursor task's priority from a numeric key string.
func wfChangePriority(v workflowOpsView, key string) tea.Cmd {
	task, project := v.getTaskRef()
	if task == nil || project == nil {
		return nil
	}
	priority, _ := strconv.Atoi(key)
	// Preserve the legacy no-op: an unchanged priority does not commit (matching
	// applyPriorityChange's early return).
	if task.Priority == priority {
		return nil
	}
	opts := wfOpts(v, task, "Set Priority")
	return v.taskItemCoordinator().SetPriorityWorkflow(taskIdentityFor(task, project), priority, opts,
		func(app.TaskResult) string {
			return fmt.Sprintf("Priority set to %d", priority)
		})
}
