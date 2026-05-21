package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func makeTestCard(section string, status models.TaskStatus) KanbanCard {
	task := testutil.NewTestTask("task", models.TaskTypeTask)
	task.Section = section
	task.Status = status
	proj := testutil.NewTestProject("p")
	return KanbanCard{Task: task, Project: proj}
}
