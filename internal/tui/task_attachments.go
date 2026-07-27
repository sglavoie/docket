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
	runtime *config.RuntimeState
	// coordinator routes attachment mutations through the application layer
	// (one atomic ChangeSet each) instead of a direct project save. See
	// feature_coordinator_secondary.go.
	coordinator AttachmentCoordinator

	// Add mode
	addMode   bool
	textInput textinput.Model
}

// NewTaskAttachmentsModel creates a new task attachments viewer.
func NewTaskAttachmentsModel(task *models.Task, project *models.Project, runtime *config.RuntimeState, coordinator AttachmentCoordinator) *TaskAttachmentsModel {
	ti := textinput.New()
	ti.Placeholder = "Enter file path, URL, or code ref (file:line)..."
	ti.CharLimit = 500
	ti.SetWidth(60)
	applyThemedTextInput(&ti)

	return &TaskAttachmentsModel{
		task:        task,
		project:     project,
		runtime:     runtime,
		coordinator: coordinator,
		textInput:   ti,
	}
}

// refreshAfterReload re-resolves the task pointer and clamps the cursor after
// the shell reloaded the project from a committed transaction.
func (m *TaskAttachmentsModel) refreshAfterReload() {
	m.resolveTask()
	if m.cursor >= len(m.task.Attachments) {
		m.cursor = len(m.task.Attachments) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
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

// ShouldCaptureEsc returns true when in add, delete, or unlink confirmation modes.
func (m *TaskAttachmentsModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.addMode
}

// GetHelpItems returns help items for the attachments view.
func (m *TaskAttachmentsModel) GetHelpItems() []HelpItem {
	return helpItemsFromKeyMap(m.buildKeyMap())
}

func (m *TaskAttachmentsModel) StatusBarLabel() string { return "Attachments" }
