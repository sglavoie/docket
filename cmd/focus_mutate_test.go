package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// setupFocusMutateCLI creates a single-project workspace with one task and a
// viper-backed config path, returning the project, config path, and a cleanup.
func setupFocusMutateCLI(t *testing.T) (*models.Project, string, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "projects", "test-project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project path: %v", err)
	}

	project, err := models.NewProject("proj-focus", "Test Project", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = "test-project"
	project.Path = projectPath
	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Build feature")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.WorkflowState = models.StateReady
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	if err := storage.SaveProject(project); err != nil {
		storage.DefaultRepository = oldRepo
		t.Fatalf("initial SaveProject: %v", err)
	}

	domains := config.NewDomains()
	domains.Catalog.UpsertProject(project)
	domains.Machine.CurrentProject = project.ID
	cfg := config.NewRuntimeState(tmpDir, domains)
	configPath := filepath.Join(tmpDir, "config.json")
	if err := saveTestRuntime(config.NewManager(configPath), cfg); err != nil {
		storage.DefaultRepository = oldRepo
		t.Fatalf("save config: %v", err)
	}

	oldConfig := viper.GetString("config")
	viper.Set("config", configPath)

	return project, configPath, func() {
		viper.Set("config", oldConfig)
		storage.DefaultRepository = oldRepo
	}
}

func reloadFocusConfig(t *testing.T, configPath string) *config.RuntimeState {
	t.Helper()
	cfg, err := config.NewManager(configPath).LoadRuntimeState()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	return cfg
}

// reloadFocusTaskState reads a task's workflow state fresh from disk so an
// assertion sees exactly what `tt task show` would report.
func reloadFocusTaskState(t *testing.T, project *models.Project, taskID string) models.WorkflowState {
	t.Helper()
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	task := fresh.GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("task %s missing after reload", taskID)
	}
	return task.State()
}

func TestFocusQueueAddRemoveMove(t *testing.T) {
	project, configPath, cleanup := setupFocusMutateCLI(t)
	defer cleanup()
	taskID := project.Tasks[0].ID

	if err := runFocusQueueAdd(focusQueueAddCmd, []string{"1"}); err != nil {
		t.Fatalf("queue add: %v", err)
	}
	cfg := reloadFocusConfig(t, configPath)
	if got := cfg.GetFocusQueue(); len(got) != 1 || got[0].TaskID != taskID {
		t.Fatalf("queue after add = %+v, want one item for %s", got, taskID)
	}
	// Queue membership is the StateNext workflow state.
	if st := reloadFocusTaskState(t, project, taskID); st != models.StateNext {
		t.Fatalf("task state after queue add = %s, want next", st)
	}

	// Adding the same task again must be idempotent.
	if err := runFocusQueueAdd(focusQueueAddCmd, []string{"1"}); err != nil {
		t.Fatalf("queue add (idempotent): %v", err)
	}
	if got := reloadFocusConfig(t, configPath).GetFocusQueue(); len(got) != 1 {
		t.Fatalf("queue should stay at 1 item after re-add, got %d", len(got))
	}

	// Remove empties the queue.
	if err := runFocusQueueRemove(focusQueueRemoveCmd, []string{"1"}); err != nil {
		t.Fatalf("queue remove: %v", err)
	}
	if got := reloadFocusConfig(t, configPath).GetFocusQueue(); len(got) != 0 {
		t.Fatalf("queue should be empty after remove, got %d", len(got))
	}
	// Leaving Next clears queue membership: the task is back to a non-Next state.
	if st := reloadFocusTaskState(t, project, taskID); st == models.StateNext {
		t.Fatalf("task state after queue remove = %s, want not next", st)
	}
}

func TestFocusMutationMembershipChecksAreProjectQualified(t *testing.T) {
	planning := &config.PlanningState{
		FocusQueue: []config.FocusQueueItem{{ProjectID: "alpha", TaskID: "shared"}},
		DailyPlan:  []config.DailyPlanItem{{ProjectID: "alpha", TaskID: "shared"}},
	}
	if !focusQueueContains(planning, "shared", "alpha") || !dailyPlanContains(planning, "shared", "alpha") {
		t.Fatal("alpha membership was not found")
	}
	if focusQueueContains(planning, "shared", "beta") || dailyPlanContains(planning, "shared", "beta") {
		t.Fatal("beta duplicate inherited alpha's planning membership")
	}
}

