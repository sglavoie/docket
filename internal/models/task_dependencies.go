package models

import (
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// AddDependency adds a dependency on another task if not already present.
// Returns an error if blockerID is the task's own ID (self-dependency).
func (t *Task) AddDependency(blockerID string) error {
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
func (t *Task) HasDependency(blockerID string) bool {
	return slices.Contains(t.DependsOn, blockerID)
}

// ClearDependencies removes all dependencies.
func (t *Task) ClearDependencies() {
	if len(t.DependsOn) > 0 {
		t.DependsOn = []string{}
		t.UpdatedAt = time.Now()
	}
}
