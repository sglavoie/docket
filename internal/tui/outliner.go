package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ZoomEntry records the state at a single zoom level so cursor position can
// be restored when zooming back out.
type ZoomEntry struct {
	TaskID      string // UUID of the task that was zoomed into
	CursorIndex int    // Cursor position at the moment of zoom-in
}

// CrossProjectZoomEntry records the state at a single zoom level for the
// cross-project outliner. It extends ZoomEntry with a ProjectID field to
// scope zoom resolution to the correct project, preventing UUID collisions
// across projects from causing incorrect zoom resolution.
type CrossProjectZoomEntry struct {
	TaskID      string // UUID of the task that was zoomed into
	ProjectID   string // ID of the project owning the task
	CursorIndex int    // Cursor position at the moment of zoom-in
}

// OutlinerModel is the outliner view for navigating tasks in a hierarchical,
// zoomable tree. It is the sole per-project primary view and implements the
// View interface as a standalone struct, owning its own navigation semantics,
// sort modes, and zoom-stack behavior.
type OutlinerModel struct {
	BaseSearchableView
	InteractiveViewState
	MultiSelectState

	runtime        *config.RuntimeState
	taskItem       TaskItemCoordinator
	projectService *ProjectService
	quickEdit      *QuickEditComponent

	// Daily-plan membership support for building Today rows.
	todayIDs map[TaskIdentity]bool

	project *models.Project

	zoomStack      []ZoomEntry             // Zoom stack for single-project mode
	crossZoomStack []CrossProjectZoomEntry // Zoom stack for cross-project mode (m.project == nil)
	zoomNotice     string                  // Transient message shown when zoom target is removed

	rows []outlinerRow // Flattened rows, rebuilt on each render
	// primaryMode is the authoritative canonical row-strategy identity.
	primaryMode OutlinerModeID

	sort     SortState // Active render sort; default while a fixed-order mode owns ordering.
	treeSort SortState // Durable user sort restored whenever a tree-order mode is active.

	filter FilterState // Unified filter state (common + sections + actionable)

	pinnedTagTab      map[string]int        // Active pinned tag tab index per projectID (0 = All)
	expandedNodes     map[TaskIdentity]bool // Project-qualified fold state: true=expanded (default all expanded)
	collapsedSections map[string]bool       // Section collapse state for All mode
	// sectionToggleRestore preserves a task focus across one S-driven
	// collapse/expand cycle. It is deliberately session-only.
	sectionToggleRestore *sectionToggleRestoreAnchor

	filterBar *FilterBarComponent // Filter bar component (dedicated line for filter/sort indicators)

	undo UndoContext // Action label and context for next undo snapshot

	// Column visibility config (loaded from project view state)
	columnConfig *ColumnConfig

	// Peek mode follows the cursor and shows additive task detail inline.
	peekMode bool

	// OpenAtTask, when set, positions the cursor on the given task the first
	// time rebuildRows produces a non-empty row list. Cleared after first use.
	OpenAtTask *models.Task

	// activeContext is the session-only Context scope narrowing the global
	// Outliner's project set. It is scope, not a filter: F x leaves it alone and
	// only the global reset returns it to All. Never persisted.
	activeContext ContextScope

	// contextScopeProjectIDs caches activeContext resolved against the catalog.
	// Nil under the All scope, which admits every project.
	contextScopeProjectIDs map[string]bool

	// Cross-project mode fields (used when m.project == nil)
	loadedProjects        []*models.Project // Successfully loaded projects
	workspaceResolver     *workspaceFilterResolver
	crossSnapshotReady    bool            // True after the first cross-project snapshot has been accepted
	crossLoading          bool            // True while cross-project load is in progress
	crossSpinnerVisible   bool            // True once the 200ms delay has elapsed during an initial load
	crossLoadGen          int             // Generation counter; discards stale load results
	crossSpinner          SpinnerModel    // View-local spinner, independent of global save spinner
	crossQuickEditProject *models.Project // Resolved project for quick edit in cross-project mode
	crossReloadTabName    string          // Pinned tag tab name preserved across an async snapshot replacement
	crossReloadTabSet     bool            // True when crossReloadTabName is an intentional live selection, including All
	crossReloadProjectID  string          // Project owning the cursor task captured before an async reload
	crossReloadTaskID     string          // Cursor task captured before an async reload; restored after rows rebuild
	crossReloadFromTop    bool            // One-shot scope-change guard that suppresses stale cursor restoration

	// Memoization guard for crossProjectRebuildRows
	crossLastRowsFingerprint string
	crossLastZoomDepth       int
	crossLastTaskCount       int
	crossForceRebuild        bool // When true, bypasses the memoization guard on next rebuild

	// scopedProjectID names the single project the global outliner is currently
	// scoped to. Empty means the authoritative all-projects scope. Only
	// meaningful in cross-project mode (m.project == nil); scoping filters the
	// loaded data set down to one project without switching to a separate
	// single-project OutlinerModel.
	scopedProjectID string

	// scopeStack records the state to restore when a project scope is popped.
	// Each push saves the previous scope's cursor, scroll, mode, multi-project
	// filter, and zoom so Esc returns exactly where the user left off. The
	// bottom of the stack is the base all-projects frame.
	scopeStack []outlinerScopeFrame

	// operationRegistry projects help, handled keys, and footer hints from the
	// same operation metadata used by shell dispatch.
	operationRegistry *OperationRegistry

	// openLoops mirrors AppModel.openLoops for the title-row Zeigarnik counter.
	openLoops OpenLoops
}

// setOpenLoops injects the latest cross-project open-loop counts into the view.
func (m *OutlinerModel) setOpenLoops(o OpenLoops) { m.openLoops = o }

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
	_ = cfgManager
	m := &OutlinerModel{
		runtime:           runtime,
		projectService:    NewProjectService(),
		quickEdit:         NewQuickEditComponent(),
		project:           project,
		primaryMode:       ModeAll,
		filterBar:         NewFilterBarComponent(),
		expandedNodes:     make(map[TaskIdentity]bool),
		collapsedSections: make(map[string]bool),
		pinnedTagTab:      make(map[string]int),
		filter:            NewFilterState(),
		sort:              NewSortState(),
		treeSort:          NewSortState(),
		MultiSelectState:  MultiSelectState{selectedTaskIdentities: make(map[TaskIdentity]bool)},
		columnConfig:      NewColumnConfig(nil, nil, DefaultOutlinerColumns, PriorityModeConditional),
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

	case MutationFlashClearMsg:
		m.mutationFlash = false
		m.rebuildRows()
		return m, nil

	case CompletionFlashFrameMsg:
		m.advanceCompletionFlashFrame(msg.Frame)
		m.rebuildRows()
		return m, nil

	case CommentAddedMsg:
		// Rebuild rows so cloned tasks from filterTaskTree() pick up the
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

	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	case outlinerRefreshMsg:
		m.rebuildRows()
		return m, nil
	case ViewDidAppearMsg:
		return m, m.handleViewDidAppear()
	case crossProjectsLoadedMsg:
		return m, m.handleCrossProjectsLoaded(msg)
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
