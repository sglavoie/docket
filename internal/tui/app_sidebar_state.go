package tui

// setProjectSidebarEnabled synchronizes every live peer model for the current
// session. Sidebar visibility is presentation state and is not persisted.
func (m *AppModel) setProjectSidebarEnabled(projectID string, enabled bool) {
	if projectID == "" {
		return
	}

	m.syncProjectSidebarEnabled(projectID, enabled)
}

// determiningProjectSidebarEnabled resolves the project whose state controls
// which action-specific bulk sidebar command is available. A live primary
// project view is authoritative; otherwise the default is enabled.
func (m *AppModel) determiningProjectSidebarEnabled() bool {
	if enabled, ok := m.liveProjectSidebarEnabled(); ok {
		return enabled
	}
	return true
}

func (m *AppModel) liveProjectSidebarEnabled() (bool, bool) {
	view := m.currentView()
	if view == ViewCommandPalette && len(m.viewStack) > 1 {
		view = m.viewStack[len(m.viewStack)-2]
	}

	switch view { //nolint:exhaustive // Only project outliner presentations own sidebar state.
	case ViewOutliner:
		if m.views != nil {
			if outliner, ok := m.views.Get(ViewOutliner).(*OutlinerModel); ok && outliner.project != nil {
				return outliner.sidebar.IsEnabled(), true
			}
		}
		if m.outliner != nil && m.outliner.project != nil {
			return m.outliner.sidebar.IsEnabled(), true
		}
	case ViewTemporaryOutliner:
		if m.views != nil {
			if outliner, ok := m.views.Get(ViewTemporaryOutliner).(*OutlinerModel); ok && outliner.project != nil {
				return outliner.sidebar.IsEnabled(), true
			}
		}
	}
	return false, false
}

// setAllProjectSidebarsEnabled synchronizes every matching live project view.
func (m *AppModel) setAllProjectSidebarsEnabled(enabled bool) {
	if m.runtime == nil {
		return
	}
	projects := m.runtime.ActiveProjects()
	for _, project := range projects {
		m.syncProjectSidebarEnabled(project.ID, enabled)
	}
}

func (m *AppModel) syncProjectSidebarEnabled(projectID string, enabled bool) {
	setOutliner := func(outliner *OutlinerModel) {
		if outliner == nil || outliner.project == nil || outliner.project.ID != projectID {
			return
		}
		outliner.sidebar.SetEnabled(enabled)
		if !enabled {
			outliner.sidebar.Blur()
		}
	}

	// The typed pointer is the normal primary-view reference. Applying to it
	// also keeps isolated AppModel tests and models between registration
	// transitions consistent.
	setOutliner(m.outliner)

	if m.views == nil {
		return
	}
	for _, viewType := range []PresentationID{ViewOutliner, ViewTemporaryOutliner} {
		if outliner, ok := m.views.Get(viewType).(*OutlinerModel); ok {
			setOutliner(outliner)
		}
	}
}
