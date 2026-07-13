package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// TestRunTaskMoveToCommitsThroughLedgerAndRepairsConfig proves the migrated
// cross-project move commits the source removal, destination insertion, and the
// durable planning-ref reassignment (Focus Queue, Daily Plan) as ONE transaction
// recorded in the workspace undo ledger — not the legacy per-project grouped undo
// history. Ledger undo restores the whole transaction (task back to source, config
// file back to its pre-move image), and ledger redo re-applies it. The
// machine-local recent visits are reassigned separately after the commit, so they
// are not part of the redone transaction image (documented non-authoritative UI
// state).
func TestRunTaskMoveToCommitsThroughLedgerAndRepairsConfig(t *testing.T) {
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

	// After the move, every config ref points at the destination and the task lives
	// in the destination project.
	movedCfg, err := config.NewManager(filepath.Join(tmpDir, "config.json")).Load()
	if err != nil {
		t.Fatalf("reload config after move: %v", err)
	}
	assertConfigRefsProject(t, movedCfg, taskID, dest.ID)

	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(tmpDir), depth, []string{tmpDir})

	// Ledger undo reverses the whole move transaction: the task returns to source and
	// the config file is restored to its full pre-move image (all refs → source).
	if _, err = ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger.Undo failed: %v", err)
	}
	if err = storage.LoadProject(source); err != nil {
		t.Fatalf("LoadProject(source): %v", err)
	}
	if err = storage.LoadProject(dest); err != nil {
		t.Fatalf("LoadProject(dest): %v", err)
	}
	if source.GetTaskByID(taskID) == nil {
		t.Fatal("expected undo to restore moved task to source project")
	}
	if dest.GetTaskByID(taskID) != nil {
		t.Fatal("expected undo to remove moved task from destination project")
	}
	undoneCfg, err := config.NewManager(filepath.Join(tmpDir, "config.json")).Load()
	if err != nil {
		t.Fatalf("reload config after undo: %v", err)
	}
	// Ledger undo reverses only the workspace transaction: the durable planning
	// refs (Focus Queue, Daily Plan) return to source. RecentTaskVisits is
	// machine-local (machine.json), excluded from the workspace transaction, so it
	// stays where the post-commit best-effort repair left it (destination) and is
	// not restored by ledger undo.
	if len(undoneCfg.FocusQueue) != 1 || undoneCfg.FocusQueue[0].ProjectID != source.ID {
		t.Fatalf("FocusQueue after undo = %+v, want project %s", undoneCfg.FocusQueue, source.ID)
	}
	if len(undoneCfg.DailyPlan) != 1 || undoneCfg.DailyPlan[0].ProjectID != source.ID {
		t.Fatalf("DailyPlan after undo = %+v, want project %s", undoneCfg.DailyPlan, source.ID)
	}

	// Ledger redo re-applies the move: the task returns to the destination and the
	// durable planning refs (Focus Queue, Daily Plan) are reassigned to it. Recent
	// visits were repaired outside the transaction, so the redone image leaves them
	// at the source — matching their documented best-effort semantics.
	if _, err = ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger.Redo failed: %v", err)
	}
	if err = storage.LoadProject(source); err != nil {
		t.Fatalf("LoadProject(source after redo): %v", err)
	}
	if err = storage.LoadProject(dest); err != nil {
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
	if len(redoneCfg.FocusQueue) != 1 || redoneCfg.FocusQueue[0].ProjectID != dest.ID {
		t.Fatalf("FocusQueue after redo = %+v, want project %s", redoneCfg.FocusQueue, dest.ID)
	}
	if len(redoneCfg.DailyPlan) != 1 || redoneCfg.DailyPlan[0].ProjectID != dest.ID {
		t.Fatalf("DailyPlan after redo = %+v, want project %s", redoneCfg.DailyPlan, dest.ID)
	}
}

