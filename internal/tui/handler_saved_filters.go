package tui

import (
	"maps"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// handleSavedFilterMessages handles saved filter messages.
// Returns true if the message was handled.
func (m *AppModel) handleSavedFilterMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ShowSaveFilterDialogMsg:
		return m.showViewInit(ViewSaveFilterInput, NewSaveFilterInputModel(m.buildCurrentViewSummary()))

	case SaveFilterMsg:
		sf := m.captureCurrentFilter(msg.Name)
		m.config.SaveFilter(sf)
		if err := m.configManager.Save(m.config); err != nil {
			return true, errorCmd("failed to save filter: %w", err)
		}
		// Pop the input dialog
		m.popView()
		// Set active saved filter indicator on the appropriate view
		if m.baseProjectView() == ViewOutliner && m.outliner != nil {
			m.outliner.activeSavedFilter = msg.Name
		} else if m.projectTree != nil {
			m.projectTree.activeSavedFilter = msg.Name
		}
		// Refresh the command palette if it's still showing
		if m.currentView() == ViewCommandPalette {
			if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
				palette.allItems = palette.loadAllItems()
				palette.filterItems()
			}
		}
		return true, nil

	case LoadSavedFilterMsg:
		if m.baseProjectView() == ViewOutliner && m.outliner != nil {
			m.applySavedFilterToOutliner(msg.Filter)
		} else if m.projectTree != nil {
			m.applySavedFilterToTree(msg.Filter)
		}
		return true, nil

	case toggleSavedFilterTabMsg:
		sf := m.config.GetSavedFilter(msg.ID)
		if sf != nil {
			sf.ShowAsTab = msg.ShowAsTab
			if err := m.configManager.Save(m.config); err != nil {
				return true, errorCmd("failed to save filter: %w", err)
			}
			// Rebuild AllTasks tab entries
			if m.allTasksView != nil {
				m.allTasksView.buildTabEntries()
				m.allTasksView.loadAllTasks()
			}
			// Refresh the command palette if it's showing
			if m.currentView() == ViewCommandPalette {
				if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
					palette.allItems = palette.loadAllItems()
					palette.filterItems()
				}
			}
		}
		return true, nil

	case DeleteSavedFilterMsg:
		m.config.DeleteSavedFilter(msg.ID)
		if err := m.configManager.Save(m.config); err != nil {
			return true, errorCmd("failed to delete saved filter: %w", err)
		}
		// Clear active indicator (filter already deleted, so just clear any indicator)
		if m.baseProjectView() == ViewOutliner && m.outliner != nil {
			m.outliner.activeSavedFilter = ""
			m.outliner.activeCriteriaFilter = nil
		} else if m.projectTree != nil {
			m.projectTree.activeSavedFilter = ""
			m.projectTree.activeCriteriaFilter = nil
		}
		// Rebuild AllTasks tab entries if on the stack
		if m.allTasksView != nil {
			m.allTasksView.buildTabEntries()
			m.allTasksView.loadAllTasks()
		}
		// Refresh the command palette if it's showing
		if m.currentView() == ViewCommandPalette {
			if palette, ok := m.views.Get(ViewCommandPalette).(*CommandPaletteModel); ok {
				palette.allItems = palette.loadAllItems()
				palette.filterItems()
			}
		}
		return true, nil
	}
	return false, nil
}

// captureCurrentFilter creates a SavedFilter from the current view's state.
// Captures from outliner when it's the active base view, otherwise from project tree.
func (m *AppModel) captureCurrentFilter(name string) config.SavedFilter {
	// Check if a filter with this name already exists (for overwrite/upsert)
	existing := m.config.GetSavedFilterByName(name)
	id := uuid.New().String()
	if existing != nil {
		id = existing.ID
	}

	if m.baseProjectView() == ViewOutliner && m.outliner != nil {
		return m.captureOutlinerFilter(name, id)
	}
	return m.captureProjectTreeFilter(name, id)
}

