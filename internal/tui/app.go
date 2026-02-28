package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/alerts"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/template"
)

// ViewType represents the different views in the TUI.
type ViewType int

const (
	ViewDashboard ViewType = iota
	ViewProjectList
	ViewProjectTree
	ViewProjectCreator
	ViewProjectEditor
	ViewTaskCreator
	ViewTaskEditor
	ViewSettings
	ViewHelp
	ViewDateInput
	ViewDependencySelector
	ViewDependencyViewer
	ViewFilter
	ViewTaskJumper
	ViewProjectPreview
	ViewDueDateOverview
	ViewTagManager
	ViewGlobalTagManager
	ViewTagFilterSelector
	ViewSectionFilterSelector
	ViewSectionPicker
	ViewSectionManager
	ViewPriorityPicker
	ViewContextPicker
	ViewCustomContextEditor
	ViewGlobalSearch
	ViewParentSelector
	ViewPriorityView
	ViewQuickCapture
	ViewBulkCapture
	ViewInboxProcessor
	ViewKanban
	ViewTagSelector
	ViewBulkTagSelector
	ViewUndoHistory
	ViewTaskComments
	ViewCommandPalette
	ViewBulkEdit
	ViewSaveSearchInput
	ViewTrashedProjects
	ViewProjectSelector
	ViewDailyReview
	ViewWorkflowRules
	ViewWorkflowRuleCreator
	ViewWorkflowRuleEditor
	ViewRecurrenceEditor
	ViewTextEditor
	ViewGlobalInboxProcessor
	ViewFocusView
	ViewProjectFocusView
	ViewBlockers
	ViewTaskAttachments
	ViewLinkedTaskCreator
	ViewLinkedTaskSelector
	ViewCrossProjectLinker
	ViewNotes
	ViewNoteTaskLinker
	ViewNoteTagSelector
	ViewNotesSearch
	ViewTaskActivity
	ViewRecentActivity
	ViewRecentActivityPreview
	ViewGlobalNotes
	ViewWeeklyReview
	ViewGlobalBlockers
	ViewSnoozeInput
	ViewTutor
	ViewAsk
	ViewProjectPicker
	ViewTaskTemplates
	ViewTaskTemplateCreator
	ViewTaskTemplateEditor
	ViewTaskTemplatePicker
	ViewTaskChecklist
	ViewTaskDetail
	ViewScoreEditor
	ViewScoreProfileManager
	ViewSaveViewInput
	ViewMetricsDashboard
	ViewKanbanDisplaySettings
	ViewAllTasks
	ViewScoringProfilePicker
	ViewFocusBoard
	ViewOutliner
	ViewColumnPicker
	ViewSmartListBuilder
	ViewWorkflowDryRun
	ViewTemplateVarFiller
	ViewStatusInput
	ViewStatusIconPicker
	ViewNotificationCenter
)

// ProjectTreeSpatialState captures cursor position and fold state for a project tree
// so it can survive view recreation within the same TUI session.
type ProjectTreeSpatialState struct {
	CursorTaskID string
	FoldState    map[string]bool // collapsed node IDs (Expanded == false)
}

// KanbanSpatialState captures cursor position, active column, and fold state
// for a kanban view so it can survive view recreation within the same TUI session.
type KanbanSpatialState struct {
	ActiveColumn int
	CursorTaskID string
	ExpandState  map[string]bool
}

