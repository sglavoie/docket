package app_test

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// stateBasedTagRule fires on the task's current status without requiring an
// event transition.
func stateBasedTagRule() workflow.WorkflowRule {
	return workflow.WorkflowRule{
		ID:      "r-status-tag",
		Name:    "Tag done tasks",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionAddTag, Value: "shipped"}},
	}
}

// contextualTagRule fires only on a status transition into done, so it requires
// the pre-mutation state (Contextual=true) to trigger.
func contextualTagRule() workflow.WorkflowRule {
	return workflow.WorkflowRule{
		ID:      "r-status-changed-tag",
		Name:    "Tag on completion",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatusChanged, Operation: workflow.OpEquals, Value: "*->done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionAddTag, Value: "shipped"}},
	}
}

func TestStateWithWorkflowStateBasedRuleFires(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	res, err := svc.StateWithWorkflow("t1", models.StateDone, app.SingleTaskWorkflowOptions{
		Rules:      []workflow.WorkflowRule{stateBasedTagRule()},
		UndoAction: "Complete Task",
	})
	if err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if t1.State() != models.StateDone {
		t.Errorf("state = %s, want done", t1.State())
	}
	if !t1.HasTag("shipped") {
		t.Errorf("state-based rule did not apply tag; tags = %v", t1.Tags)
	}
	if res.RuleTriggerCounts["r-status-tag"] != 1 {
		t.Errorf("trigger count = %d, want 1", res.RuleTriggerCounts["r-status-tag"])
	}
	if len(res.Tasks) != 1 || res.Tasks[0].TaskID != "t1" {
		t.Errorf("result tasks = %+v, want single t1", res.Tasks)
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
}

func TestStateWithWorkflowContextualEvaluationFiresChangeRule(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.StateWithWorkflow("t1", models.StateDone, app.SingleTaskWorkflowOptions{
		Rules: []workflow.WorkflowRule{contextualTagRule()},
	}); err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if !t1.HasTag("shipped") {
		t.Errorf("contextual status-changed rule did not fire; tags = %v", t1.Tags)
	}
}

func TestStateWithWorkflowAlwaysFiresContextualChangeRule(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	// Every workflow-aware mutation captures prior state, so event rules behave
	// consistently regardless of the caller.
	if _, err := svc.StateWithWorkflow("t1", models.StateDone, app.SingleTaskWorkflowOptions{
		Rules: []workflow.WorkflowRule{contextualTagRule()},
	}); err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if !t1.HasTag("shipped") {
		t.Errorf("status-changed rule did not fire; tags = %v", t1.Tags)
	}
}

func TestStateWithWorkflowDeferredMoveCommitsOneTransaction(t *testing.T) {
	t1 := newTask("t1", 1, "movable")
	projA := newProject("proj-a", "Alpha", t1)
	projB := newProject("proj-b", "Beta")
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:      "r-move-done",
		Name:    "Archive done to Beta",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionMoveToProject, Value: "Beta"}},
	}}

	res, err := svc.StateWithWorkflow("t1", models.StateDone, app.SingleTaskWorkflowOptions{Rules: rules})
	if err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if projB.GetTaskByID("t1") == nil {
		t.Fatalf("task not moved to destination; projB tasks = %d", len(projB.Tasks))
	}
	if projA.GetTaskByID("t1") != nil {
		t.Errorf("task still present in source after deferred move")
	}
	if len(repo.commits) != 1 {
		t.Fatalf("Commit called %d times, want 1 (move rides the single-task transaction)", len(repo.commits))
	}
	if got := len(repo.commits[0]); got != 2 {
		t.Errorf("changesets = %d, want 2 (source + destination)", got)
	}
	if len(res.TouchedProjectIDs) != 2 {
		t.Errorf("TouchedProjectIDs = %v, want two projects", res.TouchedProjectIDs)
	}
}

func TestStateWithWorkflowFollowupAppended(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:      "r-followup",
		Name:    "Follow up",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionCreateFollowup, Value: "verify {task}"}},
	}}

	if _, err := svc.StateWithWorkflow("t1", models.StateDone, app.SingleTaskWorkflowOptions{Rules: rules}); err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if len(projA.Tasks) != 2 {
		t.Fatalf("project tasks = %d, want 2 (original + follow-up)", len(projA.Tasks))
	}
}

func TestStateWithWorkflowGuardRejectionCommitsNothing(t *testing.T) {
	parent := newTask("p", 1, "parent")
	child := newTask("c", 2, "child")
	pid := "p"
	child.ParentID = &pid
	projA := newProject("proj-a", "Alpha", parent, child)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	// Completing a parent with an incomplete child must be rejected before any
	// commit (the incomplete-children guard).
	_, err := svc.StateWithWorkflow("p", models.StateDone, app.SingleTaskWorkflowOptions{
		Rules: []workflow.WorkflowRule{stateBasedTagRule()},
	})
	if err == nil {
		t.Fatal("expected the completion to be rejected by the children guard")
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0 (rejected mutation must not persist)", len(repo.commits))
	}
	if parent.State() == models.StateDone {
		t.Error("parent should not have transitioned when the guard rejects it")
	}
}

