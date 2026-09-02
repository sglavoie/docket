package models

import (
	"fmt"
	"time"
)

// ParentAssignment names one parent edge: Task becomes a child of Parent, or a
// root when Parent is nil. Both non-nil tasks must be the canonical objects
// owned by the project the assignment is applied to.
type ParentAssignment struct {
	Task   *Task
	Parent *Task
}

// ApplyParentAssignments validates every requested parent edge against the one
// hierarchy the whole batch would produce, then applies it atomically. Repeated
// assignments naming the same parent for a task collapse to the first; two
// assignments naming different parents for one task are rejected. Because
// cycles and nesting depth are measured on the projected graph rather than the
// current one, a hierarchy inversion that is only legal once every edge lands is
// accepted. A rejected batch mutates nothing.
//
// Returns the number of tasks whose parent actually changed. Project task order
// is never touched, so ordering stays with the caller.
func (p *Project) ApplyParentAssignments(assignments []ParentAssignment) (int, error) {
	if p == nil {
		return 0, fmt.Errorf("project is nil")
	}

	taskMap := make(map[string]*Task, len(p.Tasks))
	for _, candidate := range p.Tasks {
		if candidate == nil {
			continue
		}
		if _, duplicate := taskMap[candidate.ID]; !duplicate {
			taskMap[candidate.ID] = candidate
		}
	}

	requests, err := canonicalParentAssignments(taskMap, assignments)
	if err != nil {
		return 0, err
	}
	if len(requests) == 0 {
		return 0, nil
	}

	overrides := make(map[string]*string, len(requests))
	for _, request := range requests {
		if request.Parent == nil {
			overrides[request.Task.ID] = nil
			continue
		}
		parentID := request.Parent.ID
		overrides[request.Task.ID] = &parentID
	}

	// Every cycle the batch could introduce runs through at least one assigned
	// edge, so walking upward from each new parent finds all of them. A cycle
	// among untouched edges is left to the depth and height walks below, which
	// report it exactly as a single reparent always has.
	for _, request := range requests {
		if request.Parent == nil {
			continue
		}
		if wouldCreateCycle(taskMap, overrides, request.Task.ID, request.Parent.ID) {
			return 0, fmt.Errorf("would create a circular parent reference")
		}
	}

	children := make(map[string][]string, len(p.Tasks))
	for _, candidate := range p.Tasks {
		if candidate == nil {
			continue
		}
		if parentID := projectedParentID(candidate, overrides); parentID != nil {
			children[*parentID] = append(children[*parentID], candidate.ID)
		}
	}

	if err := validateProjectedAssignmentDepth(taskMap, overrides, children, requests); err != nil {
		return 0, err
	}

	changed := 0
	now := time.Now()
	for _, request := range requests {
		if taskHasParent(request.Task, request.Parent) {
			continue
		}
		if request.Parent == nil {
			request.Task.ParentID = nil
		} else {
			parentID := request.Parent.ID
			request.Task.ParentID = &parentID
		}
		request.Task.UpdatedAt = now
		changed++
	}
	p.BuildTaskTree()
	return changed, nil
}

// canonicalParentAssignments rejects any assignment naming a task or parent the
// project does not own, then collapses repeated assignments for one task to the
// first. Two assignments naming different parents for the same task are a
// conflict rather than a last-writer-wins overwrite.
func canonicalParentAssignments(taskMap map[string]*Task, assignments []ParentAssignment) ([]ParentAssignment, error) {
	requests := make([]ParentAssignment, 0, len(assignments))
	requestIndex := make(map[string]int, len(assignments))
	for _, assignment := range assignments {
		task := assignment.Task
		if task == nil {
			return nil, fmt.Errorf("task is nil")
		}
		if err := requireCanonicalTask(taskMap, task, "task"); err != nil {
			return nil, err
		}
		if parent := assignment.Parent; parent != nil {
			if err := requireCanonicalTask(taskMap, parent, "parent"); err != nil {
				return nil, err
			}
			if task.ID == parent.ID {
				return nil, fmt.Errorf("task cannot be its own parent")
			}
		}
		if at, seen := requestIndex[task.ID]; seen {
			if !sameAssignedParent(requests[at].Parent, assignment.Parent) {
				return nil, fmt.Errorf("task %q was assigned two different parents", task.ID)
			}
			continue
		}
		requestIndex[task.ID] = len(requests)
		requests = append(requests, assignment)
	}
	return requests, nil
}

