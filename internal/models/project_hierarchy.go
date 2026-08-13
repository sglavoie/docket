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
	return wouldCreateCycle(taskMap, nil, taskID, proposedParentID)
}

// wouldCreateCycle is WouldCreateCycle over a projected parent graph. Entries in
// overrides replace a task's stored ParentID, where a present nil value means
// the task is projected as a root; a nil map projects the graph unchanged.
func wouldCreateCycle(taskMap map[string]*Task, overrides map[string]*string, taskID, proposedParentID string) bool {
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
		if !ok {
			return false
		}
		parentID := projectedParentID(parent, overrides)
		if parentID == nil {
			return false
		}
		current = *parentID
		if current == taskID {
			return true
		}
	}
	return false
}

// projectedParentID returns the parent the task would have once overrides are
// applied. A present nil override projects the task as a root.
func projectedParentID(task *Task, overrides map[string]*string) *string {
	if override, ok := overrides[task.ID]; ok {
		return override
	}
	return task.ParentID
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
	_, err := p.ApplyParentAssignments([]ParentAssignment{{Task: task, Parent: newParent}})
	return err
}

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

// ReorderRootTasksByCompletion reorganizes the Tasks slice so that every task
// belonging to an uncompleted root appears before every task belonging to a
// completed root. It is a stable partition: within each of the two groups the
// existing slice order is kept untouched, so a reparent that does not cross the
// completion boundary leaves the persisted order exactly as it was. Gathering
// each root's descendants into one contiguous run would instead resequence
// tasks that the mutation never named.
func (p *Project) ReorderRootTasksByCompletion() {
	if len(p.Tasks) == 0 {
		return
	}

	// Build the tree to ensure parent-child relationships are set
	p.BuildTaskTree()

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

	completedRoot := make(map[string]bool)
	for _, task := range p.Tasks {
		if task != nil && task.IsRoot() {
			indexSubtree(task.ID, task)
			completedRoot[task.ID] = task.IsComplete()
		}
	}

	uncompleted := make([]*Task, 0, len(p.Tasks))
	var completed []*Task
	for _, task := range p.Tasks {
		if task == nil {
			continue
		}
		rootID := rootByTask[task.ID]
		if rootID == "" {
			rootID = task.ID
		}
		if completedRoot[rootID] {
			completed = append(completed, task)
			continue
		}
		uncompleted = append(uncompleted, task)
	}

	p.Tasks = append(uncompleted, completed...)
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
