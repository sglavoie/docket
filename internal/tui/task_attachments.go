package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TaskAttachmentsModel displays and manages attachments for a task.
type TaskAttachmentsModel struct {
	BaseView
	InteractiveViewState
	task    *models.Task
	project *models.Project
	config  *config.Config

	// Add mode
	addMode   bool
	textInput textinput.Model
}

// NewTaskAttachmentsModel creates a new task attachments viewer.
func NewTaskAttachmentsModel(task *models.Task, project *models.Project, cfg *config.Config) *TaskAttachmentsModel {
	ti := textinput.New()
	ti.Placeholder = "Enter file path, URL, or code ref (file:line)..."
	ti.CharLimit = 500
	ti.SetWidth(60)
	applyThemedTextInput(&ti)

	return &TaskAttachmentsModel{
		task:      task,
		project:   project,
		config:    cfg,
		textInput: ti,
	}
}

// resolveTask ensures m.task points to the task currently in m.project.Tasks.
func (m *TaskAttachmentsModel) resolveTask() {
	if t := m.project.GetTaskByID(m.task.ID); t != nil {
		m.task = t
	}
}

func (m *TaskAttachmentsModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *TaskAttachmentsModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	m.textInput.SetWidth(width - 4)
}

// HasTextInput returns true when add mode is active.
func (m *TaskAttachmentsModel) HasTextInput() bool {
	return m.addMode
}

// ShouldCaptureEsc returns true when in add, delete, or unlink confirmation modes, or pending prefix.
func (m *TaskAttachmentsModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.addMode
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *TaskAttachmentsModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints: PrefixHints{
			"g": NavigationPrefixHints,
		},
	}
}

// GetHelpItems returns help items for the attachments view.
func (m *TaskAttachmentsModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewTaskAttachments)
}

func (m *TaskAttachmentsModel) StatusBarLabel() string { return "Attachments" }
