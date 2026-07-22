package tui

import (
	"image/color"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
)

// FocusTask is an alias for focus.FocusTask — the cross-project ordered task the
// Planner's daily plan / Next order index are built from.
type FocusTask = focus.FocusTask

// ScoredFocusTask is an alias for focus.ScoredFocusTask.
type ScoredFocusTask = focus.ScoredFocusTask

var focusBoardTabNames = []string{"Today", "Next", "Backlog"}

const (
	boardTabToday   = 0
	boardTabNext    = 1
	boardTabBacklog = 2
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

	configManager  *config.Manager
	runtime        *config.RuntimeState
	planningCommit func(*config.RuntimeState, func(*config.PlanningState)) error
	projectService *ProjectService
	capture        CaptureCoordinator
	taskItem       TaskItemCoordinator
	bulk           BulkCoordinator

	// Queue / daily-plan indicator support
	queueIDs     map[TaskIdentity]bool
	dailyPlanIDs map[TaskIdentity]bool
	dailyPlan    *[]FocusTask // shared with AppModel for reordering
	// focusQueue is the shared, ordered Next order index (StateNext ordering
	// only; may not contain every StateNext task). Used to sort the Next tab.
	focusQueue *[]FocusTask

	// Data
	rows           []boardRow
	allRows        []boardRow // all non-complete, non-snoozed tasks — used as tag pool for autocomplete and tag filter picker
	nextRows       []boardRow
	todayRows      []boardRow
	backlogRows    []boardRow // ranked actionable tasks excluding Today/Next (Backlog tab)
	filtered       []boardRow
	carriedOverIDs map[TaskIdentity]bool

	// backlogProjectFilter narrows the Backlog tab to the selected project IDs
	// (empty = all). Populated on demand via the "By Project" quick filter
	// (FP), mirroring the AllTasks/Outliner project-filter pattern.
	backlogProjectFilter map[string]bool

	// Tabs
	tabs TabState

	// Shared task grammar state.
	selection  map[TaskIdentity]struct{}
	menuOpen   bool
	menuCursor int

	pendingReloadIntent focusBoardReloadIntent

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

	previewMode     bool
	focusMode       int // 0=list, 1=tunnel, 2=zen
	childDepthLimit int // -1 = unlimited (default), 0/1/2 = max child depth in Today tab
	// foldOverrides is a per-node fold override keyed by project-qualified identity
	// (true=collapsed, false=force-expanded, absent=follow childDepthLimit).
	// Consulted before childDepthLimit in isParentExpanded; cleared by depth commands.
	foldOverrides map[TaskIdentity]bool
	initialTabSet bool

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

func (m *FocusBoardModel) resetViewScrollOffset() {
	m.viewScrollOffset = 0
}

// SetSize updates the view dimensions and resets the persisted scroll window
// because the visible row budget may have changed.
func (m *FocusBoardModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
	formFactory.ResizeTextInputWithReserve(&m.addInput, width, 4)
	formFactory.ResizeTextInputWithReserve(&m.editInput, width, 4)
	m.resetViewScrollOffset()
}

// NewFocusBoardModel creates a new focus board model.
func NewFocusBoardModel(runtime *config.RuntimeState, manager *config.Manager, dailyPlan, focusQueue *[]FocusTask, queueIDs, dailyPlanIDs, carriedOverIDs map[TaskIdentity]bool) *FocusBoardModel {
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
		configManager:        manager,
		runtime:              runtime,
		planningCommit:       commitPlanningToRuntime,
		projectService:       NewProjectService(),
		queueIDs:             queueIDs,
		dailyPlan:            dailyPlan,
		focusQueue:           focusQueue,
		dailyPlanIDs:         dailyPlanIDs,
		carriedOverIDs:       carriedOverIDs,
		backlogProjectFilter: make(map[string]bool),
		addInput:             ti,
		editInput:            ei,
		focusMode:            focusModeList,
		childDepthLimit:      -1,
		foldOverrides:        make(map[TaskIdentity]bool),
		FilterMixin:          FilterMixin{filter: newFilterState()},
		sort:                 NewSortState(),
		tabs:                 TabState{Names: focusBoardTabNames},
		selection:            make(map[TaskIdentity]struct{}),
		activePerspective:    PerspectiveInactive,
	}
	m.rebuildPlannerDispatcher()
	m.initFilterMixinWithDispatcher(m.Dispatcher, runtime)
	m.InitSearchInput("Search...", 100, 40)
	m.loadItems()
	return m
}

// commitPlanning applies a planning mutation through the model's injectable
// commit seam. Directly-constructed test models retain the production default.
func (m *FocusBoardModel) commitPlanning(mutate func(*config.PlanningState)) error {
	commit := m.planningCommit
	if commit == nil {
		commit = commitPlanningToRuntime
	}
	return commit(m.runtime, mutate)
}

// reorderBlockedBySecondarySort rejects J/K reordering when the visible rows
// are not in the active horizon's canonical order.
func (m *FocusBoardModel) reorderBlockedBySecondarySort(canonicalOrder string) bool {
	if m.sort.Field == SortByDefault {
		return false
	}
	m.setStatus("Reorder unavailable while sorted by " + m.sort.Field.String() +
		" — press 0 to reset to " + canonicalOrder + " first")
	return true
}

func (m *FocusBoardModel) Init() tea.Cmd {
	return nil
}

func (m *FocusBoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if handled, cmd := handleCommonUpdateMsg(msg, func(w, h int) {
		m.SetSize(w, h)
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
		m.refreshData()
		return m, nil

	case tea.KeyPressMsg:
		if m.addMode {
			return m.handleAddInput(msg)
		}
		if m.editMode {
			return m.handleEditInput(msg)
		}
		if m.menuOpen {
			return m, m.handlePlannerMenuKey(msg)
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

// escSteps defines the Esc dismissal cascade: focus mode → search → filters.
func (m *FocusBoardModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.focusMode > focusModeList }, Clear: func() {
			m.focusMode = focusModeList
			m.resetViewScrollOffset()
			m.setStatus("Tunnel mode cleared")
		}},
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

// ShouldCaptureEsc returns true when add mode, edit mode, focus mode, search, prefix, confirm, or filters are active.
func (m *FocusBoardModel) ShouldCaptureEsc() bool {
	return m.addMode || m.editMode || m.menuOpen || len(m.selection) > 0 || m.SearchShouldCaptureEsc() ||
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
	m.menuOpen = false
	clear(m.selection)
	m.resetViewScrollOffset()
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

func (m *FocusBoardModel) StatusBarLabel() string { return "Planner" }

// IsFullHeight returns true because the focus board renders its own footer.
func (m *FocusBoardModel) IsFullHeight() bool { return true }
