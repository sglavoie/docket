package config

// ProjectTaskLocator identifies one task in durable cross-project planning
// collections without assuming task IDs are workspace-unique.
type ProjectTaskLocator struct {
	TaskID    string `json:"task_id"`
	ProjectID string `json:"project_id"`
}

// Valid reports whether both halves of the project-qualified identity are
// present. Planning never falls back to a workspace-wide task ID.
func (locator ProjectTaskLocator) Valid() bool {
	return locator.ProjectID != "" && locator.TaskID != ""
}
