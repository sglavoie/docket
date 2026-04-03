package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/template"
)

// TaskTemplatesModel handles the task templates management view.
// The implementation is split across multiple files:
// - template_list.go: core model, constructor, and messages
// - template_list_update.go: Update() and state mutation helpers
// - template_list_view.go: View() and list rendering
// - template_list_preview.go: preview rendering helpers
type TaskTemplatesModel struct {
	BaseSearchableView
	config        *config.Config
	configManager *config.Manager

	InteractiveViewState
	allTemplates      []template.TaskTemplate
	filteredTemplates []template.TaskTemplate
	defaultCount      int // Number of default templates (at start of allTemplates slice)

	expandedView  bool
	previewScroll int // Scroll offset for preview pane content
}

// NewTaskTemplatesModel creates a new task templates model.
func NewTaskTemplatesModel(cfg *config.Config, manager *config.Manager) *TaskTemplatesModel {
	defaultTemplates, customTemplates := cfg.GetTaskTemplatesSorted()
	allTemplates := append(defaultTemplates, customTemplates...)
	m := &TaskTemplatesModel{
		config:            cfg,
		configManager:     manager,
		allTemplates:      allTemplates,
		filteredTemplates: allTemplates,
		defaultCount:      len(defaultTemplates),
	}
	m.SetSize(80, 24)
	m.InitSearchInput("Search templates...", 50, 30)
	return m
}

// SetSize sets the view dimensions.
func (m *TaskTemplatesModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
}

func (m *TaskTemplatesModel) Init() tea.Cmd {
	return nil
}

// isDefaultTemplate returns true if the template with the given ID is a default template.
func (m *TaskTemplatesModel) isDefaultTemplate(id string) bool {
	for i := 0; i < m.defaultCount && i < len(m.allTemplates); i++ {
		if m.allTemplates[i].ID == id {
			return true
		}
	}
	return false
}

// isWideScreen returns true if the screen is wide enough for split layout (≥81 chars)
func (m *TaskTemplatesModel) isWideScreen() bool {
	return m.Width() >= 81
}

// TaskTemplatesUpdatedMsg is sent when templates are modified.
type TaskTemplatesUpdatedMsg struct{}

// ShowTaskTemplatesMsg triggers showing the task templates view.
type ShowTaskTemplatesMsg struct{}

// ShowTaskTemplateCreatorMsg triggers showing the template creator.
type ShowTaskTemplateCreatorMsg struct{}

// ShowTaskTemplateEditorMsg triggers showing the template editor.
type ShowTaskTemplateEditorMsg struct {
	Template template.TaskTemplate
}

// TaskTemplateCreatedMsg signals a new template was created.
type TaskTemplateCreatedMsg struct {
	Template template.TaskTemplate
}

// TaskTemplateEditedMsg signals an existing template was edited.
type TaskTemplateEditedMsg struct {
	Template template.TaskTemplate
}

// TemplateDeletedMsg is sent when a template deletion completes.
type TemplateDeletedMsg struct {
	DeletedID string
	Error     error
}

func (m *TaskTemplatesModel) StatusBarLabel() string { return "Task Templates" }
