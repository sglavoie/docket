package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
)

const minWidthForSplitView = 100 // Below this, use single-pane + TAB toggle

// TemplatePickerModel handles template selection during task creation.
type TemplatePickerModel struct {
	BaseSearchableView
	InteractiveViewState
	config  *config.Config
	project *models.Project

	templates         []template.TaskTemplate
	filteredTemplates []template.TaskTemplate

	// Group templates by category for display
	categoryOrder []string // Order of categories to display

	// Split-view state
	previewScroll  int  // Scroll offset for preview content
	showingPreview bool // For narrow mode: true = showing preview, false = showing list
}

// TemplatePickerDoneMsg is sent when a template is selected or skipped.
type TemplatePickerDoneMsg struct {
	Canceled bool
	Template *template.TaskTemplate
}

// ShowTemplatePickerMsg triggers showing the template picker.
type ShowTemplatePickerMsg struct {
	Project *models.Project
}

// ShowTaskCreatorWithTemplateMsg triggers showing the task creator with a template.
type ShowTaskCreatorWithTemplateMsg struct {
	Project  *models.Project
	Template *template.TaskTemplate
}
