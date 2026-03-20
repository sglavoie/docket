package format

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// BuildHierarchy establishes parent-child relationships based on heading levels.
// Tasks at level N+1 that follow a task at level N become children of that task.
// Uses the levels already captured in ParsedTask during initial parsing.
//
// If a heading skips more than one level (e.g., ## → #####), it is clamped to
// prevLevel+1 and a WarningValidation is emitted so the caller can surface it.
func BuildHierarchy(parsedTasks []*ParsedTask) ([]*models.Task, []ParseWarning) {
	tasks := make([]*models.Task, len(parsedTasks))
	for i, pt := range parsedTasks {
		tasks[i] = pt.Task
	}

	var warnings []ParseWarning

	// Build hierarchy using a stack
	// Stack tracks the most recent task at each level
	levelStack := make(map[int]*models.Task)

	// prevLevel tracks the heading level of the last processed task so we can
	// detect and clamp forward skips (e.g., ## followed by #####).
	// Starts at 1 so the minimum valid heading (##, level 2) is never clamped.
	prevLevel := 1

	for i, pt := range parsedTasks {
		task := tasks[i]
		level := pt.Level

		// Clamp heading level skip: if the level jumps by more than 1 from the
		// previous heading, treat it as one level deeper than the previous.
		// This prevents orphaned tasks on malformed files and keeps levelStack
		// in a predictable state for subsequent tasks.
		if level > prevLevel+1 {
			warnings = append(warnings, ParseWarning{
				Type:   WarningValidation,
				TaskID: task.ID,
				Field:  "heading_level",
				Message: fmt.Sprintf(
					"heading level skipped from %d to %d (task %q); treating as level %d",
					prevLevel, level, task.Title, prevLevel+1,
				),
			})
			level = prevLevel + 1
		}
		prevLevel = level

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

	return tasks, warnings
}

// ParsedTask represents a task parsed from markdown with its raw data.
type ParsedTask struct {
	Task       *models.Task
	Level      int    // Heading level (2 for ##, 3 for ###, etc.)
	RawContent string // The description content between this task and the next
}
