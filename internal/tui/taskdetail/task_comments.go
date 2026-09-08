package taskdetail

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// CommentCharLimit bounds a comment however it is typed. The Comments panel
// owns the number because it owns comments; the Outliner's quick Add Comment
// popup reads it from here rather than restating it, so the two routes cannot
// promise different lengths.
const CommentCharLimit = 2000

// TaskCommentsModel displays and manages comments for a task.
type TaskCommentsModel struct {
	listsurface.BaseView
	listsurface.InteractiveViewState
	Task        *models.Task
	project     *models.Project
	coordinator coordinator.SubviewCoordinator

	// Add mode
	AddMode   bool
	TextInput textarea.Model

	// Edit mode
	EditMode      bool
	editIndex     int
	EditTextInput textarea.Model

	// Expanded view mode (show full comment)
	ExpandedView bool

	// Editor config
	editor string

	// pendingTopChord holds the first g of gg. The shell's chord state serves the
	// Outliner only, so the panel keeps its own one-key prefix.
	pendingTopChord bool

	Dispatch operation.KeyDispatch
}

// SetOperationDispatch hands the panel the shell's key dispatch seam. Without it
// the browse grammar resolves nothing, so a host that mounts this model must
// supply it. The registry stays out because help still derives from buildKeyMap.
func (m *TaskCommentsModel) SetOperationDispatch(dispatch operation.KeyDispatch) {
	m.Dispatch = dispatch
}

// NewTaskCommentsModel creates a new task comments viewer.
func NewTaskCommentsModel(task *models.Task, project *models.Project) *TaskCommentsModel {
	ti := textarea.New()
	ti.Placeholder = "Enter comment text..."
	ti.CharLimit = CommentCharLimit
	ti.SetHeight(4)
	ti.ShowLineNumbers = false
	theme.ApplyThemedTextarea(&ti)

	editTi := textarea.New()
	editTi.Placeholder = "Edit comment text..."
	editTi.CharLimit = CommentCharLimit
	editTi.SetHeight(6)
	editTi.ShowLineNumbers = false
	theme.ApplyThemedTextarea(&editTi)

	return &TaskCommentsModel{
		Task:          task,
		project:       project,
		TextInput:     ti,
		EditTextInput: editTi,
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

// RefreshAfterReload re-resolves the task pointer and clamps the selected
// comment after a coordinator commit replaces the project task objects.
func (m *TaskCommentsModel) RefreshAfterReload() bool {
	if m.project == nil || m.Task == nil {
		return false
	}
	task := m.project.GetTaskByID(m.Task.ID)
	if task == nil {
		return false
	}
	m.Task = task
	if m.Cursor >= len(m.Task.Comments) {
		m.Cursor = max(len(m.Task.Comments)-1, 0)
	}
	return true
}

// refreshTaskReference re-resolves the task pointer before a mutation reads its
// comment slice, so an edit or a delete measures the indices the workspace holds
// rather than the ones the panel captured when it mounted.
func (m *TaskCommentsModel) refreshTaskReference() {
	if current := m.project.GetTaskByID(m.Task.ID); current != nil {
		m.Task = current
	}
}

func (m *TaskCommentsModel) GetTaskRef() (*models.Task, *models.Project) {
	return m.Task, m.project
}

func (m *TaskCommentsModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *TaskCommentsModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	cardWidth := min(width-4, 80)
	contentWidth := cardWidth - 6 // card border + padding
	m.TextInput.SetWidth(contentWidth)
	m.EditTextInput.SetWidth(contentWidth)
}

// HasTextInput returns true when add/edit mode is active.
func (m *TaskCommentsModel) HasTextInput() bool {
	return m.AddMode || m.EditMode
}

// OnExit resets all interactive input state so stale textarea content or edit
// modes don't persist in the registry after the view is popped.
func (m *TaskCommentsModel) OnExit() {
	m.AddMode = false
	m.EditMode = false
	m.ExpandedView = false
	m.editIndex = 0
	m.TextInput.Reset()
	m.EditTextInput.Reset()
}

// ShouldCaptureEsc returns true when in expanded, add/edit, or delete confirmation modes.
func (m *TaskCommentsModel) ShouldCaptureEsc() bool {
	return m.ExpandedView || m.DefaultShouldCaptureEsc() || m.AddMode || m.EditMode
}

// GetHelpItems returns help items for the comments panel. The keymap is the one
// source: the core help descriptor the strategy binds is inapplicable off the
// Outliner, so the registry half of a merge would contribute nothing and only the
// keymap can name this panel's own grammar.
func (m *TaskCommentsModel) GetHelpItems() []operation.HelpItem {
	return help.HelpItemsFromKeyMap(m.BuildKeyMap())
}

func (m *TaskCommentsModel) StatusBarLabel() string { return "Comments" }
