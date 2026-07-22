package app

import (
	"fmt"
	"maps"
	"sort"
	"strings"
)

// SetScoreDimensions merges the given manual scoring dimension values onto a
// task and commits them as one ChangeSet. Dimension keys are validated against
// the project's scoring profile at the adapter edge (the profile lives in
// config, which the application layer does not read); this use case owns the
// mutation and the atomic, undoable commit. An empty set is rejected before any
// mutation.
func (s *Service) SetScoreDimensions(taskID string, dims map[string]int) (TaskResult, error) {
	if len(dims) == 0 {
		return TaskResult{}, invalidInput("score", "at least one dimension value is required")
	}
	return s.mutateTask(taskID, "Set Score", describeScoreDims(dims), func(ref TaskRef) error {
		if ref.Task.ScoreDimensions == nil {
			ref.Task.ScoreDimensions = make(map[string]int, len(dims))
		}
		maps.Copy(ref.Task.ScoreDimensions, dims)
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

func (s *Service) SetScoreDimensionsByLocator(locator TaskLocator, dims map[string]int) (TaskResult, error) {
	if len(dims) == 0 {
		return TaskResult{}, invalidInput("score", "at least one dimension value is required")
	}
	return s.mutateTaskByLocator(locator, "Set Score", describeScoreDims(dims), func(ref TaskRef) error {
		if ref.Task.ScoreDimensions == nil {
			ref.Task.ScoreDimensions = make(map[string]int, len(dims))
		}
		maps.Copy(ref.Task.ScoreDimensions, dims)
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

// ReplaceScoreDimensions sets a task's manual scoring dimensions to exactly the
// given set, replacing any previous manual values rather than merging onto them.
// This is the interactive score-editor contract: the editor recomputes the full
// desired manual set on every save (a dimension reset back to derived/default is
// simply absent from dims), so a merge would silently keep a value the user just
// cleared. An empty set is accepted and clears every manual dimension, leaving
// the task fully derived/default. The mutation commits as one atomic, undoable
// ChangeSet through the workspace ledger, exactly like SetScoreDimensions.
func (s *Service) ReplaceScoreDimensions(taskID string, dims map[string]int) (TaskResult, error) {
	return s.mutateTask(taskID, "Set Score", describeScoreDims(dims), func(ref TaskRef) error {
		next := make(map[string]int, len(dims))
		maps.Copy(next, dims)
		ref.Task.ScoreDimensions = next
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

func (s *Service) ReplaceScoreDimensionsByLocator(locator TaskLocator, dims map[string]int) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Set Score", describeScoreDims(dims), func(ref TaskRef) error {
		next := make(map[string]int, len(dims))
		maps.Copy(next, dims)
		ref.Task.ScoreDimensions = next
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

// describeScoreDims renders the dimension assignments deterministically (sorted
// key=value pairs) for the undo-history context line.
func describeScoreDims(dims map[string]int) string {
	keys := make([]string, 0, len(dims))
	for k := range dims {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, dims[k]))
	}
	return strings.Join(parts, " ")
}
