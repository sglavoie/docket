package models

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/logging"
)

// MaxNestingDepth is the maximum allowed nesting depth for tasks in a tree.
const MaxNestingDepth = 6

// WouldCreateCycle checks whether setting proposedParentID as the parent of
// taskID would create a cycle. It walks the parent chain from proposedParentID
// upward; if it reaches taskID, a cycle would be created.
func WouldCreateCycle(taskMap map[string]*Task, taskID, proposedParentID string) bool {
	if taskID == proposedParentID {
		return true
	}
	current := proposedParentID
	for current != "" {
		parent, ok := taskMap[current]
		if !ok || parent.ParentID == nil {
			return false
		}
		current = *parent.ParentID
		if current == taskID {
			return true
		}
	}
	return false
}

// BuildTaskTree organizes flat tasks into a tree structure.
// Sets Parent, Children, and Level for each task.
func (p *Project) BuildTaskTree() {
	taskMap := make(map[string]*Task)
	for _, task := range p.Tasks {
		task.Children = []*Task{}
		task.Parent = nil
		task.Level = 0
		taskMap[task.ID] = task
	}

	// Detect and break circular parent references before building parent-child links.
	// A cycle exists when following ParentID pointers leads back to the starting task.
	// This can happen from corrupt data (e.g., manual edits, interrupted saves).
	// Re-run until a full pass finds no cycles (handles multi-task cycles like A→B→C→A).
	for {
		foundCycle := false
		for _, task := range p.Tasks {
			if task.ParentID == nil {
				continue
			}
			visited := make(map[string]bool)
			visited[task.ID] = true
			current := *task.ParentID
			for current != "" {
				if visited[current] {
					// Cycle detected — break it by clearing this task's parent
					logging.Warn("task %q (#%d) is part of a circular parent chain, clearing parent reference", task.Title, task.ShortID)
					task.ParentID = nil
					foundCycle = true
					break
				}
				visited[current] = true
				if parent, ok := taskMap[current]; ok && parent.ParentID != nil {
					current = *parent.ParentID
				} else {
					break
				}
			}
		}
		if !foundCycle {
			break
		}
	}

	for _, task := range p.Tasks {
		if task.ParentID != nil {
			if parent, ok := taskMap[*task.ParentID]; ok {
				task.Parent = parent
				parent.Children = append(parent.Children, task)
			} else {
				logging.Warn("task %q (#%d) references non-existent parent %s, treating as root", task.Title, task.ShortID, *task.ParentID)
			}
		}
	}

	// Remove orphaned dependency IDs (references to deleted tasks).
	// Cross-project qualified references (format "projectID:taskID") are preserved
	// since they point to tasks in other projects not present in this task map.
	for _, task := range p.Tasks {
		if len(task.DependsOn) > 0 {
			valid := task.DependsOn[:0]
			for _, depID := range task.DependsOn {
				if ParseTaskRef(depID).IsCrossProject() {
					valid = append(valid, depID)
				} else if _, ok := taskMap[depID]; ok {
					valid = append(valid, depID)
				} else {
					logging.Warn("task %q (#%d) depends on non-existent task %s, removing orphaned dependency", task.Title, task.ShortID, depID)
				}
			}
			task.DependsOn = valid
		}
	}

	// Detect and break circular dependency chains (defensive, handles corrupt data).
	// Re-run until a full pass finds no cycles (handles multi-task cycles like A→B→C→A).
	var canReachViaDepChain func(startID, targetID string, visited map[string]bool) bool
	canReachViaDepChain = func(startID, targetID string, visited map[string]bool) bool {
		if startID == targetID {
			return true
		}
		if visited[startID] {
			return false
		}
		visited[startID] = true
		t, ok := taskMap[startID]
		if !ok {
			return false
		}
		for _, depID := range t.DependsOn {
			if !ParseTaskRef(depID).IsCrossProject() && canReachViaDepChain(depID, targetID, visited) {
				return true
			}
		}
		return false
	}

	for {
		foundCycle := false
		for _, task := range p.Tasks {
			for i, depID := range task.DependsOn {
				if ParseTaskRef(depID).IsCrossProject() {
					continue
				}
				if canReachViaDepChain(depID, task.ID, make(map[string]bool)) {
					logging.Warn("task %q (#%d) has circular dependency on %s, removing edge", task.Title, task.ShortID, depID)
					task.DependsOn = append(task.DependsOn[:i], task.DependsOn[i+1:]...)
					foundCycle = true
					break
				}
			}
			if foundCycle {
				break
			}
		}
		if !foundCycle {
			break
		}
	}

	// Set levels via DFS (with visited set to guard against any remaining cycles)
	visited := make(map[string]bool, len(p.Tasks))
	var setLevels func(task *Task, level int)
	setLevels = func(task *Task, level int) {
		if visited[task.ID] {
			return
		}
		visited[task.ID] = true
		task.Level = level
		for _, child := range task.Children {
			setLevels(child, level+1)
		}
	}

	for _, task := range p.Tasks {
		if task.IsRoot() {
			setLevels(task, 0)
		}
	}

	// Cap nesting depth: detach tasks that exceed MaxNestingDepth
	for _, task := range p.Tasks {
		if task.Level > MaxNestingDepth && task.Parent != nil {
			logging.Warn("task %q (#%d) exceeds max nesting depth %d, treating as root", task.Title, task.ShortID, MaxNestingDepth)
			// Remove from parent's children
			parent := task.Parent
			for i, child := range parent.Children {
				if child.ID == task.ID {
					parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
					break
				}
			}
			task.Parent = nil
			task.ParentID = nil
			task.Level = 0
			// Re-level this task's subtree from 0.
			// Clear visited entries first so setLevels will traverse them.
			clearVisited(task, visited)
			setLevels(task, 0)
		}
	}
}