// AppModel is the main TUI model with a view stack.
type AppModel struct {
	config        *config.Config
	configManager *config.Manager

	// Persister for project persistence operations (enables testing with mocks)
	persister Persister

	// View stack for navigation
	viewStack []ViewType
	width     int
	height    int

	// View registry for centralized view management
	views *ViewRegistry

	// Command registry for centralized command lookup (used by command palette)
	commandRegistry *CommandRegistry

	// Typed accessors for views that need direct access (e.g., for HasActiveSearch)
	dashboard            *DashboardModel
	projectTree          *ProjectTreeModel
	helpView             *HelpViewModel
	taskJumper           *TaskJumperModel
	globalSearch         *GlobalSearchModel
	inboxProcessor       *InboxProcessorModel
	globalInboxProcessor *GlobalInboxProcessorModel
	kanban               *KanbanViewModel
	metricsDashboard     *MetricsDashboardModel
	allTasksView         *AllTasksViewModel
	outliner             *OutlinerModel

	// Per-project outliner toggle state (section collapse + node fold),
	// keyed by project ID. Survives outliner recreation within a session.
	outlinerToggleStates map[string]*OutlinerToggleState

	// Per-project spatial state caches (cursor + fold), keyed by project ID.
	// Survive view recreation within a session (in-memory only for cursor).
	projectTreeSpatialStates map[string]*ProjectTreeSpatialState
	kanbanSpatialStates      map[string]*KanbanSpatialState

	// Focus view session state (persists across view navigation within a session)
	focusQueue        []FocusTask
	focusQueueIDs     map[string]bool
	focusDismissedIDs map[string]bool

	// Current project context
	currentProject *models.Project

	// Project stack tracks project IDs for stacked ViewProjectTree entries.
	// When the project picker pushes a new project tree, the previous project ID
	// is saved so it can be restored when navigating back.
	projectStack []string

	// State
	err           error
	quitting      bool
	statusMessage string // Temporary status message (auto-clears after timeout)

	// Loading state for async operations
	isLoading      bool
	loadingMessage string
	spinner        SpinnerModel

	// Pending bulk tasks for palette operations (priority, date assignment)
	pendingBulkTasks []PaletteBulkTaskItem

	// Pending bulk section assignment from multi-select
	pendingBulkSectionTasks   []*models.Task
	pendingBulkSectionProject *models.Project

	// Pending bulk notes for multi-select operations
	pendingBulkNotes []*models.Note

	// Pending global note move operation
	pendingGlobalNoteMove *pendingNoteMove

	// Pending task creation parent (for template picker flow)
	pendingCreateTaskParent *models.Task

	// Pending template for variable filler (between picker and task creator)
	pendingVarFillerTemplate *template.TaskTemplate

	// Pending description edit context (for text editor popup flow)
	pendingDescEdit *struct {
		Task    *models.Task
		Project *models.Project
	}

	// Pending sandbox reset confirmation
	pendingSandboxReset     bool
	pendingSandboxTaskCount int

	// CLI reference for AI assistant context
	cliReference string

	// Navigation history for back/forward (Alt+O / Alt+I)
	navHistory          *NavHistory
	lastNavigatedTaskID string

	// Recent task visits for quick-switch (Alt+R)
	recentTaskVisits  []RecentTaskVisitEntry
	showQuickSwitch   bool
	quickSwitchCursor int

	// Pinned tasks popup state
	showPinnedPopup   bool
	pinnedPopupCursor int
	pinnedPopupItems  []pinnedTaskInfo

	// Project cycle popup state
	showProjectCycle   bool
	projectCycleCursor int
	projectCycleItems  []ProjectInfo

	// Last project-level view used (ProjectTree, Kanban, Outliner) for cycle preservation
	lastProjectView ViewType

	// Per-project last view memory: remembers which project-level view was active
	// for each project so switching back restores the same view.
	projectLastView map[string]ViewType

	pendingSemicolonPrefix bool

	// Proactive alert snapshot (ephemeral, regenerated on navigation)
	proactiveAlerts *alerts.AlertSnapshot
}

// pendingNoteMove holds state for a global note move operation.
type pendingNoteMove struct {
	Note      *models.Note
	SourceDir string
}

// AppOption is a functional option for configuring AppModel.
type AppOption func(*AppModel)

// WithPersister sets a custom Persister for the AppModel.
// This is primarily used for testing with mock implementations.
func WithPersister(p Persister) AppOption {
	return func(app *AppModel) {
		app.persister = p
	}
}

// WithCLIReference sets the CLI reference text for AI assistant context.
func WithCLIReference(ref string) AppOption {
	return func(app *AppModel) {
		app.cliReference = ref
	}
}

// NewAppModel creates a new TUI application model.
func NewAppModel(cfg *config.Config, manager *config.Manager, opts ...AppOption) *AppModel {
	app := &AppModel{
		config:                   cfg,
		configManager:            manager,
		persister:                NewProjectService(),
		viewStack:                []ViewType{ViewDashboard},
		width:                    80,
		height:                   24,
		views:                    NewViewRegistry(),
		commandRegistry:          NewCommandRegistry(),
		spinner:                  NewSpinner(),
		navHistory:               NewNavHistory(),
		lastProjectView:          defaultProjectViewType(cfg),
		projectLastView:          make(map[string]ViewType),
		outlinerToggleStates:     make(map[string]*OutlinerToggleState),
		projectTreeSpatialStates: make(map[string]*ProjectTreeSpatialState),
		kanbanSpatialStates:      make(map[string]*KanbanSpatialState),
		focusQueue:               []FocusTask{},
		focusQueueIDs:            make(map[string]bool),
		focusDismissedIDs:        make(map[string]bool),
	}

	// Apply functional options
	for _, opt := range opts {
		opt(app)
	}

	// Hydrate focus queue from persisted config (only from active projects)
	activeProjects := cfg.GetActiveProjects()
	for _, item := range cfg.GetFocusQueue() {
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := app.persister.Load(p); err != nil {
				break
			}
			if task := p.GetTaskByID(item.TaskID); task != nil && !task.Status.IsComplete() {
				app.focusQueue = append(app.focusQueue, FocusTask{Task: task, Project: p})
				app.focusQueueIDs[task.ID] = true
			}
			break
		}
	}
	// Hydrate dismissed IDs
	for _, id := range cfg.GetFocusDismissedIDs() {
		app.focusDismissedIDs[id] = true
	}

	// Hydrate recent task visits from persisted config
	for _, item := range cfg.GetRecentTaskVisits() {
		for _, p := range activeProjects {
			if p.ID != item.ProjectID {
				continue
			}
			if err := app.persister.Load(p); err != nil {
				break
			}
			if task := p.GetTaskByID(item.TaskID); task != nil {
				app.recentTaskVisits = append(app.recentTaskVisits, RecentTaskVisitEntry{Task: task, Project: p})
			}
			break
		}
	}

	// Ensure sandbox project exists
	if cfg.EnsureSandboxProject() {
		_ = manager.Save(cfg)
	}

	// Initialize dashboard and register it
	app.dashboard = NewDashboardModel(cfg, manager)
	app.views.Register(ViewDashboard, app.dashboard)

	// Record initial dashboard view in navigation history
	app.navHistory.Record(NavEntry{ViewType: ViewDashboard})

	// Register global commands
	app.registerGlobalCommands()

	// Load current project if set
	if current := cfg.GetCurrentProject(); current != nil {
		app.currentProject = current
	}

	return app
}

