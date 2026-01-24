package models

import (
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// AddDependency adds a dependency on another task if not already present.
func (t *Task) AddDependency(blockerID string) {
	if util.SliceContains(t.DependsOn, blockerID) {
		return
	}
	t.DependsOn = append(t.DependsOn, blockerID)
	t.UpdatedAt = time.Now()
}

// RemoveDependency removes a dependency if present.
func (t *Task) RemoveDependency(blockerID string) {
	if newDeps, removed := util.SliceRemove(t.DependsOn, blockerID); removed {
		t.DependsOn = newDeps
		t.UpdatedAt = time.Now()
	}
}

// HasDependency returns true if the task depends on the given task ID.
func (t *Task) HasDependency(blockerID string) bool {
	return util.SliceContains(t.DependsOn, blockerID)
}

// ClearDependencies removes all dependencies.
func (t *Task) ClearDependencies() {
	if len(t.DependsOn) > 0 {
		t.DependsOn = []string{}
		t.UpdatedAt = time.Now()
	}
}