// TestRunTaskArchiveAllCommitsThroughLedgerAsOneTransaction proves the migrated
// archive-all routes every project's archive through the application layer as ONE
// atomic, ledger-backed transaction — not the legacy per-project grouped undo
// history. Both projects are archived together (their completed tasks removed from
// tasks.md and appended to completed.md), a single workspace-ledger undo reverses
// the whole batch (both tasks restored to their active tasks.md), and a ledger redo
// re-applies it.
func TestRunTaskArchiveAllCommitsThroughLedgerAsOneTransaction(t *testing.T) {
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

	// Both projects' completed tasks are gone from their active tasks.md.
	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("LoadProject(alpha): %v", err)
	}
	if err := storage.LoadProject(beta); err != nil {
		t.Fatalf("LoadProject(beta): %v", err)
	}
	if alpha.GetTaskByID("alpha-done") != nil {
		t.Fatal("expected archive to remove alpha completed task from active tasks.md")
	}
	if beta.GetTaskByID("beta-done") != nil {
		t.Fatal("expected archive to remove beta completed task from active tasks.md")
	}

	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(tmpDir), depth, []string{tmpDir})

	// One ledger undo reverses the whole archive-all transaction: both tasks return
	// to their active tasks.md.
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger.Undo failed: %v", err)
	}
	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("LoadProject(alpha) after undo: %v", err)
	}
	if err := storage.LoadProject(beta); err != nil {
		t.Fatalf("LoadProject(beta) after undo: %v", err)
	}
	if alpha.GetTaskByID("alpha-done") == nil {
		t.Fatal("expected ledger undo to restore alpha archived task")
	}
	if beta.GetTaskByID("beta-done") == nil {
		t.Fatal("expected ledger undo to restore beta archived task")
	}

	// One ledger redo re-applies the archive: both tasks leave their active tasks.md
	// again.
	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger.Redo failed: %v", err)
	}
	if err := storage.LoadProject(alpha); err != nil {
		t.Fatalf("LoadProject(alpha) after redo: %v", err)
	}
	if err := storage.LoadProject(beta); err != nil {
		t.Fatalf("LoadProject(beta) after redo: %v", err)
	}
	if alpha.GetTaskByID("alpha-done") != nil {
		t.Fatal("expected ledger redo to re-archive alpha task")
	}
	if beta.GetTaskByID("beta-done") != nil {
		t.Fatal("expected ledger redo to re-archive beta task")
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
	task.WorkflowState = models.DeriveWorkflowState(status, "", "")
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

// TestProjectDeleteRestorePurgeThroughJournal proves the migrated project
// trash/restore/purge commands drive the real transaction journal end to end: a
// delete moves the project directory into a versioned trash entry and drops it from
// the catalog atomically, a restore moves it back and re-adds it, and a purge
// permanently removes the entry — all against a file-backed workspace.
func TestProjectDeleteRestorePurgeThroughJournal(t *testing.T) {
	tmpDir := t.TempDir()
	project := newFileBackedProject(t, tmpDir, "proj-doomed", "Doomed")
	addTestTask(t, project, "doomed-task", "A task", models.StatusTodo)
	saveProjectForGroupedCLITest(t, project)

	cfg := &config.Config{
		DataDir:        tmpDir,
		CurrentProject: project.ID,
		Projects:       []*models.Project{project},
	}
	configureFileBackedGroupedCLI(t, cfg)
	configPath := filepath.Join(tmpDir, "config.json")

	projectDeleteForce = true
	purgeForce = true
	t.Cleanup(func() { projectDeleteForce = false; purgeForce = false })

	// Delete → project directory is gone and the catalog no longer lists it.
	if err := runProjectDelete(projectDeleteCmd, []string{"doomed"}); err != nil {
		t.Fatalf("runProjectDelete: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, "projects", "doomed")); !os.IsNotExist(statErr) {
		t.Fatalf("expected project dir removed, stat err = %v", statErr)
	}
	entries, err := storage.ListTrashEntries(tmpDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("ListTrashEntries after delete = %d entries, err %v", len(entries), err)
	}
	afterDelete, err := config.NewManager(configPath).Load()
	if err != nil {
		t.Fatalf("reload config after delete: %v", err)
	}
	if afterDelete.GetProject("doomed") != nil {
		t.Fatal("expected project dropped from catalog after delete")
	}

	// Restore → project directory returns and the catalog lists it again.
	if err := runProjectRestore(projectRestoreCmd, []string{"doomed"}); err != nil {
		t.Fatalf("runProjectRestore: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, "projects", "doomed", "tasks.md")); statErr != nil {
		t.Fatalf("expected project restored: %v", statErr)
	}
	if entries, _ := storage.ListTrashEntries(tmpDir); len(entries) != 0 {
		t.Fatalf("expected trash empty after restore, got %d entries", len(entries))
	}
	afterRestore, err := config.NewManager(configPath).Load()
	if err != nil {
		t.Fatalf("reload config after restore: %v", err)
	}
	restored := afterRestore.GetProject("doomed")
	if restored == nil {
		t.Fatal("expected project re-added to catalog after restore")
	}
	if err := storage.LoadProject(restored); err != nil {
		t.Fatalf("LoadProject(restored): %v", err)
	}
	if restored.GetTaskByID("doomed-task") == nil {
		t.Fatal("expected restored project to retain its task")
	}

	// Delete again, then purge → the trash entry is permanently gone.
	if err := runProjectDelete(projectDeleteCmd, []string{"doomed"}); err != nil {
		t.Fatalf("runProjectDelete(2): %v", err)
	}
	if err := runProjectPurge(projectPurgeCmd, []string{"doomed"}); err != nil {
		t.Fatalf("runProjectPurge: %v", err)
	}
	if entries, _ := storage.ListTrashEntries(tmpDir); len(entries) != 0 {
		t.Fatalf("expected trash empty after purge, got %d entries", len(entries))
	}
}
