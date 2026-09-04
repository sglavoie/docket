package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskref"
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

// ParentSelectedMsg returns the selected parent from parent selector.
type ParentSelectedMsg struct {
	ParentID     *string                     // nil means "None"
	Source       presentation.PresentationID // Which view to send result to
	ReparentTask taskref.TaskIdentity        // Forwarded from OpenParentSelectorMsg for delayed reparent operations
}

// CloseParentSelectorMsg closes the parent selector without selection.
type CloseParentSelectorMsg struct{ route.PopViewMsg }

// ShowTaskComposerMsg opens the canonical typed composer preset directly.
type ShowTaskComposerMsg struct {
	Preset TaskComposerPreset
}
