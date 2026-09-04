package tui

// The parented fixture project the outline-buffer tests dump, edit, and reapply.
// Six shell test files read it, so it is homed away from any one of them.

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func outlineTestProject(t *testing.T) *models.Project {
	t.Helper()
	project := &models.Project{ID: "project-1", Name: "Sonar", Slug: "sonar"}
	specs := []struct {
		id      string
		shortID int
		title   string
		parent  string
		state   models.WorkflowState
	}{
		{id: "4f2a9c11aaaa", shortID: 1, title: "Ship auth rewrite", state: models.StateDoing},
		{id: "9b31de07bbbb", shortID: 2, title: "Token refresh path", parent: "4f2a9c11aaaa", state: models.StateBacklog},
		{id: "c0184aa2cccc", shortID: 3, title: "Retry budget", parent: "9b31de07bbbb", state: models.StateBacklog},
		{id: "71f0b3d5dddd", shortID: 4, title: "Sweep dead flags", state: models.StateInbox},
	}
	for _, spec := range specs {
		task, err := models.NewGenericTask(spec.id, spec.shortID, spec.title)
		if err != nil {
			t.Fatalf("new task %s: %v", spec.id, err)
		}
		task.ApplyWorkflowState(spec.state)
		if spec.parent != "" {
			parent := spec.parent
			task.ParentID = &parent
		}
		project.Tasks = append(project.Tasks, task)
	}
	return project
}
