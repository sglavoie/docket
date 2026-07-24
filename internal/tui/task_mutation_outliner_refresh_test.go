package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	domainapp "github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
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

func TestArchiveCompletedTasksRefreshesVisibleOutlinerSnapshot(t *testing.T) {
	tests := []struct {
		name   string
		scoped bool
		all    bool
	}{
		{name: "project archive in project scope", scoped: true},
		{name: "archive all in global scope", all: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			project := newSession2Project(t, dataDir, tt.name)
			project.Tasks[0].Complete()
			archivedTaskID := project.Tasks[0].ID
			if err := storage.SaveProject(project); err != nil {
				t.Fatalf("save completed task: %v", err)
			}

			model, outliner := newSession2OutlinerApp(t, dataDir, true, project)
			outliner.selectOutlinerMode(ModeComplete)
			installDistinctOutlinerSnapshot(t, model, outliner)
			if outliner.loadedProjects[0] == project {
				t.Fatal("Outliner snapshot unexpectedly aliases the catalog project")
			}
			if tt.scoped {
				outliner.pushProjectScope(project.ID)
			}
			if !outlinerHasTask(outliner, project.ID, archivedTaskID) {
				t.Fatal("completed task missing before archive")
			}

			var request tea.Msg
			if tt.all {
				request = ArchiveAllCompletedTasksMsg{}
			} else {
				request = ArchiveCompletedTasksMsg{Project: project}
			}
			_, cmd := model.Update(request)
			runSession2AppCommand(t, model, cmd)

			if outliner.crossLoading {
				t.Fatal("Outliner reload did not settle")
			}
			if outlinerHasTask(outliner, project.ID, archivedTaskID) {
				t.Fatal("archived task remained visible without navigation")
			}
			if got := outliner.activeOutlinerMode(); got != ModeComplete {
				t.Fatalf("Outliner mode changed to %q, want %q", got, ModeComplete)
			}
			if tt.scoped && outliner.scopedProjectID != project.ID {
				t.Fatalf("project scope changed to %q, want %q", outliner.scopedProjectID, project.ID)
			}
		})
	}
}

func TestTaskCreatedRefreshesVisibleOutlinerSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "created refresh")
	model, outliner := newSession2OutlinerApp(t, dataDir, true, project)
	// Keep this test focused on the task refresh. Without the completed
	// milestone, the isolated test runtime persists onboarding settings and
	// reloads its intentionally in-memory-only catalog.
	model.runtime.Settings().SetOnboardingProgress(config.OnboardingProgress{TaskCaptured: true})
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
	task, err := models.NewTask("created-refresh-task", committed.GetNextShortID(), models.TaskTypeTask, "Newly created")
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

func TestInlineTaskCreateRefreshesVisibleOutlinerSnapshot(t *testing.T) {
	env := newAppBulkPickerEnv(t, []*models.Task{})
	model := env.appModel()
	outliner := NewOutlinerModel(nil, model.configManager, model.runtime)
	outliner.selectOutlinerMode(ModeAll)
	model.crossOutliner = outliner
	model.views.Register(ViewCrossProjectOutliner, outliner)
	model.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner}
	model.enableTypedNavigation()
	installDistinctOutlinerSnapshot(t, model, outliner)

	_, cmd := model.Update(InlineTaskCreateMsg{
		Project: env.projects[0],
		Title:   "Inline created",
	})
	runSession2AppCommand(t, model, cmd)

	if outliner.crossLoading {
		t.Fatal("Outliner reload did not settle")
	}
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil && row.task.Title == "Inline created" {
			return
		}
	}
	t.Fatal("inline-created task did not appear without navigating away")
}
