package tui

import (
	"fmt"
	"maps"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
// zoomable tree. It implements the View interface as a standalone struct
// (not extending ProjectTreeModel) to allow independent evolution of
// navigation semantics, sort modes, and zoom-stack behavior.
type OutlinerModel struct {
	BaseSearchableView
	InteractiveViewState
	PrimaryViewBundle
	MultiSelectState

	project *models.Project

	zoomStack      []ZoomEntry             // Zoom stack for single-project mode
	crossZoomStack []CrossProjectZoomEntry // Zoom stack for cross-project mode (m.project == nil)
	zoomNotice     string                  // Transient message shown when zoom target is removed

	rows              []outlinerRow // Flattened rows, rebuilt on each render
	activePerspective int           // 0-9 index into PerspectivePresets; defaults to "All" (key 1)

	sort SortState // Current sort field and direction

	filter           FilterState      // Unified filter state (common + sections + actionable)
	scoreFilterInput ScoreFilterInput // Inline score filter threshold input

	customSectionTab  map[string]int  // Active custom section tab index per projectID (0 = All)
	expandedNodes     map[string]bool // Fold state: true=expanded (default all expanded)
	collapsedSections map[string]bool // Section collapse state for All perspective

	sidebar   *SidebarComponent   // Sidebar detail panel
	filterBar *FilterBarComponent // Filter bar component (dedicated line for filter/sort indicators)

	deleteTarget *models.Task // Task to delete when confirmation is accepted
	err          error        // Last error from an operation

	undo UndoContext // Action label and context for next undo snapshot

	// Column visibility config (loaded from project view state)
	columnConfig *ColumnConfig

	activeSavedFilter string // Name of the currently loaded saved filter (if any)

	// Active criteria filter (set when a criteria-mode SavedFilter is loaded)
	activeCriteriaFilter *config.SavedFilter

	// Inline preview mode: show description/checklist below cursor task
	previewMode bool

	// OpenAtTask, when set, positions the cursor on the given task the first
	// time rebuildRows produces a non-empty row list. Cleared after first use.
	OpenAtTask *models.Task

	// Cross-project mode fields (used when m.project == nil)
	loadedProjects        []*models.Project // Successfully loaded projects
	crossLoading          bool              // True while cross-project load is in progress
	crossSpinnerVisible   bool              // True once the 200ms delay has elapsed during load
	crossLoadGen          int               // Generation counter; discards stale load results
	crossUndoCtx          UndoContext       // Undo context scoped to cross-project operations
	crossSpinner          SpinnerModel      // View-local spinner, independent of global save spinner
	crossQuickEditProject *models.Project   // Resolved project for quick edit in cross-project mode

	// Memoization guard for crossProjectRebuildRows
	crossLastPerspective int
	crossLastFilterStr   string
	crossLastZoomDepth   int
	crossLastTaskCount   int
	crossForceRebuild    bool // When true, bypasses the memoization guard on next rebuild
}

// primaryProjectView interface implementation for shared key dispatch.
func (m *OutlinerModel) getProject() *models.Project       { return m.project }
func (m *OutlinerModel) getConfig() *config.Config         { return m.config }
func (m *OutlinerModel) getConfigManager() *config.Manager { return m.configManager }
func (m *OutlinerModel) getCursorTask() *models.Task {
	if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) {
		row := m.rows[m.cursor]
		if !row.IsSectionHeader && !row.IsSeparator {
			return row.task
		}
	}
	return nil
}
func (m *OutlinerModel) reloadFromDisk() tea.Cmd {
	if m.project == nil {
		return m.reloadCrossProjects()
	}
	if err := storage.LoadProject(m.project); err != nil {
		logging.Warn("failed to load project %s: %v", m.project.Name, err)
		m.rebuildRows()
		return func() tea.Msg { return StatusMsg(fmt.Sprintf("Failed to reload project: %v", err)) }
	}
	m.rebuildRows()
	return nil
}