// captureProjectTreeFilter creates a SavedFilter from the project tree state.
func (m *AppModel) captureProjectTreeFilter(name, id string) config.SavedFilter {
	sf := config.SavedFilter{
		ID:   id,
		Name: name,
		Mode: "snapshot",
	}
	if m.projectTree == nil {
		return sf
	}

	pt := m.projectTree
	sf.ViewMode = int(pt.viewMode)
	sf.FilterType = string(pt.filter.Type)
	sf.FilterStatus = string(pt.filter.Status)
	sf.FilterPriority = pt.filter.Priority
	sf.FilterDueDate = int(pt.filter.DueDate)
	sf.FilterActionable = pt.filter.Actionable
	sf.FilterHideDimmed = pt.filter.HideDimmed
	sf.FilterScoreMin = pt.filter.ScoreMin
	sf.SortField = int(pt.sort.Field)
	sf.SortAscending = pt.sort.Ascending

	if len(pt.filter.Tags) > 0 {
		for tag := range pt.filter.Tags {
			sf.FilterTags = append(sf.FilterTags, tag)
		}
	}
	if len(pt.filter.Sections) > 0 {
		for sec := range pt.filter.Sections {
			sf.FilterSections = append(sf.FilterSections, sec)
		}
	}
	if len(pt.filter.Attrs) > 0 {
		sf.FilterAttrs = make(map[string]string)
		maps.Copy(sf.FilterAttrs, pt.filter.Attrs)
	}

	// Capture custom section tab name
	if tabs := pt.customSectionTabs(); tabs != nil && pt.customSectionTab < len(tabs) {
		sf.CustomSectionTab = tabs[pt.customSectionTab]
	}

	return sf
}

// captureOutlinerFilter creates a SavedFilter from the outliner state.
func (m *AppModel) captureOutlinerFilter(name, id string) config.SavedFilter {
	sf := config.SavedFilter{
		ID:   id,
		Name: name,
		Mode: "snapshot",
	}
	if m.outliner == nil {
		return sf
	}

	ol := m.outliner
	sf.ViewMode = ol.activePerspective
	sf.FilterType = string(ol.filter.Type)
	sf.FilterStatus = string(ol.filter.Status)
	sf.FilterPriority = ol.filter.Priority
	sf.FilterDueDate = int(ol.filter.DueDate)
	sf.FilterActionable = ol.filter.Actionable
	sf.FilterHideDimmed = ol.filter.HideDimmed
	sf.FilterScoreMin = ol.filter.ScoreMin
	sf.SortField = int(ol.sort.Field)
	sf.SortAscending = ol.sort.Ascending

	if len(ol.filter.Tags) > 0 {
		for tag := range ol.filter.Tags {
			sf.FilterTags = append(sf.FilterTags, tag)
		}
	}
	if len(ol.filter.Sections) > 0 {
		for sec := range ol.filter.Sections {
			sf.FilterSections = append(sf.FilterSections, sec)
		}
	}
	if len(ol.filter.Attrs) > 0 {
		sf.FilterAttrs = make(map[string]string)
		maps.Copy(sf.FilterAttrs, ol.filter.Attrs)
	}

	// Capture custom section tab name
	if tabs := ol.customSectionTabs(); tabs != nil && ol.customSectionTabIndex() < len(tabs) {
		sf.CustomSectionTab = tabs[ol.customSectionTabIndex()]
	}

	return sf
}

// applySavedFilterToTree applies a saved filter's state to the project tree.
func (m *AppModel) applySavedFilterToTree(sf config.SavedFilter) {
	pt := m.projectTree
	if pt == nil {
		return
	}

	if sf.Mode == "criteria" {
		// Criteria-mode: apply per-task criteria matching, clear snapshot filters
		pt.activeCriteriaFilter = &sf
	} else {
		// Snapshot-mode: restore all flat filter/sort state
		pt.activeCriteriaFilter = nil
		pt.viewMode = ViewMode(sf.ViewMode)
		pt.filter.Type = models.TaskType(sf.FilterType)
		pt.filter.Status = models.TaskStatus(sf.FilterStatus)
		pt.filter.Priority = sf.FilterPriority
		pt.filter.DueDate = DueDateFilter(sf.FilterDueDate)
		pt.filter.Actionable = sf.FilterActionable
		pt.filter.HideDimmed = sf.FilterHideDimmed
		pt.filter.ScoreMin = sf.FilterScoreMin
		pt.sort.Field = SortField(sf.SortField)
		pt.sort.Ascending = sf.SortAscending

		// Restore tags — validate against current project
		pt.filter.Tags = validateSavedStringSet(sf.FilterTags, pt.getAllTags())

		// Restore sections — validate against current project
		pt.filter.Sections = make(map[string]bool)
		if pt.project != nil {
			existingSections := pt.project.GetSections()
			sectionSet := make(map[string]bool)
			for _, s := range existingSections {
				sectionSet[s] = true
			}
			for _, section := range sf.FilterSections {
				if sectionSet[section] {
					pt.filter.Sections[section] = true
				}
			}
		}

		// Restore attributes
		pt.filter.Attrs = make(map[string]string)
		maps.Copy(pt.filter.Attrs, sf.FilterAttrs)

		// Restore custom section tab
		if sf.CustomSectionTab != "" {
			if tabs := pt.customSectionTabs(); tabs != nil {
				for i, tab := range tabs {
					if tab == sf.CustomSectionTab {
						pt.customSectionTab = i
						break
					}
				}
			}
		}
	}

	// Set active indicator
	pt.activeSavedFilter = sf.Name

	// Rebuild tree with new sort/filter
	pt.sortNodes()
	pt.updateVisibility()
	pt.pruneInvisibleSelections()
	pt.clampCursor()
}

