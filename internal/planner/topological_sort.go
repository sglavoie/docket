package planner

import (
	"fmt"
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

// TopologicalSort returns incomplete tasks from the input slice in topological
// order using Kahn's algorithm on DependsOn edges.
//
// Complete tasks in the input are excluded from the result but are considered
// satisfied predecessors (a dependency on a complete task does not block the
// dependent).  Cross-project dependency refs (format "projectID:taskID") whose
// project-side task is not present in the input slice are treated as satisfied.
// Nil entries and tasks with empty IDs are silently skipped so callers can pass
// loosely-assembled cross-project slices without manually filtering them.
//
// Returns an error if a dependency cycle exists among the incomplete tasks.
func TopologicalSort(tasks []*models.Task) ([]*models.Task, error) {
	// Build map of incomplete tasks only.
	taskMap := make(map[string]*models.Task, len(tasks))
	for _, t := range tasks {
		if t == nil || t.ID == "" {
			continue
		}
		if !t.IsComplete() {
			taskMap[taskGraphKey(t)] = t
		}
	}

	// Build in-degree count and reverse-adjacency list (pred -> successors).
	// Only edges where the predecessor is also in taskMap (incomplete) count
	// toward in-degree; completed predecessors are treated as already satisfied.
	inDegree := make(map[string]int, len(taskMap))
	// adj maps predecessor task ID -> list of dependent task IDs.
	adj := make(map[string][]string, len(taskMap))
	for id := range taskMap {
		inDegree[id] = 0
	}
	for _, t := range taskMap {
		for _, depRef := range t.DependsOn {
			if predKey, ok := dependencyTaskKey(depRef, t, taskMap); ok {
				tKey := taskGraphKey(t)
				inDegree[tKey]++
				adj[predKey] = append(adj[predKey], tKey)
			}
		}
	}

	// Seed queue with zero in-degree nodes; sort for deterministic output.
	queue := make([]*models.Task, 0, len(taskMap))
	for id, t := range taskMap {
		if inDegree[id] == 0 {
			queue = append(queue, t)
		}
	}
	sortByID(queue)

	result := make([]*models.Task, 0, len(taskMap))
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		result = append(result, t)

		successors := adj[taskGraphKey(t)]
		sort.Strings(successors) // deterministic successor processing
		added := queue[:0:0]
		for _, succID := range successors {
			inDegree[succID]--
			if inDegree[succID] == 0 {
				added = append(added, taskMap[succID])
			}
		}
		if len(added) > 0 {
			sortByID(added)
			queue = append(queue, added...)
		}
	}

	if len(result) != len(taskMap) {
		return nil, fmt.Errorf("cycle detected in task dependencies")
	}
	return result, nil
}

// topologicalSortByPriority is like TopologicalSort but breaks ties among
// zero-in-degree nodes by priority (ascending = higher priority) then graph key.
func topologicalSortByPriority(tasks []*models.Task) ([]*models.Task, error) {
	taskMap := make(map[string]*models.Task, len(tasks))
	for _, t := range tasks {
		if t == nil || t.ID == "" {
			continue
		}
		if !t.IsComplete() {
			taskMap[taskGraphKey(t)] = t
		}
	}

	inDegree := make(map[string]int, len(taskMap))
	adj := make(map[string][]string, len(taskMap))
	for id := range taskMap {
		inDegree[id] = 0
	}
	for _, t := range taskMap {
		for _, depRef := range t.DependsOn {
			if predKey, ok := dependencyTaskKey(depRef, t, taskMap); ok {
				tKey := taskGraphKey(t)
				inDegree[tKey]++
				adj[predKey] = append(adj[predKey], tKey)
			}
		}
	}

	queue := make([]*models.Task, 0, len(taskMap))
	for id, t := range taskMap {
		if inDegree[id] == 0 {
			queue = append(queue, t)
		}
	}
	sortByPriorityThenID(queue)

	result := make([]*models.Task, 0, len(taskMap))
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		result = append(result, t)

		successors := adj[taskGraphKey(t)]
		sort.Strings(successors)
		var added []*models.Task
		for _, succID := range successors {
			inDegree[succID]--
			if inDegree[succID] == 0 {
				added = append(added, taskMap[succID])
			}
		}
		if len(added) > 0 {
			sortByPriorityThenID(added)
			queue = append(queue, added...)
			sortByPriorityThenID(queue)
		}
	}

	if len(result) != len(taskMap) {
		return nil, fmt.Errorf("cycle detected in task dependencies")
	}
	return result, nil
}

