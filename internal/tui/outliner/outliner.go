package outliner

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/grammar"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskdetail"
	"github.com/sglavoie/docket/internal/tui/taskref"
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

	// homeDir is the home directory resolved once at setup through config, so a
	// row compacting a home path never asks the environment itself; empty means
	// unknown and compacts no path.
	homeDir string

	// focus is both focus stacks, the section zoom, and the one-shot notice a
	// stack that stopped resolving leaves behind. Like the readout, the menu bar,
	// the population and the search below it, it is a named field rather than
	// promoted ones — outliner_focus_state.go owns the cluster, and the Esc
	// ladder and every wholesale reset reach each narrowing through m.focus.
	Focus focusState

	Rows []OutlinerRow // Flattened projection, rebuilt when its inputs change
	// lastViewport is the window the last render actually drew. zt/zz/zb read it
	// rather than recomputing the window, which would re-derive it from a
	// different footer state and move ViewScrollOffset.
	LastViewport outlinerViewport
	// sectionHeaderRows names each lane's header row by the section half of
	// OutlinerRowGroup. It is rebuilt with the rows, so the window authority can
	// tell a header scrolled above the window from the row's own group rather
	// than by scanning upward for one.
	sectionHeaderRows map[string]int
	projectHeaderRows map[string]int
	// hostBudget is the loan outstanding while a bounded panel is drawn over this
	// list: where the window stood when the panel took its rows, held as a
	// position in the document rather than a number of them.
	// The hostBudgetAnchor cluster below owns it, and SetSize is the one other
	// writer, because a terminal resize ends the loan rather than repaying it.
	hostBudget hostBudgetAnchor
	// shellFrame is what the shell last said its own footer costs. It is a
	// budget rather than a size, which is why it is held here instead of folded
	// into Height: SetSize drops the scroll window, and a footer that grows a row
	// when a message arrives would re-anchor the list under every keystroke.
	ShellFrame     chrome.ShellFrameMetrics
	feedbackBudget feedbackBudgetAnchor
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

	// syncAdvisory is the shell-injected sync line for the status strip; nil is
	// inert. See advisories.go.
	syncAdvisory func() string

	// advisoriesHiddenAccessor is the shell-injected opt-out every ambient
	// advisory is gated by. A nil accessor reads as showing them; see
	// advisories.go.
	advisoriesHiddenAccessor func() bool

	// nextDeferWake is the soonest instant a pending deferral in scope lapses,
	// and zero means none is pending. It is a refresh clock read by the periodic
	// tick; what a row shows stays isComputedOpenWork's decision.
	NextDeferWake time.Time

	// timeDrivenDay is the local calendar day the current canonical projection
	// was built on. Review queries and lane counts can change when a check-in day
	// arrives without any task mutation, so the shell's periodic tick compares
	// against it and leaves an invalidation owed until interaction state is safe.
	timeDrivenDay           time.Time
	timeDrivenCursor        modeCursorAnchor
	timeDrivenCursorPending bool

	// openChildCounts holds each task's open direct-child count, read from the
	// authoritative project trees rather than from a row's own task, whose
	// Children the filter pipeline may have rewritten.
	openChildCounts map[taskref.TaskIdentity]int
	// hiddenLayout is what this projection's folds hide from the row layout,
	// rebuilt with the rows so a fold opening moves no title.
	hiddenLayout  hiddenRowLayout
	projectLabels projectLabelCache

	// dependencyIndexCache is the row-rebuild pass's reading of the soft
	// reference graph over the loaded population. It is a cache and never
	// authored state: dependencyIndex builds it on demand and
	// invalidateDependencyIndex drops it whenever that population is replaced.
	dependencyIndexCache *app.DependencyIndex

	// Column visibility config (loaded from project view state)
	columnConfig *listsurface.ColumnConfig

	// Peek mode follows the cursor and shows additive task detail inline.
	PeekMode bool
	// PeekExpanded is a session preference, independent of cursor and open state.
	PeekExpanded bool

	// peek is the one page of that detail Peek is showing. It is session-only
	// and self-invalidating: inline_peek_paging.go owns the cluster, and the key
	// beside the page is what makes a different task, a changed detail, or a
	// resize read as the first page without any writer resetting it.
	peek peekOffsetState

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
	// HistoryPreview is read once when a menu opens, never during rendering.
	HistoryPreview func() (undo, redo string)
}

func (m *OutlinerModel) resetViewScrollOffset() {
	m.ViewScrollOffset = 0
}

// SetSize updates the view dimensions and resets the persisted scroll window
// because the visible row budget may have changed.
//
// It is the resize a terminal causes, and it drops the loan a bounded panel may
// have outstanding along with the window that loan was measured against: a user
// who resized the terminal is reading a different amount of the document, and
// repaying them a window that resize already dropped would put back something
// they did not keep. The loan itself is the cluster below.
func (m *OutlinerModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
	m.hostBudget = hostBudgetAnchor{}
	m.feedbackBudget = feedbackBudgetAnchor{}
	m.resetViewScrollOffset()
}

