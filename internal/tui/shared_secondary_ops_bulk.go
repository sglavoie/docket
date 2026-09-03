package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// TaskProjectRef represents a task and its owning project across single-task and
// bulk TUI flows.
type TaskProjectRef struct {
	Task    *models.Task
	Project *models.Project
}

// Identity converts a qualified task ref into the canonical TUI selection key.
// Go pins this method to TaskProjectRef's package, so it stays in the shell
// while the identity it builds lives in taskref.
func (ref TaskProjectRef) Identity() taskref.TaskIdentity {
	return taskref.For(ref.Task, ref.Project)
}

// Locator converts a qualified task ref directly into an application locator.
func (ref TaskProjectRef) Locator() app.TaskLocator {
	return ref.Identity().Locator()
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
