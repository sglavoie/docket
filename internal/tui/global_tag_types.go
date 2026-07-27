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
	Pinned    bool
}

// ShowGlobalTagManagerMsg opens the aggregate tag manager over the captured
// project set. An omitted set retains the legacy app-wide behavior.
type ShowGlobalTagManagerMsg struct {
	ProjectSet ProjectSetScope
}

// CloseGlobalTagManagerMsg is sent when the global tag manager should close.
type CloseGlobalTagManagerMsg struct{ PopViewMsg }

// GlobalTagRenamedMsg is sent after a tag has been renamed across projects.
type GlobalTagRenamedMsg struct {
	OldName      string
	NewName      string
	ProjectCount int
	TaskCount    int
}

// GlobalTagPinnedMsg is sent after a global pinned tag is toggled.
type GlobalTagPinnedMsg struct {
	Name   string
	Pinned bool
}

// GlobalTagManagerErrorMsg reports validation or persistence errors in the
// global tag manager.
type GlobalTagManagerErrorMsg struct {
	Err error
}

// GlobalTagNavigateMsg is sent when navigating to a project with a tag filter.
type GlobalTagNavigateMsg struct {
	Project *models.Project
	Tag     string
}
