package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

func newTestTaskWithDueDate(t *testing.T, title string, daysOffset int) *models.Task {
	t.Helper()
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().AddDate(0, 0, daysOffset)
	task.DueDate = &due
	return task
}

func newTestPriorityProject(t *testing.T) *models.Project {
	t.Helper()
	project, err := models.NewProject(uuid.New().String(), "Priority Test", "")
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func intPtr(i int) *int {
	return &i
}
