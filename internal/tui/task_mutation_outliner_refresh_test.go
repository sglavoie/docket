package tui

import (
	"slices"
	"testing"

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

func TestTaskLineEditCrossProjectRelocationRefreshesOutlinerSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	source := newSession2Project(t, dataDir, "line edit source")
	destination := newSession2Project(t, dataDir, "line edit destination")
	moving := source.Tasks[0]
	neighbor, err := models.NewGenericTask("line-edit-neighbor", source.GetNextShortID(), "Origin neighbor")
	if err != nil {
		t.Fatalf("create origin neighbor: %v", err)
	}
	neighbor.WorkflowState = models.StateBacklog
	neighbor.Project = source
	source.Tasks = append(source.Tasks, neighbor)
	source.BuildTaskTree()
	if err := storage.SaveProject(source); err != nil {
		t.Fatalf("save source fixture: %v", err)
	}

	model, outliner := newSession2OutlinerApp(t, dataDir, source, destination)
	installDistinctOutlinerSnapshot(t, model, outliner)
	sourceIdentity := TaskIdentity{ProjectID: source.ID, TaskID: moving.ID}
	for index, row := range outliner.rows {
		if row.isTaskRow() && row.ProjectID == source.ID && row.task.ID == moving.ID {
			outliner.cursor = index
			break
		}
	}
	outliner.syncJumpProjection()
	capture := outliner.captureOutlinerRelocation([]TaskIdentity{sourceIdentity})
	if capture == nil {
		t.Fatal("capture task-line relocation: nil")
	}

	committedSource := &models.Project{ID: source.ID, Slug: source.Slug, Name: source.Name, Path: source.Path}
	committedDestination := &models.Project{ID: destination.ID, Slug: destination.Slug, Name: destination.Name, Path: destination.Path}
	if err := storage.LoadProject(committedSource); err != nil {
		t.Fatalf("load committed source: %v", err)
	}
	if err := storage.LoadProject(committedDestination); err != nil {
		t.Fatalf("load committed destination: %v", err)
	}
	committedMoving := committedSource.GetTaskByID(moving.ID)
	if committedMoving == nil {
		t.Fatalf("load moving task %q: missing", moving.ID)
	}
	committedSource.Tasks = slices.DeleteFunc(committedSource.Tasks, func(task *models.Task) bool {
		return task != nil && task.ID == moving.ID
	})
	committedMoving.ShortID = committedDestination.GetNextShortID()
	committedMoving.Project = committedDestination
	committedMoving.Parent = nil
	committedDestination.Tasks = append(committedDestination.Tasks, committedMoving)
	committedSource.BuildTaskTree()
	committedDestination.BuildTaskTree()
	if err := storage.SaveProject(committedSource); err != nil {
		t.Fatalf("save committed source: %v", err)
	}
	if err := storage.SaveProject(committedDestination); err != nil {
		t.Fatalf("save committed destination: %v", err)
	}
	if outlinerHasTask(outliner, destination.ID, moving.ID) {
		t.Fatal("Outliner snapshot was pre-mutated before handler refresh")
	}

	finalIdentity := TaskIdentity{ProjectID: destination.ID, TaskID: moving.ID}
	model.handleTaskLineEdited(TaskLineEditedMsg{
		Result: domainapp.TaskLineEditResult{
			Task:               domainapp.TaskLocator{ProjectID: destination.ID, TaskID: moving.ID},
			ShortID:            committedMoving.ShortID,
			Title:              committedMoving.Title,
			AffectedProjectIDs: []string{source.ID, destination.ID},
			TransactionID:      "line-edit-relocation",
			Changed:            true,
		},
		Source:     sourceIdentity,
		Relocation: capture,
	})
	if got := outliner.currentTaskIdentity(); got != finalIdentity {
		t.Fatalf("post-edit cursor = %+v, want destination %+v", got, finalIdentity)
	}
	if len(outliner.relocationHistory.back) != 1 {
		t.Fatalf("post-edit origin history = %d, want 1", len(outliner.relocationHistory.back))
	}

	outliner.jumpBackward()
	if got := outliner.currentTaskIdentity(); got != (TaskIdentity{ProjectID: source.ID, TaskID: neighbor.ID}) {
		t.Fatalf("Ctrl+O destination = %+v, want origin neighbor", got)
	}
	outliner.jumpForward()
	if got := outliner.currentTaskIdentity(); got != finalIdentity {
		t.Fatalf("Ctrl+I destination = %+v, want moved task", got)
	}
}

func TestTaskCreatedRefreshesVisibleOutlinerSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	project := newSession2Project(t, dataDir, "created refresh")
	model, outliner := newSession2OutlinerApp(t, dataDir, project)
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
	task.WorkflowState = models.StateBacklog
	task.Project = committed
	committed.Tasks = append(committed.Tasks, task)
	committed.BuildTaskTree()
	if err := storage.SaveProject(committed); err != nil {
		t.Fatalf("save created task: %v", err)
	}

	model.pushView(ViewTaskComposer)
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

