package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TaskAttachmentsModel displays and manages attachments for a task.
type TaskAttachmentsModel struct {
	listsurface.BaseView
	listsurface.InteractiveViewState
	task    *models.Task
	project *models.Project
	// coordinator routes attachment mutations through the application layer
	// (one atomic ChangeSet each) instead of a direct project save. See
	// feature_coordinator_subview.go.
	coordinator coordinator.AttachmentCoordinator

	// Add mode
	addMode   bool
	textInput textinput.Model

	// pendingTopChord holds the first g of gg. The shell's chord state serves the
	// Outliner only, so the panel keeps its own one-key prefix.
	pendingTopChord bool

	dispatch operationKeyDispatch
}

// SetOperationDispatch hands the panel the shell's key dispatch seam. Without it
// the browse grammar resolves nothing, so a host that mounts this model must
// supply it. The registry stays out because help still derives from buildKeyMap.
func (m *TaskAttachmentsModel) SetOperationDispatch(dispatch operationKeyDispatch) {
	m.dispatch = dispatch
}

// NewTaskAttachmentsModel creates a new task attachments viewer.
func NewTaskAttachmentsModel(task *models.Task, project *models.Project, runtime *config.RuntimeState, coordinator coordinator.AttachmentCoordinator) *TaskAttachmentsModel {
	ti := textinput.New()
	ti.Placeholder = "Enter file path, URL, or code ref (file:line)..."
	ti.CharLimit = 500
	ti.SetWidth(60)
	theme.ApplyThemedTextInput(&ti)

	return &TaskAttachmentsModel{
		task:        task,
		project:     project,
		coordinator: coordinator,
		textInput:   ti,
	}
}

// refreshAfterReload re-resolves the task pointer and clamps the cursor after
// the shell reloaded the project from a committed transaction.
func (m *TaskAttachmentsModel) refreshAfterReload() bool {
	if !m.resolveTask() {
		return false
	}
	if m.Cursor >= len(m.task.Attachments) {
		m.Cursor = len(m.task.Attachments) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	return true
}

// resolveTask ensures m.task points to the task currently in m.project.Tasks.
func (m *TaskAttachmentsModel) resolveTask() bool {
	if m.project == nil || m.task == nil {
		return false
	}
	task := m.project.GetTaskByID(m.task.ID)
	if task == nil {
		return false
	}
	m.task = task
	return true
}

func (m *TaskAttachmentsModel) getTaskRef() (*models.Task, *models.Project) {
	return m.task, m.project
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
func (m *TaskAttachmentsModel) GetHelpItems() []operation.HelpItem {
	return helpItemsFromKeyMap(m.buildKeyMap())
}

func (m *TaskAttachmentsModel) StatusBarLabel() string { return "Attachments" }
