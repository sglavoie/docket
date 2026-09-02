package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
	BaseSearchableView
	InteractiveViewState
	MultiSelectState
	todayPaceState

	runtime        *config.RuntimeState
	configManager  *config.Manager
	taskItem       TaskItemCoordinator
	projectService *ProjectService
	quickEdit      *QuickEditComponent

	project *models.Project

	focusStack      []FocusEntry             // Focus stack for single-project mode
	crossFocusStack []CrossProjectFocusEntry // Focus stack for cross-project mode (m.project == nil)
	focusNotice     string                   // Transient message shown when focus target is removed
	zoom            sectionZoom              // Section the projection is narrowed to; zero when none

	rows []outlinerRow // Flattened rows, rebuilt on each render
	// lastViewport is the window the last render actually drew. zt/zz/zb read it
	// rather than recomputing the window, which would re-derive it from a
	// different footer state and move viewScrollOffset.
	lastViewport outlinerViewport
	// primaryMode is the authoritative canonical row-strategy identity.
	primaryMode OutlinerModeID

	sort     SortState // Active render sort; default while a fixed-order mode owns ordering.
	treeSort SortState // Durable user sort restored whenever a tree-order mode is active.

	// filterQuery is the one narrowing predicate the f and F facets author. Both
	// carriers below are caches derived from it by setFilterQuery, so nothing
	// downstream can hold a facet the query does not know about.
	filterQuery models.Query

	filter FilterState // Row facet cache derived from filterQuery.

	// projectFilter is the population facet cache the F chord's clauses derive.
	// It stays a separate value from the row filter because clearing one must not
	// clear the other, and a change here needs a reload rather than a rebuild.
	projectFilter ProjectFilterState

	expandedNodes     map[TaskIdentity]bool // Project-qualified fold state: true=expanded (default all expanded)
	collapsedSections map[string]bool       // Section collapse state for All mode
	// expandedChecklists is the project-qualified checklist fold state, and it
	// defaults to collapsed: an absent key hides the owner's items. It is
	// deliberately separate from expandedNodes, which defaults to expanded, and
	// from collapsedSections, whose string keys are shared with grouping headers
	// and are never cleared on a mode change.
	expandedChecklists map[TaskIdentity]bool
	// sectionToggleRestore preserves a task focus across one S-driven
	// collapse/expand cycle. It is deliberately session-only.
	sectionToggleRestore *sectionToggleRestoreAnchor

	// relocationHistory is a session-only Vim-style jump list for task updates
	// that automatically move the followed task to another visible row.
	relocationHistory outlinerRelocationHistory

	// recencyKeys freezes Recent mode's ordering timestamps against the
	// projection they were taken in, so triaging a row from Recent does not
	// reshuffle the list the user is reading.
	recencyKeys       map[TaskIdentity]time.Time
	recencyProjection string

	// searchProjection is the navigation frame a slash search was typed in. It
	// preserves every presentation axis the search frame may move while it stands,
	// so leaving the frame returns the user to where the query was typed.
	searchProjection *outlinerSearchSnapshot

	// rankedList shows the held query's matches as one flat relevance-ordered
	// list instead of narrowing the mode's own lanes. It is a presentation of the
	// same query, toggled deliberately, never entered by typing, and it dies with
	// the query that gave it rows.
	rankedList bool

	// searchCaseRelaxed records that the last projection rebuild found nothing
	// under the query's smart-case reading and fell back to a case-insensitive
	// pass. Highlighting and the chrome badge both read it, so what matched and
	// what is shown can never disagree.
	searchCaseRelaxed bool

	// searchRemainder is the free text left after the sigil gate was lifted out
	// of the query, and searchGate is that gate. Scoring, smart-case, highlighting
	// and the work-mode predicate all read the remainder, so a predicate token is
	// never scored or highlighted as prose.
	searchRemainder string
	searchGate      *searchPredicates

	// lensQuery is the accepted narrowing query of the saved projection the view
	// is showing, and lensGate, lensRemainder, and lensNarrowRelaxed are its
	// compiled form. It is deliberately not the slash-search query: a lens query
	// narrows its own saved mode, facets, and sort, while ordinary search
	// replaces the whole projection and stays scope-wide.
	lensQuery         string
	lensGate          *searchPredicates
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

	// noteSearchIndex is the shell-injected accessor over the workspace-wide note
	// index the search tier ranks. A nil accessor means the tier is simply
	// absent, which is what a struct-literal test model and the Project Hub's
	// preview painter both get: that preview is a task preview, and notes are
	// not tasks. The Outliner never learns where notes live or when they are
	// read; it only reads whatever the shell last retained.
	noteSearchIndex func() []LocatedNoteRow

	// noteCursorReseat is the note the cursor rested on when an accepted
	// note-index refresh forced a rebuild. The refresh lands asynchronously and
	// re-ranks the notes tier, so the numeric cursor alone would silently name a
	// different result on the next frame. It is captured with the force flag and
	// consumed by the one rebuild that flag guarantees; a rebuild that finds the
	// note gone leaves the ordinary clamp in place.
	noteCursorReseat NoteRowIdentity

	// searchNoteMatches is what the notes tier's rows are rendered from, keyed the
	// way the Notes manager keys its own match map. It lives beside the rows
	// rather than on them because a note row's render payload is wide — the note
	// itself, its owner label, and the positions that ranked its title — and
	// outlinerRow is copied for every row of every rebuild.
	searchNoteMatches map[NoteRowIdentity]outlinerSearchNoteMatch

	// searchAutocomplete is the slash-search suggestion pane. It holds no cached
	// vocabulary: every refresh reprojects the grammar and rereads the live
	// candidates, so a mutation or a scope change cannot leave a stale value
	// behind. The zero value is inert, which keeps struct-literal test models
	// working without a constructor.
	searchAutocomplete outlinerSearchAutocomplete

	// searchHistory is the session-only ring of submitted slash queries, walked
	// with Up and Down inside the input. It is never persisted, exactly like jump
	// history, and unlike jump history it is not scoped to a projection: a query
	// is worth recalling in whatever mode or scope the user reaches for it next,
	// and it names tasks rather than row positions, so no rebuild can invalidate
	// it. The zero value is inert.
	searchHistory outlinerSearchHistory

	// nextDeferWake is the soonest instant a pending deferral in scope lapses,
	// and zero means none is pending. It is a refresh clock read by the periodic
	// tick; what a row shows stays isComputedOpenWork's decision.
	nextDeferWake time.Time

	// openChildCounts holds each task's open direct-child count, read from the
	// authoritative project trees rather than from a row's own task, whose
	// Children the filter pipeline may have rewritten.
	openChildCounts map[TaskIdentity]int

	// dependencyIndexCache is the row-rebuild pass's reading of the soft
	// reference graph over the loaded population. It is a cache and never
	// authored state: dependencyIndex builds it on demand and
	// invalidateDependencyIndex drops it whenever that population is replaced.
	dependencyIndexCache *app.DependencyIndex

	// Column visibility config (loaded from project view state)
	columnConfig *ColumnConfig

	// Peek mode follows the cursor and shows additive task detail inline.
	peekMode bool

	// queryReadoutExpanded opens the narrowing stack in the status strip. It is
	// the readout's whole state: the rows themselves are read off the live axes
	// each frame, so there is nothing here that can go stale.
	queryReadoutExpanded bool

	// menuBarOpen is the menu bar's dropdown gate, and menuBarMenuIndex and
	// menuBarItemIndex are where the highlight sits inside it. Like the readout
	// above, this is the whole of the sub-state: the menus and their entries are
	// resolved off the live catalog every frame, so the two indices are the only
	// thing that could go stale and both are re-resolved against that resolution
	// before they are read.
	menuBarOpen      bool
	menuBarMenuIndex int
	menuBarItemIndex int

	// OpenAtTask, when set, positions the cursor on the given task the first
	// time rebuildRows produces a non-empty row list. Cleared after first use.
	OpenAtTask *models.Task

	// Cross-project mode fields (used when m.project == nil)
	loadedProjects       []*models.Project // Successfully loaded projects
	crossSnapshotReady   bool              // True after the first cross-project snapshot has been accepted
	crossLoading         bool              // True while cross-project load is in progress
	crossSpinnerVisible  bool              // True once the 200ms delay has elapsed during an initial load
	crossLoadGen         int               // Generation counter; discards stale load results
	crossSpinner         SpinnerModel      // View-local spinner, independent of global save spinner
	crossReloadProjectID string            // Project owning the cursor task captured before an async reload
	crossReloadTaskID    string            // Cursor task captured before an async reload; restored after rows rebuild
	crossReloadFromTop   bool              // One-shot scope-change guard that suppresses stale cursor restoration

	// crossReloadHeaderProjectID lands the cursor on one project sub-header after
	// the next reload. Parking a project deletes every row the cursor was on, and
	// an arbitrary clamp there reads as a crash rather than as a hidden project.
	crossReloadHeaderProjectID string

	// Memoization guard for crossProjectRebuildRows
	crossLastRowsFingerprint string
	crossLastFocusDepth      int
	crossLastTaskCount       int
	crossForceRebuild        bool // When true, bypasses the memoization guard on next rebuild

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
	operationRegistry *OperationRegistry

	// workspacePulse mirrors AppModel.workspacePulse for the title-row
	// operational counter.
	workspacePulse WorkspacePulse
}