func TestSetPriorityWithWorkflowFiresRuleAndCommits(t *testing.T) {
	t1 := newTask("t1", 1, "urgent")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:      "r-high-pri",
		Name:    "Tag high priority",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldPriority, Operation: workflow.OpEquals, Value: "1"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionAddTag, Value: "hot"}},
	}}

	if _, err := svc.SetPriorityWithWorkflow("t1", 1, app.SingleTaskWorkflowOptions{Rules: rules}); err != nil {
		t.Fatalf("SetPriorityWithWorkflow: %v", err)
	}
	if t1.Priority != 1 {
		t.Errorf("priority = %d, want 1", t1.Priority)
	}
	if !t1.HasTag("hot") {
		t.Errorf("priority rule did not fire on a non-state single-task mutation; tags = %v", t1.Tags)
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
}

// TestNonStateWorkflowCascadePreservesPlanningExclusivity locks the
// orchestration-level planning default: a priority mutation supplies no per-call
// builder, yet a rule-driven state change reconciles planning in the same durable
// transaction and one workspace-ledger undo restores both task and planning data.
func TestNonStateWorkflowCascadePreservesPlanningExclusivity(t *testing.T) {
	t.Run("priority rule enters Next and removes Today", func(t *testing.T) {
		dataDir := t.TempDir()
		project := realTestProject(t, dataDir, "cascade-into-next", "Today task")
		task := project.Tasks[0]
		task.Priority = models.PriorityDefault
		task.ApplyWorkflowState(models.StateReady)
		wantToday := config.DailyPlanItem{
			TaskID:      task.ID,
			ProjectID:   project.ID,
			PlannedDate: "2026-07-20",
		}

		manager := seedWorkflowPlanningDomain(t, dataDir, func(planning *config.PlanningState) {
			planning.SetDailyPlan([]config.DailyPlanItem{wantToday})
		})
		ledger, service := newWorkflowPlanningService(t, dataDir, project, manager)
		rule := priorityCascadeRule("enter-next", models.SectionNext)

		if _, err := service.SetPriorityWithWorkflow(task.ID, 1, app.SingleTaskWorkflowOptions{
			Rules:      []workflow.WorkflowRule{rule},
			UndoAction: "Prioritize into Next",
		}); err != nil {
			t.Fatalf("SetPriorityWithWorkflow: %v", err)
		}
		assertWorkflowPlanningTask(t, project, task.ID, 1, models.StateNext)
		if got := loadWorkflowPlanning(t, manager).GetDailyPlan(); len(got) != 0 {
			t.Fatalf("daily plan after cascade = %+v, want empty", got)
		}

		if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
			t.Fatalf("Undo: %v", err)
		}
		assertWorkflowPlanningTask(t, project, task.ID, models.PriorityDefault, models.StateReady)
		gotToday := loadWorkflowPlanning(t, manager).GetDailyPlan()
		if len(gotToday) != 1 || gotToday[0] != wantToday {
			t.Fatalf("daily plan after undo = %+v, want exact entry %+v", gotToday, wantToday)
		}
	})

	t.Run("priority rule leaves Next and removes stale order", func(t *testing.T) {
		dataDir := t.TempDir()
		project := realTestProject(t, dataDir, "cascade-out-of-next", "Next task")
		task := project.Tasks[0]
		task.Priority = models.PriorityDefault
		task.ApplyWorkflowState(models.StateNext)
		wantOrder := config.FocusQueueItem{TaskID: task.ID, ProjectID: project.ID}

		manager := seedWorkflowPlanningDomain(t, dataDir, func(planning *config.PlanningState) {
			planning.SetFocusQueue([]config.FocusQueueItem{wantOrder})
		})
		ledger, service := newWorkflowPlanningService(t, dataDir, project, manager)
		rule := priorityCascadeRule("leave-next", models.SectionWaiting)

		if _, err := service.SetPriorityWithWorkflow(task.ID, 1, app.SingleTaskWorkflowOptions{
			Rules:      []workflow.WorkflowRule{rule},
			UndoAction: "Prioritize out of Next",
		}); err != nil {
			t.Fatalf("SetPriorityWithWorkflow: %v", err)
		}
		assertWorkflowPlanningTask(t, project, task.ID, 1, models.StateWaiting)
		if got := loadWorkflowPlanning(t, manager).GetFocusQueue(); len(got) != 0 {
			t.Fatalf("Next order after cascade = %+v, want empty", got)
		}

		if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
			t.Fatalf("Undo: %v", err)
		}
		assertWorkflowPlanningTask(t, project, task.ID, models.PriorityDefault, models.StateNext)
		gotOrder := loadWorkflowPlanning(t, manager).GetFocusQueue()
		if len(gotOrder) != 1 || gotOrder[0] != wantOrder {
			t.Fatalf("Next order after undo = %+v, want exact entry %+v", gotOrder, wantOrder)
		}
	})
}

