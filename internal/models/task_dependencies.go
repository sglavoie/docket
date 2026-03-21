package models

import (
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

// ValidateDependencyChain checks whether adding a dependency from taskID to blockerID
// would create a circular dependency within allTasks. Returns an error if a cycle
// would be created, or if it would be a self-dependency.
func ValidateDependencyChain(taskID, blockerID string, allTasks []*Task) error {
	if taskID == blockerID {
		return fmt.Errorf("task cannot depend on itself")
	}
	taskMap := make(map[string]*Task, len(allTasks))
	for _, t := range allTasks {
		taskMap[t.ID] = t
	}
	visited := make(map[string]bool)
	var canReach func(id string) bool
	canReach = func(id string) bool {
		if id == taskID {
			return true
		}
		if visited[id] {
			return false
		}
		visited[id] = true
		t, ok := taskMap[id]
		if !ok {
			return false
		}
		for _, depID := range t.DependsOn {
			if canReach(depID) {
				return true
			}
		}
		return false
	}
	if canReach(blockerID) {
		return fmt.Errorf("circular dependency: adding this dependency would create a cycle")
	}
	return nil
}

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