// NewOutlinerModel creates a new OutlinerModel for the given project.
func NewOutlinerModel(project *models.Project, cfg *config.Config, cfgManager *config.Manager) *OutlinerModel {
	// Load column toggles from config
	var outlinerToggles map[string]bool
	var outlinerPriorityMode string
	if cfg != nil && project != nil {
		if vs, ok := cfg.ProjectViewStates[project.ID]; ok {
			outlinerToggles = vs.OutlinerColumnToggles
			outlinerPriorityMode = vs.OutlinerPriorityMode
		}
	}
	if outlinerPriorityMode == "" && cfg != nil {
		outlinerPriorityMode = cfg.OutlinerPriorityMode
	}

	m := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(cfg, cfgManager),
		project:           project,
		activePerspective: 1,
		sidebar:           NewSidebarComponent(),
		filterBar:         NewFilterBarComponent(),
		expandedNodes:     make(map[string]bool),
		collapsedSections: make(map[string]bool),
		customSectionTab:  make(map[string]int),
		filter:            NewFilterState(),
		sort:              NewSortState(),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		columnConfig:      NewColumnConfig(outlinerToggles, cfg.OutlinerDefaultColumns, DefaultOutlinerColumns, outlinerPriorityMode),
	}
	// Seed default collapsed sections from config
	maps.Copy(m.collapsedSections, cfg.GetOutlinerCollapsedSections())
	m.InitSearchInput("Search tasks...", 100, 30)
	// Restore sidebar and filter bar state from config
	if project != nil {
		if state, exists := cfg.GetProjectViewState(project.ID); exists {
			m.sidebar.SetEnabled(state.SidebarEnabled)
			m.filterBar.SetEnabled(state.OutlinerFilterBarEnabled)
		}
	} else {
		// Cross-project outliner: use global setting (nil = true)
		if cfg.CrossOutlinerSidebarEnabled != nil {
			m.sidebar.SetEnabled(*cfg.CrossOutlinerSidebarEnabled)
		}
		// Restore persisted collapsed sections for cross-project outliner
		if len(cfg.CrossOutlinerCollapsedSections) > 0 {
			maps.Copy(m.collapsedSections, cfg.CrossOutlinerCollapsedSections)
		}
	}
	return m
}

// validateCrossZoomStack walks the cross-project zoom stack from top to bottom,
// popping entries whose task no longer exists in their owning project. It
// branches on CrossProjectZoomEntry.ProjectID to scope each lookup to the
// correct project, preventing UUID collisions across projects from causing
// incorrect resolution (per ac-d348683eb1a3e343887e804dbc6a719c).
func (m *OutlinerModel) validateCrossZoomStack() {
	if len(m.crossZoomStack) == 0 {
		return
	}

	taskExists := func(entry CrossProjectZoomEntry) bool {
		for _, proj := range m.loadedProjects {
			if proj.ID == entry.ProjectID {
				return proj.GetTaskByID(entry.TaskID) != nil
			}
		}
		return false
	}

	// Find the deepest valid entry (walking from the top).
	validDepth := -1
	for i := len(m.crossZoomStack) - 1; i >= 0; i-- {
		if taskExists(m.crossZoomStack[i]) {
			validDepth = i
			break
		}
	}

	if validDepth == len(m.crossZoomStack)-1 {
		return // Stack is fully valid, nothing to do.
	}

	if validDepth < 0 {
		// No valid ancestor — reset to root.
		m.crossZoomStack = m.crossZoomStack[:0]
		m.cursor = 0
		m.zoomNotice = "Zoom target was removed — returned to root"
	} else {
		// Pop to nearest valid ancestor, restore its cursor.
		m.cursor = m.crossZoomStack[validDepth].CursorIndex
		m.crossZoomStack = m.crossZoomStack[:validDepth+1]
		m.zoomNotice = "Zoom target was removed — returned to parent"
	}
}

