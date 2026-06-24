package tui

import "github.com/sglavoie/tt/internal/models"

// ShowInboxProcessorMsg opens the inbox processor view.
type ShowInboxProcessorMsg struct {
	Project *models.Project
}

// CloseInboxProcessorMsg closes the inbox processor and returns to project tree.
type CloseInboxProcessorMsg struct {
	ProcessedCount int
	SkippedCount   int
}

// InboxTaskProcessedMsg marks current task as processed.
type InboxTaskProcessedMsg struct {
	Task *models.Task
}

// InboxTaskSkippedMsg marks current task as skipped.
type InboxTaskSkippedMsg struct {
	Task *models.Task
}

// InboxProjectSavedMsg signals that project was saved and reloaded.
type InboxProjectSavedMsg struct {
	Error error
}

// TogglePlanTodayFromPaletteMsg requests the daily-plan toggle for the
// focused inbox task. The palette emits this after closing itself so the
// AppModel can run toggleDailyPlan() against the underlying inbox view.
type TogglePlanTodayFromPaletteMsg struct{}
