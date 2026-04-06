package tui

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

var blockersTabNames = []string{"Active", "Resolved", "All"}

const (
	blockersTabActive   = 0
	blockersTabResolved = 1
	blockersTabAll      = 2
)

// blockerItem is a unified view of a blocker with its project context.
type blockerItem struct {
	Blocker *models.Blocker
	Project *models.Project
}

// blockerViewState holds all the shared UI state for blocker views.
type blockerViewState struct {
	cursor int
	tabs   TabState

	// Add mode
	addMode   bool
	textInput textarea.Model

	// Edit mode
	editMode      bool
	editIndex     int
	editTextInput textarea.Model

	// Confirmation state
	confirm ConfirmDialog

	// Expanded view mode
	expandedView bool

	// Editor config
	editor string

	// Dimensions
	width  int
	height int

	// Error display
	err error
}

func newBlockerViewState(editor string) blockerViewState {
	ti := textarea.New()
	ti.Placeholder = "Enter blocker title..."
	ti.CharLimit = 500
	ti.SetHeight(2)
	ti.ShowLineNumbers = false

	editTi := textarea.New()
	editTi.Placeholder = "Edit blocker..."
	editTi.CharLimit = 2000
	editTi.SetHeight(6)
	editTi.ShowLineNumbers = false

	return blockerViewState{
		tabs:          TabState{Names: blockersTabNames},
		editor:        editor,
		textInput:     ti,
		editTextInput: editTi,
	}
}

func (s *blockerViewState) GetConfirmDialog() *ConfirmDialog { return &s.confirm }

func (s *blockerViewState) setSize(width, height int) {
	s.width = width
	s.height = height
	s.textInput.SetWidth(max(width-4, 1))
	s.editTextInput.SetWidth(max(width-4, 1))
}

func (s *blockerViewState) hasTextInput() bool {
	return s.addMode || s.editMode
}

func (s *blockerViewState) shouldCaptureEsc() bool {
	return s.expandedView || s.confirm.IsActive() || s.addMode || s.editMode
}

// blockerDataSource abstracts the differences between project-local and global blocker views.
type blockerDataSource interface {
	// getVisibleItems returns the visible blocker items for the current tab.
	getVisibleItems(tab int) []blockerItem
	// activeCount returns the number of active blockers.
	activeCount() int
	// resolvedCount returns the number of resolved blockers.
	resolvedCount() int
	// findItemIndex finds the index of a blocker in the underlying data.
	findItemIndex(blocker *models.Blocker) int
	// saveBlockerData saves a new blocker to the appropriate project.
	saveBlockerData(blocker *models.Blocker, project *models.Project) error
	// deleteBlockerData removes a blocker and saves.
	deleteBlockerData(idx int) (*models.Project, error)
	// updateBlockerData saves changes after edit/resolve.
	updateBlockerData(idx int) (*models.Project, error)
	// getAddProject returns the project to use when adding a new blocker.
	getAddProject() *models.Project
	// getItemBlocker returns the blocker at the given data index.
	getItemBlocker(idx int) *models.Blocker
	// addBlocker appends a blocker to the data source.
	addBlocker(blocker *models.Blocker, project *models.Project)
	// titleText returns the view title.
	titleText() string
	// showProjectName returns true if project names should be shown.
	showProjectName() bool
	// closeMsg returns the message to send when closing.
	closeMsg() tea.Msg
}
