package tui

import (
	"testing"

	domainapp "github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// installDistinctOutlinerSnapshot gives the Outliner the immutable workspace
// snapshot it owns in production. Tests that point loadedProjects at the
// catalog objects cannot expose stale post-mutation rows because the mutation
// updates the view's project pointer in place.
func installDistinctOutlinerSnapshot(t *testing.T, model *AppModel, outliner *OutlinerModel) {
	t.Helper()

	_, projects, failed := LoadCrossScopeTasks(model.runtime, outliner.projectService, CrossScopeOpts{
		ActiveOnly: true,
	})
	if len(failed) > 0 {
		t.Fatalf("load Outliner snapshot: %+v", failed)
	}
	if len(projects) == 0 {
		t.Fatal("load Outliner snapshot: no active projects")
	}
	outliner.loadedProjects = projects
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
}

func outlinerHasTask(outliner *OutlinerModel, projectID, taskID string) bool {
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.ProjectID == projectID && row.task != nil && row.task.ID == taskID {
			return true
		}
	}
	return false
}

func TestTaskCreatedRefreshesVisibleOutlinerSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "created refresh")
	model, outliner := newSession2OutlinerApp(t, dataDir, true, project)
	installDistinctOutlinerSnapshot(t, model, outliner)

	committed := &models.Project{
		ID:   project.ID,
		Slug: project.Slug,
		Name: project.Name,
		Path: project.Path,
	}
	if err := storage.LoadProject(committed); err != nil {
		t.Fatalf("load committed project: %v", err)
	}
	task, err := models.NewGenericTask("created-refresh-task", committed.GetNextShortID(), "Newly created")
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	task.WorkflowState = models.StateReady
	task.Project = committed
	committed.Tasks = append(committed.Tasks, task)
	committed.BuildTaskTree()
	if err := storage.SaveProject(committed); err != nil {
		t.Fatalf("save created task: %v", err)
	}

	model.pushView(ViewTaskForm)
	result := domainapp.TaskResult{ProjectID: project.ID, TaskID: task.ID}
	_, cmd := model.Update(TaskCreatedMsg{Result: &result})
	runSession2AppCommand(t, model, cmd)

	if outliner.crossLoading {
		t.Fatal("Outliner reload did not settle")
	}
	if !outlinerHasTask(outliner, project.ID, task.ID) {
		t.Fatal("created task did not appear without navigating away")
	}
	if got := outliner.activeOutlinerMode(); got != ModeAll {
		t.Fatalf("Outliner mode changed to %q, want %q", got, ModeAll)
	}
}