// hostBudgetAnchor is the view a bounded panel borrowed rows from, held for as
// long as the loan lasts. A resize and a loan are two different acts: a terminal
// that changed size is showing a different amount of a document the user then
// re-reads, while a panel opened over this list is showing the same document
// with a strip of it covered, and the strip goes away again.
//
// What it remembers is a position in the document rather than a number of rows,
// because the panel above it can commit work: the anchor row and the cursor are
// both held as qualified task identities, and the number is only what is left
// when neither still resolves.
type hostBudgetAnchor struct {
	// active is whether a loan is outstanding. It is what makes every later
	// budget one change to the same loan rather than a second one taken out
	// against the window the first one already shortened.
	active bool

	// scrollOffset is the window's own top row when the loan was taken, and the
	// last rung of the ladder below.
	scrollOffset int

	// anchor is the first task row at or below that top row, and rowsAboveAnchor
	// how far inside the window it sat, so a window led by a section or project
	// header is anchored by the first row in it that has an identity at all.
	anchor          taskref.TaskIdentity
	rowsAboveAnchor int

	// cursor is the selected task and rowsAboveCursor its distance from the same
	// top row. It is the second rung: with the anchor row gone, the row the user
	// was standing on is the one whose screen position is worth keeping.
	cursor          taskref.TaskIdentity
	rowsAboveCursor int
}

// SetHostedBudget answers route.HostedBudgetView. It is the shell handing this
// list the rows a bounded panel left it, and it is deliberately not SetSize: the
// scroll window survives it, so opening capture over the outline covers the
// bottom of the view instead of re-seating what the user was reading.
//
// The anchor is taken once, at the first budget, and every later one re-seats
// the window against that same one. That is what makes successive height changes
// — a panel growing with its draft, a syntax guide opening, an expanded form
// collapsing back — one loan with one repayment rather than a drift whichever
// budget came last happens to decide.
func (m *OutlinerModel) SetHostedBudget(width, height int) {
	if m == nil {
		return
	}
	if !m.hostBudget.active {
		m.hostBudget = m.captureHostBudgetAnchor()
	}
	m.BaseSearchableView.SetSize(width, height)
	m.seatWindowAtAnchor(m.hostBudget)
}

// ReleaseHostedBudget answers the other half: the panel closed and this view is
// owed its whole content area back. The anchor is spent here and put down, so
// the window the loan started from is the window the release returns to wherever
// the rows still permit it, and the render's own clamp decides how far that is.
//
// A release with no loan behind it is an ordinary resize, which this view has
// always answered by dropping its window.
func (m *OutlinerModel) ReleaseHostedBudget(width, height int) {
	if m == nil {
		return
	}
	held := m.hostBudget
	m.hostBudget = hostBudgetAnchor{}
	m.BaseSearchableView.SetSize(width, height)
	if !held.active {
		m.resetViewScrollOffset()
		return
	}
	m.seatWindowAtAnchor(held)
}

// captureHostBudgetAnchor reads the current window as a position in the
// document. It writes nothing at all: the cursor, the folds, the scope, the
// search and the rows are the view's own, and a loan is not the seam that
// restores any of them.
func (m *OutlinerModel) captureHostBudgetAnchor() hostBudgetAnchor {
	held := hostBudgetAnchor{active: true, scrollOffset: m.ViewScrollOffset}
	for index := max(m.ViewScrollOffset, 0); index < len(m.Rows); index++ {
		row := m.Rows[index]
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		identity := m.TaskIdentityForRow(row)
		if !identity.Valid() {
			continue
		}
		held.anchor = identity
		held.rowsAboveAnchor = index - m.ViewScrollOffset
		break
	}
	if cursor := m.CurrentTaskIdentity(); cursor.Valid() {
		held.cursor = cursor
		held.rowsAboveCursor = m.Cursor - m.ViewScrollOffset
	}
	return held
}

// seatWindowAtAnchor writes the window back where the loan found it. It writes
// ViewScrollOffset alone, and writes a valid one: keeping the cursor visible and
// cutting the window to the document belong to ComputeOutlinerViewport, which is
// the one window authority and runs on the very frame this seat is for.
func (m *OutlinerModel) seatWindowAtAnchor(held hostBudgetAnchor) {
	if !held.active {
		return
	}
	m.ViewScrollOffset = max(m.anchoredScrollOffset(held), 0)
}

// anchoredScrollOffset is the ladder: the anchor row where it is still drawn,
// then the cursor's own row, then the number the loan started from. The first
// two rungs are qualified identities resolved against the rows as they are now,
// so a task captured into the projection while the panel was open moves the
// window with it rather than sliding the document under it.
func (m *OutlinerModel) anchoredScrollOffset(held hostBudgetAnchor) int {
	if index, ok := m.RowIndexForTask(held.anchor); ok {
		return index - held.rowsAboveAnchor
	}
	if index, ok := m.RowIndexForTask(held.cursor); ok {
		return index - held.rowsAboveCursor
	}
	return held.scrollOffset
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
	if home, ok := config.HomeDir(); ok {
		m.homeDir = home
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
	if m.IsSearchMode() && m.Search.historyPicker != nil {
		switch msg.(type) {
		case tea.KeyPressMsg, tea.PasteMsg:
			return m, m.updateSearchHistoryPicker(msg)
		}
	}
	switch msg := msg.(type) {
	case BackgroundSearchResultMsg:
		m.acceptBackgroundSearch(msg)
		return m, nil
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