func priorityCascadeRule(id, section string) workflow.WorkflowRule {
	return workflow.WorkflowRule{
		ID:      id,
		Name:    id,
		Enabled: true,
		Trigger: workflow.RuleTrigger{
			Field:     workflow.FieldPriorityChanged,
			Operation: workflow.OpEquals,
			Value:     "3->1",
		},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionSetSection, Value: section}},
	}
}

func seedWorkflowPlanningDomain(t *testing.T, dataDir string, seed func(*config.PlanningState)) *config.Manager {
	t.Helper()
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	if err := manager.CommitCatalog(config.NewWorkspaceCatalog()); err != nil {
		t.Fatalf("seed workspace catalog: %v", err)
	}
	planning := config.NewPlanningState()
	seed(planning)
	if err := manager.CommitPlanning(planning); err != nil {
		t.Fatalf("seed planning: %v", err)
	}
	return manager
}

func newWorkflowPlanningService(
	t *testing.T,
	dataDir string,
	project *models.Project,
	manager *config.Manager,
) (*storage.WorkspaceLedger, *app.Service) {
	t.Helper()
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), testWorkspaceLedgerDepth, []string{dataDir})
	commitService := storage.NewCommitServiceWithLedger(config.TransactionsDir(dataDir), []string{dataDir}, ledger)
	repo := newStorageRepo(commitService, project)
	repo.seed(t)
	service := app.NewService(repo, nil, nil).WithPlanningArtifacts(durablePlanningExclusivityBuilder(manager))
	return ledger, service
}

func durablePlanningExclusivityBuilder(manager *config.Manager) app.PlanningArtifactBuilder {
	return func(targets []app.PlanningReconcileTarget) ([]storage.ArtifactChange, error) {
		coordinator := manager.DomainCoordinator()
		planning, err := coordinator.LoadPlanning()
		if err != nil {
			return nil, err
		}
		changed := false
		for _, target := range targets {
			if target.State == models.StateNext {
				changed = planning.RemoveFromDailyPlan(target.TaskID) || changed
				continue
			}
			changed = planning.RemoveFromFocusQueue(target.TaskID) || changed
		}
		if !changed {
			return nil, nil
		}
		artifact, err := coordinator.CheckedPlanningArtifact(planning)
		if err != nil {
			return nil, err
		}
		return []storage.ArtifactChange{artifact.StorageChange()}, nil
	}
}

func loadWorkflowPlanning(t *testing.T, manager *config.Manager) *config.PlanningState {
	t.Helper()
	planning, err := manager.DomainCoordinator().LoadPlanning()
	if err != nil {
		t.Fatalf("load planning: %v", err)
	}
	return planning
}

func assertWorkflowPlanningTask(
	t *testing.T,
	project *models.Project,
	taskID string,
	wantPriority int,
	wantState models.WorkflowState,
) {
	t.Helper()
	reloaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	task := reloaded.GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("reloaded task %q is missing", taskID)
	}
	if task.Priority != wantPriority || task.State() != wantState {
		t.Fatalf("reloaded task priority/state = %d/%s, want %d/%s", task.Priority, task.State(), wantPriority, wantState)
	}
}

