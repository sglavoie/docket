package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// FocusEntry is the retained internal focus-frame representation. It records
// the complete viewport state needed to restore the parent view exactly.
type FocusEntry struct {
	TaskID       string // UUID of the task that was focused
	CursorIndex  int    // Cursor position at the moment of focus
	ScrollOffset int    // Persistent viewport offset at the moment of focus
}

// CrossProjectFocusEntry records the state at a single focus level for the
// cross-project outliner. It extends FocusEntry with a ProjectID field to
// scope focus resolution to the correct project, preventing UUID collisions
// across projects from causing incorrect focus resolution.
type CrossProjectFocusEntry struct {
	TaskID       string // UUID of the task that was focused
	ProjectID    string // ID of the project owning the task
	CursorIndex  int    // Cursor position at the moment of focus
	ScrollOffset int    // Persistent viewport offset at the moment of focus
}

// sectionZoom narrows the Outliner to one section of the active mode, plus the
// viewport Esc restores. A zero value means no zoom.
type sectionZoom struct {
	key    string // Collapse key of the zoomed section
	name   string // Display name, for chrome only
	cursor int    // Cursor row the zoom was entered from
	scroll int    // Persistent viewport offset the zoom was entered from
}

func (z sectionZoom) active() bool { return z.key != "" }

