package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// handleCommandPaletteTaskAction handles inline task actions from the command
// palette. It applies a status change (complete, start, block, cancel, reopen,
// review) through the application-layer state transition, which enforces the
// canonical workflow rules and commits one ledger-undoable ChangeSet. The legacy
// full-workflow save (event-rule cascade, completion gamification) is intentionally
// dropped, matching the primary-view migration.
func (m *AppModel) handleCommandPaletteTaskAction(msg CommandPaletteTaskActionMsg) (bool, tea.Cmd) {
	if msg.Task == nil || msg.Project == nil {
		return true, nil
	}

	// Map action name to target status; an unknown action is a no-op.
	statusMap := map[string]models.TaskStatus{
		"complete": models.StatusDone,
		"start":    models.StatusInProgress,
		"block":    models.StatusBlocked,
		"cancel":   models.StatusCanceled,
		"reopen":   models.StatusTodo,
		"review":   models.StatusReview,
	}
	targetStatus, ok := statusMap[msg.Action]
	if !ok {
		return true, nil
	}

	res, err := m.appService.Transition(app.StateRequest{
		TaskID: msg.Task.ID,
		Target: models.TargetStateForStatus(targetStatus),
	})
	if err != nil {
		return true, errorCmd("%s", err)
	}
	m.reloadProjectByID(res.ProjectID)
	return true, m.handleSaveComplete(SaveCompleteMsg{
		SuccessMessage: fmt.Sprintf("Task %s: %s", msg.Action, res.Title),
	})
}

// handleInlineTaskCreate creates a new task from the command palette inline
// creation. It routes through the application-layer capture use case (Inbox
// section, prepended for prominence), which validates the title and commits one
// ledger-undoable ChangeSet.
func (m *AppModel) handleInlineTaskCreate(msg InlineTaskCreateMsg) (bool, tea.Cmd) {
	if msg.Project == nil || msg.Title == "" {
		return true, nil
	}

	// Clamp an out-of-range priority to the domain default (0 -> PriorityDefault
	// inside Capture), preserving the legacy handler's lenient clamping rather than
	// rejecting the capture.
	priority := msg.Priority
	if priority < 1 || priority > 5 {
		priority = 0
	}

	res, err := m.appService.Capture(app.CaptureRequest{
		ProjectID: msg.Project.ID,
		Title:     msg.Title,
		Priority:  priority,
		Tags:      msg.Tags,
		Due:       msg.DueDate,
		Section:   models.SectionInbox,
		Prepend:   true,
	})
	if err != nil {
		return true, errorCmd("%s", err)
	}
	m.reloadProjectByID(res.ProjectID)
	return true, m.handleSaveComplete(SaveCompleteMsg{
		SuccessMessage: fmt.Sprintf("Created: %s", res.Title),
	})
}