// validateZoomStack walks the zoom stack from top to bottom, popping entries
// whose task no longer exists in the project. If any entries were removed, it
// sets a transient notice so the user knows their zoom target was deleted.
func (m *OutlinerModel) validateZoomStack() {
	if m.project == nil || len(m.zoomStack) == 0 {
		return
	}

	// Find the deepest valid entry (walking from the top).
	validDepth := -1
	for i := len(m.zoomStack) - 1; i >= 0; i-- {
		if m.project.GetTaskByID(m.zoomStack[i].TaskID) != nil {
			validDepth = i
			break
		}
	}

	if validDepth == len(m.zoomStack)-1 {
		return // Stack is fully valid, nothing to do.
	}

	if validDepth < 0 {
		// No valid ancestor — reset to root.
		m.zoomStack = m.zoomStack[:0]
		m.cursor = 0
		m.zoomNotice = "Zoom target was removed — returned to root"
	} else {
		// Pop to nearest valid ancestor, restore its cursor.
		m.cursor = m.zoomStack[validDepth].CursorIndex
		m.zoomStack = m.zoomStack[:validDepth+1]
		m.zoomNotice = "Zoom target was removed — returned to parent"
	}
}

// rebuildRows flattens the current task tree into the rows slice, applying the
// active perspective filter, and clamps the cursor to stay within bounds.
func (m *OutlinerModel) rebuildRows() {
	if m.project == nil {
		// Mark as a mutation-driven rebuild so the memoization guard in
		// crossProjectRebuildRows is bypassed. The render path calls
		// crossProjectRebuildRows directly (not via rebuildRows), so it
		// still benefits from the guard on no-op events like window resize.
		m.crossForceRebuild = true
		m.crossProjectRebuildRows()
		return
	}
	m.rows = m.rows[:0]

	// Ensure zoom stack entries still reference existing tasks.
	m.validateZoomStack()

	roots := m.project.GetRootTasks()

	// Scope to zoomed task's children when zoom stack is non-empty.
	if len(m.zoomStack) > 0 {
		topID := m.zoomStack[len(m.zoomStack)-1].TaskID
		if t := m.project.GetTaskByID(topID); t != nil {
			roots = t.Children
		}
	}

	p := m.currentPerspective()
	var tasks []*models.Task
	tasks = filterTaskTree(roots, p.Filter)

	// Apply custom section tab filter
	if csFilter := m.activeCustomSectionFilter(); csFilter != "" {
		tasks = filterTaskTree(tasks, func(t *models.Task) bool {
			return t.CustomSection == csFilter
		})
	}

	// Apply common filters (F-prefix)
	if hasActiveCommonFilters(&m.filter.CommonFilterState) {
		tasks = filterTaskTree(tasks, func(t *models.Task) bool {
			return passesCommonFilters(t, &m.filter.CommonFilterState, m.project.ID, m.config)
		})
	}

	// Apply criteria filter (from a loaded criteria-mode SavedFilter)
	if m.activeCriteriaFilter != nil {
		tasks = filterTaskTree(tasks, func(t *models.Task) bool {
			return passesSavedFilterCriteria(t, m.project, m.activeCriteriaFilter, m.config)
		})
	}

	// Apply section filter (from section filter selector)
	if len(m.filter.Sections) > 0 {
		tasks = filterTaskTree(tasks, func(t *models.Task) bool {
			return m.filter.Sections[t.Section] || m.filter.Sections[t.CustomSection]
		})
	}

	// Apply actionable filter (Fr)
	if m.filter.Actionable {
		tasks = filterTaskTree(tasks, func(t *models.Task) bool {
			if m.project.HasIncompleteDependencies(t) {
				return false
			}
			if t.Status == models.StatusBlocked {
				return false
			}
			if t.Section == models.SectionInbox || t.Section == models.SectionNext ||
				t.Section == models.SectionWaiting || t.Section == models.SectionSomeday ||
				t.Section == models.SectionReview || t.Section == models.SectionStandby {
				return false
			}
			return true
		})
	}

	// Apply search filter
	if q := m.SearchQuery(); q != "" {
		tasks = filterTaskTree(tasks, func(t *models.Task) bool {
			return matchesSearch(t, q)
		})
	}

	if m.sort.Field != SortByDefault {
		tasks = m.sortTaskTree(tasks)
	}

	// For the All perspective, group tasks by section with collapsible headers
	p = m.currentPerspective()
	if p.Name == "All" {
		m.buildRowsWithSectionHeaders(tasks)
	} else {
		m.flattenTaskTreeWithFolds(tasks, nil, &m.rows)
	}
	m.clampCursor()

	// Position cursor on OpenAtTask the first time rows are available, then clear.
	if m.OpenAtTask != nil && len(m.rows) > 0 {
		m.NavigateToSidebarTask(m.OpenAtTask)
		m.OpenAtTask = nil
	}
}

