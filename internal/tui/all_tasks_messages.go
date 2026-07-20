package tui

import "github.com/sglavoie/tt/internal/models"

// ShowAllTasksMsg is sent to open the all tasks view.
// When SetGroup is true, the view opens grouped by Group (e.g. By-Due-Date).
type ShowAllTasksMsg struct {
	Group    AllTasksGroupBy
	SetGroup bool
}

// CloseAllTasksMsg is sent when the all tasks view should close.
type CloseAllTasksMsg struct{ PopToAnchorMsg }

// AllTasksSelectMsg is sent when a task is selected for navigation.
type AllTasksSelectMsg struct {
	Task    *models.Task
	Project *models.Project
}

// AllTasksRefreshMsg is sent to refresh the all tasks view.
type AllTasksRefreshMsg struct{}
