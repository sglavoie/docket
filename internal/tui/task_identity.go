package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TaskIdentity is a stable task selection key. ProjectID remains part of the
// identity because Planner and cross-project Outliner selections span projects.
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

func taskIdentityFromLocator(locator app.TaskLocator) TaskIdentity {
	return TaskIdentity{ProjectID: locator.ProjectID, TaskID: locator.TaskID}
}

func configTaskLocatorsFromApp(locators []app.TaskLocator) []config.ProjectTaskLocator {
	result := make([]config.ProjectTaskLocator, 0, len(locators))
	for _, locator := range locators {
		if locator.ProjectID != "" && locator.TaskID != "" {
			result = append(result, config.ProjectTaskLocator{ProjectID: locator.ProjectID, TaskID: locator.TaskID})
		}
	}
	return result
}

func appTaskLocatorsFromConfig(locators []config.ProjectTaskLocator) []app.TaskLocator {
	result := make([]app.TaskLocator, 0, len(locators))
	for _, locator := range locators {
		if locator.ProjectID != "" && locator.TaskID != "" {
			result = append(result, app.TaskLocator{ProjectID: locator.ProjectID, TaskID: locator.TaskID})
		}
	}
	return result
}

// taskIdentityFor builds the canonical TUI identity from a task and its owning
// project. Invalid or incomplete refs deliberately produce the zero value.
func taskIdentityFor(task *models.Task, project *models.Project) TaskIdentity {
	if task == nil || project == nil || task.ID == "" || project.ID == "" {
		return TaskIdentity{}
	}
	return TaskIdentity{ProjectID: project.ID, TaskID: task.ID}
}

// Identity converts a qualified task ref into the canonical TUI selection key.
func (ref TaskProjectRef) Identity() TaskIdentity {
	return taskIdentityFor(ref.Task, ref.Project)
}

// Locator converts a qualified task ref directly into an application locator.
func (ref TaskProjectRef) Locator() app.TaskLocator {
	return ref.Identity().Locator()
}
