package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

var focusBoardTabNames = []string{"All", "Actions", "Active/Due", "Next", "By Source", "By Priority", "By Score", "Closed"}

const (
	boardTabAll        = 0
	boardTabActions    = 1
	boardTabActiveDue  = 2
	boardTabNext       = 3
	boardTabBySource   = 4
	boardTabByPriority = 5
	boardTabByScore    = 6
	boardTabClosed     = 7
)

// boardRow is a unified display item: an Action task or an aggregated active/due task.
type boardRow struct {
	Task     *models.Task
	Project  *models.Project
	Priority int
	Source   string
}

func (r boardRow) isAction() bool {
	return r.Task != nil && r.Task.Type == models.TaskTypeAction
}

func (r boardRow) isTask() bool {
	return r.Task != nil && r.Task.Type != models.TaskTypeAction
}

// FocusBoardModel displays Action tasks + aggregated active/due tasks in a unified view.
type FocusBoardModel struct {
	BaseSearchableView
	InteractiveViewState

	config         *config.Config
	configManager  *config.Manager
	projectService *ProjectService

	// Data
	rows       []boardRow
	nextRows   []boardRow
	closedRows []boardRow
	filtered   []boardRow

	// Tabs
	tabs TabState

	// Sort
	sort SortState

	// Filter
	FilterMixin

	// Quick-add input
	addMode  bool
	addInput textinput.Model

	// Autocomplete state for add mode
	ac             AutocompleteState
	acProjectCache []*models.Project
	acTagCache     []string

	// Quick-edit mode
	editMode  bool
	editInput textinput.Model
	editIdx   int

	previewMode bool
}

// NewFocusBoardModel creates a new focus board model.
func NewFocusBoardModel(cfg *config.Config, manager *config.Manager) *FocusBoardModel {
	ti := textinput.New()
	ti.Placeholder = "Action title (%project !priority #tag -s source -u url)"
	ti.CharLimit = 200
	ti.Width = 60

	ei := textinput.New()
	ei.Placeholder = "New title"
	ei.CharLimit = 200
	ei.Width = 60

	m := &FocusBoardModel{
		config:         cfg,
		configManager:  manager,
		projectService: NewProjectService(),
		addInput:       ti,
		editInput:      ei,
		FilterMixin:    FilterMixin{filter: newFilterState()},
		sort:           NewSortState(),
		tabs:           TabState{Names: focusBoardTabNames},
	}
	m.initFilterMixin(&m.pendingPrefix, cfg)
	m.InitSearchInput("Search...", 100, 40)
	m.loadItems()
	return m
}

func (m *FocusBoardModel) Init() tea.Cmd {
	return nil
}

func (m *FocusBoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		inputWidth := max(msg.Width-8, 30)
		m.addInput.Width = inputWidth
		m.editInput.Width = inputWidth
		return m, nil

	case FocusBoardRefreshMsg:
		m.loadItems()
		m.clampCursor()
		return m, nil

	case tea.KeyMsg:
		if m.addMode {
			return m.handleAddInput(msg)
		}
		if m.editMode {
			return m.handleEditInput(msg)
		}
		if model, cmd, handled := m.HandleSearchMode(msg, m, func() {
			m.applyFilter()
			m.clampCursor()
		}); handled {
			return model, cmd
		}
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

// getRowAtCursor returns the row at the current cursor position.
func (m *FocusBoardModel) getRowAtCursor() *boardRow {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.filtered[m.cursor]
}

// clampCursor ensures cursor is within valid range.
func (m *FocusBoardModel) clampCursor() {
	m.ClampCursorTo(len(m.filtered))
}

// ShouldCaptureEsc returns true when add mode, search mode, or confirm dialog is active.
func (m *FocusBoardModel) ShouldCaptureEsc() bool {
	return m.addMode || m.editMode || m.searchActive || m.confirm.IsActive() || m.pendingPrefix != "" || m.HasActiveFilters()
}

// HasTextInput returns true when a text input is focused.
func (m *FocusBoardModel) HasTextInput() bool {
	return m.addMode || m.editMode || m.searchActive
}