// OutlinerModel is the outliner view for navigating tasks in a hierarchical,
// focusable tree. It is the sole per-project primary view and implements the
// View interface as a standalone struct, owning its own navigation semantics,
// sort modes, and focus-stack behavior.
type OutlinerModel struct {
	listsurface.BaseSearchableView
	listsurface.InteractiveViewState
	listsurface.MultiSelectState
	pulse.TodayPaceState

	runtime        *config.RuntimeState
	configManager  *config.Manager
	taskItem       coordinator.TaskItemCoordinator
	projectService *coordinator.ProjectService
	quickEdit      *grammar.QuickEditComponent

	project *models.Project

	focusStack      []FocusEntry             // Focus stack for single-project mode
	crossFocusStack []CrossProjectFocusEntry // Focus stack for cross-project mode (m.project == nil)
	focusNotice     string                   // Transient message shown when focus target is removed
	zoom            sectionZoom              // Section the projection is narrowed to; zero when none

	rows []outlinerRow // Flattened rows, rebuilt on each render
	// lastViewport is the window the last render actually drew. zt/zz/zb read it
	// rather than recomputing the window, which would re-derive it from a
	// different footer state and move ViewScrollOffset.
	lastViewport outlinerViewport
	// primaryMode is the authoritative canonical row-strategy identity.
	primaryMode chrome.OutlinerModeID

	sort     query.SortState // Active render sort; default while a fixed-order mode owns ordering.
	treeSort query.SortState // Durable user sort restored whenever a tree-order mode is active.

	// filterQuery is the one narrowing predicate the f and F facets author. Both
	// carriers below are caches derived from it by setFilterQuery, so nothing
	// downstream can hold a facet the query does not know about.
	filterQuery models.Query

	filter query.FilterState // Row facet cache derived from filterQuery.

	// projectFilter is the population facet cache the F chord's clauses derive.
	// It stays a separate value from the row filter because clearing one must not
	// clear the other, and a change here needs a reload rather than a rebuild.
	projectFilter query.ProjectFilterState

	expandedNodes     map[taskref.TaskIdentity]bool // Project-qualified fold state: true=expanded (default all expanded)
	collapsedSections map[string]bool               // Section collapse state for All mode
	// expandedChecklists is the project-qualified checklist fold state, and it
	// defaults to collapsed: an absent key hides the owner's items. It is
	// deliberately separate from expandedNodes, which defaults to expanded, and
	// from collapsedSections, whose string keys are shared with grouping headers
	// and are never cleared on a mode change.
	expandedChecklists map[taskref.TaskIdentity]bool
	// sectionToggleRestore preserves a task focus across one S-driven
	// collapse/expand cycle. It is deliberately session-only.
	sectionToggleRestore *sectionToggleRestoreAnchor

	// relocationHistory is a session-only Vim-style jump list for task updates
	// that automatically move the followed task to another visible row.
	relocationHistory outlinerRelocationHistory

	// recencyKeys freezes Recent mode's ordering timestamps against the
	// projection they were taken in, so triaging a row from Recent does not
	// reshuffle the list the user is reading.
	recencyKeys       map[taskref.TaskIdentity]time.Time
	recencyProjection string

	// search is the slash-search frame, the compiled reading of the query it
	// holds, the input's suggestion pane and recall ring, and the notes tier that
	// query may reach. Like the readout, the menu bar and the population below it
	// is a named field rather than promoted ones — outliner_search_state.go owns
	// the cluster, and the two clear paths reach every part of it through
	// m.search rather than by assigning each field they happen to remember.
	search searchState

	// lensQuery is the accepted narrowing query of the saved projection the view
	// is showing, and lensGate, lensRemainder, and lensNarrowRelaxed are its
	// compiled form. It is deliberately not the slash-search query: a lens query
	// narrows its own saved mode, facets, and sort, while ordinary search
	// replaces the whole projection and stays scope-wide.
	lensQuery         string
	lensGate          *query.SearchPredicates
	lensRemainder     string
	lensNarrowRelaxed bool

	// activeLens is the one live named lens, and pendingLens an application whose
	// project population is still loading. There is never a stack of either: a
	// switch replaces the definition and keeps the original restoration point, so
	// one Esc always returns to the view the user actually left. Neither survives
	// a restart or a global reset.
	activeLens  *activeLensFrame
	pendingLens *pendingLensApplication

	// persistActiveLens records which lens the Outliner should reopen with, or
	// clears that pointer when passed the empty string. The shell injects it, so
	// the Outliner never learns where machine-local state is committed; a nil seam
	// is inert and simply remembers nothing.
	persistActiveLens func(id string) error

	// lensQueryEditing points the shared search input at the lens's narrowing
	// query instead of the ordinary search query, and lensQueryEditOrigin is the
	// query Esc restores when that edit is abandoned.
	lensQueryEditing    bool
	lensQueryEditOrigin string

	// outstandingConflicts is the shell-injected count of recorded merge-conflict
	// bundles the status strip reports. A nil accessor is inert, so a
	// struct-literal test model and the Hub's preview painter simply carry no
	// indicator; the Outliner never learns where conflicts are stored.
	outstandingConflicts func() int

	// advisoriesHiddenAccessor is the shell-injected opt-out every ambient
	// advisory is gated by. A nil accessor reads as showing them; see
	// advisories.go.
	advisoriesHiddenAccessor func() bool

	// nextDeferWake is the soonest instant a pending deferral in scope lapses,
	// and zero means none is pending. It is a refresh clock read by the periodic
	// tick; what a row shows stays isComputedOpenWork's decision.
	nextDeferWake time.Time

	// openChildCounts holds each task's open direct-child count, read from the
	// authoritative project trees rather than from a row's own task, whose
	// Children the filter pipeline may have rewritten.
	openChildCounts map[taskref.TaskIdentity]int

	// dependencyIndexCache is the row-rebuild pass's reading of the soft
	// reference graph over the loaded population. It is a cache and never
	// authored state: dependencyIndex builds it on demand and
	// invalidateDependencyIndex drops it whenever that population is replaced.
	dependencyIndexCache *app.DependencyIndex

	// Column visibility config (loaded from project view state)
	columnConfig *listsurface.ColumnConfig

	// Peek mode follows the cursor and shows additive task detail inline.
	peekMode bool

	// queryReadout opens the narrowing stack in the status strip. Its own file
	// carries the whole of that sub-state; nothing here can go stale, because the
	// rows are read off the live axes each frame.
	queryReadout queryReadoutState

	// menuBar is the menu bar's dropdown gate and the two indices that say where
	// the highlight sits inside it. Like the readout above it is a named field
	// rather than promoted ones: menu_bar_state.go owns the cluster, and every
	// reset that replaces the whole projection reaches it through closeMenuBar.
	menuBar menuBarState

	// OpenAtTask, when set, positions the cursor on the given task the first
	// time rebuildRows produces a non-empty row list. Cleared after first use.
	OpenAtTask *models.Task

	// population is the cross-project population itself (used when m.project ==
	// nil), the lifecycle of the read that replaces it, where the cursor lands
	// once it has, and the memoization guard crossProjectRebuildRows consults
	// over it. Like the readout and the menu bar above it, it is a named field
	// rather than promoted ones — outliner_population_state.go owns the cluster,
	// and every caller that changes what the population would produce says so
	// through m.population.invalidate() rather than by writing a flag.
	population populationState

	// scopedProjectID names the single project the global outliner is currently
	// scoped to. Empty means the authoritative all-projects scope. Only
	// meaningful in cross-project mode (m.project == nil); scoping filters the
	// loaded data set down to one project without switching to a separate
	// single-project OutlinerModel.
	scopedProjectID string

	// pendingParkedFocusID names a parked project whose focus load is in flight.
	// The scope is pushed only once that load lands, so a failed or superseded
	// request leaves the base snapshot exactly as it was.
	pendingParkedFocusID string

	// parkedFocusProjectID names the parked project the current snapshot admits
	// as an explicit exception. Every later reload re-requests it, so an ordinary
	// mutation refresh cannot silently drop the focused tree.
	parkedFocusProjectID string

	// parkedFocusReloadPending records that a parked focus was just dropped and
	// the shell still owes a reload to shed the parked project from the snapshot.
	// The Esc cascade cannot return a Cmd, so the shell claims it afterwards.
	parkedFocusReloadPending bool

	// scopeStack records the state to restore when a project scope is popped.
	// Each push saves the previous scope's cursor, scroll, mode, multi-project
	// filter, and focus so Esc returns exactly where the user left off. The
	// bottom of the stack is the base all-projects frame.
	scopeStack []outlinerScopeFrame

	// operationRegistry projects help, handled keys, and footer hints from the
	// same operation metadata used by shell dispatch.
	operationRegistry *operation.OperationRegistry

	// workspacePulse mirrors AppModel.workspacePulse for the title-row
	// operational counter.
	workspacePulse pulse.WorkspacePulse
}

