package config

// This file mirrors the config.Config facade's project-view-state and dashboard
// info-panel accessors onto the MachineState domain, so a caller that reads or
// mutates machine-local layout directly (via RuntimeState) normalizes and stores
// values identically to the legacy facade. The bodies mirror their (*Config)
// counterparts in view_state.go and ui_state.go field-for-field; keeping the
// defaulting/clamping semantics on the owning domain is what lets the outliner,
// project-tree, dashboard, and layout consumers migrate off config.Config
// without changing any observable behavior.

// --- MachineState: per-project view state ----------------------------------

// GetProjectViewState mirrors (*Config).GetProjectViewState.
func (m *MachineState) GetProjectViewState(projectID string) (ProjectViewState, bool) {
	if m.ProjectViewStates == nil {
		return ProjectViewState{}, false
	}
	state, exists := m.ProjectViewStates[projectID]
	return state, exists
}

// SetProjectViewState mirrors (*Config).SetProjectViewState.
func (m *MachineState) SetProjectViewState(projectID string, state ProjectViewState) {
	if m.ProjectViewStates == nil {
		m.ProjectViewStates = make(map[string]ProjectViewState)
	}
	m.ProjectViewStates[projectID] = state
}

// --- MachineState: dashboard info panel ------------------------------------

// GetInfoPanelMode mirrors (*Config).GetInfoPanelMode, including the migration
// from the legacy ShowInfoPanel bool when InfoPanelMode is unset.
func (m *MachineState) GetInfoPanelMode() InfoPanelMode {
	if m.InfoPanelMode == nil {
		if m.ShowInfoPanel == nil || *m.ShowInfoPanel {
			return InfoPanelInfoCards
		}
		return InfoPanelOff
	}
	mode := InfoPanelMode(*m.InfoPanelMode)
	if mode < 0 || mode >= infoPanelModeCount {
		return InfoPanelInfoCards
	}
	return mode
}

// SetInfoPanelMode mirrors (*Config).SetInfoPanelMode: it stores the mode and
// clears the legacy ShowInfoPanel field, clamping invalid values.
func (m *MachineState) SetInfoPanelMode(mode InfoPanelMode) {
	if mode < 0 || mode >= infoPanelModeCount {
		mode = InfoPanelInfoCards
	}
	i := int(mode)
	m.InfoPanelMode = &i
	m.ShowInfoPanel = nil
}

// CycleInfoPanelMode mirrors (*Config).CycleInfoPanelMode.
func (m *MachineState) CycleInfoPanelMode() InfoPanelMode {
	next := (m.GetInfoPanelMode() + 1) % infoPanelModeCount
	m.SetInfoPanelMode(next)
	return next
}

// GetShowInfoPanel mirrors (*Config).GetShowInfoPanel.
func (m *MachineState) GetShowInfoPanel() bool {
	return m.GetInfoPanelMode() != InfoPanelOff
}

// SetShowInfoPanel mirrors (*Config).SetShowInfoPanel.
func (m *MachineState) SetShowInfoPanel(show bool) {
	m.ShowInfoPanel = &show
	if show {
		mode := int(InfoPanelInfoCards)
		m.InfoPanelMode = &mode
		return
	}
	mode := int(InfoPanelOff)
	m.InfoPanelMode = &mode
}