// requireCanonicalTask reports whether the project owns this exact task object,
// since a copy carrying the same ID would mutate a graph nothing else reads.
func requireCanonicalTask(taskMap map[string]*Task, task *Task, role string) error {
	switch canonical := taskMap[task.ID]; {
	case canonical == nil:
		return fmt.Errorf("%s %q is not in project", role, task.ID)
	case canonical != task:
		return fmt.Errorf("%s %q does not match project task", role, task.ID)
	}
	return nil
}

// validateProjectedAssignmentDepth measures each moved subtree against the graph
// the whole batch produces, so an inversion that is only legal once every edge
// lands is judged on its final depth rather than an intermediate one.
func validateProjectedAssignmentDepth(taskMap map[string]*Task, overrides map[string]*string, children map[string][]string, requests []ParentAssignment) error {
	for _, request := range requests {
		newDepth := 0
		if request.Parent != nil {
			parentDepth, err := taskHierarchyDepth(taskMap, overrides, request.Parent.ID)
			if err != nil {
				return err
			}
			newDepth = parentDepth + 1
		}
		height, err := taskSubtreeHeight(request.Task.ID, children, make(map[string]bool))
		if err != nil {
			return err
		}
		if newDepth+height > MaxNestingDepth {
			return fmt.Errorf("subtree would exceed maximum nesting depth of %d levels", MaxNestingDepth)
		}
	}
	return nil
}

// sameAssignedParent reports whether two assignments for one task name the same
// parent.
func sameAssignedParent(a, b *Task) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ID == b.ID
}

// taskHasParent reports whether the task already holds the assigned parent edge.
func taskHasParent(task *Task, parent *Task) bool {
	if parent == nil {
		return task.ParentID == nil
	}
	return task.ParentID != nil && *task.ParentID == parent.ID
}

// taskHierarchyDepth returns the root-relative depth of taskID without relying
// on computed Level fields, which may be stale before a project rebuild.
// Entries in overrides replace a task's stored ParentID, where a present nil
// value means the task is projected as a root; a nil map measures the stored
// graph.
func taskHierarchyDepth(taskMap map[string]*Task, overrides map[string]*string, taskID string) (int, error) {
	depth := 0
	seen := make(map[string]bool)
	currentID := taskID
	for {
		if seen[currentID] {
			return 0, fmt.Errorf("task %q has a circular parent reference", taskID)
		}
		seen[currentID] = true
		current := taskMap[currentID]
		if current == nil {
			return 0, fmt.Errorf("task %q is not in project", currentID)
		}
		parentID := projectedParentID(current, overrides)
		if parentID == nil {
			return depth, nil
		}
		if taskMap[*parentID] == nil {
			return 0, fmt.Errorf("task %q references missing parent %q", current.ID, *parentID)
		}
		depth++
		currentID = *parentID
	}
}

// taskSubtreeHeight returns the maximum descendant distance below taskID.
func taskSubtreeHeight(taskID string, children map[string][]string, visiting map[string]bool) (int, error) {
	if visiting[taskID] {
		return 0, fmt.Errorf("task %q has a circular child reference", taskID)
	}
	visiting[taskID] = true
	defer delete(visiting, taskID)

	height := 0
	for _, childID := range children[taskID] {
		childHeight, err := taskSubtreeHeight(childID, children, visiting)
		if err != nil {
			return 0, err
		}
		if childHeight+1 > height {
			height = childHeight + 1
		}
	}
	return height, nil
}
