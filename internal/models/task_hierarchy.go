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

// IsNote returns true if this is a Note type task.
func (t *Task) IsNote() bool {
	return t.Type == TaskTypeNote
}

// GetProgress returns the completion percentage based on children.
// For leaf tasks, returns 100 if complete, 0 otherwise.
// For parent tasks, returns percentage of completed children.
// Notes are excluded from progress calculation as they are not completable items.
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
		if child.IsNote() {
			continue
		}
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

// HasProgressChildren returns true if the task has non-note children.
// Used to determine whether to display progress indicators.
func (t *Task) HasProgressChildren() bool {
	for _, child := range t.Children {
		if !child.IsNote() {
			return true
		}
	}
	return false
}

// CanAddChild returns true if the given task type can be added as a child.
func (t *Task) CanAddChild(childType TaskType) bool {
	return childType.CanBeChildOf(t.Type)
}

// AddChild adds a task as a child, setting the parent reference.
func (t *Task) AddChild(child *Task) {
	child.ParentID = &t.ID
	child.Parent = t
	child.Level = t.Level + 1
	t.Children = append(t.Children, child)
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

	// Check if all children remain valid (they stay under this task, so no change)
	// Children keep the same parent type, so they're still valid

	return true, ""
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
	t.UpdatedAt = time.Now()
	return nil
}
