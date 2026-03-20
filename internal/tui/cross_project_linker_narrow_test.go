package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestCrossProjectLinkerNarrowTerminal verifies that viewTaskSelection does not
// panic when the terminal width is very narrow (width-35 would be negative).
func TestCrossProjectLinkerNarrowTerminal(t *testing.T) {
	task := &models.Task{ID: "t1", Title: "A task with a long title that might cause issues", ShortID: 1}

	m := &CrossProjectLinkerModel{
		sourceTask:    task,
		targetProject: &models.Project{Name: "Other Project"},
		step:          StepSelectTask,
		filteredTasks: []*models.Task{
			{ID: "t2", Title: "Another task", ShortID: 2},
		},
		taskCursor: 0,
	}
	m.SetSize(20, 10) // width=20, so width-35 = -15 without the fix

	// Should not panic
	_ = m.viewTaskSelection()
}