func TestFocusQueueMoveReorders(t *testing.T) {
	project, configPath, cleanup := setupFocusMutateCLI(t)
	defer cleanup()

	// Add a second task so the queue has two entries to reorder.
	second, err := models.NewTask("task-2", 2, models.TaskTypeTask, "Second")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = append(project.Tasks, second)
	project.NextShortID = 3
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}

	if err := runFocusQueueAdd(focusQueueAddCmd, []string{"1"}); err != nil {
		t.Fatalf("queue add 1: %v", err)
	}
	if err := runFocusQueueAdd(focusQueueAddCmd, []string{"2"}); err != nil {
		t.Fatalf("queue add 2: %v", err)
	}

	// Move task 2 to the front.
	if err := runFocusQueueMove(focusQueueMoveCmd, []string{"2", "1"}); err != nil {
		t.Fatalf("queue move: %v", err)
	}
	got := reloadFocusConfig(t, configPath).GetFocusQueue()
	if len(got) != 2 || got[0].TaskID != project.Tasks[1].ID {
		t.Fatalf("queue after move = %+v, want task-2 first", got)
	}
}

// setupFocusMoveScenario builds a single-project workspace whose StateNext
// shortlist diverges from the persisted order index: an indexed StateNext member,
// an UNINDEXED StateNext member (Next by state, absent from the order index), and
// two genuinely-stale terminal tasks that linger in the order index. Terminal
// tasks are used for the stale entries so the legacy-queue migration leaves them
// alone (it revives only non-terminal indexed members), making them the true
// prune-not-migrate case. Returns the project, config path, the three relevant
// task IDs, and a cleanup.
func setupFocusMoveScenario(t *testing.T) (project *models.Project, configPath string, indexedID, unindexedID, staleID string, cleanup func()) {
	t.Helper()
	project, configPath, cleanup = setupFocusMutateCLI(t)

	// task-1 (from setup) becomes the indexed StateNext member.
	indexed := project.Tasks[0]
	indexed.WorkflowState = models.StateNext

	unindexed, err := models.NewTask("task-2", 2, models.TaskTypeTask, "Unindexed on deck")
	if err != nil {
		cleanup()
		t.Fatalf("NewTask unindexed: %v", err)
	}
	unindexed.WorkflowState = models.StateNext

	stale, err := models.NewTask("task-3", 3, models.TaskTypeTask, "Stale index entry")
	if err != nil {
		cleanup()
		t.Fatalf("NewTask stale: %v", err)
	}
	stale.WorkflowState = models.StateDone

	staleTwo, err := models.NewTask("task-4", 4, models.TaskTypeTask, "Second stale index entry")
	if err != nil {
		cleanup()
		t.Fatalf("NewTask stale two: %v", err)
	}
	staleTwo.WorkflowState = models.StateDone

	project.Tasks = []*models.Task{indexed, unindexed, stale, staleTwo}
	project.NextShortID = 5
	if saveErr := storage.SaveProject(project); saveErr != nil {
		cleanup()
		t.Fatalf("save project: %v", saveErr)
	}

	// Seed the order index with the indexed StateNext member and TWO stale non-Next
	// entries; the unindexed StateNext task is deliberately absent. The index length
	// (3) therefore differs from the StateNext shortlist (indexed + unindexed = 2),
	// so the reported size can only be correct if it reflects the shortlist.
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		cleanup()
		t.Fatalf("load workspace: %v", err)
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		cleanup()
		t.Fatalf("clone domains: %v", err)
	}
	domains.Planning.SetFocusQueue([]config.FocusQueueItem{
		{TaskID: indexed.ID, ProjectID: project.ID},
		{TaskID: stale.ID, ProjectID: project.ID},
		{TaskID: staleTwo.ID, ProjectID: project.ID},
	})
	if err := commitCLIPlanning(workspace, domains.Planning); err != nil {
		cleanup()
		t.Fatalf("seed queue: %v", err)
	}

	return project, configPath, indexed.ID, unindexed.ID, stale.ID, cleanup
}

