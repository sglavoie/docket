package tui

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TaskChecklistModel displays and manages checklist items for a task.
type TaskChecklistModel struct {
	BaseView
	InteractiveViewState
	task    *models.Task
	project *models.Project

	// Add mode
	addMode   bool
	textInput textarea.Model

	// Edit mode
	editMode      bool
	editIndex     int
	editTextInput textarea.Model

	// Editor config
	editor string
}

// NewTaskChecklistModel creates a new task checklist viewer.
func NewTaskChecklistModel(task *models.Task, project *models.Project) *TaskChecklistModel {
	ti := textarea.New()
	ti.Placeholder = "Enter checklist item..."
	ti.CharLimit = 500
	ti.SetHeight(2)
	ti.ShowLineNumbers = false
	applyThemedTextarea(&ti)

	editTi := textarea.New()
	editTi.Placeholder = "Edit checklist item..."
	editTi.CharLimit = 500
	editTi.SetHeight(2)
	editTi.ShowLineNumbers = false
	applyThemedTextarea(&editTi)

	return &TaskChecklistModel{
		task:          task,
		project:       project,
		textInput:     ti,
		editTextInput: editTi,
	}
}

// NewTaskChecklistModelWithEditor creates a task checklist viewer with editor config.
func NewTaskChecklistModelWithEditor(task *models.Task, project *models.Project, editor string) *TaskChecklistModel {
	m := NewTaskChecklistModel(task, project)
	m.editor = editor
	return m
}

// resolveTask ensures m.task points to the task currently in m.project.Tasks.
func (m *TaskChecklistModel) resolveTask() {
	if t := m.project.GetTaskByID(m.task.ID); t != nil {
		m.task = t
	}
}

func (m *TaskChecklistModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *TaskChecklistModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	cardWidth := min(width-4, 80)
	contentWidth := cardWidth - 6 // card border + padding
	m.textInput.SetWidth(contentWidth)
	m.editTextInput.SetWidth(contentWidth)
}

// HasTextInput returns true when add/edit mode is active.
func (m *TaskChecklistModel) HasTextInput() bool {
	return m.addMode || m.editMode
}

// ShouldCaptureEsc returns true when in add/edit, delete confirmation, or pending prefix modes.
func (m *TaskChecklistModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.addMode || m.editMode
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *TaskChecklistModel) GetPrefixPopupInfo() *PrefixPopupInfo {
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

func (m *TaskChecklistModel) StatusBarLabel() string { return "Checklist" }
