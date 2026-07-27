package models

import "time"

// MoveTaskUp swaps the task with its previous sibling in the Tasks slice.
// For root tasks, completed tasks cannot move above uncompleted tasks.
// Returns nil without changes if the task is already at the top or cannot move.
func (p *Project) MoveTaskUp(task *Task) error {
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

// SwapTasks swaps the positions of two tasks in the Tasks slice.
// This is used when moving tasks within a filtered view where the target
// task is known explicitly rather than being the immediate sibling.
// Both tasks have UpdatedAt set so the markdown round-trip records the
// reorder for each side (previously only task1 was touched, which left
// task2 with a stale modification timestamp despite having moved). The task
// tree is rebuilt after a successful swap so Parent.Children reflects the new
// flat task order immediately.
func (p *Project) SwapTasks(task1, task2 *Task) {
	idx1 := p.findTaskIndex(task1)
	idx2 := p.findTaskIndex(task2)
	if idx1 == -1 || idx2 == -1 {
		return
	}
	moved1 := p.Tasks[idx1]
	moved2 := p.Tasks[idx2]
	p.Tasks[idx1], p.Tasks[idx2] = p.Tasks[idx2], p.Tasks[idx1]
	now := time.Now()
	moved1.UpdatedAt = now
	moved2.UpdatedAt = now
	p.BuildTaskTree()
}
