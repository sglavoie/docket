package cmd

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestRepairConfigTaskRefsToLoadedProjectsFailsClosedForDuplicateIDs(t *testing.T) {
	deps := setupTestDeps(t)
	project := func(id string, taskIDs ...string) *models.Project {
		p, err := models.NewProject(id, id, "")
		if err != nil {
			t.Fatalf("NewProject(%s): %v", id, err)
		}
		p.Path = filepath.Join(deps.DataDir, "projects", p.Slug)
		for i, taskID := range taskIDs {
			task, taskErr := models.NewTask(taskID, i+1, models.TaskTypeTask, taskID)
			if taskErr != nil {
				t.Fatalf("NewTask(%s): %v", taskID, taskErr)
			}
			p.Tasks = append(p.Tasks, task)
		}
		p.NextShortID = len(p.Tasks) + 1
		if err := storage.SaveProject(p); err != nil {
			t.Fatalf("SaveProject(%s): %v", id, err)
		}
		deps.Config.Catalog().UpsertProject(p)
		return p
	}

	alpha := project("alpha", "outside-shared")
	beta := project("beta", "outside-shared", "unique-restored", "ambiguous-restored", "current-ambiguous")
	gamma := project("gamma", "ambiguous-restored", "current-ambiguous")

	refs := []config.FocusQueueItem{
		{ProjectID: alpha.ID, TaskID: "outside-shared"},
		{ProjectID: "removed-project", TaskID: "unique-restored"},
		{ProjectID: "removed-project", TaskID: "ambiguous-restored"},
		{ProjectID: gamma.ID, TaskID: "current-ambiguous"},
	}
	deps.Config.Planning().FocusQueue = refs
	for _, ref := range refs {
		deps.Config.Planning().DailyPlan = append(deps.Config.Planning().DailyPlan, config.DailyPlanItem{
			ProjectID: ref.ProjectID,
			TaskID:    ref.TaskID,
		})
		deps.Config.Machine().RecentTaskVisits = append(deps.Config.Machine().RecentTaskVisits, config.RecentTaskVisit{
			ProjectID: ref.ProjectID,
			TaskID:    ref.TaskID,
		})
	}
	manager := config.NewManager(deps.ConfigPath)
	if err := saveTestRuntime(manager, deps.Config); err != nil {
		t.Fatalf("saveTestRuntime: %v", err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}

	if err := repairConfigTaskRefsToLoadedProjects(runtime, []*models.Project{beta, gamma}); err != nil {
		t.Fatalf("repairConfigTaskRefsToLoadedProjects: %v", err)
	}

	wantProjects := []string{alpha.ID, beta.ID, "removed-project", gamma.ID}
	for i, want := range wantProjects {
		if got := runtime.Planning().FocusQueue[i].ProjectID; got != want {
			t.Errorf("FocusQueue[%d].ProjectID = %q, want %q", i, got, want)
		}
		if got := runtime.Planning().DailyPlan[i].ProjectID; got != want {
			t.Errorf("DailyPlan[%d].ProjectID = %q, want %q", i, got, want)
		}
		if got := runtime.Machine().RecentTaskVisits[i].ProjectID; got != want {
			t.Errorf("RecentTaskVisits[%d].ProjectID = %q, want %q", i, got, want)
		}
	}
}
