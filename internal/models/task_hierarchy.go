package models

import (
	"fmt"
)

// IsRoot returns true if this task has no parent.
func (t *Task) IsRoot() bool {
	return t.ParentID == nil
}

// IncompleteChildren returns the direct children that are not in a terminal
// workflow state. Leaf tasks (no children) return nil.
func (t *Task) IncompleteChildren() []*Task {
	var incomplete []*Task
	for _, child := range t.Children {
		if !child.IsTerminalState() {
			incomplete = append(incomplete, child)
		}
	}
	return incomplete
}

// HasIncompleteChildren reports whether the task has at least one direct child
// that is not complete.
func (t *Task) HasIncompleteChildren() bool {
	return len(t.IncompleteChildren()) > 0
}

// CountChildren returns the total number of descendants.
func (t *Task) CountChildren() int {
	count := len(t.Children)
	for _, child := range t.Children {
		count += child.CountChildren()
	}
	return count
}

// CountCompletedChildren returns the number of completed descendants.
func (t *Task) CountCompletedChildren() int {
	count := 0
	for _, child := range t.Children {
		if child.IsComplete() {
			count++
		}
		count += child.CountCompletedChildren()
	}
	return count
}

// AddChild adds a task as a child, setting the parent reference.
// Returns an error if the child is nil, the nesting depth would be exceeded,
// or a cycle would be created.
func (t *Task) AddChild(child *Task) error {
	if child == nil {
		return fmt.Errorf("cannot add nil child")
	}
	if t.Level+1 > MaxNestingDepth {
		return fmt.Errorf("cannot add child: would exceed maximum nesting depth of %d levels", MaxNestingDepth)
	}
	// When the child already has descendants, attaching it pushes the whole
	// subtree down by (t.Level+1 - child.Level) levels. Reject if the deepest
	// descendant would land past MaxNestingDepth; otherwise BuildTaskTree
	// would later detach those descendants and silently restructure the tree.
	subtreeDepth := maxDescendantDepth(child) - child.Level
	if t.Level+1+subtreeDepth > MaxNestingDepth {
		return fmt.Errorf("cannot add child: subtree would exceed maximum nesting depth of %d levels", MaxNestingDepth)
	}
	// Cycle check: walk t's ancestor chain; if child appears, adding it would
	// create a circular reference.
	if t.ID == child.ID {
		return fmt.Errorf("cannot add child: task cannot be its own parent")
	}
	for cur := t.Parent; cur != nil; cur = cur.Parent {
		if cur.ID == child.ID {
			return fmt.Errorf("cannot add child: would create a circular parent reference")
		}
	}
	child.ParentID = &t.ID
	child.Parent = t
	child.Level = t.Level + 1
	t.Children = append(t.Children, child)
	return nil
}

// CanIndent checks if this task can become a child of the given target parent.
// Returns (allowed, reason) where reason explains why if not allowed.
func (t *Task) CanIndent(targetParent *Task) (bool, string) {
	if targetParent == nil {
		return false, "no target parent"
	}

	// Check nesting depth: the task (and its subtree) would be pushed one level
	// deeper. Verify that the deepest descendant would not exceed MaxNestingDepth.
	if targetParent.Level+1 > MaxNestingDepth {
		return false, fmt.Sprintf("would exceed maximum nesting depth of %d levels", MaxNestingDepth)
	}
	subtreeDepth := maxDescendantDepth(t) - t.Level
	if targetParent.Level+1+subtreeDepth > MaxNestingDepth {
		return false, fmt.Sprintf("subtree would exceed maximum nesting depth of %d levels", MaxNestingDepth)
	}

	// Check if all children remain valid (they stay under this task, so no change)
	// Children keep the same parent type, so they're still valid

	return true, ""
}

// maxDescendantDepth returns the maximum Level value among a task and all its descendants.
func maxDescendantDepth(t *Task) int {
	maxLevel := t.Level
	for _, child := range t.Children {
		if d := maxDescendantDepth(child); d > maxLevel {
			maxLevel = d
		}
	}
	return maxLevel
}

// CanOutdent checks if this task can move to be a sibling of its current parent.
// Returns (allowed, reason) where reason explains why if not allowed.
func (t *Task) CanOutdent() (bool, string) {
	// Must have a parent to outdent
	if t.Parent == nil {
		return false, "already at root level"
	}

	return true, ""
}