// TestFocusQueueMoveUsesStateNextMembership locks FR-011-003: `tt focus queue move`
// operates on the StateNext shortlist, not the raw order index. It must (1) accept
// an unindexed StateNext task and insert it at the requested position, (2) reject a
// non-Next task that merely lingers in the order index, ignoring/pruning stale
// entries, and (3) report a size that reflects the StateNext shortlist.
func TestFocusQueueMoveUsesStateNextMembership(t *testing.T) {
	t.Run("moves an unindexed StateNext task into position", func(t *testing.T) {
		project, configPath, indexedID, unindexedID, staleID, cleanup := setupFocusMoveScenario(t)
		defer cleanup()

		// task-2 is StateNext but absent from the order index. Moving it to the front
		// must succeed (old behavior rejected it as "not in queue").
		if err := runFocusQueueMove(focusQueueMoveCmd, []string{"2", "1"}); err != nil {
			t.Fatalf("queue move (unindexed StateNext): %v", err)
		}

		// The persisted order index now encodes the canonical shortlist with the moved
		// task first: [unindexed, indexed]. Stale non-Next entries are pruned.
		got := reloadFocusConfig(t, configPath).GetFocusQueue()
		if len(got) != 2 {
			t.Fatalf("queue after move = %+v, want 2 entries (StateNext shortlist, stale pruned)", got)
		}
		if got[0].TaskID != unindexedID {
			t.Errorf("first entry = %q, want moved unindexed task %q", got[0].TaskID, unindexedID)
		}
		if got[1].TaskID != indexedID {
			t.Errorf("second entry = %q, want indexed task %q", got[1].TaskID, indexedID)
		}
		for _, item := range got {
			if item.TaskID == staleID {
				t.Errorf("stale non-Next entry %q must be pruned from the order index", staleID)
			}
		}
		_ = project
	})

	t.Run("reports the StateNext shortlist size", func(t *testing.T) {
		_, _, _, _, _, cleanup := setupFocusMoveScenario(t)
		defer cleanup()

		if flagErr := focusQueueMoveCmd.Flags().Set("json", "true"); flagErr != nil {
			t.Fatalf("enable JSON output: %v", flagErr)
		}
		t.Cleanup(func() {
			if flagErr := focusQueueMoveCmd.Flags().Set("json", "false"); flagErr != nil {
				t.Errorf("disable JSON output: %v", flagErr)
			}
		})

		stdout, _, err := captureDoctorOutput(t, func() error {
			return runFocusQueueMove(focusQueueMoveCmd, []string{"2", "1"})
		})
		if err != nil {
			t.Fatalf("queue move (json): %v", err)
		}
		var res focusMutationResult
		if err := json.Unmarshal([]byte(stdout), &res); err != nil {
			t.Fatalf("unmarshal %q: %v", stdout, err)
		}
		// The seeded order index had 3 entries; the StateNext shortlist has 2. The
		// reported size must be the shortlist, not the stale-padded index.
		if res.Size != 2 {
			t.Fatalf("reported size = %d, want 2 (StateNext shortlist, not the 3-entry index)", res.Size)
		}
	})

	t.Run("rejects a non-Next task with a stale index entry", func(t *testing.T) {
		project, configPath, indexedID, _, staleID, cleanup := setupFocusMoveScenario(t)
		defer cleanup()

		// task-3 is a terminal (non-Next) task that lingers in the order index. Moving
		// it must be a no-op reorder (old behavior accepted the stale entry and moved
		// it to the front) and must not revive it into Next.
		if err := runFocusQueueMove(focusQueueMoveCmd, []string{"3", "1"}); err != nil {
			t.Fatalf("queue move (stale non-Next): %v", err)
		}

		// The stale task did not change state and did not jump to the front of the
		// order index — the reject path leaves the seeded index untouched.
		if st := reloadFocusTaskState(t, project, staleID); st == models.StateNext {
			t.Fatalf("stale task state = %s, want unchanged (not next)", st)
		}
		got := reloadFocusConfig(t, configPath).GetFocusQueue()
		if len(got) == 0 || got[0].TaskID != indexedID {
			t.Fatalf("order index after rejected move = %+v, want indexed member still first", got)
		}
	})
}