// clearVisited removes a task and all its descendants from the visited map
// so setLevels can re-traverse them after detachment.
func clearVisited(task *Task, visited map[string]bool) {
	delete(visited, task.ID)
	for _, child := range task.Children {
		clearVisited(child, visited)
	}
}

// GetSiblings returns all tasks with the same parent as the given task,
// in their slice order. Includes the task itself.
func (p *Project) GetSiblings(task *Task) []*Task {
	var siblings []*Task
	for _, t := range p.Tasks {
		if (task.ParentID == nil && t.ParentID == nil) ||
			(task.ParentID != nil && t.ParentID != nil && *task.ParentID == *t.ParentID) {
			siblings = append(siblings, t)
		}
	}
	return siblings
}

// FindSiblingIndex returns the index of task among its siblings, or -1 if not found.
func (p *Project) FindSiblingIndex(task *Task) int {
	siblings := p.GetSiblings(task)
	for i, s := range siblings {
		if s.ID == task.ID {
			return i
		}
	}
	return -1
}

// findTaskIndex returns the index of the task in the Tasks slice, or -1 if not found.
func (p *Project) findTaskIndex(task *Task) int {
	for i, t := range p.Tasks {
		if t.ID == task.ID {
			return i
		}
	}
	return -1
}

// GetTaskAbove returns the task that appears visually above the given task
// in a tree traversal (for indent operations). Returns nil if task is first.
func (p *Project) GetTaskAbove(task *Task) *Task {
	// Build the visual order by doing a depth-first traversal
	var visualOrder []*Task
	visited := make(map[string]bool)
	var traverse func(t *Task)
	traverse = func(t *Task) {
		if visited[t.ID] {
			return
		}
		visited[t.ID] = true
		visualOrder = append(visualOrder, t)
		for _, child := range t.Children {
			traverse(child)
		}
	}

	// First ensure tree is built
	p.BuildTaskTree()

	// Traverse all roots
	for _, root := range p.GetRootTasks() {
		traverse(root)
	}

	// Find the task and return the one above it
	for i, t := range visualOrder {
		if t.ID == task.ID {
			if i > 0 {
				return visualOrder[i-1]
			}
			return nil
		}
	}
	return nil
}

