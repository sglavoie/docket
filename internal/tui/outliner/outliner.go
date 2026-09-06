package outliner

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

// OutlinerModel is the outliner view for navigating tasks in a hierarchical,
// focusable tree. It is the sole per-project primary view and implements the
// View interface as a standalone struct, owning its own navigation semantics,
// sort modes, and focus-stack behavior.
type OutlinerModel struct {
	listsurface.BaseSearchableView
	listsurface.InteractiveViewState
	listsurface.MultiSelectState
	pulse.TodayPaceState

	Runtime        *config.RuntimeState
	configManager  *config.Manager
	taskItem       coordinator.TaskItemCoordinator
	ProjectService *coordinator.ProjectService
	QuickEdit      *grammar.QuickEditComponent

	Project *models.Project

	// focus is both focus stacks, the section zoom, and the one-shot notice a
	// stack that stopped resolving leaves behind. Like the readout, the menu bar,
	// the population and the search below it, it is a named field rather than
	// promoted ones — outliner_focus_state.go owns the cluster, and the Esc
	// ladder and every wholesale reset reach each narrowing through m.focus.
	Focus focusState

	Rows []OutlinerRow // Flattened rows, rebuilt on each render
	// lastViewport is the window the last render actually drew. zt/zz/zb read it
	// rather than recomputing the window, which would re-derive it from a
	// different footer state and move ViewScrollOffset.
	LastViewport outlinerViewport
	// primaryMode is the authoritative canonical row-strategy identity.
	PrimaryMode chrome.OutlinerModeID

	Sort     query.SortState // Active render sort; default while a fixed-order mode owns ordering.
	TreeSort query.SortState // Durable user sort restored whenever a tree-order mode is active.

	// filterQuery is the one narrowing predicate the f and F facets author. Both
	// carriers below are caches derived from it by setFilterQuery, so nothing
	// downstream can hold a facet the query does not know about.
	FilterQuery models.Query

	Filter query.FilterState // Row facet cache derived from filterQuery.

	// projectFilter is the population facet cache the F chord's clauses derive.
	// It stays a separate value from the row filter because clearing one must not
	// clear the other, and a change here needs a reload rather than a rebuild.
	ProjectFilter query.ProjectFilterState

	// folds is the three fold maps — task subtrees, sections and project groups,
	// checklists — and the anchor an s-driven collapse leaves for the expansion
	// that follows it. Their polarities and key spaces disagree deliberately, so
	// outliner_fold_state.go names them together rather than merging them, and
	// clearAllFolds is one act over three axes through m.folds.
	Folds foldState

	// relocationHistory is a session-only Vim-style jump list for task updates
	// that automatically move the followed task to another visible row.
	RelocationHistory outlinerRelocationHistory

	// recency freezes Recent mode's ordering timestamps against the projection
	// they were taken in, so triaging a row from Recent does not reshuffle the
	// list the user is reading. Like the search and the lens below it, it is a
	// named field rather than promoted ones — outliner_recency_state.go owns the
	// pair, and no caller can move the fingerprint without the keys that hang off
	// it.
	Recency recencyState

	// search is the slash-search frame, the compiled reading of the query it
	// holds, the input's suggestion pane and recall ring, and the notes tier that
	// query may reach. Like the readout, the menu bar and the population below it
	// is a named field rather than promoted ones — outliner_search_state.go owns
	// the cluster, and the two clear paths reach every part of it through
	// m.search rather than by assigning each field they happen to remember.
	Search searchState

	// lens is the one live named lens, the application whose population is still
	// loading behind it, the narrowing query that lens installs and the compiled
	// reading of it, the shell seam that remembers which lens to reopen with, and
	// the input mode that retypes that query. Like the search beside it, it is a
	// named field rather than promoted ones — outliner_lens_view_state.go owns the
	// cluster, and putting a lens down reaches every part of it through m.lens
	// rather than by assigning each field the caller happens to remember.
	Lens outlinerLensState

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
	NextDeferWake time.Time

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
	PeekMode bool

	// queryReadout opens the narrowing stack in the status strip. Its own file
	// carries the whole of that sub-state; nothing here can go stale, because the
	// rows are read off the live axes each frame.
	QueryReadout queryReadoutState

	// menuBar is the menu bar's dropdown gate and the two indices that say where
	// the highlight sits inside it. Like the readout above it is a named field
	// rather than promoted ones: menu_bar_state.go owns the cluster, and every
	// reset that replaces the whole projection reaches it through closeMenuBar.
	MenuBar menuBarState

	// relationPicker is `g~`'s list of the cursor task's dependencies and
	// dependents, and relationTrail the return frames the hops it takes leave
	// behind. They are one cluster: only the picker pushes a frame, and only Esc
	// or a global reset takes one back. relation_picker.go and
	// relation_navigation.go own them.
	RelationPicker relationPickerState
	relationTrail  []relationReturnFrame

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
	Population populationState

	// scope is the single project focus the cross-project Outliner is standing in,
	// the one base frame Esc restores behind it, and the parked-project exception
	// that can be the reason that focus is reachable at all. Like the population
	// above it, it is a named field rather than promoted ones —
	// outliner_scope_state.go owns the cluster, and the wide resets below stay
	// orchestrators that name it rather than being distributed into it.
	Scope scopeState

	// operationRegistry projects help, handled keys, and footer hints from the
	// same operation metadata used by shell dispatch.
	OperationRegistry *operation.OperationRegistry
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
	if len(m.Rows) > 0 && m.Cursor >= 0 && m.Cursor < len(m.Rows) {
		row := m.Rows[m.Cursor]
		if row.IsTaskRow() {
			return row.Task
		}
	}
	return nil
}

