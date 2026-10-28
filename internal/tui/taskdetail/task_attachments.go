package taskdetail

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/help"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// TaskAttachmentsModel displays and manages attachments for a task.
type TaskAttachmentsModel struct {
	listsurface.BaseView
	listsurface.InteractiveViewState
	Task    *models.Task
	project *models.Project
	// Coordinator routes attachment mutations through the application layer
	// (one atomic ChangeSet each) instead of a direct project save. See
	// feature_coordinator_subview.go.
	Coordinator coordinator.AttachmentCoordinator

	// Add mode
	AddMode   bool
	TextInput textinput.Model

	// pointerEntries is which attachment each row of the last list frame drew.
	// See HandlePointer.
	pointerEntries listsurface.ListPointerFrame

	// pendingTopChord holds the first g of gg. The shell's chord state serves the
	// Outliner only, so the panel keeps its own one-key prefix.
	pendingTopChord bool

	// pendingRemoval is the attachment a delete or unlink prompt asked about,
	// frozen when the prompt opens. The answer removes that attachment or
	// nothing: a reload while the prompt is open may move the cursor or shift
	// the list, and neither may retarget a confirmed removal.
	pendingRemoval *pendingAttachmentRemoval

	Dispatch operation.KeyDispatch
}

// pendingAttachmentRemoval is the position and stored value a removal prompt
// named.
type pendingAttachmentRemoval struct {
	index      int
	attachment models.Attachment
}

// SetOperationDispatch hands the panel the shell's key dispatch seam. Without it
// the browse grammar resolves nothing, so a host that mounts this model must
// supply it. The registry stays out because help still derives from buildKeyMap.
func (m *TaskAttachmentsModel) SetOperationDispatch(dispatch operation.KeyDispatch) {
	m.Dispatch = dispatch
}

// NewTaskAttachmentsModel creates a new task attachments viewer.
func NewTaskAttachmentsModel(task *models.Task, project *models.Project, runtime *config.RuntimeState, coordinator coordinator.AttachmentCoordinator) *TaskAttachmentsModel {
	ti := textinput.New()
	ti.Placeholder = "Enter file path, URL, or code ref (file:line)..."
	ti.CharLimit = 500
	ti.SetWidth(60)
	theme.ApplyThemedTextInput(&ti)

	return &TaskAttachmentsModel{
		Task:        task,
		project:     project,
		Coordinator: coordinator,
		TextInput:   ti,
	}
}

// RefreshAfterReload re-resolves the task pointer and clamps the cursor after
// the shell reloaded the project from a committed transaction.
func (m *TaskAttachmentsModel) RefreshAfterReload() bool {
	if !m.resolveTask() {
		return false
	}
	if m.Cursor >= len(m.Task.Attachments) {
		m.Cursor = len(m.Task.Attachments) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	return true
}

// resolveTask ensures m.task points to the task currently in m.project.Tasks.
func (m *TaskAttachmentsModel) resolveTask() bool {
	if m.project == nil || m.Task == nil {
		return false
	}
	task := m.project.GetTaskByID(m.Task.ID)
	if task == nil {
		return false
	}
	m.Task = task
	return true
}

func (m *TaskAttachmentsModel) GetTaskRef() (*models.Task, *models.Project) {
	return m.Task, m.project
}

func (m *TaskAttachmentsModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *TaskAttachmentsModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	m.TextInput.SetWidth(width - 4)
}

// HasTextInput returns true when add mode is active.
func (m *TaskAttachmentsModel) HasTextInput() bool {
	return m.AddMode
}

// ShouldCaptureEsc returns true when in add, delete, or unlink confirmation modes.
func (m *TaskAttachmentsModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.AddMode
}

// GetHelpItems returns help items for the attachments view.
func (m *TaskAttachmentsModel) GetHelpItems() []operation.HelpItem {
	return help.HelpItemsFromKeyMap(m.BuildKeyMap())
}

func (m *TaskAttachmentsModel) StatusBarLabel() string { return "Attachments" }

// QuitBlockReason names content a desktop quit would abandon.
func (m *TaskAttachmentsModel) QuitBlockReason() string {
	if m.AddMode && m.TextInput.Value() != "" {
		return "Your attachment entry has not been saved."
	}
	return ""
}

func (m *TaskAttachmentsModel) DesktopTextBuffer() *chrome.DesktopTextBuffer {
	if m.AddMode {
		return chrome.DesktopInput(&m.TextInput, nil)
	}
	return nil
}
