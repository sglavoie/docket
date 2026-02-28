package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

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
