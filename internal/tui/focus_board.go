package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

var focusBoardTabNames = []string{"Today", "All", "Actions", "Active/Due", "Next", "By Source", "By Priority", "By Score", "Closed"}

const (
	boardTabToday      = 0
	boardTabActions    = 2
	boardTabActiveDue  = 3
	boardTabNext       = 4
	boardTabBySource   = 5
	boardTabByPriority = 6
	boardTabByScore    = 7
	boardTabClosed     = 8
)

// boardRow is a unified display item: an Action task or an aggregated active/due task.
type boardRow struct {
	Task     *models.Task
	Project  *models.Project
	Priority int
	Source   string
}

func (r boardRow) isAction() bool {
	return false
}

func (r boardRow) isTask() bool {
	return r.Task != nil
}

// FocusBoardModel displays Action tasks + aggregated active/due tasks in a unified view.
type FocusBoardModel struct {
	BaseSearchableView
	InteractiveViewState

	config         *config.Config
	configManager  *config.Manager
	projectService *ProjectService

	// Queue / daily-plan indicator support
	queueIDs     map[string]bool
	dailyPlanIDs map[string]bool

	// Data
	rows           []boardRow
	nextRows       []boardRow
	closedRows     []boardRow
	todayRows      []boardRow
	filtered       []boardRow
	carriedOverIDs map[string]bool

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
func NewFocusBoardModel(cfg *config.Config, manager *config.Manager, queueIDs, dailyPlanIDs, carriedOverIDs map[string]bool) *FocusBoardModel {
	ti := textinput.New()
	ti.Placeholder = "Action title (%project !priority #tag -s source -u url)"
	ti.CharLimit = 200
	ti.SetWidth(60)

	ei := textinput.New()
	ei.Placeholder = "New title"
	ei.CharLimit = 200
	ei.SetWidth(60)

	m := &FocusBoardModel{
		config:         cfg,
		configManager:  manager,
		projectService: NewProjectService(),
		queueIDs:       queueIDs,
		dailyPlanIDs:   dailyPlanIDs,
		carriedOverIDs: carriedOverIDs,
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
		m.addInput.SetWidth(inputWidth)
		m.editInput.SetWidth(inputWidth)
		return m, nil

	case FocusBoardRefreshMsg:
		m.loadItems()
		m.clampCursor()
		return m, nil

	case tea.KeyPressMsg:
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

func (m *FocusBoardModel) StatusBarLabel() string { return "Focus Board" }
