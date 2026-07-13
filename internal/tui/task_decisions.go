package tui

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TaskDecisionsModel displays and manages design/architecture decisions for a task.
type TaskDecisionsModel struct {
	BaseView
	InteractiveViewState
	task        *models.Task
	project     *models.Project
	coordinator SubviewCoordinator

	// Add mode
	addMode        bool
	textInput      textarea.Model
	rationaleInput textarea.Model
	categoryIndex  int
	activeField    int // 0=text, 1=rationale

	// Edit mode
	editMode        bool
	editIndex       int
	editTextInput   textarea.Model
	editRationale   textarea.Model
	editCategoryIdx int
	editActiveField int // 0=text, 1=rationale

	// Expanded view mode (show full decision)
	expandedView bool

	// Editor config
	editor string
}

// NewTaskDecisionsModel creates a new task decisions viewer.
func NewTaskDecisionsModel(task *models.Task, project *models.Project) *TaskDecisionsModel {
	ti := textarea.New()
	ti.Placeholder = "Enter decision text..."
	ti.CharLimit = 2000
	ti.SetHeight(4)
	ti.ShowLineNumbers = false
	applyThemedTextarea(&ti)

	ri := textarea.New()
	ri.Placeholder = "Enter rationale (optional)..."
	ri.CharLimit = 2000
	ri.SetHeight(3)
	ri.ShowLineNumbers = false
	applyThemedTextarea(&ri)

	editTi := textarea.New()
	editTi.Placeholder = "Edit decision text..."
	editTi.CharLimit = 2000
	editTi.SetHeight(4)
	editTi.ShowLineNumbers = false
	applyThemedTextarea(&editTi)

	editRi := textarea.New()
	editRi.Placeholder = "Edit rationale (optional)..."
	editRi.CharLimit = 2000
	editRi.SetHeight(3)
	editRi.ShowLineNumbers = false
	applyThemedTextarea(&editRi)

	return &TaskDecisionsModel{
		task:           task,
		project:        project,
		textInput:      ti,
		rationaleInput: ri,
		editTextInput:  editTi,
		editRationale:  editRi,
	}
}

// NewTaskDecisionsModelWithEditor creates a task decisions viewer with editor
// config and the coordinator its mutations commit through.
func NewTaskDecisionsModelWithEditor(task *models.Task, project *models.Project, editor string, coordinator SubviewCoordinator) *TaskDecisionsModel {
	m := NewTaskDecisionsModel(task, project)
	m.editor = editor
	m.coordinator = coordinator
	return m
}

func (m *TaskDecisionsModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *TaskDecisionsModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	cardWidth := min(width-4, 80)
	contentWidth := cardWidth - 6
	m.textInput.SetWidth(contentWidth)
	m.rationaleInput.SetWidth(contentWidth)
	m.editTextInput.SetWidth(contentWidth)
	m.editRationale.SetWidth(contentWidth)
}

// HasTextInput returns true when add/edit mode is active.
func (m *TaskDecisionsModel) HasTextInput() bool {
	return m.addMode || m.editMode
}

// OnExit resets all interactive input state.
func (m *TaskDecisionsModel) OnExit() {
	m.addMode = false
	m.editMode = false
	m.expandedView = false
	m.editIndex = 0
	m.activeField = 0
	m.editActiveField = 0
	m.categoryIndex = 0
	m.editCategoryIdx = 0
	m.textInput.Reset()
	m.rationaleInput.Reset()
	m.editTextInput.Reset()
	m.editRationale.Reset()
}

// ShouldCaptureEsc returns true when in expanded, add/edit, delete confirmation, or pending prefix modes.
func (m *TaskDecisionsModel) ShouldCaptureEsc() bool {
	return m.expandedView || m.DefaultShouldCaptureEsc() || m.addMode || m.editMode
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *TaskDecisionsModel) GetPrefixPopupInfo() *PrefixPopupInfo {
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

func (m *TaskDecisionsModel) StatusBarLabel() string { return "Decisions" }

// allCategories returns the slice of valid decision categories.
func allDecisionCategories() []models.DecisionCategory {
	return models.ValidDecisionCategories()
}