// setWorkspacePulse injects the latest cross-project operational counts.
func (m *OutlinerModel) setWorkspacePulse(pulse WorkspacePulse) { m.workspacePulse = pulse }

func (m *OutlinerModel) resetViewScrollOffset() {
	m.viewScrollOffset = 0
}

// SetSize updates the view dimensions and resets the persisted scroll window
// because the visible row budget may have changed.
func (m *OutlinerModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
	m.resetViewScrollOffset()
}

func (m *OutlinerModel) getCursorTask() *models.Task {
	if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) {
		row := m.rows[m.cursor]
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
		projectService:     NewProjectService(),
		quickEdit:          NewQuickEditComponent(),
		project:            project,
		primaryMode:        ModeAll,
		expandedNodes:      make(map[TaskIdentity]bool),
		collapsedSections:  make(map[string]bool),
		expandedChecklists: make(map[TaskIdentity]bool),
		filter:             NewFilterState(),
		projectFilter:      newProjectFilterState(),
		sort:               NewSortState(),
		treeSort:           NewSortState(),
		MultiSelectState:   MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		columnConfig:       NewColumnConfig(nil, nil, DefaultOutlinerColumns),
	}
	m.InitSearchInput("Search tasks...", 100, 30)
	return m
}

// SetTaskItemCoordinator wires the retained application mutation seam.
func (m *OutlinerModel) SetTaskItemCoordinator(coordinator TaskItemCoordinator) {
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
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil

	case CommentAddedMsg:
		// Rebuild rows so cloned tasks from tree projections pick up the
		// in-place comment mutation on the original task.
		m.rebuildRows()
		return m, nil

	case ParentSelectedMsg:
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
	case ViewDidAppearMsg:
		return m, m.handleViewDidAppear()
	case crossProjectsLoadedMsg:
		return m, m.handleCrossProjectsLoaded(msg)
	case lensProjectsLoadedMsg:
		cmd, _ := m.handleLensProjectsLoaded(msg)
		return m, cmd
	case crossSpinnerDelayMsg:
		if m.crossLoading && !m.hasCrossProjectSnapshot() {
			m.crossSpinnerVisible = true
			m.crossSpinner = NewSpinner()
			return m, m.crossSpinner.Tick()
		}
		return m, nil
	case crossSpinnerTickMsg:
		if m.crossSpinnerVisible {
			var cmd tea.Cmd
			m.crossSpinner, cmd = m.crossSpinner.Update(msg.inner)
			return m, crossSpinnerTickFwd(cmd)
		}
		return m, nil
	}
	return m, nil
}
