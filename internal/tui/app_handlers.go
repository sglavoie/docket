package tui

import (
	"fmt"
	"maps"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

// saveOutlinerToggleState captures the current outliner's section/fold state
// into the per-project cache so it can be restored after recreation.
func (m *AppModel) saveOutlinerToggleState() {
	if m.outliner != nil && m.outliner.project != nil {
		id := m.outliner.project.ID
		m.outlinerToggleStates[id] = m.outliner.SaveToggleState()
		for len(m.outlinerToggleStates) > maxProjectLastViewEntries {
			for k := range m.outlinerToggleStates {
				if k != id {
					delete(m.outlinerToggleStates, k)
					break
				}
			}
		}
	}
}

// restoreOutlinerToggleState applies any cached section/fold state to the
// current outliner for its project.
func (m *AppModel) restoreOutlinerToggleState() {
	if m.outliner != nil && m.outliner.project != nil {
		if state, ok := m.outlinerToggleStates[m.outliner.project.ID]; ok {
			m.outliner.RestoreToggleState(state)
		}
	}
}

// saveCrossOutlinerState persists the cross-project outliner's collapsed
// sections to config so they survive app restarts.
func (m *AppModel) saveCrossOutlinerState() {
	if m.crossOutliner == nil {
		return
	}
	m.config.CrossOutlinerCollapsedSections = maps.Clone(m.crossOutliner.collapsedSections)
	if m.crossOutliner.columnConfig != nil {
		m.config.CrossOutlinerColumnToggles = m.crossOutliner.columnConfig.Toggles()
	}
	if err := m.configManager.Save(m.config); err != nil {
		logging.Warn("failed to save cross-outliner state: %v", err)
	}
}

// saveOutlinerFilterState persists the current outliner filter state to config.
func (m *AppModel) saveOutlinerFilterState() {
	if m.outliner == nil || m.outliner.project == nil {
		return
	}

	var filterTags []string
	for tag := range m.outliner.filter.Tags {
		filterTags = append(filterTags, tag)
	}
	var filterSections []string
	for section := range m.outliner.filter.Sections {
		filterSections = append(filterSections, section)
	}

	state := m.config.ProjectViewStates[m.outliner.project.ID]
	state.OutlinerFilterType = string(m.outliner.filter.Type)
	state.OutlinerFilterStatus = string(m.outliner.filter.Status)
	state.OutlinerFilterPriority = m.outliner.filter.Priority
	state.OutlinerFilterTags = filterTags
	state.OutlinerFilterDueDate = int(m.outliner.filter.DueDate)
	state.OutlinerFilterSections = filterSections
	state.OutlinerFilterActionable = m.outliner.filter.Actionable
	state.OutlinerFilterScoreMin = m.outliner.filter.ScoreMin
	state.OutlinerColumnToggles = m.outliner.columnConfig.Toggles()

	// Persist fold state to disk (only collapsed nodes)
	collapsedNodes := make(map[string]bool)
	for id, expanded := range m.outliner.expandedNodes {
		if !expanded {
			collapsedNodes[id] = true
		}
	}
	if len(collapsedNodes) > 0 {
		state.OutlinerFoldState = collapsedNodes
	} else {
		state.OutlinerFoldState = nil
	}

	if len(m.outliner.collapsedSections) > 0 {
		state.OutlinerCollapsedSections = m.outliner.collapsedSections
	} else {
		state.OutlinerCollapsedSections = nil
	}

	if len(m.outliner.filter.Attrs) > 0 {
		state.OutlinerFilterAttrs = make(map[string]string)
		maps.Copy(state.OutlinerFilterAttrs, m.outliner.filter.Attrs)
	} else {
		state.OutlinerFilterAttrs = nil
	}

	state.OutlinerSortField = int(m.outliner.sort.Field)
	state.OutlinerSortAscending = m.outliner.sort.Ascending
	state.OutlinerActivePerspective = m.outliner.activePerspective
	state.OutlinerFilterBarEnabled = m.outliner.filterBar.IsEnabled()

	m.config.ProjectViewStates[m.outliner.project.ID] = state
	if err := m.configManager.Save(m.config); err != nil {
		logging.Warn("failed to save config: %v", err)
	}
}

// restoreOutlinerFilterState applies saved outliner filter state from config.
func (m *AppModel) restoreOutlinerFilterState() {
	if m.outliner == nil || m.outliner.project == nil {
		return
	}
	state, exists := m.config.GetProjectViewState(m.outliner.project.ID)
	if !exists {
		return
	}

	m.outliner.filter.Type = models.TaskType(state.OutlinerFilterType)
	m.outliner.filter.Status = models.TaskStatus(state.OutlinerFilterStatus)
	m.outliner.filter.Priority = state.OutlinerFilterPriority
	m.outliner.filter.DueDate = DueDateFilter(state.OutlinerFilterDueDate)
	m.outliner.filter.Actionable = state.OutlinerFilterActionable
	m.outliner.filter.ScoreMin = state.OutlinerFilterScoreMin

	// Restore attribute filters
	if len(state.OutlinerFilterAttrs) > 0 {
		m.outliner.filter.Attrs = make(map[string]string)
		maps.Copy(m.outliner.filter.Attrs, state.OutlinerFilterAttrs)
	}

	// Restore tags - validate they still exist in the project
	m.outliner.filter.Tags = validateSavedStringSet(state.OutlinerFilterTags, CollectProjectTagNames(m.outliner.project.Tasks))

	// Restore sections - validate they still exist
	m.outliner.filter.Sections = validateSavedStringSet(state.OutlinerFilterSections, m.outliner.project.GetSections())

	// Restore sort state
	m.outliner.sort.Field = SortField(state.OutlinerSortField)
	m.outliner.sort.Ascending = state.OutlinerSortAscending

	// Restore filter bar state
	m.outliner.filterBar.SetEnabled(state.OutlinerFilterBarEnabled)

	// Restore active perspective (0 is Snoozed, 1 is All/default;
	// only override the default if explicitly saved as non-zero or if sort
	// was also saved, indicating outliner state was persisted at least once)
	if state.OutlinerActivePerspective != 0 || state.OutlinerSortField != 0 {
		m.outliner.activePerspective = state.OutlinerActivePerspective
	}

	// Restore section collapse state from disk if no in-memory cache exists
	if _, hasCached := m.outlinerToggleStates[m.outliner.project.ID]; !hasCached && len(state.OutlinerCollapsedSections) > 0 {
		maps.Copy(m.outliner.collapsedSections, state.OutlinerCollapsedSections)
	}

	// Restore fold state from disk if no in-memory toggle cache exists
	if _, hasCached := m.outlinerToggleStates[m.outliner.project.ID]; !hasCached && len(state.OutlinerFoldState) > 0 {
		m.outliner.ensureExpandedNodes()
		for id := range state.OutlinerFoldState {
			m.outliner.expandedNodes[id] = false
		}
	}

	m.outliner.rebuildRows()
}

// reloadAndRefreshProjectTree reloads project from storage and refreshes the tree.
// Preserves filter/sort state across the reload.
func (m *AppModel) reloadAndRefreshProjectTree() {
	if m.projectTree != nil && m.currentProject != nil {
		m.switchToProject(m.currentProject, true)
	}
}

// applyTagsToTask applies selected tags to a single task and saves the project.
func (m *AppModel) applyTagsToTask(task *models.Task, project *models.Project, selectedTags []string) tea.Cmd {
	// Re-lookup task to avoid stale pointer after concurrent reload
	if !project.MutateTask(task.ID, func(t *models.Task) {
		t.Tags = selectedTags
	}) {
		return errorCmd("task no longer exists in project")
	}

	// Create backup and save
	if err := saveProjectSync(project); err != nil {
		return errorCmd("save failed: %w", err)
	}

	// Refresh tree/outliner and set undo context
	if m.projectTree != nil {
		m.projectTree.setUndoContext("Set Tags", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
		m.projectTree.undo.TaskIDs = []string{task.ID}
		m.projectTree.buildTree()
	}
	if m.outliner != nil {
		m.outliner.setUndoContext("Set Tags", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
		m.outliner.undo.TaskIDs = []string{task.ID}
		m.outliner.rebuildRows()
	}
	if m.crossOutliner != nil {
		m.crossOutliner.crossUndoCtx.Set("Set Tags", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
		m.crossOutliner.crossUndoCtx.ProjectIDs = []string{project.ID}
		m.crossOutliner.crossUndoCtx.TaskIDs = []string{task.ID}
		m.crossOutliner.rebuildRows()
	}

	return nil
}

// applyTagsToTasks applies selected tags to multiple tasks and saves the project.
func (m *AppModel) applyTagsToTasks(tasks []*models.Task, project *models.Project, selectedTags []string) tea.Cmd {
	// Update all tasks with the selected tags
	for _, task := range tasks {
		project.MutateTask(task.ID, func(t *models.Task) {
			t.Tags = selectedTags
		})
	}

	// Create backup and save
	if err := saveProjectSync(project); err != nil {
		return errorCmd("save failed: %w", err)
	}

	// Refresh tree/outliner, clear selection, and set undo context
	bulkIDs := taskIDsFromSlice(tasks)
	if m.projectTree != nil {
		m.projectTree.clearSelection()
		m.projectTree.setUndoContext("Set Tags", fmt.Sprintf("%d tasks", len(tasks)))
		m.projectTree.undo.TaskIDs = bulkIDs
		m.projectTree.buildTree()
	}
	if m.outliner != nil {
		m.outliner.clearMultiSelect()
		m.outliner.setUndoContext("Set Tags", fmt.Sprintf("%d tasks", len(tasks)))
		m.outliner.undo.TaskIDs = bulkIDs
		m.outliner.rebuildRows()
	}

	return nil
}

// processMessage routes a message through the app's handler chain.
// Used by CommandPaletteExecuteMsg to process the inner message synchronously.
func (m *AppModel) processMessage(msg tea.Msg) (bool, tea.Cmd) {
	// Handle QuitMsg directly since it's not in the handler chains
	if _, ok := msg.(QuitMsg); ok {
		m.saveProjectTreeState()
		m.saveOutlinerFilterState()
		m.saveCrossOutlinerState()
		m.quitting = true
		return true, tea.Quit
	}
	if handled, cmd := m.handleProjectMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleTaskMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleViewMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleTagMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleSectionMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleInboxMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleClipboardMessages(msg); handled {
		return true, cmd
	}
	if handled, cmd := m.handleProjectCycleMessages(msg); handled {
		return true, cmd
	}
	return false, nil
}
