package cmd

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

func TestRunTaskMoveToRecordsGroupedUndoAndRepairsConfigOnUndo(t *testing.T) {
	tmpDir := t.TempDir()
	depth := 10
	source := newFileBackedProject(t, tmpDir, "proj-source", "Source")
	dest := newFileBackedProject(t, tmpDir, "proj-dest", "Dest")
	task := addTestTask(t, source, "task-move", "Move me", models.StatusTodo)
	taskID := task.ID
	saveProjectForGroupedCLITest(t, source)
	saveProjectForGroupedCLITest(t, dest)

	cfg := &config.Config{
		DataDir:          tmpDir,
		CurrentProject:   source.ID,
		Projects:         []*models.Project{source, dest},
		UndoHistoryDepth: &depth,
		FocusQueue: []config.FocusQueueItem{
			{TaskID: taskID, ProjectID: source.ID},
		},
		DailyPlan: []config.DailyPlanItem{
			{TaskID: taskID, ProjectID: source.ID, PlannedDate: "2026-06-18"},
		},
		RecentTaskVisits: []config.RecentTaskVisit{
			{TaskID: taskID, ProjectID: source.ID},
		},
	}
	configureFileBackedGroupedCLI(t, cfg)

	taskMoveToForce = true
	t.Cleanup(func() { taskMoveToForce = false })

	if err := runTaskMoveTo(taskMoveToCmd, []string{"1", dest.Slug}); err != nil {
		t.Fatalf("runTaskMoveTo failed: %v", err)
	}

	assertMatchingTopGroupHistory(t, source, dest)
	movedCfg, err := config.NewManager(filepath.Join(tmpDir, "config.json")).Load()
	if err != nil {
		t.Fatalf("reload config after move: %v", err)
	}
	assertConfigRefsProject(t, movedCfg, taskID, dest.ID)

	if err := runUndo(nil, nil); err != nil {
		t.Fatalf("runUndo failed: %v", err)
	}

	if err := storage.LoadProject(source); err != nil {
		t.Fatalf("LoadProject(source): %v", err)
	}
	if err := storage.LoadProject(dest); err != nil {
		t.Fatalf("LoadProject(dest): %v", err)
	}
	if source.GetTaskByID(taskID) == nil {
		t.Fatal("expected undo to restore moved task to source project")
	}
	if dest.GetTaskByID(taskID) != nil {
		t.Fatal("expected undo to remove moved task from destination project")
	}
	reloaded, err := config.NewManager(filepath.Join(tmpDir, "config.json")).Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	assertConfigRefsProject(t, reloaded, taskID, source.ID)

	if err := runRedo(nil, nil); err != nil {
		t.Fatalf("runRedo failed: %v", err)
	}
	if err := storage.LoadProject(source); err != nil {
		t.Fatalf("LoadProject(source after redo): %v", err)
	}
	if err := storage.LoadProject(dest); err != nil {
		t.Fatalf("LoadProject(dest after redo): %v", err)
	}
	if source.GetTaskByID(taskID) != nil {
		t.Fatal("expected redo to remove moved task from source project")
	}
	if dest.GetTaskByID(taskID) == nil {
		t.Fatal("expected redo to restore moved task to destination project")
	}
	redoneCfg, err := config.NewManager(filepath.Join(tmpDir, "config.json")).Load()
	if err != nil {
		t.Fatalf("reload config after redo: %v", err)
	}
	assertConfigRefsProject(t, redoneCfg, taskID, dest.ID)
}

func TestRunTaskArchiveAllRecordsGroupedUndo(t *testing.T) {
	tmpDir := t.TempDir()
	depth := 10
	alpha := newFileBackedProject(t, tmpDir, "proj-alpha", "Alpha")
	beta := newFileBackedProject(t, tmpDir, "proj-beta", "Beta")
	addTestTask(t, alpha, "alpha-done", "Alpha done", models.StatusDone)
	addTestTask(t, beta, "beta-done", "Beta done", models.StatusDone)
	saveProjectForGroupedCLITest(t, alpha)
	saveProjectForGroupedCLITest(t, beta)

	cfg := &config.Config{
		DataDir:          tmpDir,
		CurrentProject:   alpha.ID,
		Projects:         []*models.Project{alpha, beta},
		UndoHistoryDepth: &depth,
	}
	configureFileBackedGroupedCLI(t, cfg)

	archiveForce = true
	t.Cleanup(func() { archiveForce = false })

	if err := runTaskArchiveAll(MutateOutput{Quiet: true}); err != nil {
		t.Fatalf("runTaskArchiveAll failed: %v", err)
	}

	assertMatchingTopGroupHistory(t, alpha, beta)

	if err := runUndo(nil, nil); err != nil {
		t.Fatalf("runUndo failed: %v", err)
	}
	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("LoadProject(alpha): %v", err)
	}
	if err := storage.LoadProject(beta); err != nil {
		t.Fatalf("LoadProject(beta): %v", err)
	}
	if alpha.GetTaskByID("alpha-done") == nil {
		t.Fatal("expected grouped undo to restore alpha archived task")
	}
	if beta.GetTaskByID("beta-done") == nil {
		t.Fatal("expected grouped undo to restore beta archived task")
	}
}

