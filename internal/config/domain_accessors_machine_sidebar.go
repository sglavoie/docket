package config

// GetProjectSidebarEnabled returns the machine-local sidebar preference for a
// project. Projects without an explicit preference keep the enabled default.
func (m *MachineState) GetProjectSidebarEnabled(projectID string) bool {
	if m == nil || projectID == "" {
		return true
	}
	if enabled, ok := m.ProjectSidebarEnabled[projectID]; ok {
		return enabled
	}
	return true
}

// SetProjectSidebarEnabled stores the machine-local sidebar preference for a
// project.
func (m *MachineState) SetProjectSidebarEnabled(projectID string, enabled bool) {
	if m == nil || projectID == "" {
		return
	}
	if m.ProjectSidebarEnabled == nil {
		m.ProjectSidebarEnabled = make(map[string]bool)
	}
	m.ProjectSidebarEnabled[projectID] = enabled
}