// buildRowsWithSectionHeaders groups tasks by GTD section and inserts collapsible
// section headers for the All perspective.
func (m *OutlinerModel) buildRowsWithSectionHeaders(tasks []*models.Task) {
	// Group tasks by section
	tasksBySection := m.groupTasksBySection(tasks)

	for _, sectionName := range SectionOrder {
		sectionTasks := tasksBySection[sectionName]
		if len(sectionTasks) == 0 {
			continue
		}

		// Count total tasks in this section (including children)
		count := m.countTasksRecursive(sectionTasks)

		// Add section header row
		m.rows = append(m.rows, outlinerRow{
			IsSectionHeader: true,
			SectionName:     sectionName,
			SectionCount:    count,
		})

		// Add tasks if section is not collapsed
		if !m.collapsedSections[sectionName] {
			m.flattenTaskTreeWithFolds(sectionTasks, nil, &m.rows)
		}
	}
}

// groupTasksBySection groups tasks by their GTD section, preserving hierarchy.
func (m *OutlinerModel) groupTasksBySection(tasks []*models.Task) map[string][]*models.Task {
	groups := make(map[string][]*models.Task)
	for _, task := range tasks {
		groups[task.Section] = append(groups[task.Section], task)
	}
	return groups
}

// countTasksRecursive counts all tasks in a tree, including descendants.
func (m *OutlinerModel) countTasksRecursive(tasks []*models.Task) int {
	count := 0
	for _, task := range tasks {
		count++                                       // Count this task
		count += m.countTasksRecursive(task.Children) // Count children
	}
	return count
}

// currentPerspective returns the active perspective preset.
// Defaults to the "All" perspective (key 1) when activePerspective is unset or negative.
func (m *OutlinerModel) currentPerspective() Perspective {
	idx := m.activePerspective
	if idx < 0 || idx > 9 {
		idx = 1
	}
	p, _ := PerspectiveByKey(idx)
	return p
}

// filterTaskTree returns a copy of the task tree containing only tasks that
// match the predicate or have descendants that match. Parent tasks are kept
// visible when any child matches, preserving hierarchy context.
func filterTaskTree(tasks []*models.Task, predicate func(*models.Task) bool) []*models.Task {
	var result []*models.Task
	for _, task := range tasks {
		filteredChildren := filterTaskTree(task.Children, predicate)
		if predicate(task) || len(filteredChildren) > 0 {
			clone := *task
			clone.Children = filteredChildren
			result = append(result, &clone)
		}
	}
	return result
}

// sortTaskTree returns a sorted copy of the task tree. Sorting is applied at
// each hierarchy level (siblings are reordered but parent-child relationships
// are preserved). The original slices are not mutated.
func (m *OutlinerModel) sortTaskTree(tasks []*models.Task) []*models.Task {
	sorted := make([]*models.Task, len(tasks))
	copy(sorted, tasks)
	sort.SliceStable(sorted, func(i, j int) bool {
		cmp := CompareTasks(sorted[i], sorted[j], m.sort.Field, m.project.ID, m.config)
		return ApplySortDirection(cmp, m.sort.Ascending)
	})
	for idx, task := range sorted {
		if len(task.Children) > 0 {
			clone := *task
			clone.Children = m.sortTaskTree(task.Children)
			sorted[idx] = &clone
		}
	}
	return sorted
}

// toggleSort sets the sort field, toggling ascending/descending if already set.
func (m *OutlinerModel) toggleSort(field SortField) {
	m.sort.Toggle(field)
	m.rebuildRows()
}

// clampCursor ensures the cursor stays within [0, len(rows)-1].
func (m *OutlinerModel) clampCursor() {
	m.ClampCursorTo(len(m.rows))
}

// customSectionTabs returns ["All"] + sorted custom sections, or nil if no custom sections exist.
func (m *OutlinerModel) customSectionTabs() []string {
	return CustomSectionTabs(m.project)
}

