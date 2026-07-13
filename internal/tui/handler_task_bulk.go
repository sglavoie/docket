package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// handleCommandPaletteBulkAction handles bulk actions on multiple selected tasks.
// The transition commits through the application layer's bulk-transition use case:
// every touched project (including workflow-rule move destinations) commits as one
// transaction with a workspace-ledger undo entry, and the workflow-rule cascade
// runs exactly as it does on a single-task save. SkipInvalid makes the batch
// best-effort — tasks that cannot transition (already terminal, dependency-blocked
// completion, or incomplete children) are dropped and the toast reports the count
// actually changed — preserving the legacy multiselect semantics.
func (m *AppModel) handleCommandPaletteBulkAction(msg CommandPaletteBulkActionMsg) (bool, tea.Cmd) {
	if len(msg.Tasks) == 0 {
		return true, nil
	}

	refs := validTaskProjectRefs(msg.Tasks)
	taskIDs := taskIDsFromTaskRefs(refs)
	if len(taskIDs) == 0 {
		return true, nil
	}

	var target models.WorkflowState
	var undoAction, actionPast string
	switch msg.Action {
	case "complete":
		target, undoAction, actionPast = models.StateDone, "Complete Task", "completed"
	case "start":
		target, undoAction, actionPast = models.StateInProgress, "Start Task", "started"
	case "review":
		target, undoAction, actionPast = models.StateReview, "Review Task", "sent to review"
	default:
		return true, nil
	}

	// Defer clearing the multiselect to the shell's batch-result handler: the
	// transition commits asynchronously through the application layer, and the
	// selection is cleared only once the commit succeeds (a rejected batch keeps it
	// so the user can retry).
	cmd := m.secondaryFeatures.Bulk.TransitionWithWorkflow(
		app.BulkTransitionRequest{
			TaskIDs:     taskIDs,
			Target:      target,
			Rules:       m.config.GetEnabledWorkflowRules(),
			UndoAction:  undoAction,
			UndoContext: fmt.Sprintf("%d tasks", len(taskIDs)),
			SkipInvalid: true,
		},
		func(changed int) string {
			if changed == 0 {
				return "No tasks were changed"
			}
			return fmt.Sprintf("%d tasks %s", changed, actionPast)
		},
	)
	return true, finalizeBatchResult(cmd, taskIDs, "")
}
