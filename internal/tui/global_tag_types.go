package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// TagProjectUsage tracks how many tasks in a project use a specific tag.
type TagProjectUsage struct {
	Project   *models.Project
	TaskCount int
}

// GlobalTagInfo holds a tag name, its total usage count, and per-project breakdown.
type GlobalTagInfo struct {
	Name      string
	TaskCount int // Total across all projects
	Projects  []TagProjectUsage
}

// ShowGlobalTagManagerMsg is sent to open the global tag manager view.
type ShowGlobalTagManagerMsg struct{}

// CloseGlobalTagManagerMsg is sent when the global tag manager should close.
type CloseGlobalTagManagerMsg struct{ PopViewMsg }

// GlobalTagRenamedMsg is sent after a tag has been renamed across projects.
type GlobalTagRenamedMsg struct {
	OldName      string
	NewName      string
	ProjectCount int
	TaskCount    int
}

// GlobalTagNavigateMsg is sent when navigating to a project with a tag filter.
type GlobalTagNavigateMsg struct {
	Project *models.Project
	Tag     string
}