// makePriorityHandler creates a command handler that sets task priority.
func (m *AppModel) makePriorityHandler(priority int) func() tea.Msg {
	return func() tea.Msg {
		if m.projectTree == nil {
			return nil
		}
		visible := m.projectTree.visibleNodes()
		if m.projectTree.multiSelectMode && len(m.projectTree.selectedTasks) > 0 {
			for id := range m.projectTree.selectedTasks {
				m.projectTree.project.MutateTask(id, func(t *models.Task) {
					t.Priority = priority
				})
			}
			m.projectTree.clearSelection()
			m.projectTree.buildTree()
			return nil
		}
		if CursorInBounds(m.projectTree.cursor, len(visible)) {
			task := visible[m.projectTree.cursor].Task
			m.projectTree.project.MutateTask(task.ID, func(t *models.Task) {
				t.Priority = priority
			})
			return TaskPropertyChangedMsg{Task: task}
		}
		return nil
	}
}

// registerGlobalCommands registers commands that are available from any view.

func (m *AppModel) registerGlobalCommands() {
	_ = m.commandRegistry.RegisterBatch(m.globalCommands())
	_ = m.commandRegistry.RegisterBatch(m.filterCommands())
	_ = m.commandRegistry.RegisterBatch(m.viewCommands())
	_ = m.commandRegistry.RegisterBatch(m.navCommands())
	_ = m.commandRegistry.RegisterBatch(m.taskCommands())
	_ = m.commandRegistry.RegisterBatch(m.projectCommands())
	_ = m.commandRegistry.RegisterBatch(m.taskActionCommands())
	_ = m.commandRegistry.RegisterBatch(m.viewModeCommands())
	_ = m.commandRegistry.RegisterBatch(m.statusCommands())
	_ = m.commandRegistry.RegisterBatch(m.dueDateCommands())
	_ = m.commandRegistry.RegisterBatch(m.notesCommands())
	_ = m.commandRegistry.RegisterBatch(m.otherCommands())
	_ = m.commandRegistry.RegisterBatch(m.priorityCommands())
	_ = m.commandRegistry.RegisterBatch(m.sortCommands())
	_ = m.commandRegistry.RegisterBatch(m.structureCommands())
	_ = m.commandRegistry.RegisterBatch(m.sectionSendCommands())
	_ = m.commandRegistry.RegisterBatch(m.linkedTaskCommands())
	_ = m.commandRegistry.RegisterBatch(m.multiSelectCommands())
	_ = m.commandRegistry.RegisterBatch(m.foldCommands())
	_ = m.commandRegistry.RegisterBatch(m.filterBarCommands())
	_ = m.commandRegistry.RegisterBatch(m.notesViewCommands())
	_ = m.commandRegistry.RegisterBatch(m.kanbanCommands())
	_ = m.commandRegistry.RegisterBatch(m.kanbanExtCommands())
	_ = m.commandRegistry.RegisterBatch(m.blockersCommands())
	_ = m.commandRegistry.RegisterBatch(m.taskCommentsCommands())
	_ = m.commandRegistry.RegisterBatch(m.undoHistoryCommands())
	_ = m.commandRegistry.RegisterBatch(m.globalTagManagerCommands())
	_ = m.commandRegistry.RegisterBatch(m.attachmentCommands())
	_ = m.commandRegistry.RegisterBatch(m.focusViewCommands())
	_ = m.commandRegistry.RegisterBatch(m.dailyReviewCommands())
	_ = m.commandRegistry.RegisterBatch(m.priorityViewCommands())
	_ = m.commandRegistry.RegisterBatch(m.inboxProcessorCommands())
	_ = m.commandRegistry.RegisterBatch(m.dueDateOverviewCommands())
	_ = m.commandRegistry.RegisterBatch(m.workflowRulesCommands())
	_ = m.commandRegistry.RegisterBatch(m.sidebarCommands())
	_ = m.commandRegistry.RegisterBatch(m.scoringCommands())
	_ = m.commandRegistry.RegisterBatch(m.allTasksCommands())
	_ = m.commandRegistry.RegisterBatch(m.outlinerCommands())
}

