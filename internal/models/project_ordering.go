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

	// Swap in the Tasks slice
	p.Tasks[taskIdx], p.Tasks[aboveIdx] = p.Tasks[aboveIdx], p.Tasks[taskIdx]
	task.UpdatedAt = time.Now()

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

	// Swap in the Tasks slice
	p.Tasks[taskIdx], p.Tasks[belowIdx] = p.Tasks[belowIdx], p.Tasks[taskIdx]
	task.UpdatedAt = time.Now()

	return nil
}

// RelocateTaskToEndOfSection moves a task to appear after the last task
// in the given section within the Tasks slice. This ensures that when a
// task is moved to a new section, it appears at the bottom rather than
// retaining its original position (which could place it at the top).
func (p *Project) RelocateTaskToEndOfSection(taskID string, section string) {
	taskIdx := -1
	lastSectionIdx := -1
	for i, t := range p.Tasks {
		if t.ID == taskID {
			taskIdx = i
		} else if t.Section == section {
			lastSectionIdx = i
		}
	}
	if taskIdx == -1 {
		return
	}
	// Already after all section tasks (or no other tasks in section)
	if lastSectionIdx == -1 || taskIdx > lastSectionIdx {
		return
	}
	// Remove task from current position and insert after lastSectionIdx
	task := p.Tasks[taskIdx]
	p.Tasks = append(p.Tasks[:taskIdx], p.Tasks[taskIdx+1:]...)
	// After removal, lastSectionIdx shifted down by 1
	insertAt := lastSectionIdx // was lastSectionIdx, but after removal it's the correct position
	p.Tasks = append(p.Tasks[:insertAt], append([]*Task{task}, p.Tasks[insertAt:]...)...)
}

// SwapTasks swaps the positions of two tasks in the Tasks slice.
// This is used when moving tasks within a filtered view where the target
// task is known explicitly rather than being the immediate sibling.
func (p *Project) SwapTasks(task1, task2 *Task) {
	idx1 := p.findTaskIndex(task1)
	idx2 := p.findTaskIndex(task2)
	if idx1 == -1 || idx2 == -1 {
		return
	}
	p.Tasks[idx1], p.Tasks[idx2] = p.Tasks[idx2], p.Tasks[idx1]
	task1.UpdatedAt = time.Now()
}