func TestPlanningCaptureRefreshesRuntimeAndOutlinerSection(t *testing.T) {
	for _, tc := range []struct {
		destination domainapp.CaptureDestination
		group       models.WorkflowStateGroup
	}{
		{destination: domainapp.CaptureDestinationNext, group: models.StateGroupNext},
		{destination: domainapp.CaptureDestinationToday, group: models.StateGroupToday},
	} {
		t.Run(string(tc.destination), func(t *testing.T) {
			dataDir := t.TempDir()
			project := newSession2Project(t, dataDir, "planning capture "+string(tc.destination))
			model, outliner := newSession2OutlinerApp(t, dataDir, project)
			if err := model.configManager.CommitPlanning(config.NewPlanningState()); err != nil {
				t.Fatalf("seed planning: %v", err)
			}
			if !model.refreshRuntime() {
				t.Fatal("seeded planning snapshot did not refresh")
			}
			model.appService.WithPlanningArtifacts(planningReconcileArtifactBuilder(model.configManager))
			model.features = newFeatureCoordinators(model.appService)
			installDistinctOutlinerSnapshot(t, model, outliner)

			composer := NewTaskComposerModel(TaskComposerPreset{
				Kind:        ComposerProject,
				Project:     project,
				AllProjects: []*models.Project{project},
				Shared: TaskComposerSharedAttributes{
					Destination: domainapp.CaptureDestinationInbox,
				},
			}, model.runtime)
			composer.SetCaptureCoordinator(model.features.Capture)
			composer.input.SetValue("Planned task $" + string(tc.destination))

			cmd := composer.createCompact(false)
			if cmd == nil {
				t.Fatalf("$%s capture returned nil command: %+v", tc.destination, composer.errors)
			}
			created, ok := cmd().(TaskCreatedMsg)
			if !ok || created.Result == nil || !created.PlanningChanged {
				t.Fatalf("capture result = %#v, want planning-aware TaskCreatedMsg", created)
			}

			_, refreshCmd := model.handleTaskCreated(created)
			if want := "Add Task: #2 Planned task · u undo"; model.statusMessage != want {
				t.Fatalf("status = %q, want %q", model.statusMessage, want)
			}
			runSession2AppCommand(t, model, refreshCmd)

			identity := TaskIdentity{ProjectID: project.ID, TaskID: created.Result.TaskID}
			planning, err := model.configManager.DomainCoordinator().LoadPlanning()
			if err != nil {
				t.Fatalf("load durable planning: %v", err)
			}
			switch tc.destination {
			case domainapp.CaptureDestinationNext:
				if !nextOrderContains(planning, identity) || !model.nextOrderIDs[identity] {
					t.Fatalf("Next membership disk/runtime = %+v / %+v", planning.GetNext(), model.nextOrderIDs)
				}
			case domainapp.CaptureDestinationToday:
				if !todayContains(planning, identity) || !model.todayIDs[identity] {
					t.Fatalf("Today membership disk/runtime = %+v / %+v", planning.GetToday(), model.todayIDs)
				}
			default:
				t.Fatalf("destination %q has no planning membership assertion", tc.destination)
			}

			matches := 0
			for _, row := range outliner.rows {
				if !row.isTaskRow() || row.ProjectID != identity.ProjectID || row.task == nil || row.task.ID != identity.TaskID {
					continue
				}
				matches++
				if tc.group == models.StateGroupNext && !row.IsNextSection {
					t.Fatalf("created row is not in Next: %+v", row)
				}
				if tc.group == models.StateGroupToday && !row.IsTodaySection {
					t.Fatalf("created row is not in Today: %+v", row)
				}
				if row.task.State() != models.StateBacklog {
					t.Fatalf("planning task state = %s, want backlog lifecycle", row.task.State())
				}
			}
			if matches != 1 {
				t.Fatalf("created task rendered %d times, want one planning-section row", matches)
			}
		})
	}
}

func TestPlanningOrderMutationRefreshPreservesQualifiedCursorIdentity(t *testing.T) {
	alphaTask := primaryRowsTask("shared", models.StateBacklog)
	betaTask := primaryRowsTask("shared", models.StateBacklog)
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)
	outliner, runtime := primaryRowsOutliner(alpha, beta)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: alphaTask.ID},
		{ProjectID: beta.ID, TaskID: betaTask.ID},
	})
	outliner.selectOutlinerMode(ModeToday)
	if !outliner.moveCursorToProjectTask(betaTask.ID, beta.ID) {
		t.Fatal("cursor did not land on beta/shared")
	}

	runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: beta.ID, TaskID: betaTask.ID},
		{ProjectID: alpha.ID, TaskID: alphaTask.ID},
	})
	outliner.crossForceRebuild = true
	rebuildOutlinerPreservingCursor(outliner)

	var got []string
	for _, row := range outliner.rows {
		if row.isTaskRow() && row.task != nil {
			got = append(got, row.ProjectID+"/"+row.task.ID)
		}
	}
	want := []string{"beta/shared", "alpha/shared"}
	if !slices.Equal(got, want) {
		t.Fatalf("refreshed Today rows = %v, want %v", got, want)
	}
	selectedTask, selectedProject := outliner.getTaskRef()
	if selectedTask == nil || selectedProject == nil ||
		selectedTask.ID != betaTask.ID || selectedProject.ID != beta.ID {
		t.Fatalf("cursor after refresh = %v/%v, want beta/shared", selectedProject, selectedTask)
	}
}