// effectivePriority returns the task's priority, substituting PriorityDefault
// for the zero value. Priority 0 means "unset" in the model (parsed tasks
// without an explicit priority); without this remap they would sort *before*
// PriorityMin (=1, the most-critical bucket) in SerialSchedule.
func effectivePriority(p int) int {
	if p == 0 {
		return models.PriorityDefault
	}
	return p
}

// sortByPriorityThenID sorts tasks by priority ascending (lower = higher
// priority), then by graph key for determinism. Priority 0 is treated as
// PriorityDefault so unset tasks do not sort ahead of critical ones.
func sortByPriorityThenID(tasks []*models.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		pi, pj := effectivePriority(tasks[i].Priority), effectivePriority(tasks[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return taskGraphKey(tasks[i]) < taskGraphKey(tasks[j])
	})
}

// sortByID sorts a slice of tasks by ID for deterministic ordering.
func sortByID(tasks []*models.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		return taskGraphKey(tasks[i]) < taskGraphKey(tasks[j])
	})
}

// TaskKey returns the stable project-qualified identity used by cross-project
// planning graphs and renderer indexes. It falls back to the bare task ID only
// for projectless inputs retained by project-scoped compatibility callers.
func TaskKey(t *models.Task) string {
	if t == nil {
		return ""
	}
	if t.Project != nil {
		if t.Project.ID != "" {
			return t.Project.ID + ":" + t.ID
		}
		if t.Project.Slug != "" {
			return t.Project.Slug + ":" + t.ID
		}
	}
	return t.ID
}

func taskGraphKey(t *models.Task) string { return TaskKey(t) }

// dependencyTaskKey resolves a dependency reference against the local task map.
// Unqualified refs may match by parsed task ID. Qualified refs must match an
// exact local ID or a task whose Project ID/slug matches the ref prefix.
// Unqualified refs from project-scoped tasks only match tasks in that project;
// otherwise unresolved refs are treated as already satisfied by the planner's
// local graph.
func dependencyTaskKey(depRef string, dependent *models.Task, taskMap map[string]*models.Task) (string, bool) {
	ref := models.ParseTaskRef(depRef)
	if ref.ProjectID != "" {
		if _, ok := taskMap[depRef]; ok {
			return depRef, true
		}
		for key, t := range taskMap {
			if t == nil || t.ID != ref.TaskID || t.Project == nil {
				continue
			}
			if t.Project.ID == ref.ProjectID || t.Project.Slug == ref.ProjectID {
				return key, true
			}
		}
		return "", false
	}
	if dependent != nil && dependent.Project != nil {
		hasProjectScope := false
		if dependent.Project.ID != "" {
			hasProjectScope = true
			if _, ok := taskMap[dependent.Project.ID+":"+ref.TaskID]; ok {
				return dependent.Project.ID + ":" + ref.TaskID, true
			}
		}
		if dependent.Project.Slug != "" {
			hasProjectScope = true
			if _, ok := taskMap[dependent.Project.Slug+":"+ref.TaskID]; ok {
				return dependent.Project.Slug + ":" + ref.TaskID, true
			}
		}
		if hasProjectScope {
			return "", false
		}
	}
	if _, ok := taskMap[ref.TaskID]; ok {
		return ref.TaskID, true
	}
	return "", false
}