// IndentTask makes the task a child of the task visually above it.
// Returns an error if the operation is not valid.
func (p *Project) IndentTask(task *Task) error {
	// Get the task above in visual order
	targetParent := p.GetTaskAbove(task)
	if targetParent == nil {
		return fmt.Errorf("cannot indent: no task above to become parent")
	}

	// Validate the indent operation
	if ok, reason := task.CanIndent(targetParent); !ok {
		return fmt.Errorf("cannot indent: %s", reason)
	}

	// Check for cycles before reparenting
	taskMap := make(map[string]*Task, len(p.Tasks))
	for _, t := range p.Tasks {
		taskMap[t.ID] = t
	}
	if WouldCreateCycle(taskMap, task.ID, targetParent.ID) {
		return fmt.Errorf("cannot indent: would create a circular parent reference")
	}

	// Update the parent reference
	task.ParentID = &targetParent.ID
	task.UpdatedAt = time.Now()

	// Rebuild the tree to update computed fields
	p.BuildTaskTree()

	return nil
}

// OutdentTask moves the task to be a sibling of its current parent.
// Returns an error if the operation is not valid.
func (p *Project) OutdentTask(task *Task) error {
	// Validate the outdent operation
	if ok, reason := task.CanOutdent(); !ok {
		return fmt.Errorf("cannot outdent: %s", reason)
	}

	// Get current parent before modifying
	currentParent := task.Parent
	if currentParent == nil {
		return fmt.Errorf("cannot outdent: already at root level")
	}

	// Set new parent to grandparent (or nil if parent is root)
	if currentParent.ParentID != nil {
		task.ParentID = currentParent.ParentID
	} else {
		task.ParentID = nil
	}
	task.UpdatedAt = time.Now()

	// Rebuild the tree to update computed fields
	p.BuildTaskTree()

	return nil
}

// ReorderRootTasksByCompletion reorganizes the Tasks slice so that uncompleted
// root tasks appear before completed root tasks. Child tasks move with their
// parent root task. The relative order within each group is preserved.
func (p *Project) ReorderRootTasksByCompletion() {
	if len(p.Tasks) == 0 {
		return
	}

	// Build the tree to ensure parent-child relationships are set
	p.BuildTaskTree()

	// Collect root tasks with their subtrees (all descendants)
	type rootSubtree struct {
		root  *Task
		tasks []*Task // root + all descendants in original slice order
	}

	// Collect all descendants of a root task (visited check prevents cycles)
	var collectDescendants func(task *Task, collected map[string]bool)
	collectDescendants = func(task *Task, collected map[string]bool) {
		if collected[task.ID] {
			return
		}
		collected[task.ID] = true
		for _, child := range task.Children {
			collectDescendants(child, collected)
		}
	}

	// Process each root task
	var uncompletedSubtrees []rootSubtree
	var completedSubtrees []rootSubtree
	processedTasks := make(map[string]bool)

	for _, task := range p.Tasks {
		if !task.IsRoot() || processedTasks[task.ID] {
			continue
		}

		// Collect all task IDs in this subtree
		subtreeIDs := make(map[string]bool)
		collectDescendants(task, subtreeIDs)

		// Gather tasks in original slice order
		var subtreeTasks []*Task
		for _, t := range p.Tasks {
			if subtreeIDs[t.ID] {
				subtreeTasks = append(subtreeTasks, t)
				processedTasks[t.ID] = true
			}
		}

		subtree := rootSubtree{root: task, tasks: subtreeTasks}
		if task.IsComplete() {
			completedSubtrees = append(completedSubtrees, subtree)
		} else {
			uncompletedSubtrees = append(uncompletedSubtrees, subtree)
		}
	}

	// Rebuild the Tasks slice: uncompleted subtrees first, then completed
	newTasks := make([]*Task, 0, len(p.Tasks))
	for _, subtree := range uncompletedSubtrees {
		newTasks = append(newTasks, subtree.tasks...)
	}
	for _, subtree := range completedSubtrees {
		newTasks = append(newTasks, subtree.tasks...)
	}

	p.Tasks = newTasks
}

// collectSubtree collects a task and all its descendants in depth-first order.
// Uses a visited set to prevent infinite loops on circular references.
func collectSubtree(task *Task) []*Task {
	var result []*Task
	visited := make(map[string]bool)
	var traverse func(t *Task)
	traverse = func(t *Task) {
		if visited[t.ID] {
			return
		}
		visited[t.ID] = true
		result = append(result, t)
		for _, child := range t.Children {
			traverse(child)
		}
	}
	traverse(task)
	return result
}
