package tui

import (
	"maps"
)

// sharedViewState carries filter/sort/sidebar/filterBar state between
// ProjectTree and Outliner when switching through project navigation.
type sharedViewState struct {
	Filter           FilterState
	Sort             SortState
	SidebarEnabled   bool
	FilterBarEnabled bool
	PinnedTagTab     string
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
		PinnedTagTab:     m.projectTree.activePinnedTagFilter(),
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
		PinnedTagTab:     m.outliner.activePinnedTagFilter(),
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
	m.projectTree.selectPinnedTagTab(s.PinnedTagTab)

	// Validate tags/sections against what exists in this view
	m.projectTree.filter.Tags = validateSavedStringSet(
		mapKeys(s.Filter.Tags), m.projectTree.getAllTags(),
	)
	m.projectTree.filter.Sections = validateSavedStringSet(
		mapKeys(s.Filter.Sections), gtdSections(m.currentProject.GetSections()),
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
	m.outliner.selectPinnedTagTab(s.PinnedTagTab)

	// Validate tags/sections against what exists in this view
	m.outliner.filter.Tags = validateSavedStringSet(
		mapKeys(s.Filter.Tags), CollectTagNamesForProject(m.outliner.project),
	)
	m.outliner.filter.Sections = validateSavedStringSet(
		mapKeys(s.Filter.Sections), gtdSections(m.outliner.project.GetSections()),
	)

	m.outliner.rebuildRows()
}
