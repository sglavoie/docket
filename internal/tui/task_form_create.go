package tui

import (
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// NewTaskFormModelCreate creates a TaskFormModel for creating a new task.
func NewTaskFormModelCreate(
	project *models.Project,
	runtime *config.RuntimeState,
	suggestedParent *models.Task,
) *TaskFormModel {
	draft := taskFormDraft{
		priority: models.PriorityUnset,
		state:    models.StateBacklog,
	}
	draft.parentID = formNearestValidParentID(suggestedParent)

	m := &TaskFormModel{
		project:    project,
		draft:      draft,
		activeTab:  formTabBasics,
		focusIndex: 0,
		width:      80,
		height:     24,
	}
	m.initTitleInput("")
	m.initDueInput("")
	m.initChecklistItemInput("")
	m.initSubtaskItemInput("")
	return m
}

// formNearestValidParentID walks up from the suggested parent to the nearest
// ancestor that can hold a child without breaching the nesting cap.
func formNearestValidParentID(suggestedParent *models.Task) *string {
	for candidate := suggestedParent; candidate != nil; candidate = candidate.Parent {
		if candidate.Level+1 <= models.MaxNestingDepth {
			id := candidate.ID
			return &id
		}
	}
	return nil
}

// cloneTime returns a deep copy of a *time.Time, or nil if the input is nil.
func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}
