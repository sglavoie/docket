package taskref

import (
	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

// TaskProjectRef represents a task and its owning project across single-task and
// bulk TUI flows. It lives beside the identity it converts to, so the packages
// that only need a qualified task can name one without reaching into the shell.
type TaskProjectRef struct {
	Task    *models.Task
	Project *models.Project
}

// Identity converts a qualified task ref into the canonical TUI selection key.
func (ref TaskProjectRef) Identity() TaskIdentity {
	return For(ref.Task, ref.Project)
}

// Locator converts a qualified task ref directly into an application locator.
func (ref TaskProjectRef) Locator() app.TaskLocator {
	return ref.Identity().Locator()
}

// TaskLocatorsFromTaskRefs extracts locators in input order, skipping invalid refs.
func TaskLocatorsFromTaskRefs(refs []TaskProjectRef) []app.TaskLocator {
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
