package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// =============================================================================
// Clipboard Messages
// =============================================================================

// CopyTaskTitleMsg copies a single task's title to clipboard.
type CopyTaskTitleMsg struct{ Task *models.Task }

// CopyTaskDetailsMsg copies a single task's full details to clipboard.
type CopyTaskDetailsMsg struct{ Task *models.Task }

// CopyTaskTitleAndDescMsg copies a single task's title and description to clipboard.
type CopyTaskTitleAndDescMsg struct{ Task *models.Task }

// TaskClipboardProjectedMsg carries application-owned clipboard text back to
// the TUI adapter, which performs the platform clipboard write.
type TaskClipboardProjectedMsg struct {
	Projection app.TaskClipboardProjection
}

// TaskCopiedMsg confirms task(s) were copied to clipboard.
type TaskCopiedMsg struct {
	Count        int
	FullDetail   bool
	TitleAndDesc bool
}