// NewOutlinerModel creates a new OutlinerModel for the given project.
func NewOutlinerModel(project *models.Project, cfgManager *config.Manager, runtime *config.RuntimeState) *OutlinerModel {
	m := &OutlinerModel{
		Runtime:          runtime,
		configManager:    cfgManager,
		ProjectService:   coordinator.NewProjectService(),
		QuickEdit:        grammar.NewQuickEditComponent(),
		Project:          project,
		PrimaryMode:      chrome.ModeAll,
		Filter:           query.NewFilterState(),
		ProjectFilter:    query.NewProjectFilterState(),
		Sort:             query.NewSortState(),
		TreeSort:         query.NewSortState(),
		MultiSelectState: listsurface.MultiSelectState{Selected: make(map[taskref.TaskIdentity]bool)},
		columnConfig:     listsurface.NewColumnConfig(nil, nil, listsurface.DefaultOutlinerColumns),
	}
	m.Folds.ResetAll()
	m.InitSearchInput("Search tasks...", 100, 30)
	return m
}

// SetTaskItemCoordinator wires the retained application mutation seam.
func (m *OutlinerModel) SetTaskItemCoordinator(coordinator coordinator.TaskItemCoordinator) {
	m.taskItem = coordinator
}

// Init implements tea.Model.
func (m *OutlinerModel) Init() tea.Cmd {
	if m.Project == nil {
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
		m.RebuildRows()
		return m, nil

	case listsurface.ParentSelectedMsg:
		if !msg.ReparentTask.Valid() {
			return m, nil
		}
		if m.Project == nil {
			return m.handleCrossProjectReparent(msg)
		}
		if msg.ReparentTask.ProjectID != m.Project.ID || m.Project.GetTaskByID(msg.ReparentTask.TaskID) == nil {
			return m, nil
		}
		return m, m.taskItem.Reparent(msg.ReparentTask, msg.ParentID)

	case StartTaskLineEditMsg:
		if msg.Task == nil || msg.Project == nil || msg.Project.GetTaskByID(msg.Task.ID) != msg.Task {
			return m, nil
		}
		return m, m.QuickEdit.EnterWithContext(msg.Task, msg.Project, m.quickEditProjects(), false)

	case tea.KeyPressMsg:
		return m.HandleKeyMsg(msg)
	case tea.PasteMsg:
		if m.QuickEdit.IsActive() {
			return m.HandleQuickEditMode(msg)
		}
		return m, nil
	case status.ViewDidAppearMsg:
		return m, m.handleViewDidAppear()
	case CrossProjectsLoadedMsg:
		return m, m.HandleCrossProjectsLoaded(msg)
	case LensProjectsLoadedMsg:
		cmd, _ := m.HandleLensProjectsLoaded(msg)
		return m, cmd
	case CrossSpinnerDelayMsg:
		if m.Population.Loading && !m.hasCrossProjectSnapshot() {
			m.Population.showSpinner()
			return m, m.Population.spinner.Tick()
		}
		return m, nil
	case crossSpinnerTickMsg:
		if m.Population.SpinnerVisible {
			return m, crossSpinnerTickFwd(m.Population.advanceSpinner(msg.inner))
		}
		return m, nil
	}
	return m, nil
}
