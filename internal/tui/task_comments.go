package tui

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TaskCommentsModel displays and manages comments for a task.
type TaskCommentsModel struct {
	BaseView
	InteractiveViewState
	task        *models.Task
	project     *models.Project
	coordinator SubviewCoordinator

	// Add mode
	addMode   bool
	textInput textarea.Model

	// Edit mode
	editMode      bool
	editIndex     int
	editTextInput textarea.Model

	// Expanded view mode (show full comment)
	expandedView bool

	// Editor config
	editor string
}

// NewTaskCommentsModel creates a new task comments viewer.
func NewTaskCommentsModel(task *models.Task, project *models.Project) *TaskCommentsModel {
	ti := textarea.New()
	ti.Placeholder = "Enter comment text..."
	ti.CharLimit = 2000
	ti.SetHeight(4)
	ti.ShowLineNumbers = false
	applyThemedTextarea(&ti)

	editTi := textarea.New()
	editTi.Placeholder = "Edit comment text..."
	editTi.CharLimit = 2000
	editTi.SetHeight(6)
	editTi.ShowLineNumbers = false
	applyThemedTextarea(&editTi)

	return &TaskCommentsModel{
		task:          task,
		project:       project,
		textInput:     ti,
		editTextInput: editTi,
	}
}

// NewTaskCommentsModelWithEditor creates a task comments viewer with editor
// config and the coordinator its mutations commit through.
func NewTaskCommentsModelWithEditor(task *models.Task, project *models.Project, editor string, coordinator SubviewCoordinator) *TaskCommentsModel {
	m := NewTaskCommentsModel(task, project)
	m.editor = editor
	m.coordinator = coordinator
	return m
}

func (m *TaskCommentsModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *TaskCommentsModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	cardWidth := min(width-4, 80)
	contentWidth := cardWidth - 6 // card border + padding
	m.textInput.SetWidth(contentWidth)
	m.editTextInput.SetWidth(contentWidth)
}

// HasTextInput returns true when add/edit mode is active.
func (m *TaskCommentsModel) HasTextInput() bool {
	return m.addMode || m.editMode
}

// OnExit resets all interactive input state so stale textarea content or edit
// modes don't persist in the registry after the view is popped.
func (m *TaskCommentsModel) OnExit() {
	m.addMode = false
	m.editMode = false
	m.expandedView = false
	m.editIndex = 0
	m.textInput.Reset()
	m.editTextInput.Reset()
}

// ShouldCaptureEsc returns true when in expanded, add/edit, or delete confirmation modes.
func (m *TaskCommentsModel) ShouldCaptureEsc() bool {
	return m.expandedView || m.DefaultShouldCaptureEsc() || m.addMode || m.editMode
}

func (m *TaskCommentsModel) StatusBarLabel() string { return "Comments" }
