package alerts

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func testProject(name string, tasks ...*models.Task) ProjectData {
	return ProjectData{
		Project: &models.Project{ID: "proj-" + name, Name: name},
		Tasks:   tasks,
	}
}

func testTask(title string, status models.TaskStatus, updatedAt time.Time) *models.Task {
	return &models.Task{
		ID:            "task-" + title,
		Title:         title,
		WorkflowState: models.DeriveWorkflowState(status, "", ""),
		UpdatedAt:     updatedAt,
	}
}

func timePtr(t time.Time) *time.Time { return &t }
