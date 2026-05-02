package tui

import (
	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// CustomContextEditorModel handles creating/editing custom contexts.
type CustomContextEditorModel struct {
	BaseSearchableView
	config *config.Config

	// Editing state
	editMode     bool   // true if editing existing, false if creating new
	originalName string // Original name when editing

	// Form fields
	nameInput textinput.Model

	// Project selection
	projects         []*models.Project
	filteredProjects []*models.Project
	selectedIDs      map[string]bool // Project IDs that are selected
	cursor           int             // Cursor position in project list
	scrollOffset     int             // Scroll offset for project list
	projectListMode  bool            // true when focus is on project list

	discardConfirmState
	initialSelectedIDs map[string]bool
}

// CustomContextSavedMsg is sent when a custom context is saved.
type CustomContextSavedMsg struct {
	Name         string
	IsNew        bool
	ProjectCount int
}