// applySavedFilterToOutliner applies a saved filter's state to the outliner.
func (m *AppModel) applySavedFilterToOutliner(sf config.SavedFilter) {
	ol := m.outliner
	if ol == nil {
		return
	}

	if sf.Mode == "criteria" {
		// Criteria-mode: apply per-task criteria matching, clear snapshot filters
		ol.activeCriteriaFilter = &sf
	} else {
		// Snapshot-mode: restore all flat filter/sort state
		ol.activeCriteriaFilter = nil
		ol.activePerspective = sf.ViewMode
		ol.filter.Type = models.TaskType(sf.FilterType)
		ol.filter.Status = models.TaskStatus(sf.FilterStatus)
		ol.filter.Priority = sf.FilterPriority
		ol.filter.DueDate = DueDateFilter(sf.FilterDueDate)
		ol.filter.Actionable = sf.FilterActionable
		ol.filter.HideDimmed = sf.FilterHideDimmed
		ol.filter.ScoreMin = sf.FilterScoreMin
		ol.sort.Field = SortField(sf.SortField)
		ol.sort.Ascending = sf.SortAscending

		// Restore tags — validate against current project
		ol.filter.Tags = validateSavedStringSet(sf.FilterTags, CollectProjectTagNames(ol.project.Tasks))

		// Restore sections — validate against current project
		ol.filter.Sections = make(map[string]bool)
		if ol.project != nil {
			existingSections := ol.project.GetSections()
			sectionSet := make(map[string]bool)
			for _, s := range existingSections {
				sectionSet[s] = true
			}
			for _, section := range sf.FilterSections {
				if sectionSet[section] {
					ol.filter.Sections[section] = true
				}
			}
		}

		// Restore attributes
		ol.filter.Attrs = make(map[string]string)
		maps.Copy(ol.filter.Attrs, sf.FilterAttrs)

		// Restore custom section tab
		if sf.CustomSectionTab != "" {
			if tabs := ol.customSectionTabs(); tabs != nil {
				for i, tab := range tabs {
					if tab == sf.CustomSectionTab {
						ol.setCustomSectionTabIndex(i)
						break
					}
				}
			}
		}
	}

	// Set active indicator
	ol.activeSavedFilter = sf.Name

	// Rebuild with new sort/filter
	ol.rebuildRows()
}

// buildCurrentViewSummary returns a human-readable summary of the current filter state.
func (m *AppModel) buildCurrentViewSummary() string {
	if m.baseProjectView() == ViewOutliner && m.outliner != nil {
		return m.buildOutlinerViewSummary()
	}
	return m.buildProjectTreeViewSummary()
}

// buildProjectTreeViewSummary returns a summary of the project tree's filter state.
func (m *AppModel) buildProjectTreeViewSummary() string {
	if m.projectTree == nil {
		return ""
	}
	pt := m.projectTree

	var tags []string
	for tag := range pt.filter.Tags {
		tags = append(tags, tag)
	}
	var sections []string
	for sec := range pt.filter.Sections {
		sections = append(sections, sec)
	}

	return describeSnapshotFilters(
		string(pt.filter.Type),
		string(pt.filter.Status),
		pt.filter.Priority,
		tags,
		sections,
		int(pt.filter.DueDate),
		pt.filter.Actionable,
		pt.filter.HideDimmed,
		pt.filter.Attrs,
		pt.filter.ScoreMin,
		int(pt.sort.Field),
		int(pt.viewMode),
	)
}

// buildOutlinerViewSummary returns a summary of the outliner's filter state.
func (m *AppModel) buildOutlinerViewSummary() string {
	if m.outliner == nil {
		return ""
	}
	ol := m.outliner

	var tags []string
	for tag := range ol.filter.Tags {
		tags = append(tags, tag)
	}
	var sections []string
	for sec := range ol.filter.Sections {
		sections = append(sections, sec)
	}

	return describeSnapshotFilters(
		string(ol.filter.Type),
		string(ol.filter.Status),
		ol.filter.Priority,
		tags,
		sections,
		int(ol.filter.DueDate),
		ol.filter.Actionable,
		ol.filter.HideDimmed,
		ol.filter.Attrs,
		ol.filter.ScoreMin,
		int(ol.sort.Field),
		ol.activePerspective,
	)
}
