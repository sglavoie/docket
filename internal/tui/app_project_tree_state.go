package tui

import (
	"github.com/sglavoie/tt/internal/logging"
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
	m.projectTree.queueIDs = m.focusQueueIDs
	m.projectTree.dailyPlanIDs = m.dailyPlanIDs
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
	id := m.currentProject.ID
	m.projectTreeSpatialStates[id] = &ProjectTreeSpatialState{
		CursorTaskID: cursorTaskID,
		FoldState:    foldState,
	}
	for len(m.projectTreeSpatialStates) > maxProjectLastViewEntries {
		for k := range m.projectTreeSpatialStates {
			if k != id {
				delete(m.projectTreeSpatialStates, k)
				break
			}
		}
	}

	// Read existing state to preserve outliner fields (read-modify-write).
	state := m.config.ProjectViewStates[m.currentProject.ID]
	state.FilterType = string(m.projectTree.filter.Type)
	state.FilterStatus = string(m.projectTree.filter.Status)
	state.FilterPriority = m.projectTree.filter.Priority
	state.FilterTags = filterTags
	state.FilterSections = filterSections
	state.FilterDueDate = int(m.projectTree.filter.DueDate)
	state.FilterActionable = m.projectTree.filter.Actionable
	state.FilterHideDimmed = m.projectTree.filter.HideDimmed
	state.FilterStage = m.projectTree.filter.Stage
	state.SortField = int(m.projectTree.sort.Field)
	state.SortAscending = m.projectTree.sort.Ascending
	state.SidebarEnabled = m.projectTree.sidebar.IsEnabled()
	state.FilterBarEnabled = m.projectTree.filterBar.IsEnabled()
	state.ViewMode = int(m.projectTree.viewMode)
	state.CustomSectionTab = m.projectTree.activeCustomSectionFilter()
	state.TreeColumnToggles = m.projectTree.columnConfig.Toggles()
	state.TreeFoldState = foldState

	m.config.SetProjectViewState(m.currentProject.ID, state)
	if err := m.configManager.Save(m.config); err != nil {
		logging.Warn("failed to save config: %v", err)
	}
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
	m.projectTree.queueIDs = m.focusQueueIDs
	m.projectTree.dailyPlanIDs = m.dailyPlanIDs
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
	m.projectTree.filter.HideDimmed = state.FilterHideDimmed
	m.projectTree.filter.Stage = state.FilterStage
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
