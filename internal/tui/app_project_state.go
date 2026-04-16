package tui

import (
	"maps"

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

// sharedViewState carries filter/sort/sidebar/filterBar state between
// ProjectTree and Outliner when switching views with Ctrl+P / Ctrl+O.
type sharedViewState struct {
	Filter           FilterState
	Sort             SortState
	SidebarEnabled   bool
	FilterBarEnabled bool
}

// extractSharedStateFromTree reads transferable state from the project tree.
func (m *AppModel) extractSharedStateFromTree() *sharedViewState {
	if m.projectTree == nil {
		return nil
	}
	s := &sharedViewState{
		Filter: FilterState{
			CommonFilterState: CommonFilterState{
				Type:       m.projectTree.filter.Type,
				Status:     m.projectTree.filter.Status,
				Priority:   m.projectTree.filter.Priority,
				DueDate:    m.projectTree.filter.DueDate,
				ScoreMin:   m.projectTree.filter.ScoreMin,
				Actionable: m.projectTree.filter.Actionable,
				HideDimmed: m.projectTree.filter.HideDimmed,
				Stage:      m.projectTree.filter.Stage,
			},
		},
		Sort:             m.projectTree.sort,
		SidebarEnabled:   m.projectTree.sidebar.IsEnabled(),
		FilterBarEnabled: m.projectTree.filterBar.IsEnabled(),
	}
	// Deep-copy maps
	if len(m.projectTree.filter.Tags) > 0 {
		s.Filter.Tags = make(map[string]bool, len(m.projectTree.filter.Tags))
		maps.Copy(s.Filter.Tags, m.projectTree.filter.Tags)
	}
	if len(m.projectTree.filter.Sections) > 0 {
		s.Filter.Sections = make(map[string]bool, len(m.projectTree.filter.Sections))
		maps.Copy(s.Filter.Sections, m.projectTree.filter.Sections)
	}
	if len(m.projectTree.filter.Attrs) > 0 {
		s.Filter.Attrs = make(map[string]string, len(m.projectTree.filter.Attrs))
		maps.Copy(s.Filter.Attrs, m.projectTree.filter.Attrs)
	}
	return s
}

// extractSharedStateFromOutliner reads transferable state from the outliner.
func (m *AppModel) extractSharedStateFromOutliner() *sharedViewState {
	if m.outliner == nil {
		return nil
	}
	s := &sharedViewState{
		Filter: FilterState{
			CommonFilterState: CommonFilterState{
				Type:       m.outliner.filter.Type,
				Status:     m.outliner.filter.Status,
				Priority:   m.outliner.filter.Priority,
				DueDate:    m.outliner.filter.DueDate,
				ScoreMin:   m.outliner.filter.ScoreMin,
				Actionable: m.outliner.filter.Actionable,
				HideDimmed: m.outliner.filter.HideDimmed,
				Stage:      m.outliner.filter.Stage,
			},
		},
		Sort:             m.outliner.sort,
		SidebarEnabled:   m.outliner.sidebar.IsEnabled(),
		FilterBarEnabled: m.outliner.filterBar.IsEnabled(),
	}
	if len(m.outliner.filter.Tags) > 0 {
		s.Filter.Tags = make(map[string]bool, len(m.outliner.filter.Tags))
		maps.Copy(s.Filter.Tags, m.outliner.filter.Tags)
	}
	if len(m.outliner.filter.Sections) > 0 {
		s.Filter.Sections = make(map[string]bool, len(m.outliner.filter.Sections))
		maps.Copy(s.Filter.Sections, m.outliner.filter.Sections)
	}
	if len(m.outliner.filter.Attrs) > 0 {
		s.Filter.Attrs = make(map[string]string, len(m.outliner.filter.Attrs))
		maps.Copy(s.Filter.Attrs, m.outliner.filter.Attrs)
	}
	return s
}

// applySharedStateToTree writes shared state onto the project tree and rebuilds.
func (m *AppModel) applySharedStateToTree(s *sharedViewState) {
	if m.projectTree == nil || s == nil {
		return
	}
	m.projectTree.filter.Type = s.Filter.Type
	m.projectTree.filter.Status = s.Filter.Status
	m.projectTree.filter.Priority = s.Filter.Priority
	m.projectTree.filter.DueDate = s.Filter.DueDate
	m.projectTree.filter.ScoreMin = s.Filter.ScoreMin
	m.projectTree.filter.Actionable = s.Filter.Actionable
	m.projectTree.filter.HideDimmed = s.Filter.HideDimmed
	m.projectTree.filter.Stage = s.Filter.Stage
	m.projectTree.filter.Attrs = s.Filter.Attrs
	m.projectTree.sort = s.Sort
	m.projectTree.sidebar.SetEnabled(s.SidebarEnabled)
	m.projectTree.filterBar.SetEnabled(s.FilterBarEnabled)

	// Validate tags/sections against what exists in this view
	m.projectTree.filter.Tags = validateSavedStringSet(
		mapKeys(s.Filter.Tags), m.projectTree.getAllTags(),
	)
	m.projectTree.filter.Sections = validateSavedStringSet(
		mapKeys(s.Filter.Sections), m.currentProject.GetSections(),
	)

	m.projectTree.buildTree()
}

// applySharedStateToOutliner writes shared state onto the outliner and rebuilds.
func (m *AppModel) applySharedStateToOutliner(s *sharedViewState) {
	if m.outliner == nil || s == nil {
		return
	}
	m.outliner.filter.Type = s.Filter.Type
	m.outliner.filter.Status = s.Filter.Status
	m.outliner.filter.Priority = s.Filter.Priority
	m.outliner.filter.DueDate = s.Filter.DueDate
	m.outliner.filter.ScoreMin = s.Filter.ScoreMin
	m.outliner.filter.Actionable = s.Filter.Actionable
	m.outliner.filter.HideDimmed = s.Filter.HideDimmed
	m.outliner.filter.Stage = s.Filter.Stage
	m.outliner.filter.Attrs = s.Filter.Attrs
	m.outliner.sort = s.Sort
	m.outliner.sidebar.SetEnabled(s.SidebarEnabled)
	m.outliner.filterBar.SetEnabled(s.FilterBarEnabled)

	// Validate tags/sections against what exists in this view
	m.outliner.filter.Tags = validateSavedStringSet(
		mapKeys(s.Filter.Tags), CollectProjectTagNames(m.outliner.project.Tasks),
	)
	m.outliner.filter.Sections = validateSavedStringSet(
		mapKeys(s.Filter.Sections), m.outliner.project.GetSections(),
	)

	m.outliner.rebuildRows()
}

// saveCurrentViewFilterState snapshots filter state for the current view
// so it can be restored after history navigation reconstructs the view.
func (m *AppModel) saveCurrentViewFilterState() {
	switch m.currentView() {
	case ViewPriorityView:
		if pv, ok := m.views.Get(ViewPriorityView).(*PriorityViewModel); ok && m.currentProject != nil {
			m.priorityViewFilterCache[m.currentProject.ID] = &PriorityViewCache{
				Filter:    *copyCommonFilterState(&pv.filter),
				TabActive: pv.tabs.Active,
			}
			// Evict oldest entries when over cap
			for len(m.priorityViewFilterCache) > maxProjectLastViewEntries {
				for id := range m.priorityViewFilterCache {
					if id != m.currentProject.ID {
						delete(m.priorityViewFilterCache, id)
						break
					}
				}
			}
		}
	case ViewAllTasks:
		if m.allTasksView != nil {
			m.allTasksFilterCache = &AllTasksCache{
				Filter:    *copyCommonFilterState(&m.allTasksView.filter),
				GroupBy:   m.allTasksView.groupBy,
				TabActive: m.allTasksView.tabs.Active,
			}
		}
	default:
	}
}

// mapKeys returns the keys of a map[string]bool as a slice.
func mapKeys(m map[string]bool) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