// SetWorkspacePulse injects the latest cross-project operational counts.
func (m *OutlinerModel) SetWorkspacePulse(workspacePulse pulse.WorkspacePulse) {
	m.workspacePulse = workspacePulse
}

func (m *OutlinerModel) resetViewScrollOffset() {
	m.ViewScrollOffset = 0
}

// SetSize updates the view dimensions and resets the persisted scroll window
// because the visible row budget may have changed.
func (m *OutlinerModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
	m.resetViewScrollOffset()
}

func (m *OutlinerModel) getCursorTask() *models.Task {
	if len(m.rows) > 0 && m.Cursor >= 0 && m.Cursor < len(m.rows) {
		row := m.rows[m.Cursor]
		if row.isTaskRow() {
			return row.task
		}
	}
	return nil
}

// NewOutlinerModel creates a new OutlinerModel for the given project.
func NewOutlinerModel(project *models.Project, cfgManager *config.Manager, runtime *config.RuntimeState) *OutlinerModel {
	m := &OutlinerModel{
		runtime:            runtime,
		configManager:      cfgManager,
		projectService:     coordinator.NewProjectService(),
		quickEdit:          grammar.NewQuickEditComponent(),
		project:            project,
		primaryMode:        chrome.ModeAll,
		expandedNodes:      make(map[taskref.TaskIdentity]bool),
		collapsedSections:  make(map[string]bool),
		expandedChecklists: make(map[taskref.TaskIdentity]bool),
		filter:             query.NewFilterState(),
		projectFilter:      query.NewProjectFilterState(),
		sort:               query.NewSortState(),
		treeSort:           query.NewSortState(),
		MultiSelectState:   listsurface.MultiSelectState{Selected: make(map[taskref.TaskIdentity]bool)},
		columnConfig:       listsurface.NewColumnConfig(nil, nil, listsurface.DefaultOutlinerColumns),
	}
	m.InitSearchInput("Search tasks...", 100, 30)
	return m
}

// SetTaskItemCoordinator wires the retained application mutation seam.
func (m *OutlinerModel) SetTaskItemCoordinator(coordinator coordinator.TaskItemCoordinator) {
	m.taskItem = coordinator
}

// Init implements tea.Model.
func (m *OutlinerModel) Init() tea.Cmd {
	if m.project == nil {
		return m.startCrossProjectLoad()
	}
	return nil
}

// Update implements tea.Model.
func (m *OutlinerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case status.CursorFlashClearMsg:
		m.CursorFlash = false
		return m, nil

	case taskdetail.CommentAddedMsg:
		// Rebuild rows so cloned tasks from tree projections pick up the
		// in-place comment mutation on the original task.
		m.rebuildRows()
		return m, nil

	case listsurface.ParentSelectedMsg:
		if !msg.ReparentTask.Valid() {
			return m, nil
		}
		if m.project == nil {
			return m.handleCrossProjectReparent(msg)
		}
		if msg.ReparentTask.ProjectID != m.project.ID || m.project.GetTaskByID(msg.ReparentTask.TaskID) == nil {
			return m, nil
		}
		return m, m.taskItem.Reparent(msg.ReparentTask, msg.ParentID)

	case StartTaskLineEditMsg:
		if msg.Task == nil || msg.Project == nil || msg.Project.GetTaskByID(msg.Task.ID) != msg.Task {
			return m, nil
		}
		return m, m.quickEdit.EnterWithContext(msg.Task, msg.Project, m.quickEditProjects(), false)

	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	case tea.PasteMsg:
		if m.quickEdit.IsActive() {
			return m.handleQuickEditMode(msg)
		}
		return m, nil
	case status.ViewDidAppearMsg:
		return m, m.handleViewDidAppear()
	case crossProjectsLoadedMsg:
		return m, m.handleCrossProjectsLoaded(msg)
	case lensProjectsLoadedMsg:
		cmd, _ := m.handleLensProjectsLoaded(msg)
		return m, cmd
	case crossSpinnerDelayMsg:
		if m.population.loading && !m.hasCrossProjectSnapshot() {
			m.population.showSpinner()
			return m, m.population.spinner.Tick()
		}
		return m, nil
	case crossSpinnerTickMsg:
		if m.population.spinnerVisible {
			return m, crossSpinnerTickFwd(m.population.advanceSpinner(msg.inner))
		}
		return m, nil
	}
	return m, nil
}
