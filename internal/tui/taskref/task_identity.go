// Package taskref owns the presentation layer's stable task selection key and
// its conversions to and from the application layer's locator. It sits at L0:
// it names no view, no model of the shell, and nothing above itself.
package taskref

import (
	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

// TaskIdentity is a stable task selection key. ProjectID remains part of the
// identity because planning and cross-project Outliner selections span projects.
type TaskIdentity struct {
	ProjectID string
	TaskID    string
}

func (id TaskIdentity) Valid() bool { return id.ProjectID != "" && id.TaskID != "" }

// Locator converts the presentation-layer identity into the application-layer
// locator without discarding project ownership.
func (id TaskIdentity) Locator() app.TaskLocator {
	return app.TaskLocator{ProjectID: id.ProjectID, TaskID: id.TaskID}
}

// FromLocator is Locator's inverse, so a locator that came back from the
// application layer addresses the same selection it was built from.
func FromLocator(locator app.TaskLocator) TaskIdentity {
	return TaskIdentity{ProjectID: locator.ProjectID, TaskID: locator.TaskID}
}

// For builds the canonical TUI identity from a task and its owning project.
// Invalid or incomplete refs deliberately produce the zero value.
func For(task *models.Task, project *models.Project) TaskIdentity {
	if task == nil || project == nil || task.ID == "" || project.ID == "" {
		return TaskIdentity{}
	}
	return TaskIdentity{ProjectID: project.ID, TaskID: task.ID}
}
