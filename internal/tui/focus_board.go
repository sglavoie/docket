package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

var focusBoardTabNames = []string{"Today", "All", "Active/Due", "Next", "By Source", "By Priority", "By Score", "Closed"}

const (
	boardTabToday      = 0
	boardTabActiveDue  = 2
	boardTabNext       = 3
	boardTabBySource   = 4
	boardTabByPriority = 5
	boardTabByScore    = 6
	boardTabClosed     = 7
)

// boardRow is a unified display item for an active/due task.
type boardRow struct {
	Task     *models.Task
	Project  *models.Project
	Priority int
	Source   string
}

func (r boardRow) isTask() bool {
	return r.Task != nil
}

// FocusBoardModel displays active/due tasks in a unified cross-project view.
type FocusBoardModel struct {
	BaseSearchableView
	InteractiveViewState

	config         *config.Config
	configManager  *config.Manager
	projectService *ProjectService

	// Queue / daily-plan indicator support
	queueIDs     map[string]bool
	dailyPlanIDs map[string]bool
	dailyPlan    *[]FocusTask // shared with AppModel for reordering

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
func NewFocusBoardModel(cfg *config.Config, manager *config.Manager, dailyPlan *[]FocusTask, queueIDs, dailyPlanIDs, carriedOverIDs map[string]bool) *FocusBoardModel {
	ti := textinput.New()
	ti.Placeholder = "Task title (%project !priority #tag -s source -u url)"
	ti.CharLimit = 200
	ti.SetWidth(60)
	applyThemedTextInput(&ti)

	ei := textinput.New()
	ei.Placeholder = "New title"
	ei.CharLimit = 200
	ei.SetWidth(60)
	applyThemedTextInput(&ei)

	m := &FocusBoardModel{
		config:         cfg,
		configManager:  manager,
		projectService: NewProjectService(),
		queueIDs:       queueIDs,
		dailyPlan:      dailyPlan,
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
	if handled, cmd := handleCommonUpdateMsg(msg, func(w, h int) {
		m.SetSize(w, h)
		inputWidth := max(w-8, 30)
		m.addInput.SetWidth(inputWidth)
		m.editInput.SetWidth(inputWidth)
	}, m.loadItems, m.clampCursor, m.setStatus); handled {
		return m, cmd
	}

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
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

// escSteps defines the Esc dismissal cascade: search → filters.
func (m *FocusBoardModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.SearchQuery() != "" }, Clear: func() {
			m.ClearSearch()
			m.applyFilter()
			m.clampCursor()
			m.setStatus("Search cleared")
		}},
		FilterClearEscStep(m.HasActiveFilters, func() {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			m.setStatus("Filters cleared")
		}),
	}
}

// ShouldCaptureEsc returns true when add mode, edit mode, search, prefix, confirm, or filters are active.
func (m *FocusBoardModel) ShouldCaptureEsc() bool {
	return m.addMode || m.editMode || m.SearchShouldCaptureEsc() ||
		m.DefaultShouldCaptureEsc() || AnyEscStepActive(m.escSteps())
}

// HasTextInput returns true when a text input is focused.
func (m *FocusBoardModel) HasTextInput() bool {
	return m.addMode || m.editMode || m.searchActive
}

// OnExit resets interactive state when the view is popped from the stack.
func (m *FocusBoardModel) OnExit() {
	m.addMode = false
	m.editMode = false
	m.addInput.Blur()
	m.editInput.Blur()
	m.ac.Clear()
	m.pendingPrefix = ""
	m.confirm.Cancel()
	m.ClearSearch()
	m.ClearAllFilters()
}

func (m *FocusBoardModel) StatusBarLabel() string { return "Focus Board" }

// IsFullHeight returns true because the focus board renders its own footer.
func (m *FocusBoardModel) IsFullHeight() bool { return true }