func TestFocusTodayAddRemoveClear(t *testing.T) {
	project, configPath, cleanup := setupFocusMutateCLI(t)
	defer cleanup()
	taskID := project.Tasks[0].ID

	if err := runFocusTodayAdd(focusTodayAddCmd, []string{"1"}); err != nil {
		t.Fatalf("today add: %v", err)
	}
	plan := reloadFocusConfig(t, configPath).Planning().GetDailyPlan()
	if len(plan) != 1 || plan[0].TaskID != taskID || plan[0].PlannedDate == "" {
		t.Fatalf("plan after add = %+v, want one dated item for %s", plan, taskID)
	}

	if err := runFocusTodayRemove(focusTodayRemoveCmd, []string{"1"}); err != nil {
		t.Fatalf("today remove: %v", err)
	}
	if got := reloadFocusConfig(t, configPath).Planning().GetDailyPlan(); len(got) != 0 {
		t.Fatalf("plan should be empty after remove, got %d", len(got))
	}

	// Re-add then clear.
	if err := runFocusTodayAdd(focusTodayAddCmd, []string{"1"}); err != nil {
		t.Fatalf("today re-add: %v", err)
	}
	if err := runFocusTodayClear(focusTodayClearCmd, nil); err != nil {
		t.Fatalf("today clear: %v", err)
	}
	if got := reloadFocusConfig(t, configPath).Planning().GetDailyPlan(); len(got) != 0 {
		t.Fatalf("plan should be empty after clear, got %d", len(got))
	}
}

// TestFocusCLITodayNextExclusivity locks FR-008-001: the CLI Today and Next
// horizons are mutually exclusive, matching the TUI toggle behavior. Adding a
// planned (Today) task to the queue moves it to Next and drops the Daily Plan
// entry; adding a queued (Next) task to Today transitions it out of Next and
// drops its order entry. Neither leaves the task in both horizons.
func TestFocusCLITodayNextExclusivity(t *testing.T) {
	t.Run("today then queue add leaves only Next", func(t *testing.T) {
		project, configPath, cleanup := setupFocusMutateCLI(t)
		defer cleanup()
		taskID := project.Tasks[0].ID

		if err := runFocusTodayAdd(focusTodayAddCmd, []string{"1"}); err != nil {
			t.Fatalf("today add: %v", err)
		}
		if plan := reloadFocusConfig(t, configPath).Planning().GetDailyPlan(); len(plan) != 1 {
			t.Fatalf("precondition: task should be planned for Today, got %d entries", len(plan))
		}

		if err := runFocusQueueAdd(focusQueueAddCmd, []string{"1"}); err != nil {
			t.Fatalf("queue add: %v", err)
		}

		cfg := reloadFocusConfig(t, configPath)
		if st := reloadFocusTaskState(t, project, taskID); st != models.StateNext {
			t.Fatalf("task state after queue add = %s, want next", st)
		}
		if q := cfg.GetFocusQueue(); len(q) != 1 || q[0].TaskID != taskID {
			t.Fatalf("focus queue after queue add = %+v, want one entry for %s", q, taskID)
		}
		if plan := cfg.Planning().GetDailyPlan(); len(plan) != 0 {
			t.Fatalf("daily plan after queue add = %+v, want empty (Today and Next are exclusive)", plan)
		}
	})

	t.Run("queue then today add leaves only Today", func(t *testing.T) {
		project, configPath, cleanup := setupFocusMutateCLI(t)
		defer cleanup()
		taskID := project.Tasks[0].ID

		if err := runFocusQueueAdd(focusQueueAddCmd, []string{"1"}); err != nil {
			t.Fatalf("queue add: %v", err)
		}
		if st := reloadFocusTaskState(t, project, taskID); st != models.StateNext {
			t.Fatalf("precondition: task should be StateNext, got %s", st)
		}

		if err := runFocusTodayAdd(focusTodayAddCmd, []string{"1"}); err != nil {
			t.Fatalf("today add: %v", err)
		}

		cfg := reloadFocusConfig(t, configPath)
		if st := reloadFocusTaskState(t, project, taskID); st == models.StateNext {
			t.Fatalf("task state after today add = %s, want out of Next", st)
		}
		if q := cfg.GetFocusQueue(); len(q) != 0 {
			t.Fatalf("focus queue after today add = %+v, want empty order entry dropped", q)
		}
		plan := cfg.Planning().GetDailyPlan()
		if len(plan) != 1 || plan[0].TaskID != taskID {
			t.Fatalf("daily plan after today add = %+v, want one entry for %s", plan, taskID)
		}
	})
}

func TestFocusQueueAddUnknownTaskErrors(t *testing.T) {
	_, _, cleanup := setupFocusMutateCLI(t)
	defer cleanup()

	if err := runFocusQueueAdd(focusQueueAddCmd, []string{"999"}); err == nil {
		t.Fatal("expected error adding a nonexistent task to the queue")
	}
}
