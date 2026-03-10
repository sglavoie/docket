package models

import (
	"fmt"
	"time"
)

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

	for _, task := range p.Tasks {
		if task.ParentID != nil {
			if parent, ok := taskMap[*task.ParentID]; ok {
				task.Parent = parent
				parent.Children = append(parent.Children, task)
			}
		}
	}

	// Set levels via BFS
	var setLevels func(task *Task, level int)
	setLevels = func(task *Task, level int) {
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
	var traverse func(t *Task)
	traverse = func(t *Task) {
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

	// Collect all descendants of a root task
	var collectDescendants func(task *Task, collected map[string]bool)
	collectDescendants = func(task *Task, collected map[string]bool) {
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
func collectSubtree(task *Task) []*Task {
	var result []*Task
	var traverse func(t *Task)
	traverse = func(t *Task) {
		result = append(result, t)
		for _, child := range t.Children {
			traverse(child)
		}
	}
	traverse(task)
	return result
}
