package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// Persist the canonical mode DTO for the active Outliner scope. Filter facets
// are run-local and typed navigation continues to own scope and route hierarchy.
func (m *AppModel) saveOutlinerFilterState() {
	outliner := m.crossOutliner
	if outliner == nil {
		return
	}
	if err := outliner.persistActiveRouteState(); err != nil {
		logging.Warn("failed to persist Outliner route state: %v", err)
	}
}

// applyTagsToTask applies selected tags to a single task as one asynchronous
// transaction through the task operation path. Setting tags is a value
// operation, so it commits through TaskItem.ExecuteOperation like every other
// `#` edit; the shell owns reload and toast via the batch result, and the
// durable write records through the workspace undo ledger.
func (m *AppModel) applyTagsToTask(identity taskref.TaskIdentity, task *models.Task, selectedTags []string) tea.Cmd {
	if task == nil || !identity.Valid() {
		return nil
	}
	return m.secondaryFeatures.TaskItem.ExecuteOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetTags,
		Targets:   []app.TaskLocator{identity.Locator()},
		Tags:      selectedTags,
	}, "Set Tags", fmt.Sprintf("Tagged #%d", task.ShortID))
}
