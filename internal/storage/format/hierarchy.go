package format

import (
	"github.com/sglavoie/tt/internal/models"
)

// BuildHierarchy establishes parent-child relationships based on heading levels.
// Tasks at level N+1 that follow a task at level N become children of that task.
// Uses the levels already captured in ParsedTask during initial parsing.
func BuildHierarchy(parsedTasks []*ParsedTask) []*models.Task {
	tasks := make([]*models.Task, len(parsedTasks))
	for i, pt := range parsedTasks {
		tasks[i] = pt.Task
	}

	// Build hierarchy using a stack
	// Stack tracks the most recent task at each level
	levelStack := make(map[int]*models.Task)

	for i, pt := range parsedTasks {
		task := tasks[i]
		level := pt.Level

		// If task already has an explicit parent ID from metadata, skip
		if task.ParentID != nil {
			levelStack[level] = task
			continue
		}

		// Find potential parent at a higher level (lower number)
		for checkLevel := level - 1; checkLevel >= 2; checkLevel-- {
			if parent, ok := levelStack[checkLevel]; ok {
				// Check if this task type can be a child of the parent type
				if task.Type.CanBeChildOf(parent.Type) {
					task.ParentID = &parent.ID
					break
				}
			}
		}

		// Update the stack
		levelStack[level] = task

		// Clear any lower levels (higher numbers) since we've moved past them
		for l := level + 1; l <= 6; l++ {
			delete(levelStack, l)
		}
	}

	return tasks
}

// ParsedTask represents a task parsed from markdown with its raw data.
type ParsedTask struct {
	Task       *models.Task
	Level      int    // Heading level (2 for ##, 3 for ###, etc.)
	RawContent string // The description content between this task and the next
}
