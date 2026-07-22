package tui

// TaskIdentity is a stable task selection key. ProjectID remains part of the
// identity because Planner and cross-project Outliner selections span projects.
type TaskIdentity struct {
	ProjectID string
	TaskID    string
}

func (id TaskIdentity) Valid() bool { return id.ProjectID != "" && id.TaskID != "" }
