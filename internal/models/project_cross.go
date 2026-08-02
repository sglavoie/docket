package models

import (
	"fmt"
	"time"
)

// MoveTaskToProjectResult contains the result of a cross-project task move.
type MoveTaskToProjectResult struct {
	MovedTasks  []*Task // Tasks that were moved (root + descendants)
	OldShortIDs []int   // Original short IDs of moved tasks (parallel with MovedTasks)
}

// MoveTaskToProject moves a task and all its descendants to another project.
// Workflow states are preserved and short IDs are regenerated in the destination.
func (p *Project) MoveTaskToProject(task *Task, dest *Project) (*MoveTaskToProjectResult, error) {
	if task == nil {
		return nil, fmt.Errorf("cannot move nil task")
	}
	if dest == nil {
		return nil, fmt.Errorf("destination project is nil")
	}
	if p.ID == dest.ID {
		return nil, fmt.Errorf("cannot move task to the same project")
	}
	sourceTask := p.GetTaskByID(task.ID)
	if sourceTask == nil {
		return nil, fmt.Errorf("task %q is not in source project", task.ID)
	}
	if sourceTask != task {
		return nil, fmt.Errorf("task %q does not match source project task", task.ID)
	}
	prospectiveMovedIDs := map[string]bool{task.ID: true}
	for changed := true; changed; {
		changed = false
		for _, candidate := range p.Tasks {
			if candidate == nil || candidate.ParentID == nil || prospectiveMovedIDs[candidate.ID] {
				continue
			}
			if prospectiveMovedIDs[*candidate.ParentID] {
				prospectiveMovedIDs[candidate.ID] = true
				changed = true
			}
		}
	}
	for _, candidate := range dest.Tasks {
		if candidate != nil && prospectiveMovedIDs[candidate.ID] {
			return nil, fmt.Errorf("destination project already contains task %q", candidate.ID)
		}
	}

	// Build tree to ensure parent-child relationships are set
	p.BuildTaskTree()

	// Collect task and all descendants
	movedTasks := collectSubtree(task)
	movedIDs := make(map[string]bool)
	for _, t := range movedTasks {
		movedIDs[t.ID] = true
	}

	// Track old short IDs.
	oldShortIDs := make([]int, len(movedTasks))

	now := time.Now()
	// Process each task being moved
	for i, t := range movedTasks {
		oldShortIDs[i] = t.ShortID
		reassignTaskToDest(t, dest, now)
	}

	// Clear the root task's ParentID since it becomes a root in destination
	task.ParentID = nil

	// Remove tasks from source project
	newCap := max(len(p.Tasks)-len(movedTasks), 0)
	newTasks := make([]*Task, 0, newCap)
	for _, t := range p.Tasks {
		if !movedIDs[t.ID] {
			newTasks = append(newTasks, t)
		}
	}
	p.Tasks = newTasks
	p.UpdatedAt = now

	// Add tasks to destination project
	dest.Tasks = append(dest.Tasks, movedTasks...)
	dest.UpdatedAt = now

	// Rebuild trees for both projects
	p.BuildTaskTree()
	dest.BuildTaskTree()

	return &MoveTaskToProjectResult{
		MovedTasks:  movedTasks,
		OldShortIDs: oldShortIDs,
	}, nil
}

// reassignTaskToDest refreshes the short ID and clears the project pointer so
// the task is ready to live under dest.
func reassignTaskToDest(t *Task, dest *Project, now time.Time) {
	t.UpdatedAt = now
	t.ShortID = dest.GetNextShortID()
	t.Project = nil
}
