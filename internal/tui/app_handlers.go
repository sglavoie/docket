package tui

import (
	"fmt"
	"maps"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
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

	// Persist per-task fold state (only collapsed nodes, sparse representation)
	collapsedNodes := make(map[string]bool)
	for id, expanded := range m.crossOutliner.expandedNodes {
		if !expanded {
			collapsedNodes[id] = true
		}
	}

	var crossTags []string
	for tag := range m.crossOutliner.filter.Tags {
		crossTags = append(crossTags, tag)
	}
	var crossSections []string
	for section := range m.crossOutliner.filter.Sections {
		crossSections = append(crossSections, section)
	}

	if err := m.commitMachineMutation(func(machine *config.MachineState) {
		machine.CrossOutlinerCollapsedSections = maps.Clone(m.crossOutliner.collapsedSections)
		if len(collapsedNodes) > 0 {
			machine.CrossOutlinerFoldState = collapsedNodes
		} else {
			machine.CrossOutlinerFoldState = nil
		}
		if m.crossOutliner.columnConfig != nil {
			machine.CrossOutlinerColumnToggles = m.crossOutliner.columnConfig.Toggles()
		}
		machine.CrossOutlinerFilterType = string(m.crossOutliner.filter.Type)
		machine.CrossOutlinerFilterStatus = string(m.crossOutliner.filter.Status)
		machine.CrossOutlinerFilterPriority = m.crossOutliner.filter.Priority
		machine.CrossOutlinerFilterDueDate = int(m.crossOutliner.filter.DueDate)
		machine.CrossOutlinerFilterActionable = m.crossOutliner.filter.Actionable
		machine.CrossOutlinerFilterHideDimmed = m.crossOutliner.filter.HideDimmed
		machine.CrossOutlinerFilterGitHubIssue = m.crossOutliner.filter.GitHubIssue
		machine.CrossOutlinerFilterParked = m.crossOutliner.filter.Parked
		machine.CrossOutlinerFilterScoreMin = m.crossOutliner.filter.ScoreMin
		machine.CrossOutlinerSortField = int(m.crossOutliner.sort.Field)
		machine.CrossOutlinerSortAscending = m.crossOutliner.sort.Ascending
		machine.CrossOutlinerFilterBarEnabled = m.crossOutliner.filterBar.IsEnabled()
		machine.CrossOutlinerFilterTags = crossTags
		machine.CrossOutlinerFilterSections = crossSections
		if len(m.crossOutliner.filter.Attrs) > 0 {
			machine.CrossOutlinerFilterAttrs = maps.Clone(m.crossOutliner.filter.Attrs)
		} else {
			machine.CrossOutlinerFilterAttrs = nil
		}
		machine.CrossOutlinerPinnedTagTab = m.crossOutliner.activePinnedTagFilter()
	}); err != nil {
		logging.Warn("failed to save cross-outliner state: %v", err)
	}
}

// saveOutlinerFilterState persists the current outliner filter state to config.
func (m *AppModel) saveOutlinerFilterState() {
	if m.outliner == nil {
		return
	}
	m.saveOutlinerFilterStateFrom(m.outliner)
}

// saveOutlinerFilterStateFrom persists the given outliner's filter state to config.
// Used for both the primary and temporary outliners.
func (m *AppModel) saveOutlinerFilterStateFrom(o *OutlinerModel) {
	if o == nil || o.project == nil {
		return
	}

	var filterTags []string
	for tag := range o.filter.Tags {
		filterTags = append(filterTags, tag)
	}
	var filterSections []string
	for section := range o.filter.Sections {
		filterSections = append(filterSections, section)
	}

	// Persist fold state to disk (only collapsed nodes)
	collapsedNodes := make(map[string]bool)
	for id, expanded := range o.expandedNodes {
		if !expanded {
			collapsedNodes[id] = true
		}
	}

	if err := m.commitMachineMutation(func(machine *config.MachineState) {
		state, _ := machine.GetProjectViewState(o.project.ID)
		state.OutlinerFilterType = string(o.filter.Type)
		state.OutlinerFilterStatus = string(o.filter.Status)
		state.OutlinerFilterPriority = o.filter.Priority
		state.OutlinerFilterTags = filterTags
		state.OutlinerFilterDueDate = int(o.filter.DueDate)
		state.OutlinerFilterSections = filterSections
		state.OutlinerFilterActionable = o.filter.Actionable
		state.OutlinerFilterHideDimmed = o.filter.HideDimmed
		state.OutlinerFilterGitHubIssue = o.filter.GitHubIssue
		state.OutlinerFilterParked = o.filter.Parked
		state.OutlinerFilterScoreMin = o.filter.ScoreMin
		state.OutlinerColumnToggles = o.columnConfig.Toggles()

		if len(collapsedNodes) > 0 {
			state.OutlinerFoldState = collapsedNodes
		} else {
			state.OutlinerFoldState = nil
		}

		if len(o.collapsedSections) > 0 {
			state.OutlinerCollapsedSections = maps.Clone(o.collapsedSections)
		} else {
			state.OutlinerCollapsedSections = nil
		}

		if len(o.filter.Attrs) > 0 {
			state.OutlinerFilterAttrs = make(map[string]string)
			maps.Copy(state.OutlinerFilterAttrs, o.filter.Attrs)
		} else {
			state.OutlinerFilterAttrs = nil
		}

		state.OutlinerSortField = int(o.sort.Field)
		state.OutlinerSortAscending = o.sort.Ascending
		state.OutlinerActivePerspective = o.activePerspective
		state.OutlinerFilterBarEnabled = o.filterBar.IsEnabled()
		state.SidebarEnabled = o.sidebar.IsEnabled()
		state.PinnedTagTab = o.activePinnedTagFilter()

		machine.SetProjectViewState(o.project.ID, state)
	}); err != nil {
		logging.Warn("failed to save config: %v", err)
	}
}