// getGlobalHelpItems returns help items that apply across all views.
func getGlobalHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "?", Description: "Show help", Category: "General"},
		{Key: "q", Description: "Quit / go back", Category: "General"},
		{Key: "esc", Description: "Go back / cancel", Category: "General"},
		{Key: "ctrl+c", Description: "Quit application", Category: "General"},
		{Key: "/", Description: "Global search", Category: "Navigation"},
		{Key: ":", Description: "Command palette", Category: "Navigation"},
		{Key: "j/k", Description: "Move cursor down/up", Category: "Navigation"},
		{Key: "↑/↓", Description: "Move cursor up/down", Category: "Navigation"},
		{Key: "g/G", Description: "Go to first/last item", Category: "Navigation"},
		{Key: "Enter", Description: "Select / confirm", Category: "Actions"},
		{Key: ",", Description: "Settings", Category: "Actions"},
		{Key: ";n", Description: "Notification Center", Category: "Navigation"},
		{Key: ";s", Description: "Settings", Category: "Navigation"},
		{Key: ";d", Description: "Daily Review", Category: "Navigation"},
		{Key: ";w", Description: "Weekly Review", Category: "Navigation"},
		{Key: "alt+o", Description: "History back", Category: "Navigation"},
		{Key: "alt+i", Description: "History forward", Category: "Navigation"},
		{Key: "|", Description: "Recent tasks", Category: "Navigation"},
	}
}

func (m *AppModel) Init() tea.Cmd {
	return nil
}

// reevaluateAlerts loads all active projects and runs the alert engine.
func (m *AppModel) reevaluateAlerts() {
	cfg := m.buildAlertConfig()
	activeProjects := m.config.GetActiveProjects()

	var projectData []alerts.ProjectData
	for _, p := range activeProjects {
		if err := storage.LoadProject(p); err != nil {
			continue
		}
		projectData = append(projectData, alerts.ProjectData{
			Project: p,
			Tasks:   p.Tasks,
		})
	}

	m.proactiveAlerts = alerts.Evaluate(cfg, projectData)
}

// buildAlertConfig creates an AlertConfig from the current config state.
func (m *AppModel) buildAlertConfig() alerts.AlertConfig {
	return alerts.AlertConfig{
		DailyReviewHour: m.config.GetAlertDailyReviewHour(),
		WeeklyReviewDay: m.config.GetAlertWeeklyReviewDay(),

		LastDailyReviewAt:  m.config.GetLastDailyReviewAt(),
		LastWeeklyReviewAt: m.config.GetLastWeeklyReviewAt(),

		DailyReviewEnabled:   m.config.GetAlertDailyReviewEnabled(),
		WeeklyReviewEnabled:  m.config.GetAlertWeeklyReviewEnabled(),
		OverdueEnabled:       m.config.GetAlertOverdueEnabled(),
		DueSoonEnabled:       m.config.GetAlertDueSoonEnabled(),
		StaleEnabled:         m.config.GetAlertStaleEnabled(),
		InboxEnabled:         m.config.GetAlertInboxEnabled(),
		BlockedEnabled:       m.config.GetAlertBlockedEnabled(),
		ProjectHealthEnabled: m.config.GetAlertProjectHealthEnabled(),

		DueSoonDays:      m.config.GetAlertDueSoonDays(),
		StaleDays:        m.config.GetAlertStaleDays(),
		InboxThreshold:   m.config.GetAlertInboxThreshold(),
		BlockedDays:      m.config.GetAlertBlockedDays(),
		ProjectStaleDays: m.config.GetAlertProjectStaleDays(),
	}
}

// alertCount returns the current proactive alert count, or 0 if not evaluated.
func (m *AppModel) alertCount() int {
	if m.proactiveAlerts == nil {
		return 0
	}
	return m.proactiveAlerts.TotalCount
}

// GetCommandRegistry returns the command registry for the application.
// This is used by the command palette and for testing.
func (m *AppModel) GetCommandRegistry() *CommandRegistry {
	return m.commandRegistry
}

