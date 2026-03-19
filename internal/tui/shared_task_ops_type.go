package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// TypeChangeOp represents the kind of type change operation.
type TypeChangeOp int

const (
	TypePromote    TypeChangeOp = iota // Promote type (task → story → epic)
	TypeDemote                         // Demote type (epic → story → task)
	TypeCycleLeft                      // Cycle leaf type left
	TypeCycleRight                     // Cycle leaf type right
)

// applyTypeChange validates and applies a type change (promote, demote, cycle)
// on the given task. It handles the can-do guard, persist, and status message.
func applyTypeChange(pf persistFn, project *models.Project, taskID string, op TypeChangeOp) TaskOpResult {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return TaskOpResult{StatusMessage: "Task no longer exists in project"}
	}

	var canDo bool
	switch op {
	case TypePromote:
		canDo, _ = task.CanPromote()
	case TypeDemote:
		canDo, _ = task.CanDemote()
	case TypeCycleLeft:
		canDo, _ = task.CanCycleLeafType(true)
	case TypeCycleRight:
		canDo, _ = task.CanCycleLeafType(false)
	}
	if !canDo {
		return TaskOpResult{}
	}

	result := pf(project, taskID, func(t *models.Task) {
		switch op {
		case TypePromote:
			_ = t.Promote()
		case TypeDemote:
			_ = t.Demote()
		case TypeCycleLeft:
			_ = t.CycleLeafType(true)
		case TypeCycleRight:
			_ = t.CycleLeafType(false)
		}
	})
	if result.Err != nil || result.StatusMessage != "" {
		return result
	}

	// Re-read the task after persist to get the updated type
	updated := project.GetTaskByID(taskID)
	if updated != nil {
		result.StatusMessage = fmt.Sprintf("Changed to %s", updated.Type.HeadingPrefix())
	}
	return result
}

// applyTypeChangeOp applies a type change operation to a task in-place without
// persisting. Returns true if the change was applied, false if the task was
// skipped (not found or constraint violation). Used by bulk type change.
func applyTypeChangeOp(project *models.Project, taskID string, op TypeChangeOp) bool {
	task := project.GetTaskByID(taskID)
	if task == nil {
		return false
	}

	var canDo bool
	switch op {
	case TypePromote:
		canDo, _ = task.CanPromote()
	case TypeDemote:
		canDo, _ = task.CanDemote()
	case TypeCycleLeft:
		canDo, _ = task.CanCycleLeafType(true)
	case TypeCycleRight:
		canDo, _ = task.CanCycleLeafType(false)
	}
	if !canDo {
		return false
	}

	project.MutateTask(taskID, func(t *models.Task) {
		switch op {
		case TypePromote:
			_ = t.Promote()
		case TypeDemote:
			_ = t.Demote()
		case TypeCycleLeft:
			_ = t.CycleLeafType(true)
		case TypeCycleRight:
			_ = t.CycleLeafType(false)
		}
	})
	return true
}

// applyBulkTypeCycle sets all selected tasks to the same leaf type, then cycles
// them together. If the tasks already share a common type, that type is cycled.
// If types are mixed, all tasks are first normalized to Task before cycling.
// Non-leaf tasks and tasks that fail hierarchy validation are skipped.
func applyBulkTypeCycle(project *models.Project, tasks []*models.Task, op TypeChangeOp) int {
	// Determine whether all tasks share the same leaf type.
	var commonType models.TaskType
	uniform := true
	first := true
	for _, task := range tasks {
		t := project.GetTaskByID(task.ID)
		if t == nil || !t.Type.IsLeafType() {
			continue
		}
		if first {
			commonType = t.Type
			first = false
		} else if t.Type != commonType {
			uniform = false
			break
		}
	}

	// Compute the target type: cycle from common type, or reset to Task then cycle.
	var targetType models.TaskType
	if uniform && !first {
		// All same leaf type — cycle from it.
		if op == TypeCycleLeft {
			targetType = commonType.CycleLeafLeft()
		} else {
			targetType = commonType.CycleLeafRight()
		}
	} else {
		// Mixed types — normalize to Task first, then cycle.
		if op == TypeCycleLeft {
			targetType = models.TaskTypeTask.CycleLeafLeft()
		} else {
			targetType = models.TaskTypeTask.CycleLeafRight()
		}
	}

	// Apply the target type to all eligible tasks.
	changed := 0
	for _, task := range tasks {
		t := project.GetTaskByID(task.ID)
		if t == nil || !t.Type.IsLeafType() {
			continue
		}
		if t.Type == targetType {
			continue
		}
		// Validate hierarchy: new type must be valid under parent.
		if t.Parent != nil && !targetType.CanBeChildOf(t.Parent.Type) {
			continue
		}
		project.MutateTask(t.ID, func(mt *models.Task) {
			mt.Type = targetType
			mt.ClearTypeAttrs()
		})
		changed++
	}
	return changed
}

// IndentTask attempts to indent a task under a valid parent by walking up from
// the candidate (the task visually above). It sets ParentID and rebuilds the tree.
// Returns true if the indent was performed.
func IndentTask(project *models.Project, task *models.Task, candidate *models.Task) bool {
	taskMap := make(map[string]*models.Task, len(project.Tasks))
	for _, t := range project.Tasks {
		taskMap[t.ID] = t
	}
	for candidate != nil {
		if ok, _ := task.CanIndent(candidate); ok {
			if !models.WouldCreateCycle(taskMap, task.ID, candidate.ID) {
				candidateID := candidate.ID
				task.ParentID = &candidateID
				project.BuildTaskTree()
				return true
			}
		}
		candidate = candidate.Parent
	}
	return false
}

// OutdentTask attempts to outdent a task to its parent's level.
// Returns true if the outdent was performed.
func OutdentTask(project *models.Project, task *models.Task) bool {
	if ok, _ := task.CanOutdent(); !ok {
		return false
	}
	_ = project.OutdentTask(task)
	return true
}
