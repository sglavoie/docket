package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// taskOpsView abstracts the view-specific operations needed by the shared
// task-action keymaps. Task-row views that support task mutations (status,
// priority, due date, etc.) implement this interface so the standard keymap
// helpers can look up the cursor task and surface status/errors uniformly.
//
// Embedding tea.Model ensures that concrete types satisfy BubbleTea's model
// interface at compile time, preventing runtime panics from unchecked type
// assertions in the shared helpers that return tea.Model.
type taskOpsView interface {
	tea.Model
	getTaskRef() (task *models.Task, project *models.Project)
	clampCursor()
	setStatus(string)
}