// customSectionTabIndex returns the active tab index for m.project.
// Must only be called when m.project != nil (i.e. customSectionTabs() != nil).
func (m *OutlinerModel) customSectionTabIndex() int {
	return m.customSectionTab[m.project.ID]
}

// setCustomSectionTabIndex sets the active tab index for m.project.
func (m *OutlinerModel) setCustomSectionTabIndex(i int) {
	m.customSectionTab[m.project.ID] = i
}

// activeCustomSectionFilter returns the custom section name being filtered, or "" for "All".
func (m *OutlinerModel) activeCustomSectionFilter() string {
	return ActiveCustomSectionFilter(m.project, m.customSectionTabIndex())
}

// contentHeight returns the number of task rows visible on screen,
// accounting for the title header, optional tab bar, and optional breadcrumb line.
func (m *OutlinerModel) contentHeight() int {
	headerLines := 1 // title (filter/sort info is now inline on this line)
	if m.customSectionTabs() != nil {
		headerLines += 2 // custom section tab bar (text + bottom border)
	}
	if (m.project != nil && len(m.zoomStack) > 0) || (m.project == nil && len(m.crossZoomStack) > 0) {
		headerLines++ // breadcrumb
	}
	h := m.height - headerLines
	if h < 1 {
		return 1
	}
	return h
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
		// Rebuild rows so cloned tasks from filterTaskTree() pick up the
		// in-place comment mutation on the original task.
		m.rebuildRows()
		return m, nil

	case ParentSelectedMsg:
		if msg.ReparentTaskID == "" || m.project == nil {
			return m, nil
		}
		task := m.project.GetTaskByID(msg.ReparentTaskID)
		if task == nil {
			return m, nil
		}
		prevStates := captureTaskStates(m.project.Tasks)
		if msg.ParentID == nil {
			m.project.MutateTask(task.ID, func(t *models.Task) {
				t.ParentID = nil
			})
		} else {
			targetID := *msg.ParentID
			taskMap := make(map[string]*models.Task, len(m.project.Tasks))
			for _, t := range m.project.Tasks {
				taskMap[t.ID] = t
			}
			targetParent := taskMap[targetID]
			if targetParent != nil && task.Type.CanBeChildOf(targetParent.Type) && !models.WouldCreateCycle(taskMap, task.ID, targetID) {
				m.project.MutateTask(task.ID, func(t *models.Task) {
					t.ParentID = &targetID
				})
			}
		}
		m.project.BuildTaskTree()
		m.setUndoContext("Reparent Task", task.Title)
		return m, m.saveAndRefreshBulk(prevStates)

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
		if m.crossLoading {
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

// taskOpsView implementation for OutlinerModel.

func (m *OutlinerModel) getTaskRef() (*models.Task, *models.Project) {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil, nil
	}
	row := m.rows[m.cursor]
	if row.IsSectionHeader || row.IsSeparator {
		return nil, nil
	}
	if m.project == nil {
		for _, proj := range m.loadedProjects {
			if proj.ID == row.ProjectID {
				return row.task, proj
			}
		}
		return nil, nil
	}
	return row.task, m.project
}

func (m *OutlinerModel) getPersister() Persister { return m.projectService }
func (m *OutlinerModel) getDataDir() string      { return m.config.DataDir }
func (m *OutlinerModel) reloadData()             { m.rebuildRows() }
func (m *OutlinerModel) setErr(err error)        { m.err = err }
func (m *OutlinerModel) setStatus(msg string)    { m.InteractiveViewState.setStatus(msg) }

// toggleMultiSelectMode enters or exits multi-select mode.
func (m *OutlinerModel) toggleMultiSelectMode() {
	m.multiSelectMode = !m.multiSelectMode
	if !m.multiSelectMode {
		m.ClearSelection()
		m.clearStatus()
	} else {
		m.setStatus("Multi-select: Space=toggle, a=all, A=deselect, D=delete, v/Esc=exit")
	}
}

// selectAllVisible adds all currently visible rows to the selection.
func (m *OutlinerModel) selectAllVisible() {
	for _, row := range m.rows {
		if row.IsSectionHeader || row.IsSeparator || row.task == nil {
			continue
		}
		m.selectedTasks[row.task.ID] = true
	}
}

// deselectAllVisible removes all currently visible rows from the selection.
func (m *OutlinerModel) deselectAllVisible() {
	for _, row := range m.rows {
		if row.IsSectionHeader || row.IsSeparator || row.task == nil {
			continue
		}
		delete(m.selectedTasks, row.task.ID)
	}
}

// getSelectedTasks returns a slice of selected tasks.
// In cross-project mode (m.project == nil), tasks are collected from m.rows
// to avoid a nil dereference on m.project.
func (m *OutlinerModel) getSelectedTasks() []*models.Task {
	if m.project == nil {
		var tasks []*models.Task
		for _, row := range m.rows {
			if row.IsSectionHeader || row.IsSeparator || row.task == nil {
				continue
			}
			if m.selectedTasks[row.task.ID] {
				tasks = append(tasks, row.task)
			}
		}
		return tasks
	}
	var tasks []*models.Task
	for _, task := range m.project.Tasks {
		if m.selectedTasks[task.ID] {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// clearMultiSelect exits multi-select mode and clears all selections.
func (m *OutlinerModel) clearMultiSelect() {
	m.ExitMultiSelect()
	m.clearStatus()
}

// getMultiSelectedTasks returns selected tasks when in multi-select mode, nil otherwise.
func (m *OutlinerModel) getMultiSelectedTasks() []*models.Task {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		return m.getSelectedTasks()
	}
	return nil
}

// refreshView rebuilds the outliner rows.
func (m *OutlinerModel) refreshView() {
	m.rebuildRows()
}

// saveAndRefreshBulk saves the project with workflow rules and rebuilds rows.
func (m *OutlinerModel) saveAndRefreshBulk(prevStates map[string]*models.Task) tea.Cmd {
	opts := m.buildWorkflowOpts(prevStates)
	return func() tea.Msg {
		if err := persistWithWorkflow(opts); err != nil {
			return ErrorMsg{Err: err}
		}
		return outlinerRefreshMsg{}
	}
}

// setStatusMessage sets the status bar text.
func (m *OutlinerModel) setStatusMessage(msg string) {
	m.setStatus(msg)
}

// buildWorkflowOpts constructs a WorkflowSaveOptions from the current model
// state, draining the pending undo context.
func (m *OutlinerModel) buildWorkflowOpts(prevStates map[string]*models.Task) WorkflowSaveOptions {
	opts := BuildWorkflowOpts(m.projectService, m.project, m.config, m.configManager, prevStates, m.undo.Action, m.undo.Context)
	m.undo.Clear()
	return opts
}

// workflowPersistFn returns a persistFn that runs the full save pipeline:
// auto-unblock, recurring, workflow rules, undo snapshots.
func (m *OutlinerModel) workflowPersistFn() persistFn {
	return newWorkflowPersistFn(m)
}

// saveAndRefreshWithPrevStates saves the project with the workflow pipeline
// and rebuilds rows. This is the outliner's equivalent of ProjectTree's method.
func (m *OutlinerModel) saveAndRefreshWithPrevStates(prevStates map[string]*models.Task) error {
	opts := m.buildWorkflowOpts(prevStates)
	if err := persistWithWorkflow(opts); err != nil {
		return err
	}
	m.rebuildRows()
	return nil
}

// setUndoContext sets the action context for the next save operation.
func (m *OutlinerModel) setUndoContext(action, context string) {
	m.undo.Set(action, context)
}

// performUndo restores the project from backup.
func (m *OutlinerModel) performUndo() tea.Cmd {
	return performUndoCmd(m.projectService, m.project)
}

// OutlinerToggleState captures section/fold state so it can survive outliner
// recreation within the same TUI session.
type OutlinerToggleState struct {
	CollapsedSections map[string]bool // Section collapse state for All perspective
	ExpandedNodes     map[string]bool // Node fold state (empty = all expanded)
	CursorTaskID      string          // Task ID at cursor position (session-only)
	ZoomStack         []ZoomEntry     // Zoom stack (session-only)
}

// SaveToggleState snapshots the current section/fold state.
func (m *OutlinerModel) SaveToggleState() *OutlinerToggleState {
	var cursorTaskID string
	if task := m.getCursorTask(); task != nil {
		cursorTaskID = task.ID
	}
	var zoomCopy []ZoomEntry
	if len(m.zoomStack) > 0 {
		zoomCopy = make([]ZoomEntry, len(m.zoomStack))
		copy(zoomCopy, m.zoomStack)
	}
	return &OutlinerToggleState{
		CollapsedSections: copyBoolMap(m.collapsedSections),
		ExpandedNodes:     copyBoolMap(m.expandedNodes),
		CursorTaskID:      cursorTaskID,
		ZoomStack:         zoomCopy,
	}
}

// RestoreToggleState applies a previously saved section/fold state.
func (m *OutlinerModel) RestoreToggleState(state *OutlinerToggleState) {
	if state == nil {
		return
	}
	m.collapsedSections = copyBoolMap(state.CollapsedSections)
	m.expandedNodes = copyBoolMap(state.ExpandedNodes)
	if len(state.ZoomStack) > 0 {
		m.zoomStack = make([]ZoomEntry, len(state.ZoomStack))
		copy(m.zoomStack, state.ZoomStack)
	}
	m.rebuildRows()
	if state.CursorTaskID != "" {
		m.moveCursorToTask(state.CursorTaskID)
	}
}

// copyBoolMap returns a shallow copy of a map[string]bool.
func copyBoolMap(src map[string]bool) map[string]bool {
	if src == nil {
		return nil
	}
	dst := make(map[string]bool, len(src))
	maps.Copy(dst, src)
	return dst
}

// outlinerRefreshMsg signals that the outliner should refresh its rows.
type outlinerRefreshMsg struct{}

// saveWithWorkflow persists the project using the full workflow pipeline
// and rebuilds rows on success.
func (m *OutlinerModel) saveWithWorkflow() tea.Cmd {
	opts := m.buildWorkflowOpts(nil)
	return func() tea.Msg {
		if err := persistWithWorkflow(opts); err != nil {
			return ErrorMsg{Err: err}
		}
		return outlinerRefreshMsg{}
	}
}

// archiveProject archives or unarchives the project and saves config.
func (m *OutlinerModel) archiveProject() tea.Cmd {
	return func() tea.Msg {
		if m.project.Archived {
			m.project.Unarchive()
		} else {
			m.project.Archive()
			if m.config.CurrentProject == m.project.ID {
				m.config.CurrentProject = ""
			}
		}
		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}
		return ProjectArchivedMsg{Project: m.project}
	}
}

// resolveProjectForRow returns the project that owns the given row.
// In single-project mode it returns m.project; in cross-project mode it
// resolves via crossResolveProject using the row's ProjectID.
func (m *OutlinerModel) resolveProjectForRow(row outlinerRow) *models.Project {
	if m.project != nil {
		return m.project
	}
	return m.crossResolveProject(row.ProjectID)
}

// SidebarHost interface implementation for OutlinerModel.

// SidebarProject returns the project for sidebar display.
func (m *OutlinerModel) SidebarProject() *models.Project {
	if m.project != nil {
		return m.project
	}
	// Cross-project: resolve from cursor row.
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		row := m.rows[m.cursor]
		if row.ProjectID != "" {
			return m.crossResolveProject(row.ProjectID)
		}
	}
	return nil
}

// SidebarConfig returns the config for sidebar display.
func (m *OutlinerModel) SidebarConfig() *config.Config {
	return m.config
}

// SidebarGetSidebar returns the sidebar component.
func (m *OutlinerModel) SidebarGetSidebar() *SidebarComponent {
	return m.sidebar
}

// SidebarHeight returns the visible height available for sidebar content.
func (m *OutlinerModel) SidebarHeight() int {
	return m.height - CardVerticalBorderOverhead
}

// SidebarSelectedTask returns the currently selected task.
func (m *OutlinerModel) SidebarSelectedTask() *models.Task {
	if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) {
		return m.rows[m.cursor].task
	}
	return nil
}

