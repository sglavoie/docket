package taskform

import (
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// =============================================================================
// Task CRUD Messages
// =============================================================================

// CreateTaskMsg opens the task creator, optionally with a parent task.
type CreateTaskMsg struct {
	ParentTask *models.Task
	Project    *models.Project // Optional: when set, creates in this project instead of currentProject
}

// =============================================================================
// Parent Selector Messages (Show/Close/Done Pattern)
// =============================================================================

// OpenParentSelectorMsg opens the parent task selector.
type OpenParentSelectorMsg struct {
	ValidParents []*models.Task
	CurrentID    *string                     // Pre-select current parent
	AllowNone    bool                        // Whether "None" is valid option
	Source       presentation.PresentationID // Which view opened it
	ReparentTask taskref.TaskIdentity        // When valid, reparent this qualified task instead of updating a form field
}

// The parent selector's own two outcome messages live with the selector that
// constructs them; only the opening message is written here, because the shell
// is what opens it.

// ShowTaskComposerMsg opens the canonical typed composer preset directly.
type ShowTaskComposerMsg struct {
	Preset TaskComposerPreset
}
