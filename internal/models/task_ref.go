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
// A reference is only considered cross-project when it has both a non-empty
// ProjectID and a non-empty TaskID — a malformed value like "proj:" (project
// id followed by an empty task id) is not a usable cross-project reference,
// and treating it as one would cause orphan-ref pruning to preserve it
// indefinitely while no lookup could ever resolve it.
func (r TaskRef) IsCrossProject() bool {
	return r.ProjectID != "" && r.TaskID != ""
}

// IsEmpty returns true if this reference has no task ID.
func (r TaskRef) IsEmpty() bool {
	return r.TaskID == ""
}

// IsNumericShortID reports whether the reference is an unqualified, purely
// numeric short ID. Short IDs are only unique within a single project, so an
// unqualified numeric reference is ambiguous across projects and must not be
// accepted by cross-project destructive operations without a project
// qualifier (project-slug:#short-id) or an unambiguous UUID/prefix.
func (r TaskRef) IsNumericShortID() bool {
	if r.ProjectID != "" || r.TaskID == "" {
		return false
	}
	for _, c := range r.TaskID {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
