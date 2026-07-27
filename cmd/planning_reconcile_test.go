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

// TestFocusPlanningUndoRestoresExactPreMutationState locks FR-009-001: a
// Today->Next move folds the workflow-state change, the Daily Plan drop, and the
// Next order entry into one durable ledger transaction, so a single `tt undo`
// restores the exact pre-mutation workflow state, Today membership, and Next
// ordering together.
func TestFocusPlanningUndoRestoresExactPreMutationState(t *testing.T) {
	project, configPath, cleanup := setupFocusMutateCLI(t)
	defer cleanup()
	taskID := project.Tasks[0].ID

	// Pre-mutation horizon: the task is planned for Today (a pure planning commit
	// that does not touch the ledger).
	if err := runFocusTodayAdd(focusTodayAddCmd, []string{"1"}); err != nil {
		t.Fatalf("today add: %v", err)
	}
	if st := reloadFocusTaskState(t, project, taskID); st != models.StateReady {
		t.Fatalf("precondition state = %s, want ready", st)
	}

	// Move Today -> Next: one ledger transaction carrying task state + planning.
	if err := runFocusQueueAdd(focusQueueAddCmd, []string{"1"}); err != nil {
		t.Fatalf("queue add: %v", err)
	}
	moved := reloadFocusConfig(t, configPath)
	if st := reloadFocusTaskState(t, project, taskID); st != models.StateNext {
		t.Fatalf("after move state = %s, want next", st)
	}
	if q := moved.GetFocusQueue(); len(q) != 1 || q[0].TaskID != taskID {
		t.Fatalf("after move queue = %+v, want one entry", q)
	}
	if plan := moved.Planning().GetDailyPlan(); len(plan) != 0 {
		t.Fatalf("after move daily plan = %+v, want empty", plan)
	}

	// Undo restores every part of the pre-mutation horizon atomically.
	if err := runUndo(undoCmd, nil); err != nil {
		t.Fatalf("undo: %v", err)
	}
	restored := reloadFocusConfig(t, configPath)
	if st := reloadFocusTaskState(t, project, taskID); st != models.StateReady {
		t.Fatalf("after undo state = %s, want ready (pre-mutation)", st)
	}
	if q := restored.GetFocusQueue(); len(q) != 0 {
		t.Fatalf("after undo queue = %+v, want empty (Next order restored)", q)
	}
	plan := restored.Planning().GetDailyPlan()
	if len(plan) != 1 || plan[0].TaskID != taskID {
		t.Fatalf("after undo daily plan = %+v, want the pre-mutation Today entry", plan)
	}
}

// TestTaskNextRemovesToday locks FR-009-002 for the canonical CLI state command:
// `tt task next` on a Today task removes it from the Daily Plan as part of the
// same transaction, regardless of the fact that the move went through the generic
// state transition rather than a dedicated focus adapter.
func TestTaskNextRemovesToday(t *testing.T) {
	project, configPath, cleanup := setupFocusMutateCLI(t)
	defer cleanup()
	taskID := project.Tasks[0].ID

	if err := runFocusTodayAdd(focusTodayAddCmd, []string{"1"}); err != nil {
		t.Fatalf("today add: %v", err)
	}
	if plan := reloadFocusConfig(t, configPath).Planning().GetDailyPlan(); len(plan) != 1 {
		t.Fatalf("precondition: task should be planned for Today, got %d", len(plan))
	}

	if err := runAppTransition(taskStateCmd, []string{"1"}, models.StateNext); err != nil {
		t.Fatalf("task next: %v", err)
	}

	cfg := reloadFocusConfig(t, configPath)
	if st := reloadFocusTaskState(t, project, taskID); st != models.StateNext {
		t.Fatalf("state after task next = %s, want next", st)
	}
	if plan := cfg.Planning().GetDailyPlan(); len(plan) != 0 {
		t.Fatalf("daily plan after task next = %+v, want empty (Today/Next exclusive)", plan)
	}
}

// TestWorkflowRuleNextPreservesPlanningExclusivity locks FR-009-002 for workflow
// cascades: when a workflow rule moves a Today task to the Next section (StateNext)
// during a transition, the task is dropped from the Daily Plan in the same
// transaction, so a rule cascade cannot leave a task in both horizons.
func TestWorkflowRuleNextPreservesPlanningExclusivity(t *testing.T) {
	rule := workflow.WorkflowRule{
		ID:       "r-start-to-next",
		Name:     "Starting a task queues it as Next",
		Enabled:  true,
		Triggers: []workflow.RuleTrigger{{Field: workflow.FieldStateChanged, Operation: workflow.OpEquals, Value: "*->in_progress"}},
		Actions:  []workflow.RuleAction{{ActionType: workflow.ActionSetState, Value: string(models.StateNext)}},
	}
	project, configPath, cleanup := setupFocusMutateCLIWithRules(t, rule)
	defer cleanup()
	taskID := project.Tasks[0].ID

	if err := runFocusTodayAdd(focusTodayAddCmd, []string{"1"}); err != nil {
		t.Fatalf("today add: %v", err)
	}
	if plan := reloadFocusConfig(t, configPath).Planning().GetDailyPlan(); len(plan) != 1 {
		t.Fatalf("precondition: task should be planned for Today, got %d", len(plan))
	}

	// Transition to in_progress; the rule cascade sets the task to the Next section.
	if err := runAppTransition(taskStateCmd, []string{"1"}, models.StateInProgress); err != nil {
		t.Fatalf("task in_progress: %v", err)
	}

	cfg := reloadFocusConfig(t, configPath)
	if st := reloadFocusTaskState(t, project, taskID); st != models.StateNext {
		t.Fatalf("state after rule cascade = %s, want next (rule moved it to Next)", st)
	}
	if plan := cfg.Planning().GetDailyPlan(); len(plan) != 0 {
		t.Fatalf("daily plan after rule cascade = %+v, want empty (rule-driven Next drops Today)", plan)
	}
}

// setupFocusMutateCLIWithRules mirrors setupFocusMutateCLI but seeds the
// automation domain with the given workflow rules so a transition exercises the
// rule cascade.
func setupFocusMutateCLIWithRules(t *testing.T, rules ...workflow.WorkflowRule) (*models.Project, string, func()) {
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
	domains.Automation.WorkflowRules = rules
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
