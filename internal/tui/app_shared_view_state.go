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

// extractSharedStateFromOutliner reads transferable state from the outliner.
func (m *AppModel) extractSharedStateFromOutliner() *sharedViewState {
	if m.outliner == nil {
		return nil
	}
	s := &sharedViewState{
		Filter: FilterState{
			CommonFilterState: CommonFilterState{
				Type:        m.outliner.filter.Type,
				Status:      m.outliner.filter.Status,
				Priority:    m.outliner.filter.Priority,
				DueDate:     m.outliner.filter.DueDate,
				ScoreMin:    m.outliner.filter.ScoreMin,
				Actionable:  m.outliner.filter.Actionable,
				HideDimmed:  m.outliner.filter.HideDimmed,
				GitHubIssue: m.outliner.filter.GitHubIssue,
				Parked:      m.outliner.filter.Parked,
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
	m.outliner.filter.GitHubIssue = s.Filter.GitHubIssue
	m.outliner.filter.Parked = s.Filter.Parked
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
