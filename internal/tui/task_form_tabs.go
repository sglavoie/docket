package tui

import "github.com/sglavoie/tt/internal/models"

// Task form tab constants (D3).
const (
	formTabBasics   = 0
	formTabSchedule = 1
	formTabChildren = 2
	formTabDetails  = 3
	formTabCount    = 4
)

var formTabNames = []string{"Basics", "Schedule", "Children", "Details"}

// Static field counts per tab (dynamic items like checklist are added on top).
const (
	formBasicsFieldCount   = 8 // Title, Description, Type, Priority, Parent, Section, Flags, Tags
	formScheduleFieldCount = 2 // Due, Recurrence
	formDetailsFieldCount  = 2 // URL, Source
)

// formStaticFieldCount returns the number of static focusable fields in a tab.
// Children tab returns 0 here; its count is computed dynamically.
func formStaticFieldCount(tab int) int {
	switch tab {
	case formTabBasics:
		return formBasicsFieldCount
	case formTabSchedule:
		return formScheduleFieldCount
	case formTabDetails:
		return formDetailsFieldCount
	case formTabChildren:
		return 0
	}
	return 0
}

// formAttrFieldCount returns the number of focusable attr rows for a given task type.
// Types without attrs return 0.
func formAttrFieldCount(t models.TaskType) int {
	switch t {
	case models.TaskTypeBug:
		return 3 // Reproducible, Severity, Workaround
	case models.TaskTypeIdea:
		return 4 // Category, Effort, Value, Validated
	case models.TaskTypeTask:
		return 2 // Effort, WorkType
	case models.TaskTypeStory:
		return 2 // StoryPoints, Persona
	case models.TaskTypeEpic:
		return 3 // Scope, Milestone, SuccessMetrics
	}
	return 0
}
