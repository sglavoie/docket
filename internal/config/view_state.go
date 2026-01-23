package config

// GetProjectViewState returns the saved view state for a project.
func (c *Config) GetProjectViewState(projectID string) (ProjectViewState, bool) {
	if c.ProjectViewStates == nil {
		return ProjectViewState{}, false
	}
	state, exists := c.ProjectViewStates[projectID]
	return state, exists
}

// SetProjectViewState saves the view state for a project.
func (c *Config) SetProjectViewState(projectID string, state ProjectViewState) {
	if c.ProjectViewStates == nil {
		c.ProjectViewStates = make(map[string]ProjectViewState)
	}
	c.ProjectViewStates[projectID] = state
}
