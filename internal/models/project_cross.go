package models

import (
	"fmt"
	"time"
)

// MoveTaskToProjectResult contains the result of a cross-project task move.
type MoveTaskToProjectResult struct {
	MovedTasks  []*Task  // Tasks that were moved (root + descendants)
	ClearedDeps []string // Dependency IDs that were cleared (pointed to tasks not moved)
	OldShortIDs []int    // Original short IDs of moved tasks (parallel with MovedTasks)
}

// MoveTaskToProject moves a task and all its descendants to another project.
// Dependencies pointing to tasks not in the moved set are cleared.
// Sections are preserved if they exist in destination, otherwise reset to Next.
// Short IDs are regenerated in the destination project.
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

	// Build tree to ensure parent-child relationships are set
	p.BuildTaskTree()

	// Collect task and all descendants
	movedTasks := collectSubtree(task)
	movedIDs := make(map[string]bool)
	for _, t := range movedTasks {
		movedIDs[t.ID] = true
	}

	// Track cleared dependencies and old short IDs
	var clearedDeps []string
	oldShortIDs := make([]int, len(movedTasks))

	now := time.Now()
	destSections := dest.GetSections()
	destSectionSet := make(map[string]bool)
	for _, s := range destSections {
		destSectionSet[s] = true
	}

	// Process each task being moved
	for i, t := range movedTasks {
		oldShortIDs[i] = t.ShortID

		// Clear same-project dependencies pointing to tasks not in moved set.
		// Preserve cross-project qualified references (format "projectID:taskID").
		newDeps := make([]string, 0, len(t.DependsOn))
		for _, depID := range t.DependsOn {
			if ParseTaskRef(depID).IsCrossProject() || movedIDs[depID] {
				newDeps = append(newDeps, depID)
			} else {
				clearedDeps = append(clearedDeps, depID)
			}
		}
		t.DependsOn = newDeps

		// Clear same-project RelatedTo references pointing to tasks not in moved set.
		// Preserve cross-project qualified references.
		newRelated := make([]string, 0, len(t.RelatedTo))
		for _, relID := range t.RelatedTo {
			if ParseTaskRef(relID).IsCrossProject() || movedIDs[relID] {
				newRelated = append(newRelated, relID)
			}
		}
		t.RelatedTo = newRelated

		// Reset section if it doesn't exist in destination.
		// GTD sections (Inbox, Next, Waiting, etc.) are built-in and always
		// valid, so only non-GTD sections stored in the Section field need
		// the existence check.
		if t.Section != "" && !IsGTDSection(t.Section) && !destSectionSet[t.Section] {
			t.SetSection(SectionNext)
		} else if t.Section != "" && IsGTDSection(t.Section) && !destSectionSet[t.Section] {
			// GTD section is preserved on the task, but dest's Sections list
			// may not enumerate it (e.g. dest was created without
			// EnsureDefaultSections). Append so the bucket renders in UI
			// listings derived from dest.Sections.
			dest.EnsureSection(t.Section)
			destSectionSet[t.Section] = true
		}

		// Ensure custom section exists in destination
		if t.CustomSection != "" && !dest.HasSection(t.CustomSection) {
			dest.EnsureSection(t.CustomSection)
			destSectionSet[t.CustomSection] = true
		}

		// Update timestamp
		t.UpdatedAt = now

		// Regenerate short ID in destination
		t.ShortID = dest.GetNextShortID()

		// Clear project reference (will be set when added to dest)
		t.Project = nil
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

	// Clear same-project dependencies and RelatedTo in source project that pointed to moved tasks.
	// Cross-project qualified references are never affected here since movedIDs only contains
	// plain task UUIDs.
	for _, t := range p.Tasks {
		newDeps := make([]string, 0, len(t.DependsOn))
		for _, depID := range t.DependsOn {
			if !movedIDs[depID] {
				newDeps = append(newDeps, depID)
			}
		}
		if len(newDeps) != len(t.DependsOn) {
			t.DependsOn = newDeps
			t.UpdatedAt = now
		}

		newRelated := make([]string, 0, len(t.RelatedTo))
		for _, relID := range t.RelatedTo {
			if !movedIDs[relID] {
				newRelated = append(newRelated, relID)
			}
		}
		if len(newRelated) != len(t.RelatedTo) {
			t.RelatedTo = newRelated
			t.UpdatedAt = now
		}
	}

	// Add tasks to destination project
	dest.Tasks = append(dest.Tasks, movedTasks...)
	dest.UpdatedAt = now

	// Rebuild trees for both projects
	p.BuildTaskTree()
	dest.BuildTaskTree()

	return &MoveTaskToProjectResult{
		MovedTasks:  movedTasks,
		ClearedDeps: clearedDeps,
		OldShortIDs: oldShortIDs,
	}, nil
}
