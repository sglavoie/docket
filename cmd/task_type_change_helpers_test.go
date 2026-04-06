package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func addTaskTypeChangeTestTask(t *testing.T, project *models.Project, id string, taskType models.TaskType, title string) *models.Task {
	t.Helper()

	task, err := models.NewTask(id, project.GetNextShortID(), taskType, title)
	if err != nil {
		t.Fatal(err)
	}

	project.Tasks = append(project.Tasks, task)
	return task
}
