package tui

// escSteps defines the Esc dismissal cascade: search → filters.
func (m *RecentActivityModel) escSteps() []EscStep {
	return []EscStep{
		{Active: func() bool { return m.SearchQuery() != "" }, Clear: func() {
			m.ClearSearch()
			m.applyFilter()
			m.clampCursor()
		}},
		FilterClearEscStep(m.HasActiveFilters, func() {
			m.ClearAllFilters()
			m.applyFilter()
			m.clampCursor()
			m.setStatus("Filters cleared")
		}),
	}
}

// ShouldCaptureEsc returns true when search is active, a query is set, prefix is pending, or filters are active.
func (m *RecentActivityModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.SearchShouldCaptureEsc() || AnyEscStepActive(m.escSteps())
}

// GetHelpItems returns help items for this view.
func (m *RecentActivityModel) GetHelpItems() []HelpItem {
	if m.projectScope != nil {
		return GetKeybindingRegistry().GetForView(ViewProjectRecentActivity)
	}
	return GetKeybindingRegistry().GetForView(ViewRecentActivity)
}

func (m *RecentActivityModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *RecentActivityModel) clampCursor() {
	m.ClampCursorTo(len(m.filtered))
}

func (m *RecentActivityModel) StatusBarLabel() string { return "Recent Activity" }

// IsFullHeight returns true because the recent activity view renders its own footer.
func (m *RecentActivityModel) IsFullHeight() bool { return true }
