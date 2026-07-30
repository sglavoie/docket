package models

import (
	"fmt"
	"time"
)

// MoveTaskUp swaps the task with its previous sibling in the Tasks slice.
// For root tasks, completed tasks cannot move above uncompleted tasks.
// Returns nil without changes if the task is already at the top or cannot move.
func (p *Project) MoveTaskUp(task *Task) error {
	if task == nil {
		return nil
	}
	task = p.GetTaskByID(task.ID)
	if task == nil {
		return nil
	}
	siblings := p.GetSiblings(task)
	if len(siblings) < 2 {
		return nil // No siblings to swap with
	}

	siblingIdx := p.FindSiblingIndex(task)
	if siblingIdx <= 0 {
		return nil // Already at top or not found
	}

	// Get the sibling above
	siblingAbove := siblings[siblingIdx-1]

	// For root tasks, enforce completion boundary:
	// A completed root task cannot move above an uncompleted root task
	if task.IsRoot() && task.IsComplete() && !siblingAbove.IsComplete() {
		return nil // Cannot cross completion boundary
	}

	// Find their positions in the main Tasks slice
	taskIdx := p.findTaskIndex(task)
	aboveIdx := p.findTaskIndex(siblingAbove)

	if taskIdx == -1 || aboveIdx == -1 {
		return nil
	}

	moved := p.Tasks[taskIdx]
	displaced := p.Tasks[aboveIdx]

	// Swap in the Tasks slice
	p.Tasks[taskIdx], p.Tasks[aboveIdx] = p.Tasks[aboveIdx], p.Tasks[taskIdx]
	// Touch both tasks' UpdatedAt: both moved, and the markdown round-trip
	// records modification timestamps per task, so updating only one leaves
	// the sibling with a stale timestamp despite its position changing
	// (mirrors the fix applied to SwapTasks).
	now := time.Now()
	moved.UpdatedAt = now
	displaced.UpdatedAt = now
	p.BuildTaskTree()

	return nil
}

// MoveTaskDown swaps the task with its next sibling in the Tasks slice.
// For root tasks, uncompleted tasks cannot move below completed tasks.
// Returns nil without changes if the task is already at the bottom or cannot move.
func (p *Project) MoveTaskDown(task *Task) error {
	if task == nil {
		return nil
	}
	task = p.GetTaskByID(task.ID)
	if task == nil {
		return nil
	}
	siblings := p.GetSiblings(task)
	if len(siblings) < 2 {
		return nil // No siblings to swap with
	}

	siblingIdx := p.FindSiblingIndex(task)
	if siblingIdx == -1 || siblingIdx >= len(siblings)-1 {
		return nil // Already at bottom or not found
	}

	// Get the sibling below
	siblingBelow := siblings[siblingIdx+1]

	// For root tasks, enforce completion boundary:
	// An uncompleted root task cannot move below a completed root task
	if task.IsRoot() && !task.IsComplete() && siblingBelow.IsComplete() {
		return nil // Cannot cross completion boundary
	}

	// Find their positions in the main Tasks slice
	taskIdx := p.findTaskIndex(task)
	belowIdx := p.findTaskIndex(siblingBelow)

	if taskIdx == -1 || belowIdx == -1 {
		return nil
	}

	moved := p.Tasks[taskIdx]
	displaced := p.Tasks[belowIdx]

	// Swap in the Tasks slice
	p.Tasks[taskIdx], p.Tasks[belowIdx] = p.Tasks[belowIdx], p.Tasks[taskIdx]
	// Touch both tasks' UpdatedAt: both moved, and the markdown round-trip
	// records modification timestamps per task, so updating only one leaves
	// the sibling with a stale timestamp despite its position changing
	// (mirrors the fix applied to SwapTasks).
	now := time.Now()
	moved.UpdatedAt = now
	displaced.UpdatedAt = now
	p.BuildTaskTree()

	return nil
}

// SwapSiblingTasks swaps two explicitly selected siblings in the Tasks slice.
// This is the safe primitive for filtered or collapsed views: the presentation
// layer identifies the visible neighbor, while the model refuses to reorder
// tasks from different parent groups.
// Both tasks have UpdatedAt set so the markdown round-trip records the
// reorder for each side (previously only task1 was touched, which left
// task2 with a stale modification timestamp despite having moved). The task
// tree is rebuilt after a successful swap so Parent.Children reflects the new
// flat task order immediately.
func (p *Project) SwapSiblingTasks(task1, task2 *Task) error {
	if task1 == nil || task2 == nil {
		return nil
	}
	idx1 := p.findTaskIndex(task1)
	idx2 := p.findTaskIndex(task2)
	if idx1 == -1 || idx2 == -1 {
		return nil
	}
	if idx1 == idx2 {
		return nil
	}
	moved1 := p.Tasks[idx1]
	moved2 := p.Tasks[idx2]
	if !sameParent(moved1, moved2) {
		return fmt.Errorf("tasks %q and %q are not siblings", moved1.ID, moved2.ID)
	}
	p.Tasks[idx1], p.Tasks[idx2] = p.Tasks[idx2], p.Tasks[idx1]
	now := time.Now()
	moved1.UpdatedAt = now
	moved2.UpdatedAt = now
	p.BuildTaskTree()
	return nil
}

// SwapTasks is the legacy no-result adapter retained for callers that already
// resolve visible siblings. Invalid cross-parent swaps are now safe no-ops.
func (p *Project) SwapTasks(task1, task2 *Task) {
	_ = p.SwapSiblingTasks(task1, task2)
}

func sameParent(task1, task2 *Task) bool {
	if task1 == nil || task2 == nil {
		return false
	}
	if task1.ParentID == nil || task2.ParentID == nil {
		return task1.ParentID == nil && task2.ParentID == nil
	}
	return *task1.ParentID == *task2.ParentID
}