// isNotesFullscreen returns true when the notes view is in fullscreen pane mode.
func (m *AppModel) isNotesFullscreen() bool {
	cv := m.currentView()
	if cv != ViewNotes && cv != ViewGlobalNotes {
		return false
	}
	if nv := m.getActiveNotesView(); nv != nil {
		return nv.fullscreenPane
	}
	return false
}

// getActiveNotesView returns the currently active NotesViewModel (project or global).
func (m *AppModel) getActiveNotesView() *NotesViewModel {
	cv := m.currentView()
	var viewType ViewType
	switch cv {
	case ViewNotes:
		viewType = ViewNotes
	case ViewGlobalNotes:
		viewType = ViewGlobalNotes
	default:
		// Check if either is on the stack
		for i := len(m.viewStack) - 1; i >= 0; i-- {
			if m.viewStack[i] == ViewNotes || m.viewStack[i] == ViewGlobalNotes {
				viewType = m.viewStack[i]
				break
			}
		}
		if viewType == 0 {
			return nil
		}
	}
	if v := m.views.Get(viewType); v != nil {
		if nv, ok := v.(*NotesViewModel); ok {
			return nv
		}
	}
	return nil
}

// currentView returns the current view type.
func (m *AppModel) currentView() ViewType {
	if len(m.viewStack) == 0 {
		return ViewDashboard
	}
	return m.viewStack[len(m.viewStack)-1]
}

// baseProjectView walks the view stack from the top to find the underlying
// project-level view (ProjectTree, Kanban, or Outliner), skipping overlays
// like the command palette. This is needed by command handlers that run inside
// the palette's Cmd — at that point currentView() returns ViewCommandPalette,
// but the handler needs to know which project view it was invoked from.
func (m *AppModel) baseProjectView() ViewType {
	for i := len(m.viewStack) - 1; i >= 0; i-- {
		switch m.viewStack[i] {
		case ViewProjectTree, ViewKanban, ViewOutliner:
			return m.viewStack[i]
		}
	}
	return ViewDashboard
}

// pushView pushes a new view onto the stack and records it in navigation history
// if it is a tracked view and history is not locked.
func (m *AppModel) pushView(view ViewType) {
	m.viewStack = append(m.viewStack, view)
	if view == ViewProjectTree || view == ViewKanban || view == ViewOutliner {
		m.lastProjectView = view
	}
	if !m.navHistory.IsLocked() && isTrackedView(view) {
		entry := NavEntry{ViewType: view}
		if m.currentProject != nil {
			entry.ProjectID = m.currentProject.ID
		}
		if isTaskScopedView(view) {
			entry.TaskID = m.lastNavigatedTaskID
		}
		m.navHistory.Record(entry)
	}
}

// popView removes the top view from the stack.
func (m *AppModel) popView() {
	if len(m.viewStack) > 1 {
		m.viewStack = m.viewStack[:len(m.viewStack)-1]
	}
	// Re-evaluate alerts when returning to dashboard
	if m.currentView() == ViewDashboard {
		m.reevaluateAlerts()
	}
}

// refreshProjectTree recreates the project tree view with current project.
// Preserves filter/sort state across the refresh.
func (m *AppModel) refreshProjectTree() {
	if m.currentProject == nil {
		return
	}
	// Save focus state (transient, not persisted to config)
	var focusedTaskID string
	if m.projectTree != nil && m.projectTree.focusedTask != nil {
		focusedTaskID = m.projectTree.focusedTask.ID
	}

	m.saveProjectTreeState()

	// Rebuild task tree to ensure parent-child relationships are current
	m.currentProject.BuildTaskTree()

	m.projectTree = NewProjectTreeModel(m.currentProject, m.config, m.configManager)
	m.projectTree.SetSize(m.width, m.height-2)
	m.views.Register(ViewProjectTree, m.projectTree)
	m.restoreProjectTreeState()

	// Restore focus state if the task still exists
	if focusedTaskID != "" {
		if task := m.currentProject.GetTaskByID(focusedTaskID); task != nil {
			m.projectTree.focusedTask = task
		}
	}

	// Rebuild tree after restoring state to ensure nodes, visibility, and
	// filter counts reflect the restored filter/sort settings
	m.projectTree.buildTree()
}

// showView is a helper that registers a view, sets its size, and pushes it onto the view stack.
// This consolidates the common pattern used by Show*Msg handlers.
func (m *AppModel) showView(viewType ViewType, view View) {
	if caps, ok := view.(ViewCapabilities); ok && caps.IsFullHeight() {
		view.SetSize(m.width, m.height)
	} else {
		view.SetSize(m.width, m.height-2)
	}
	m.views.Register(viewType, view)
	m.pushView(viewType)
}

