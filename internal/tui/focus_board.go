package tui

import (
	"fmt"
	"image/color"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

var focusBoardTabNames = []string{"Today", "Next", "Closed"}

const (
	boardTabToday  = 0
	boardTabNext   = 1
	boardTabClosed = 2
)

const (
	focusModeList   = 0
	focusModeTunnel = 1
	focusModeZen    = 2
)

// boardRow is a unified display item for an active/due task.
type boardRow struct {
	Task        *models.Task
	Project     *models.Project
	Priority    int
	Source      string
	TreePrefix  string // non-empty for auto-expanded children of planned items in Today tab
	GroupHeader string // non-empty for non-selectable group separator rows (Task will be nil)
	ParentID    string // immediate parent task ID for child rows in Today tab; empty for roots/non-tree rows
	Depth       int    // 0 for root tasks; n for n-th level descendant in Today tab
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
	rows            []boardRow
	allRows         []boardRow // all non-complete, non-snoozed tasks — used as tag pool for autocomplete and tag filter picker
	nextRows        []boardRow
	closedRows      []boardRow
	todayRows       []boardRow
	filtered        []boardRow
	carriedOverIDs  map[string]bool
	parentOverrides map[string]bool // per-parent fold override in Today tab: true=force expand, false=force collapse, absent=follow childDepthLimit

	// Tabs
	tabs TabState

	// Sort
	sort SortState

	// Filter
	FilterMixin

	// Multi-select
	MultiSelectState

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

	previewMode     bool
	focusMode       int // 0=list, 1=tunnel, 2=zen
	childDepthLimit int // -1 = unlimited (default), 0/1/2 = max child depth in Today tab
	initialTabSet   bool

	// Perspective lens. PerspectiveInactive (-1) means no lens is applied;
	// 0..9 selects a preset from PerspectivePresets() and AND-composes with
	// the active tab filter.
	activePerspective int

	// openLoops mirrors AppModel.openLoops, injected before render via
	// setOpenLoops. Used to render the Zeigarnik counter in the title row.
	openLoops OpenLoops
}

// setOpenLoops injects the latest cross-project open-loop counts into the
// view. Called by AppModel before each render so the title-row counter stays
// in sync with mutations across daily-driver views.
func (m *FocusBoardModel) setOpenLoops(o OpenLoops) { m.openLoops = o }

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

	initialFocusMode := focusModeList
	switch cfg.GetFocusBoardDefaultMode() {
	case "tunnel":
		initialFocusMode = focusModeTunnel
	case "zen":
		initialFocusMode = focusModeZen
	}

	m := &FocusBoardModel{
		config:            cfg,
		configManager:     manager,
		projectService:    NewProjectService(),
		queueIDs:          queueIDs,
		dailyPlan:         dailyPlan,
		dailyPlanIDs:      dailyPlanIDs,
		carriedOverIDs:    carriedOverIDs,
		parentOverrides:   make(map[string]bool),
		addInput:          ti,
		editInput:         ei,
		focusMode:         initialFocusMode,
		childDepthLimit:   -1,
		FilterMixin:       FilterMixin{filter: newFilterState()},
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		sort:              NewSortState(),
		tabs:              TabState{Names: focusBoardTabNames},
		activePerspective: PerspectiveInactive,
	}
	km := m.buildKeyMap()
	m.Dispatcher = keymap.NewDispatcher(km)
	m.Dispatcher.ModalGuards = []keymap.ModalGuard{
		{
			Active: func() bool { return m.confirm.IsActive() },
			Handle: func(key string) (tea.Model, tea.Cmd, bool) {
				if confirmed, handled := m.HandleConfirmKey(key); handled {
					if confirmed {
						model, cmd := m.deleteItem()
						return model, cmd, true
					}
					return m, nil, true
				}
				return nil, nil, false
			},
		},
		{
			Active: func() bool { return m.focusMode > focusModeList },
			Handle: func(key string) (tea.Model, tea.Cmd, bool) {
				switch key {
				case "tab", "shift+tab", "shift+left", "shift+right",
					"/", "a", "v", "A",
					"1", "2", "3", "4", "5", "0",
					"n", "N", "r", "J", "K":
					return m, nil, true // swallow non-zen keys
				default:
					return nil, nil, false // pass through to normal dispatch
				}
			},
		},
	}
	m.initFilterMixinWithDispatcher(m.Dispatcher, cfg)
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

	case MutationFlashClearMsg:
		m.mutationFlash = false
		return m, nil

	case CompletionFlashFrameMsg:
		m.advanceCompletionFlashFrame(msg.Frame)
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

// clampCursor ensures cursor is within valid range, then skips past group headers.
func (m *FocusBoardModel) clampCursor() {
	m.ClampCursorTo(len(m.filtered))
	m.skipGroupHeaderForward()
}

// skipGroupHeaderForward advances cursor past any group header it lands on.
func (m *FocusBoardModel) skipGroupHeaderForward() {
	for m.cursor < len(m.filtered) && m.filtered[m.cursor].GroupHeader != "" {
		m.cursor++
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
		for m.cursor >= 0 && m.filtered[m.cursor].GroupHeader != "" {
			m.cursor--
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
	}
}

// skipGroupHeaderBackward retreats cursor past any group header it lands on.
func (m *FocusBoardModel) skipGroupHeaderBackward() {
	for m.cursor >= 0 && m.cursor < len(m.filtered) && m.filtered[m.cursor].GroupHeader != "" {
		if m.cursor == 0 {
			m.skipGroupHeaderForward()
			return
		}
		m.cursor--
	}
}

// escSteps defines the Esc dismissal cascade: search → filters.
func (m *FocusBoardModel) escSteps() []EscStep {
	return []EscStep{
		MultiSelectEscStep(func() bool { return m.multiSelectMode }, m.toggleMultiSelectMode),
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

// IsZenMode returns true when zen mode is active (deepest focus level — hides status bar).
func (m *FocusBoardModel) IsZenMode() bool {
	return m.focusMode == focusModeZen
}

// ActiveMode implements ModeIndicator.
func (m *FocusBoardModel) ActiveMode() (string, color.Color) {
	switch m.focusMode {
	case focusModeZen:
		return "ZEN", ColorPurple
	case focusModeTunnel:
		return "TUNNEL", ColorOrange
	}
	if m.multiSelectMode {
		return fmt.Sprintf("MULTI-SELECT: %d", m.SelectedCount()), ColorWarning
	}
	if m.addMode {
		return "ADD", ColorSuccess
	}
	if m.editMode {
		return "EDIT", ColorWarning
	}
	if m.IsSearchMode() {
		return "SEARCH", AccentSearch
	}
	return "", nil
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
	if m.Dispatcher != nil {
		m.Dispatcher.ClearPrefix()
	}
	m.confirm.Cancel()
	m.ClearSearch()
	m.ClearAllFilters()
}

func (m *FocusBoardModel) StatusBarLabel() string { return "Today" }

// IsFullHeight returns true because the focus board renders its own footer.
func (m *FocusBoardModel) IsFullHeight() bool { return true }
