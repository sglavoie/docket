package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// TaskProjectRef represents a task and its owning project across single-task and
// bulk TUI flows.
type TaskProjectRef struct {
	Task    *models.Task
	Project *models.Project
}

// taskIDsFromTaskRefs extracts task IDs in input order, skipping invalid refs.
func taskLocatorsFromTaskRefs(refs []TaskProjectRef) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(refs))
	for _, ref := range refs {
		locator := ref.Locator()
		if !locator.Valid() {
			continue
		}
		locators = append(locators, locator)
	}
	return locators
}

// finishTaskBulkSelection clears task multi-select state after a successful bulk action.
