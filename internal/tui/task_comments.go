package tui

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TaskCommentsModel displays and manages comments for a task.
type TaskCommentsModel struct {
	listsurface.BaseView
	listsurface.InteractiveViewState
	task        *models.Task
	project     *models.Project
	coordinator coordinator.SubviewCoordinator

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

	// pendingTopChord holds the first g of gg. The shell's chord state serves the
	// Outliner only, so the panel keeps its own one-key prefix.
	pendingTopChord bool

	dispatch operationKeyDispatch
}

// SetOperationDispatch hands the panel the shell's key dispatch seam. Without it
// the browse grammar resolves nothing, so a host that mounts this model must
// supply it. The registry stays out because help still derives from buildKeyMap.
func (m *TaskCommentsModel) SetOperationDispatch(dispatch operationKeyDispatch) {
	m.dispatch = dispatch
}

// NewTaskCommentsModel creates a new task comments viewer.
func NewTaskCommentsModel(task *models.Task, project *models.Project) *TaskCommentsModel {
	ti := textarea.New()
	ti.Placeholder = "Enter comment text..."
	ti.CharLimit = 2000
	ti.SetHeight(4)
	ti.ShowLineNumbers = false
	theme.ApplyThemedTextarea(&ti)

	editTi := textarea.New()
	editTi.Placeholder = "Edit comment text..."
	editTi.CharLimit = 2000
	editTi.SetHeight(6)
	editTi.ShowLineNumbers = false
	theme.ApplyThemedTextarea(&editTi)

	return &TaskCommentsModel{
		task:          task,
		project:       project,
		textInput:     ti,
		editTextInput: editTi,
	}
}

// NewTaskCommentsModelWithEditor creates a task comments viewer with editor
// config and the coordinator its mutations commit through.
func NewTaskCommentsModelWithEditor(task *models.Task, project *models.Project, editor string, coordinator coordinator.SubviewCoordinator) *TaskCommentsModel {
	m := NewTaskCommentsModel(task, project)
	m.editor = editor
	m.coordinator = coordinator
	return m
}

// refreshAfterReload re-resolves the task pointer and clamps the selected
// comment after a coordinator commit replaces the project task objects.
func (m *TaskCommentsModel) refreshAfterReload() bool {
	if m.project == nil || m.task == nil {
		return false
	}
	task := m.project.GetTaskByID(m.task.ID)
	if task == nil {
		return false
	}
	m.task = task
	if m.Cursor >= len(m.task.Comments) {
		m.Cursor = max(len(m.task.Comments)-1, 0)
	}
	return true
}

// refreshTaskReference re-resolves the task pointer before a mutation reads its
// comment slice, so an edit or a delete measures the indices the workspace holds
// rather than the ones the panel captured when it mounted.
func (m *TaskCommentsModel) refreshTaskReference() {
	if current := m.project.GetTaskByID(m.task.ID); current != nil {
		m.task = current
	}
}

func (m *TaskCommentsModel) getTaskRef() (*models.Task, *models.Project) {
	return m.task, m.project
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

// GetHelpItems returns help items for the comments panel. The keymap is the one
// source: the core help descriptor the strategy binds is inapplicable off the
// Outliner, so the registry half of a merge would contribute nothing and only the
// keymap can name this panel's own grammar.
func (m *TaskCommentsModel) GetHelpItems() []operation.HelpItem {
	return helpItemsFromKeyMap(m.buildKeyMap())
}

func (m *TaskCommentsModel) StatusBarLabel() string { return "Comments" }
