package tui

import "github.com/sglavoie/tt/internal/models"

// GlobalInboxTask wraps a task with its parent project for cross-project display.
type GlobalInboxTask struct {
	Task    *models.Task
	Project *models.Project
}

// ShowGlobalInboxProcessorMsg opens the global inbox processor view.
type ShowGlobalInboxProcessorMsg struct{}

// CloseGlobalInboxProcessorMsg closes the global inbox processor and returns to dashboard.
type CloseGlobalInboxProcessorMsg struct {
	ProcessedCount int
	SkippedCount   int
}

// GlobalInboxProjectSavedMsg signals that project was saved and reloaded.
type GlobalInboxProjectSavedMsg struct {
	Error error
}
