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
//
// A visited set guards against pre-existing cycles in taskMap that do not
// involve taskID (e.g. A->B->C->A). Without it, walking such a chain when
// proposedParentID is part of the cycle would loop forever.
func WouldCreateCycle(taskMap map[string]*Task, taskID, proposedParentID string) bool {
	if taskID == proposedParentID {
		return true
	}
	visited := make(map[string]bool)
	current := proposedParentID
	for current != "" {
		if visited[current] {
			return false
		}
		visited[current] = true
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
// Cross-project refs are validated in storage/markdown.go LoadProject()
func (p *Project) BuildTaskTree() {
	taskMap := make(map[string]*Task, len(p.Tasks))
	for _, task := range p.Tasks {
		if task == nil {
			continue
		}
		task.Children = []*Task{}
		task.Parent = nil
		task.Level = 0
		taskMap[task.ID] = task
	}

	breakParentCycles(p.Tasks, taskMap)
	linkParentChild(p.Tasks, taskMap)

	// Set levels via DFS (with a visited set to guard against any remaining
	// cycles). Detach the first node that exceeds the supported depth while it is
	// visited, then continue from that node as a new root. Filtering every
	// parent's adjacency list once afterward avoids the former repeated child
	// slice scans when many nodes overflowed at the same depth.
	visited := make(map[string]bool, len(p.Tasks))
	detached := make(map[string]bool)
	setLevels := func(task *Task, level int) {}
	setLevels = func(task *Task, level int) {
		if task == nil {
			return
		}
		if visited[task.ID] {
			return
		}
		visited[task.ID] = true
		if level > MaxNestingDepth && task.Parent != nil {
			logging.Warn("task %q (#%d) exceeds max nesting depth %d, treating as root", task.Title, task.ShortID, MaxNestingDepth)
			detached[task.ID] = true
			task.Parent = nil
			task.ParentID = nil
			level = 0
		}
		task.Level = level
		for _, child := range task.Children {
			setLevels(child, level+1)
		}
	}

	for _, task := range p.Tasks {
		if task != nil && task.IsRoot() {
			setLevels(task, 0)
		}
	}

	if len(detached) > 0 {
		for _, parent := range p.Tasks {
			if parent == nil || len(parent.Children) == 0 {
				continue
			}
			kept := parent.Children[:0]
			for _, child := range parent.Children {
				if !detached[child.ID] {
					kept = append(kept, child)
				}
			}
			parent.Children = kept
		}
	}
}

// breakParentCycles detects and breaks circular parent references in O(n). A
// parent graph has at most one outgoing edge per task, so a three-color DFS can
// remove the edge that closes each cycle without walking the same parent chain
// once from every task.
func breakParentCycles(tasks []*Task, taskMap map[string]*Task) {
	const (
		unvisited uint8 = iota
		visiting
		visited
	)
	state := make(map[string]uint8, len(taskMap))
	var visit func(*Task)
	visit = func(task *Task) {
		if task == nil || state[task.ID] == visited {
			return
		}
		state[task.ID] = visiting
		if task.ParentID != nil {
			if parent := taskMap[*task.ParentID]; parent != nil {
				switch state[parent.ID] {
				case unvisited:
					visit(parent)
				case visiting:
					logging.Warn("task %q (#%d) is part of a circular parent chain, clearing parent reference", task.Title, task.ShortID)
					task.ParentID = nil
				case visited:
				}
			}
		}
		state[task.ID] = visited
	}
	for _, task := range tasks {
		if task != nil && state[task.ID] == unvisited {
			visit(task)
		}
	}
}

// linkParentChild builds parent-child links from ParentID pointers.
// Tasks referencing non-existent parents are treated as roots.
func linkParentChild(tasks []*Task, taskMap map[string]*Task) {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if task.ParentID == nil {
			continue
		}
		if parent, ok := taskMap[*task.ParentID]; ok {
			task.Parent = parent
			parent.Children = append(parent.Children, task)
		} else {
			logging.Warn("task %q (#%d) references non-existent parent %s, treating as root", task.Title, task.ShortID, *task.ParentID)
			task.ParentID = nil
		}
	}
}

// GetSiblings returns all tasks with the same parent as the given task,
// in their slice order. Includes the task itself.
func (p *Project) GetSiblings(task *Task) []*Task {
	if task == nil {
		return nil
	}
	task = p.GetTaskByID(task.ID)
	if task == nil {
		return nil
	}
	var siblings []*Task
	for _, t := range p.Tasks {
		if t == nil {
			continue
		}
		if (task.ParentID == nil && t.ParentID == nil) ||
			(task.ParentID != nil && t.ParentID != nil && *task.ParentID == *t.ParentID) {
			siblings = append(siblings, t)
		}
	}
	return siblings
}

// FindSiblingIndex returns the index of task among its siblings, or -1 if not found.
func (p *Project) FindSiblingIndex(task *Task) int {
	if task == nil {
		return -1
	}
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
	if task == nil {
		return -1
	}
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
	if err := p.ReparentTask(task, targetParent); err != nil {
		return fmt.Errorf("cannot indent: %w", err)
	}
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

	if err := p.ReparentTask(task, currentParent.Parent); err != nil {
		return fmt.Errorf("cannot outdent: %w", err)
	}
	return nil
}

// ReparentTask moves task under newParent, or promotes it to a root when
// newParent is nil. Both non-nil tasks must be the canonical objects owned by
// this project. Validation is completed before mutation so a rejected request
// cannot partially rewrite the tree.
func (p *Project) ReparentTask(task, newParent *Task) error {
	if p == nil {
		return fmt.Errorf("project is nil")
	}
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	canonicalTask := p.GetTaskByID(task.ID)
	if canonicalTask == nil {
		return fmt.Errorf("task %q is not in project", task.ID)
	}
	if canonicalTask != task {
		return fmt.Errorf("task %q does not match project task", task.ID)
	}

	taskMap := make(map[string]*Task, len(p.Tasks))
	children := make(map[string][]string, len(p.Tasks))
	for _, candidate := range p.Tasks {
		if candidate == nil {
			continue
		}
		taskMap[candidate.ID] = candidate
		if candidate.ParentID != nil {
			children[*candidate.ParentID] = append(children[*candidate.ParentID], candidate.ID)
		}
	}

	newDepth := 0
	if newParent != nil {
		canonicalParent := p.GetTaskByID(newParent.ID)
		if canonicalParent == nil {
			return fmt.Errorf("parent %q is not in project", newParent.ID)
		}
		if canonicalParent != newParent {
			return fmt.Errorf("parent %q does not match project task", newParent.ID)
		}
		if task.ID == newParent.ID {
			return fmt.Errorf("task cannot be its own parent")
		}
		if WouldCreateCycle(taskMap, task.ID, newParent.ID) {
			return fmt.Errorf("would create a circular parent reference")
		}
		parentDepth, err := taskHierarchyDepth(taskMap, newParent.ID)
		if err != nil {
			return err
		}
		newDepth = parentDepth + 1
	}

	height, err := taskSubtreeHeight(task.ID, children, make(map[string]bool))
	if err != nil {
		return err
	}
	if newDepth+height > MaxNestingDepth {
		return fmt.Errorf("subtree would exceed maximum nesting depth of %d levels", MaxNestingDepth)
	}

	if newParent == nil {
		task.ParentID = nil
	} else {
		parentID := newParent.ID
		task.ParentID = &parentID
	}
	task.UpdatedAt = time.Now()
	p.BuildTaskTree()
	return nil
}

// taskHierarchyDepth returns the root-relative depth of taskID without relying
// on computed Level fields, which may be stale before a project rebuild.
func taskHierarchyDepth(taskMap map[string]*Task, taskID string) (int, error) {
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
		if current.ParentID == nil {
			return depth, nil
		}
		if taskMap[*current.ParentID] == nil {
			return 0, fmt.Errorf("task %q references missing parent %q", current.ID, *current.ParentID)
		}
		depth++
		currentID = *current.ParentID
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

// ReorderRootTasksByCompletion reorganizes the Tasks slice so that uncompleted
// root tasks appear before completed root tasks. Child tasks move with their
// parent root task. The relative order within each group is preserved.
func (p *Project) ReorderRootTasksByCompletion() {
	if len(p.Tasks) == 0 {
		return
	}

	// Build the tree to ensure parent-child relationships are set
	p.BuildTaskTree()

	// Collect root tasks with their subtrees (all descendants).
	type rootSubtree struct {
		root  *Task
		tasks []*Task // root + all descendants in original slice order
	}

	// Index every task by its root once. The previous implementation built a
	// descendant set for each root and then rescanned the full task slice per
	// root, which was O(n²) for a project containing many root tasks.
	rootByTask := make(map[string]string, len(p.Tasks))
	var indexSubtree func(rootID string, task *Task)
	indexSubtree = func(rootID string, task *Task) {
		if task == nil || rootByTask[task.ID] != "" {
			return
		}
		rootByTask[task.ID] = rootID
		for _, child := range task.Children {
			indexSubtree(rootID, child)
		}
	}

	var roots []*Task
	for _, task := range p.Tasks {
		if task != nil && task.IsRoot() {
			roots = append(roots, task)
			indexSubtree(task.ID, task)
		}
	}

	subtrees := make(map[string][]*Task, len(roots))
	for _, task := range p.Tasks {
		if task == nil {
			continue
		}
		rootID := rootByTask[task.ID]
		if rootID == "" {
			rootID = task.ID
		}
		subtrees[rootID] = append(subtrees[rootID], task)
	}

	var uncompletedSubtrees []rootSubtree
	var completedSubtrees []rootSubtree
	for _, root := range roots {
		subtree := rootSubtree{root: root, tasks: subtrees[root.ID]}
		if root.IsComplete() {
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

// ReconcileParentCompletion uncompletes any completed task that has an
// incomplete child, so that a parent (task with children) is never left
// complete while a child is still open. The fixpoint loop repeats until a full
// pass makes no changes, so reopening a parent propagates upward to
// grandparents. Returns the IDs of the tasks reopened (empty when the tree was
// already consistent). Assumes an up-to-date tree (t.Children populated via
// BuildTaskTree). Callers that record a targeted undo snapshot should add the
// returned IDs to their undo task set so a single undo reverts the cascade
// atomically.
func (p *Project) ReconcileParentCompletion() []string {
	var reopened []string
	for changed := true; changed; {
		changed = false
		for _, t := range p.Tasks {
			if t.IsTerminalState() && t.HasIncompleteChildren() {
				t.Uncomplete()
				reopened = append(reopened, t.ID)
				changed = true
			}
		}
	}
	return reopened
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