func TestProcessDeferredMovesWithUndoRecordsGroupedSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	depth := 10
	source := newFileBackedProject(t, tmpDir, "proj-source", "Source")
	dest := newFileBackedProject(t, tmpDir, "proj-dest", "Dest")
	task := addTestTask(t, source, "workflow-move", "Workflow move", models.StatusTodo)
	saveProjectForGroupedCLITest(t, source)
	saveProjectForGroupedCLITest(t, dest)

	cfg := &config.Config{
		DataDir:          tmpDir,
		CurrentProject:   source.ID,
		Projects:         []*models.Project{source, dest},
		UndoHistoryDepth: &depth,
	}
	configureFileBackedGroupedCLI(t, cfg)

	recorded, err := processDeferredMovesWithUndo(source, []workflow.DeferredMove{
		{TaskID: task.ID, Destination: dest.Slug},
	}, cfg, nil, deferredMoveUndoOptions{
		Action:  "Workflow Move",
		Context: "#1 Workflow move",
		Depth:   depth,
	})
	if err != nil {
		t.Fatalf("processDeferredMovesWithUndo failed: %v", err)
	}
	if !recorded {
		t.Fatal("expected deferred move to record grouped undo")
	}
	saveProjectForGroupedCLITest(t, source)

	assertMatchingTopGroupHistory(t, source, dest)

	if _, err := storage.PerformUndoForProjects(source, []*models.Project{source, dest}); err != nil {
		t.Fatalf("PerformUndoForProjects failed: %v", err)
	}
	if err := storage.LoadProject(source); err != nil {
		t.Fatalf("LoadProject(source): %v", err)
	}
	if err := storage.LoadProject(dest); err != nil {
		t.Fatalf("LoadProject(dest): %v", err)
	}
	if source.GetTaskByID(task.ID) == nil {
		t.Fatal("expected grouped undo to restore workflow-moved task to source")
	}
	if dest.GetTaskByID(task.ID) != nil {
		t.Fatal("expected grouped undo to remove workflow-moved task from destination")
	}
}

func newFileBackedProject(t *testing.T, root, id, name string) *models.Project {
	t.Helper()
	project, err := models.NewProject(id, name, filepath.Join(root, "projects", models.Slugify(name)))
	if err != nil {
		t.Fatalf("NewProject(%s): %v", name, err)
	}
	return project
}

func addTestTask(t *testing.T, project *models.Project, id, title string, status models.TaskStatus) *models.Task {
	t.Helper()
	task, err := models.NewTask(id, project.GetNextShortID(), models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("NewTask(%s): %v", title, err)
	}
	task.Status = status
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()
	return task
}

func saveProjectForGroupedCLITest(t *testing.T, project *models.Project) {
	t.Helper()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(%s): %v", project.Name, err)
	}
}

func configureFileBackedGroupedCLI(t *testing.T, cfg *config.Config) {
	t.Helper()
	configPath := filepath.Join(cfg.DataDir, "config.json")
	if err := config.NewManager(configPath).Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	oldConfig := viper.GetString("config")
	viper.Set("config", configPath)
	t.Cleanup(func() { viper.Set("config", oldConfig) })

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	t.Cleanup(func() { storage.DefaultRepository = oldRepo })
}

func assertMatchingTopGroupHistory(t *testing.T, first, second *models.Project) {
	t.Helper()
	firstEntries, err := storage.LoadUndoHistory(first)
	if err != nil {
		t.Fatalf("LoadUndoHistory(%s): %v", first.Name, err)
	}
	secondEntries, err := storage.LoadUndoHistory(second)
	if err != nil {
		t.Fatalf("LoadUndoHistory(%s): %v", second.Name, err)
	}
	if len(firstEntries) == 0 || len(secondEntries) == 0 {
		t.Fatalf("expected undo history for both projects, got %d and %d entries", len(firstEntries), len(secondEntries))
	}
	if firstEntries[0].GroupID == "" {
		t.Fatal("expected first project top undo entry to have a group id")
	}
	if firstEntries[0].GroupID != secondEntries[0].GroupID {
		t.Fatalf("group ids differ: %q vs %q", firstEntries[0].GroupID, secondEntries[0].GroupID)
	}
	wantIDs := []string{first.ID, second.ID}
	if got := firstEntries[0].GroupProjectIDs; !sameStringSet(got, wantIDs) {
		t.Fatalf("first group project ids = %v, want %v", got, wantIDs)
	}
	if got := secondEntries[0].GroupProjectIDs; !sameStringSet(got, wantIDs) {
		t.Fatalf("second group project ids = %v, want %v", got, wantIDs)
	}
}

func assertConfigRefsProject(t *testing.T, cfg *config.Config, taskID, projectID string) {
	t.Helper()
	if len(cfg.FocusQueue) != 1 || cfg.FocusQueue[0].TaskID != taskID || cfg.FocusQueue[0].ProjectID != projectID {
		t.Fatalf("FocusQueue = %+v, want task %s in project %s", cfg.FocusQueue, taskID, projectID)
	}
	if len(cfg.DailyPlan) != 1 || cfg.DailyPlan[0].TaskID != taskID || cfg.DailyPlan[0].ProjectID != projectID {
		t.Fatalf("DailyPlan = %+v, want task %s in project %s", cfg.DailyPlan, taskID, projectID)
	}
	if len(cfg.RecentTaskVisits) != 1 || cfg.RecentTaskVisits[0].TaskID != taskID || cfg.RecentTaskVisits[0].ProjectID != projectID {
		t.Fatalf("RecentTaskVisits = %+v, want task %s in project %s", cfg.RecentTaskVisits, taskID, projectID)
	}
}

func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, value := range got {
		seen[value]++
	}
	for _, value := range want {
		if seen[value] == 0 {
			return false
		}
		seen[value]--
	}
	return true
}