// replaceView replaces the top of the view stack instead of pushing.
// Used when switching between peer project views (ProjectTree, Kanban, Outliner)
// so the stack doesn't accumulate entries.
func (m *AppModel) replaceView(viewType ViewType, view View) {
	if caps, ok := view.(ViewCapabilities); ok && caps.IsFullHeight() {
		view.SetSize(m.width, m.height)
	} else {
		view.SetSize(m.width, m.height-2)
	}
	m.views.Register(viewType, view)
	if len(m.viewStack) > 1 {
		m.viewStack[len(m.viewStack)-1] = viewType
	} else {
		m.viewStack = append(m.viewStack, viewType)
	}
	if viewType == ViewProjectTree || viewType == ViewKanban || viewType == ViewOutliner {
		m.lastProjectView = viewType
	}
	if !m.navHistory.IsLocked() && isTrackedView(viewType) {
		entry := NavEntry{ViewType: viewType}
		if m.currentProject != nil {
			entry.ProjectID = m.currentProject.ID
		}
		if isTaskScopedView(viewType) {
			entry.TaskID = m.lastNavigatedTaskID
		}
		m.navHistory.Record(entry)
	}
}

// isOnProjectView returns true if the current view is a peer project view.
func (m *AppModel) isOnProjectView() bool {
	cur := m.currentView()
	return cur == ViewProjectTree || cur == ViewKanban || cur == ViewOutliner
}

// saveProjectTreeState persists the current project tree filter/sort state to config.
func (m *AppModel) saveProjectTreeState() {
	if m.projectTree == nil || m.currentProject == nil {
		return
	}

	var filterTags []string
	for tag := range m.projectTree.filter.Tags {
		filterTags = append(filterTags, tag)
	}
	var filterSections []string
	for section := range m.projectTree.filter.Sections {
		filterSections = append(filterSections, section)
	}

	// Capture fold state (only collapsed nodes) and cursor task ID
	foldState := make(map[string]bool)
	for _, node := range m.projectTree.nodes {
		if !node.Expanded {
			foldState[node.Task.ID] = true
		}
	}
	var cursorTaskID string
	if task := m.projectTree.getCursorTask(); task != nil {
		cursorTaskID = task.ID
	}
	m.projectTreeSpatialStates[m.currentProject.ID] = &ProjectTreeSpatialState{
		CursorTaskID: cursorTaskID,
		FoldState:    foldState,
	}

	state := config.ProjectViewState{
		FilterType:        string(m.projectTree.filter.Type),
		FilterStatus:      string(m.projectTree.filter.Status),
		FilterPriority:    m.projectTree.filter.Priority,
		FilterTags:        filterTags,
		FilterSections:    filterSections,
		FilterDueDate:     int(m.projectTree.filter.DueDate),
		FilterActionable:  m.projectTree.filter.Actionable,
		SortField:         int(m.projectTree.sort.Field),
		SortAscending:     m.projectTree.sort.Ascending,
		SidebarEnabled:    m.projectTree.sidebar.IsEnabled(),
		FilterBarEnabled:  m.projectTree.filterBar.IsEnabled(),
		ViewMode:          int(m.projectTree.viewMode),
		CustomSectionTab:  m.projectTree.activeCustomSectionFilter(),
		TreeColumnToggles: m.projectTree.columnConfig.Toggles(),
		TreeFoldState:     foldState,
	}

	// Preserve kanban/outliner column toggles and fold states from existing state
	if existing, ok := m.config.ProjectViewStates[m.currentProject.ID]; ok {
		state.KanbanColumnToggles = existing.KanbanColumnToggles
		state.OutlinerColumnToggles = existing.OutlinerColumnToggles
		state.KanbanFoldState = existing.KanbanFoldState
		state.OutlinerFoldState = existing.OutlinerFoldState
	}

	m.config.SetProjectViewState(m.currentProject.ID, state)
	_ = m.configManager.Save(m.config)
}

// switchToProject handles switching to a different project, loading it and creating the tree view.
// It saves current state, loads the project from storage, applies colors, and creates the tree.
// Pass restoreState=false to skip restoring saved filter/sort state (e.g., when applying custom filters).
func (m *AppModel) switchToProject(project *models.Project, restoreState bool) {
	m.saveProjectTreeState()
	m.currentProject = project
	_ = m.persister.Load(project)
	m.config.ApplyGlobalColorsToProject(project)
	m.projectTree = NewProjectTreeModel(project, m.config, m.configManager)
	m.projectTree.SetSize(m.width, m.height-2)
	if restoreState {
		m.restoreProjectTreeState()
	}
	m.views.Register(ViewProjectTree, m.projectTree)
}

