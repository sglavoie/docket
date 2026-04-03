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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