func TestWithWorkflowNoRulesCommitsBareMutation(t *testing.T) {
	t1 := newTask("t1", 1, "plain")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	res, err := svc.SetSectionWithWorkflow("t1", models.SectionSomeday, app.SingleTaskWorkflowOptions{UndoAction: "Move to Section"})
	if err != nil {
		t.Fatalf("SetSectionWithWorkflow: %v", err)
	}
	if res.RuleTriggerCounts != nil {
		t.Errorf("RuleTriggerCounts = %v, want nil with no rules", res.RuleTriggerCounts)
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
	if got := repo.commits[0][0].UndoAction; got != "Move to Section" {
		t.Errorf("UndoAction = %q, want Move to Section", got)
	}
}

func TestSetSectionWithWorkflowDoesNotAutoCompleteEligibleParent(t *testing.T) {
	story := newTask("story", 1, "story")
	story.Type = models.TaskTypeStory
	child := newTask("child", 2, "child")
	child.ApplyWorkflowState(models.StateDone)
	child.ParentID = &story.ID
	project := newProject("proj-a", "Alpha", story, child)
	repo := newFakeRepo(project)
	rule := workflow.WorkflowRule{
		ID: "story-done", Name: "Auto-complete story", Enabled: true,
		Triggers: []workflow.RuleTrigger{
			{Field: workflow.FieldType, Operation: workflow.OpEquals, Value: "story"},
			{Field: workflow.FieldAllChildrenDone, Operation: workflow.OpEquals, Value: "true"},
		},
		TriggerLogic: workflow.LogicAnd,
		Actions:      []workflow.RuleAction{{ActionType: workflow.ActionSetStatus, Value: "done"}},
	}

	if _, err := newTestService(repo).SetSectionWithWorkflow(story.ID, models.SectionSomeday, app.SingleTaskWorkflowOptions{Rules: []workflow.WorkflowRule{rule}}); err != nil {
		t.Fatalf("SetSectionWithWorkflow: %v", err)
	}
	if story.State() != models.StateSomeday {
		t.Fatalf("story state = %s, want someday", story.State())
	}
}

func TestSingleTaskWorkflowAttributeWrappers(t *testing.T) {
	task := newTask("t1", 1, "attributes")
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)
	service := newTestService(repo)
	opts := app.SingleTaskWorkflowOptions{UndoContext: "attribute update"}

	due := time.Date(2026, 7, 20, 9, 30, 0, 0, time.UTC)
	if _, err := service.SetDueDateWithWorkflow(task.ID, &due, true, opts); err != nil {
		t.Fatalf("SetDueDateWithWorkflow: %v", err)
	}
	if task.DueDate == nil || !task.DueDate.Equal(due) || !task.HasTime {
		t.Fatalf("due date = %v hasTime=%v", task.DueDate, task.HasTime)
	}
	if _, err := service.SetDueDateWithWorkflow(task.ID, nil, true, opts); err != nil {
		t.Fatalf("clear due date: %v", err)
	}
	if task.DueDate != nil || task.HasTime {
		t.Fatalf("cleared due date = %v hasTime=%v", task.DueDate, task.HasTime)
	}

	until := due.Add(24 * time.Hour)
	if _, err := service.SnoozeWithWorkflow(task.ID, &until, opts); err != nil {
		t.Fatalf("SnoozeWithWorkflow: %v", err)
	}
	if task.SnoozedUntil == nil || !task.SnoozedUntil.Equal(until) {
		t.Fatalf("snoozed until = %v", task.SnoozedUntil)
	}
	if _, err := service.SnoozeWithWorkflow(task.ID, nil, opts); err != nil {
		t.Fatalf("clear snooze: %v", err)
	}
	if task.SnoozedUntil != nil {
		t.Fatalf("cleared snooze = %v", task.SnoozedUntil)
	}

	if _, err := service.SetPinnedWithWorkflow(task.ID, nil, opts); err != nil {
		t.Fatalf("toggle pinned: %v", err)
	}
	if !task.Pinned {
		t.Fatal("nil pinned should toggle the task on")
	}
	forceOff := false
	if _, err := service.SetPinnedWithWorkflow(task.ID, &forceOff, opts); err != nil {
		t.Fatalf("force unpinned: %v", err)
	}
	if task.Pinned {
		t.Fatal("forced unpin did not clear Pinned")
	}
	forceOn := true
	if _, err := service.SetPinnedWithWorkflow(task.ID, &forceOn, opts); err != nil {
		t.Fatalf("force pinned: %v", err)
	}
	if !task.Pinned {
		t.Fatal("forced pin did not set Pinned")
	}

	if _, err := service.ToggleDimmedWithWorkflow(task.ID, opts); err != nil {
		t.Fatalf("ToggleDimmedWithWorkflow: %v", err)
	}
	if !task.Dimmed {
		t.Fatal("toggle dimmed did not set Dimmed")
	}
	if got := len(repo.commits); got != 8 {
		t.Fatalf("commits = %d, want 8 one-mutation transactions", got)
	}
}

func TestSingleTaskWorkflowWrappersRejectInvalidInput(t *testing.T) {
	task := newTask("t1", 1, "invalid")
	repo := newFakeRepo(newProject("proj-a", "Alpha", task))
	service := newTestService(repo)

	if _, err := service.SetPriorityWithWorkflow(task.ID, 99, app.SingleTaskWorkflowOptions{}); err == nil {
		t.Fatal("expected invalid priority error")
	}
	if _, err := service.StateWithWorkflow(task.ID, models.WorkflowState("invalid"), app.SingleTaskWorkflowOptions{}); err == nil {
		t.Fatal("expected invalid state error")
	}
	if len(repo.commits) != 0 {
		t.Fatalf("invalid mutations committed %d times", len(repo.commits))
	}
}