// restoreProjectTreeState applies saved filter/sort state to the current project tree.
func (m *AppModel) restoreProjectTreeState() {
	if m.projectTree == nil || m.currentProject == nil {
		return
	}
	state, exists := m.config.GetProjectViewState(m.currentProject.ID)
	if !exists {
		return
	}

	m.projectTree.filter.Type = models.TaskType(state.FilterType)
	m.projectTree.filter.Status = models.TaskStatus(state.FilterStatus)
	m.projectTree.filter.Priority = state.FilterPriority
	m.projectTree.filter.DueDate = DueDateFilter(state.FilterDueDate)
	m.projectTree.filter.Actionable = state.FilterActionable
	m.projectTree.sort.Field = SortField(state.SortField)
	m.projectTree.sort.Ascending = state.SortAscending
	m.projectTree.sidebar.SetEnabled(state.SidebarEnabled)
	m.projectTree.filterBar.SetEnabled(state.FilterBarEnabled)
	m.projectTree.viewMode = ViewMode(state.ViewMode)

	// Restore tags - validate they still exist
	m.projectTree.filter.Tags = validateSavedStringSet(state.FilterTags, m.projectTree.getAllTags())

	// Restore sections - validate they still exist
	m.projectTree.filter.Sections = validateSavedStringSet(state.FilterSections, m.currentProject.GetSections())

	// Restore custom section tab - validate it still exists
	if state.CustomSectionTab != "" {
		if tabs := m.projectTree.customSectionTabs(); tabs != nil {
			for i, tab := range tabs {
				if tab == state.CustomSectionTab {
					m.projectTree.customSectionTab = i
					break
				}
			}
		}
	}

	// Restore fold state: prefer session cache, fall back to disk
	var foldState map[string]bool
	if spatial, ok := m.projectTreeSpatialStates[m.currentProject.ID]; ok && spatial.FoldState != nil {
		foldState = spatial.FoldState
	} else if state.TreeFoldState != nil {
		foldState = state.TreeFoldState
	}
	if foldState != nil {
		for _, node := range m.projectTree.nodes {
			if foldState[node.Task.ID] {
				node.Expanded = false
			}
		}
	}

	m.projectTree.updateVisibility()
	m.projectTree.clampCursor()

	// Restore cursor position from session cache
	if spatial, ok := m.projectTreeSpatialStates[m.currentProject.ID]; ok && spatial.CursorTaskID != "" {
		visible := m.projectTree.visibleNodes()
		for i, node := range visible {
			if node.Task.ID == spatial.CursorTaskID {
				m.projectTree.cursor = i
				break
			}
		}
	}
}

// saveKanbanState persists the current kanban filter/tab state to config.
func (m *AppModel) saveKanbanState() {
	if m.kanban == nil || m.currentProject == nil {
		return
	}

	var filterTags []string
	for tag := range m.kanban.filter.Tags {
		filterTags = append(filterTags, tag)
	}

	// Capture cursor task ID and expand state for session cache
	var cursorTaskID string
	if task := m.kanban.getCursorTask(); task != nil {
		cursorTaskID = task.ID
	}
	m.kanbanSpatialStates[m.currentProject.ID] = &KanbanSpatialState{
		ActiveColumn: m.kanban.ActiveColumn,
		CursorTaskID: cursorTaskID,
		ExpandState:  copyBoolMap(m.kanban.ExpandState),
	}

	// Build fold state for disk persistence (only collapsed nodes)
	collapsedNodes := make(map[string]bool)
	for id, expanded := range m.kanban.ExpandState {
		if !expanded {
			collapsedNodes[id] = true
		}
	}

	state := m.config.ProjectViewStates[m.currentProject.ID]
	state.KanbanFilterType = string(m.kanban.filter.Type)
	state.KanbanFilterStatus = string(m.kanban.filter.Status)
	state.KanbanFilterPriority = m.kanban.filter.Priority
	state.KanbanFilterTags = filterTags
	state.KanbanFilterDueDate = int(m.kanban.filter.DueDate)
	state.KanbanFilterScoreMin = m.kanban.filter.ScoreMin
	state.KanbanActiveTab = m.kanban.tabs.Active
	state.KanbanColumnToggles = m.kanban.columnConfig.Toggles()
	if len(collapsedNodes) > 0 {
		state.KanbanFoldState = collapsedNodes
	} else {
		state.KanbanFoldState = nil
	}

	if len(m.kanban.filter.Attrs) > 0 {
		state.KanbanFilterAttrs = make(map[string]string)
		for k, v := range m.kanban.filter.Attrs {
			state.KanbanFilterAttrs[k] = v
		}
	} else {
		state.KanbanFilterAttrs = nil
	}

	m.config.ProjectViewStates[m.currentProject.ID] = state
	_ = m.configManager.Save(m.config)
}

