package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestEditRoutesThroughApp proves the CLI edit adapter persists title and
// description changes through the application-layer Edit use case (fallback save
// path under the in-memory test repository) — the same wiring runTaskEdit uses
// once the external editor produces the new content.
func TestEditRoutesThroughApp(t *testing.T) {
	deps, project := setupStateTest(t)
	task := findTodoLeaf(t, project)

	newTitle := "Edited Title"
	newDesc := "Edited body."
	svc := newAppService(config.NewManager(deps.ConfigPath))
	res, err := withTaskRetry(func() (app.TaskResult, error) {
		return svc.Edit(app.EditRequest{TaskID: task.ID, Title: &newTitle, Description: &newDesc})
	})
	testutil.AssertNoError(t, err)
	if res.Title != newTitle {
		t.Errorf("result title = %q, want %q", res.Title, newTitle)
	}

	got := deps.Repository.Projects[project.ID].GetTaskByID(task.ID)
	if got == nil {
		t.Fatalf("task %s missing from saved project", task.ID)
	}
	if got.Title != newTitle {
		t.Errorf("persisted title = %q, want %q", got.Title, newTitle)
	}
	if got.Description != newDesc {
		t.Errorf("persisted description = %q, want %q", got.Description, newDesc)
	}
}
