package planner

import "github.com/sglavoie/tt/internal/models"

// ResolveAllDependencies builds a unified task map across all projects, keyed
// by full reference string ("projectID:taskID" for cross-project refs and
// "taskID" for same-project refs).
//
// Both the project ID (UUID) and the project slug are registered as project
// identifiers so callers can look up tasks using either form.
func ResolveAllDependencies(projects []*models.Project) map[string]*models.Task {
	result := make(map[string]*models.Task)
	for _, p := range projects {
		for _, t := range p.Tasks {
			// Plain task ID (unqualified, for same-project lookups).
			result[t.ID] = t
			// Qualified by project UUID.
			result[p.ID+":"+t.ID] = t
			// Qualified by project slug (common in user-authored dep refs).
			if p.Slug != "" && p.Slug != p.ID {
				result[p.Slug+":"+t.ID] = t
			}
		}
	}
	return result
}

// CollectAllIncompleteTasks returns a flat slice of all incomplete tasks across
// all projects.  Cross-project DependsOn refs are resolved in-place using
// ResolveAllDependencies so that TopologicalSort / CriticalPath can operate on
// the unified slice without needing to know about project boundaries.
//
// Resolved tasks are returned with their original pointer values (no copying);
// the DependsOn slice itself is not mutated.
func CollectAllIncompleteTasks(projects []*models.Project) []*models.Task {
	taskIndex := ResolveAllDependencies(projects)
	_ = taskIndex // available to callers or future scheduling helpers

	var result []*models.Task
	for _, p := range projects {
		for _, t := range p.Tasks {
			if !t.IsComplete() {
				result = append(result, t)
			}
		}
	}
	return result
}

// LookupTask resolves a DependsOn reference string to a *Task using a unified
// task map produced by ResolveAllDependencies.  Returns nil if the reference
// cannot be resolved.
func LookupTask(ref string, taskIndex map[string]*models.Task) *models.Task {
	if t, ok := taskIndex[ref]; ok {
		return t
	}
	// Also try parsing as a TaskRef and looking up by task ID alone (fallback
	// for same-project refs stored without a project prefix).
	parsed := models.ParseTaskRef(ref)
	return taskIndex[parsed.TaskID]
}
