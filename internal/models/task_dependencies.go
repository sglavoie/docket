package models

import (
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// AddDependency adds a dependency on another task if not already present.
// Returns an error if blockerID is empty or is the task's own ID
// (self-dependency). An empty blockerID would silently corrupt the DependsOn
// list and match every empty lookup, so it must be rejected.
func (t *Task) AddDependency(blockerID string) error {
	if blockerID == "" {
		return fmt.Errorf("dependency blocker ID cannot be empty")
	}
	if blockerID == t.ID {
		return fmt.Errorf("task cannot depend on itself")
	}
	var added bool
	t.DependsOn, added = util.SliceAdd(t.DependsOn, blockerID)
	if added {
		t.UpdatedAt = time.Now()
	}
	return nil
}

// RemoveDependency removes a dependency if present.
func (t *Task) RemoveDependency(blockerID string) {
	var removed bool
	t.DependsOn, removed = util.SliceRemove(t.DependsOn, blockerID)
	if removed {
		t.UpdatedAt = time.Now()
	}
}

// HasDependency returns true if the task depends on the given task ID.
// An empty blockerID always returns false so a missing argument cannot
// accidentally match a malformed empty entry stored on disk.
func (t *Task) HasDependency(blockerID string) bool {
	if blockerID == "" {
		return false
	}
	return slices.Contains(t.DependsOn, blockerID)
}

// ClearDependencies removes all dependencies.
func (t *Task) ClearDependencies() {
	if len(t.DependsOn) > 0 {
		t.DependsOn = []string{}
		t.UpdatedAt = time.Now()
	}
}
