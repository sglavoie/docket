package tui

import (
	"image/color"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// primaryProjectView is the interface that primary views (ProjectTree,
// Outliner) must implement to use the shared key dispatcher.
type primaryProjectView interface {
	getProject() *models.Project
	getCursorTask() *models.Task
	getConfig() *config.Config
	reloadFromDisk() tea.Cmd
	performUndo() tea.Cmd
	performRedo() tea.Cmd
}

// prefixableView is the interface that primary views must implement to use
// the shared prefix handlers for !, d, and s keys.
type prefixableView interface {
	getCursorTask() *models.Task
	getProject() *models.Project
	getMultiSelectedTasks() []*models.Task // nil = not in multi-select
	clearMultiSelect()
	refreshView()                                                  // buildTree / loadTasks / rebuildRows
	workflowPersistFn() persistFn                                  // for single-task operations
	setUndoContext(action, detail string)                          // sets undo label before persist
	setUndoTaskIDs(ids []string)                                   // sets targeted task IDs for undo
	saveAndRefreshBulk(prevStates map[string]*models.Task) tea.Cmd // bulk: persist + refresh
	setStatusMessage(msg string)                                   // sets the status bar text
	setMutationFlash(bg color.Color)                              // triggers mutation flash with semantic color
}

// deleteConfirmView is the interface that views must implement to use the
// shared delete confirm handler.
type deleteConfirmView interface {
	getCursorTask() *models.Task
	getMultiSelectedTasks() []*models.Task
	showDeleteConfirm(task *models.Task)
	showBulkDeleteConfirm()
}

// taskSpecificPrefixKeys lists prefix keys that require the cursor to be on a task.
var taskSpecificPrefixKeys = map[string]bool{
	",": true, "!": true, "d": true, "s": true, "t": true, "b": true, "m": true, ".": true, "c": true,
}
