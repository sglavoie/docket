package models

import (
	"fmt"
	"time"
)

// IsLeaf returns true if this task has no children.
func (t *Task) IsLeaf() bool {
	return len(t.Children) == 0
}

// IsRoot returns true if this task has no parent.
func (t *Task) IsRoot() bool {
	return t.ParentID == nil
}

// GetProgress returns the completion percentage based on children.
// For leaf tasks, returns 100 if complete, 0 otherwise.
// For parent tasks, returns percentage of completed children.
func (t *Task) GetProgress() float64 {
	if len(t.Children) == 0 {
		if t.IsComplete() {
			return 100.0
		}
		return 0.0
	}

	total := 0
	completed := 0
	for _, child := range t.Children {
		total++
		if child.IsComplete() {
			completed++
		}
	}
	if total == 0 {
		return 0.0
	}
	return float64(completed) / float64(total) * 100.0
}

// IncompleteChildren returns the direct children that are not complete
// (Status != Done && != Canceled). Leaf tasks (no children) return nil.
func (t *Task) IncompleteChildren() []*Task {
	var incomplete []*Task
	for _, child := range t.Children {
		if child.Status != StatusDone && child.Status != StatusCanceled {
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

// HasProgressChildren returns true if the task has children.
// Used to determine whether to display progress indicators.
func (t *Task) HasProgressChildren() bool {
	return len(t.Children) > 0
}

// CanAddChild returns true if the given task type can be added as a child.
func (t *Task) CanAddChild(childType TaskType) bool {
	return childType.CanBeChildOf(t.Type)
}

// AddChild adds a task as a child, setting the parent reference.
// Returns an error if the child is nil, the child type is not allowed, the
// nesting depth would be exceeded, or a cycle would be created. The nil
// guard prevents a panic in CanAddChild reading child.Type.
func (t *Task) AddChild(child *Task) error {
	if child == nil {
		return fmt.Errorf("cannot add nil child")
	}
	if !t.CanAddChild(child.Type) {
		return fmt.Errorf("cannot add %s as child of %s", child.Type, t.Type)
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

// CanPromote checks if this task can be promoted to the next higher type.
// Returns (allowed, reason) where reason explains why if not allowed.
func (t *Task) CanPromote() (bool, string) {
	newType := t.Type.Promote()

	// Already at top of hierarchy
	if newType == t.Type {
		return false, "already at top of hierarchy"
	}

	// Check if new type is valid under current parent
	if t.Parent != nil {
		if !newType.CanBeChildOf(t.Parent.Type) {
			return false, newType.String() + " cannot be child of " + t.Parent.Type.String()
		}
	}

	// Check if all children are valid under new type
	for _, child := range t.Children {
		if !child.Type.CanBeChildOf(newType) {
			return false, child.Type.String() + " cannot be child of " + newType.String()
		}
	}

	return true, ""
}

// CanDemote checks if this task can be demoted to the next lower type.
// Returns (allowed, reason) where reason explains why if not allowed.
func (t *Task) CanDemote() (bool, string) {
	newType := t.Type.Demote()

	// Already at bottom of hierarchy
	if newType == t.Type {
		return false, "already at bottom of hierarchy"
	}

	// Check if new type can have children (if we have any)
	if len(t.Children) > 0 && !newType.CanHaveChildren() {
		return false, newType.String() + " cannot have children"
	}

	// Check if new type is valid under current parent
	if t.Parent != nil {
		if !newType.CanBeChildOf(t.Parent.Type) {
			return false, newType.String() + " cannot be child of " + t.Parent.Type.String()
		}
	}

	// Check if all children are valid under new type
	for _, child := range t.Children {
		if !child.Type.CanBeChildOf(newType) {
			return false, child.Type.String() + " cannot be child of " + newType.String()
		}
	}

	return true, ""
}

// CanCycleLeafType checks if this task can cycle its leaf type.
// Returns (allowed, reason) where reason explains why if not allowed.
func (t *Task) CanCycleLeafType(left bool) (bool, string) {
	if !t.Type.IsLeafType() {
		return false, "not a leaf type"
	}

	var newType TaskType
	if left {
		newType = t.Type.CycleLeafLeft()
	} else {
		newType = t.Type.CycleLeafRight()
	}

	// Check if new type is valid under current parent
	if t.Parent != nil {
		if !newType.CanBeChildOf(t.Parent.Type) {
			return false, newType.String() + " cannot be child of " + t.Parent.Type.String()
		}
	}

	return true, ""
}

// CanIndent checks if this task can become a child of the given target parent.
// Returns (allowed, reason) where reason explains why if not allowed.
func (t *Task) CanIndent(targetParent *Task) (bool, string) {
	if targetParent == nil {
		return false, "no target parent"
	}

	// Check if target can have children
	if !targetParent.Type.CanHaveChildren() {
		return false, targetParent.Type.String() + " cannot have children"
	}

	// Check if this task's type can be a child of target
	if !t.Type.CanBeChildOf(targetParent.Type) {
		return false, t.Type.String() + " cannot be child of " + targetParent.Type.String()
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

	// If parent has a grandparent, check if our type is valid under grandparent
	if t.Parent.Parent != nil {
		if !t.Type.CanBeChildOf(t.Parent.Parent.Type) {
			return false, t.Type.String() + " cannot be child of " + t.Parent.Parent.Type.String()
		}
	}

	// Children stay under this task, so they remain valid

	return true, ""
}

// Promote changes this task's type to the next higher type in the hierarchy.
// Returns an error if promotion is not allowed.
func (t *Task) Promote() error {
	if allowed, reason := t.CanPromote(); !allowed {
		return fmt.Errorf("cannot promote: %s", reason)
	}

	t.Type = t.Type.Promote()
	t.ClearTypeAttrs() // Clear type-specific attributes when type changes
	t.UpdatedAt = time.Now()
	return nil
}

// Demote changes this task's type to the next lower type in the hierarchy.
// Returns an error if demotion is not allowed.
func (t *Task) Demote() error {
	if allowed, reason := t.CanDemote(); !allowed {
		return fmt.Errorf("cannot demote: %s", reason)
	}

	t.Type = t.Type.Demote()
	t.ClearTypeAttrs() // Clear type-specific attributes when type changes
	t.UpdatedAt = time.Now()
	return nil
}

// CycleLeafType cycles this task's type among leaf types (Task, Bug, Idea).
// If left is true, cycles left (Task -> Idea -> Bug -> Task).
// If left is false, cycles right (Task -> Bug -> Idea -> Task).
// Returns an error if cycling is not allowed.
func (t *Task) CycleLeafType(left bool) error {
	if allowed, reason := t.CanCycleLeafType(left); !allowed {
		return fmt.Errorf("cannot cycle type: %s", reason)
	}

	if left {
		t.Type = t.Type.CycleLeafLeft()
	} else {
		t.Type = t.Type.CycleLeafRight()
	}
	t.ClearTypeAttrs() // Clear type-specific attributes when type changes
	t.UpdatedAt = time.Now()
	return nil
}