// NavigateToSidebarTask navigates to a task from the sidebar.
// If the task is inside a collapsed parent, ancestors are expanded first.
func (m *OutlinerModel) NavigateToSidebarTask(task *models.Task) {
	// First try to find the task in the current rows.
	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID {
			m.cursor = i
			return
		}
	}

	// Task not found — it may be inside a collapsed section or parent.
	// Uncollapse the task's section and expand all ancestor nodes, then rebuild.
	// In cross-project mode (m.project == nil) collapsedSections keys are
	// composite "projectID:sectionName", so a bare-section delete would be a
	// no-op. Skip it and rely on the cursor-search fallback below.
	if task.Section != "" && m.project != nil {
		delete(m.collapsedSections, task.Section)
	}
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.expandedNodes[ancestor.ID] = true
	}
	m.rebuildRows()

	for i, row := range m.rows {
		if row.task != nil && row.task.ID == task.ID {
			m.cursor = i
			return
		}
	}
}

// UnlinkSelectedSidebarTask removes the currently selected link from the sidebar.
func (m *OutlinerModel) UnlinkSelectedSidebarTask(sourceTask *models.Task) tea.Cmd {
	section := m.sidebar.GetDependencySection()
	cursor := m.sidebar.GetDependencyCursor()
	ps := m.projectService

	m.setUndoContext("Unlink", sourceTask.Title)
	opts := m.buildWorkflowOpts(nil)

	return func() tea.Msg {
		switch section {
		case 0: // Blocked by
			if cursor < 0 || cursor >= len(sourceTask.DependsOn) {
				return nil
			}
			ref := sourceTask.DependsOn[cursor]
			sourceTask.RemoveDependency(ref)

		case 1: // Blocking
			blockedTasks := m.project.GetBlockedByTasks(sourceTask)
			if cursor < 0 || cursor >= len(blockedTasks) {
				return nil
			}
			blockedTask := blockedTasks[cursor]
			blockedTask.RemoveDependency(sourceTask.ID)

		case 2: // Related
			if cursor < 0 || cursor >= len(sourceTask.RelatedTo) {
				return nil
			}
			ref := sourceTask.RelatedTo[cursor]
			taskRef := models.ParseTaskRef(ref)

			sourceTask.RemoveRelatedTo(ref)

			if taskRef.IsCrossProject() {
				// Cross-project: direct save (workflow is scoped to current project)
				if m.config != nil {
					targetProject := m.config.GetProject(taskRef.ProjectID)
					if targetProject != nil {
						if len(targetProject.Tasks) == 0 {
							if err := storage.LoadProject(targetProject); err != nil {
								return ErrorMsg{Err: err}
							}
						}
						if targetTask := targetProject.GetTaskByID(taskRef.TaskID); targetTask != nil {
							reverseRef := m.project.ID + ":" + sourceTask.ID
							targetTask.RemoveRelatedTo(reverseRef)

							if err := storage.BackupProject(targetProject); err != nil {
								return ErrorMsg{Err: err}
							}
							if err := storage.SaveProjectLocked(targetProject); err != nil {
								return ErrorMsg{Err: err}
							}
						}
					}
				}
			} else {
				if targetTask := m.project.GetTaskByID(taskRef.TaskID); targetTask != nil {
					targetTask.RemoveRelatedTo(sourceTask.ID)
				}
			}
		}

		if err := persistWithWorkflow(opts); err != nil {
			return ErrorMsg{Err: err}
		}
		if err := ps.Load(m.project); err != nil {
			return ErrorMsg{Err: err}
		}
		return LinkRemovedMsg{SourceTask: sourceTask, Section: section}
	}
}

func (m *OutlinerModel) StatusBarLabel() string {
	if m.project != nil {
		status := fmt.Sprintf("Outliner: %s", m.project.Name) + FormatCyclePosition(m.config, m.project.ID)
		if m.sidebar.IsFocused() {
			status += " " + SidebarFocusBadgeStyle.Render("[SIDEBAR]")
		}
		return status
	}
	return m.crossStatusBarLabel()
}
