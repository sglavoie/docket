package models

import "strings"

// TaskRef represents a reference to a task, optionally in another project.
// Used for cross-project task linking.
type TaskRef struct {
	ProjectID string // empty = current project (same-project reference)
	TaskID    string
}

// ParseTaskRef parses a task reference string.
// Formats: "taskID" (same project) or "projectID:taskID" (cross-project).
func ParseTaskRef(ref string) TaskRef {
	if idx := strings.Index(ref, ":"); idx > 0 {
		return TaskRef{
			ProjectID: ref[:idx],
			TaskID:    ref[idx+1:],
		}
	}
	return TaskRef{TaskID: ref}
}

// String returns the string representation of the task reference.
// Same-project references return just the task ID.
// Cross-project references return "projectID:taskID".
func (r TaskRef) String() string {
	if r.ProjectID == "" {
		return r.TaskID
	}
	return r.ProjectID + ":" + r.TaskID
}

// IsCrossProject returns true if this reference points to a different project.
func (r TaskRef) IsCrossProject() bool {
	return r.ProjectID != ""
}

// IsEmpty returns true if this reference has no task ID.
func (r TaskRef) IsEmpty() bool {
	return r.TaskID == ""
}