// restoreKanbanState applies saved kanban filter state from config.
func (m *AppModel) restoreKanbanState() {
	if m.kanban == nil || m.currentProject == nil {
		return
	}
	state, exists := m.config.GetProjectViewState(m.currentProject.ID)
	if !exists {
		return
	}

	m.kanban.filter.Type = models.TaskType(state.KanbanFilterType)
	m.kanban.filter.Status = models.TaskStatus(state.KanbanFilterStatus)
	m.kanban.filter.Priority = state.KanbanFilterPriority
	m.kanban.filter.DueDate = DueDateFilter(state.KanbanFilterDueDate)
	m.kanban.filter.ScoreMin = state.KanbanFilterScoreMin
	m.kanban.tabs.Active = state.KanbanActiveTab

	// Restore attribute filters
	if len(state.KanbanFilterAttrs) > 0 {
		m.kanban.filter.Attrs = make(map[string]string)
		for k, v := range state.KanbanFilterAttrs {
			m.kanban.filter.Attrs[k] = v
		}
	}

	// Restore tags - validate they still exist in the project
	m.kanban.filter.Tags = validateSavedStringSet(state.KanbanFilterTags, CollectProjectTagNames(m.kanban.Project.Tasks))

	// Restore expand state: prefer session cache, fall back to disk
	if spatial, ok := m.kanbanSpatialStates[m.currentProject.ID]; ok && spatial.ExpandState != nil {
		m.kanban.ExpandState = copyBoolMap(spatial.ExpandState)
	} else if len(state.KanbanFoldState) > 0 {
		for id := range state.KanbanFoldState {
			m.kanban.ExpandState[id] = false
		}
	}

	// Reload tasks with restored filters and expand state applied
	m.kanban.loadTasks()

	// Restore active column and cursor position from session cache
	if spatial, ok := m.kanbanSpatialStates[m.currentProject.ID]; ok {
		if spatial.ActiveColumn >= 0 && spatial.ActiveColumn < len(m.kanban.columnDefs) {
			m.kanban.ActiveColumn = spatial.ActiveColumn
			m.kanban.ensureActiveColumnVisible()
		}
		if spatial.CursorTaskID != "" {
			visibleNodes := m.kanban.currentVisibleNodes()
			cs := m.kanban.CurrentColumnState()
			for i, node := range visibleNodes {
				if node.Task.ID == spatial.CursorTaskID {
					cs.Cursor = i
					break
				}
			}
		}
	}
}

// defaultProjectViewType maps the config's default_project_view string to a ViewType.
func defaultProjectViewType(cfg *config.Config) ViewType {
	switch cfg.GetDefaultProjectView() {
	case "project_tree":
		return ViewProjectTree
	case "kanban":
		return ViewKanban
	default:
		return ViewOutliner
	}
}

// saveCurrentProjectView records the current project-level view for the active project.
func (m *AppModel) saveCurrentProjectView() {
	if m.currentProject == nil {
		return
	}
	cv := m.currentView()
	if cv == ViewProjectTree || cv == ViewKanban || cv == ViewOutliner {
		m.projectLastView[m.currentProject.ID] = cv
	}
}

// getProjectLastView returns the last project-level view for the given project ID,
// falling back to the configured default project view.
func (m *AppModel) getProjectLastView(projectID string) ViewType {
	if v, ok := m.projectLastView[projectID]; ok {
		return v
	}
	return defaultProjectViewType(m.config)
}

// navigateToTaskInProject switches to the given project using the user's
// configured default view (or last-used view) and positions the cursor on
// the specified task. When task is nil it just opens the project view.
func (m *AppModel) navigateToTaskInProject(task *models.Task, project *models.Project) {
	m.saveCurrentProjectView()
	m.switchToProject(project, true)

	// Re-resolve task in the freshly loaded project so parent pointers are valid.
	if task != nil {
		if resolved := project.GetTaskByID(task.ID); resolved != nil {
			task = resolved
		}
	}

	targetView := m.getProjectLastView(project.ID)
	switch targetView {
	case ViewKanban:
		m.kanban = NewKanbanViewModel(project, m.config, m.configManager)
		m.showView(ViewKanban, m.kanban)
		m.restoreKanbanState()
	case ViewProjectTree:
		if task != nil {
			m.projectTree.NavigateToTask(task)
		}
		m.showView(ViewProjectTree, m.projectTree)
	default:
		m.saveOutlinerToggleState()
		m.saveOutlinerFilterState()
		m.outliner = NewOutlinerModel(project, m.config, m.configManager)
		m.restoreOutlinerToggleState()
		m.restoreOutlinerFilterState()
		if task != nil {
			m.outliner.NavigateToSidebarTask(task)
		}
		m.showView(ViewOutliner, m.outliner)
	}
}

// errorCmd creates a tea.Cmd that returns an ErrorMsg with a formatted error.
func errorCmd(format string, args ...any) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Err: fmt.Errorf(format, args...)}
	}
}