// restoreOutlinerFilterState applies saved outliner filter state from config
// to m.outliner.
func (m *AppModel) restoreOutlinerFilterState() {
	if m.outliner == nil {
		return
	}
	m.restoreOutlinerFilterStateTo(m.outliner)
}

// restoreOutlinerFilterStateTo applies saved outliner filter state from config
// to the given outliner model. Used for both the primary and temporary outliners.
func (m *AppModel) restoreOutlinerFilterStateTo(o *OutlinerModel) {
	if o == nil || o.project == nil {
		return
	}
	state, exists := m.runtime.Machine().GetProjectViewState(o.project.ID)
	if !exists {
		return
	}

	o.filter.Type = models.TaskType(state.OutlinerFilterType)
	o.filter.Status = models.TaskStatus(state.OutlinerFilterStatus)
	o.filter.Priority = state.OutlinerFilterPriority
	o.filter.DueDate = DueDateFilter(state.OutlinerFilterDueDate)
	o.filter.Actionable = state.OutlinerFilterActionable
	o.filter.HideDimmed = state.OutlinerFilterHideDimmed
	o.filter.GitHubIssue = state.OutlinerFilterGitHubIssue
	o.filter.Parked = state.OutlinerFilterParked
	o.filter.ScoreMin = state.OutlinerFilterScoreMin

	// Restore attribute filters
	if len(state.OutlinerFilterAttrs) > 0 {
		o.filter.Attrs = make(map[string]string)
		maps.Copy(o.filter.Attrs, state.OutlinerFilterAttrs)
	}

	// Restore tags - validate they still exist in the project
	o.filter.Tags = validateSavedStringSet(state.OutlinerFilterTags, CollectTagNamesForProject(o.project))

	// Restore sections - validate they still exist
	o.filter.Sections = validateSavedStringSet(state.OutlinerFilterSections, gtdSections(o.project.GetSections()))

	// Restore sort state
	o.sort.Field = SortField(state.OutlinerSortField)
	o.sort.Ascending = state.OutlinerSortAscending

	// Restore filter bar state
	o.filterBar.SetEnabled(state.OutlinerFilterBarEnabled)

	// Restore active perspective (0 is Snoozed, 1 is All/default;
	// only override the default if explicitly saved as non-zero or if sort
	// was also saved, indicating outliner state was persisted at least once)
	if state.OutlinerActivePerspective != 0 || state.OutlinerSortField != 0 {
		o.activePerspective = state.OutlinerActivePerspective
	}

	o.selectPinnedTagTab(state.PinnedTagTab)

	// Restore section collapse state from disk if no in-memory cache exists
	if _, hasCached := m.outlinerToggleStates[o.project.ID]; !hasCached && len(state.OutlinerCollapsedSections) > 0 {
		maps.Copy(o.collapsedSections, state.OutlinerCollapsedSections)
	}

	// Restore fold state from disk if no in-memory toggle cache exists
	if _, hasCached := m.outlinerToggleStates[o.project.ID]; !hasCached && len(state.OutlinerFoldState) > 0 {
		o.ensureExpandedNodes()
		for id := range state.OutlinerFoldState {
			o.expandedNodes[id] = false
		}
	}

	o.rebuildRows()
}

// reloadAndRefreshProjectTree reloads project from storage and refreshes the tree.
// Preserves filter/sort state across the reload.
func (m *AppModel) reloadAndRefreshProjectTree() {
	if m.projectTree != nil && m.currentProject != nil {
		m.switchToProject(m.currentProject, true)
	}
}

// applyTagsToTask applies selected tags to a single task as one asynchronous
// transaction through the BulkCoordinator, mirroring the multi-task
// applyTagsToTaskRefs sibling. Setting tags is an attribute mutation (no workflow
// rules run); the shell owns reload and toast via the batch result, and the
// durable write records through the workspace undo ledger.
func (m *AppModel) applyTagsToTask(task *models.Task, project *models.Project, selectedTags []string) tea.Cmd {
	if task == nil {
		return nil
	}
	cmd := m.secondaryFeatures.Bulk.SetTags([]string{task.ID}, selectedTags)
	return finalizeBatchResult(cmd, nil, fmt.Sprintf("Tagged #%d", task.ShortID))
}

// applyTagsToTaskRefs applies selected tags to multiple task refs as one
// asynchronous cross-project transaction through the BulkCoordinator. Setting
// tags is an attribute mutation (no workflow rules run); the shell owns reload
// and selection-clear via the batch result.
func (m *AppModel) applyTagsToTaskRefs(refs []TaskProjectRef, selectedTags []string) tea.Cmd {
	refs = validTaskProjectRefs(refs)
	taskIDs := taskIDsFromTaskRefs(refs)
	if len(taskIDs) == 0 {
		return nil
	}
	cmd := m.secondaryFeatures.Bulk.SetTags(taskIDs, selectedTags)
	return finalizeBatchResult(cmd, taskIDs, fmt.Sprintf("Tagged %d tasks", len(taskIDs)))
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
	if handled, cmd := m.handleAttrFilterMessages(msg); handled {
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
