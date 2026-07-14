package app_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func strptr(value string) *string { return &value }
func intptr(value int) *int       { return &value }

func TestFilterActionPlanUsesLocalRiskClassification(t *testing.T) {
	canceled := models.StateCanceled
	plan := app.ActionPlan{
		Version: app.ActionPlanVersion,
		Summary: "mixed",
		Actions: []app.Action{
			{Type: app.ActionSetPriority, TaskID: "safe", Priority: intptr(1)},
			{Type: app.ActionDeleteTask, TaskID: "delete"},
			{Type: app.ActionSetState, TaskID: "cancel", State: &canceled},
		},
	}

	safe := app.FilterActionPlan(plan, app.ActionRiskSafe)
	if safe.Version != plan.Version || safe.Summary != plan.Summary || len(safe.Actions) != 1 || safe.Actions[0].TaskID != "safe" {
		t.Fatalf("safe plan = %#v", safe)
	}
	destructive := app.FilterActionPlan(plan, app.ActionRiskDestructive)
	if len(destructive.Actions) != 2 || destructive.Actions[0].TaskID != "delete" || destructive.Actions[1].TaskID != "cancel" {
		t.Fatalf("destructive plan = %#v", destructive)
	}
}

func TestActionPlanDestructiveRiskCannotBeDowngraded(t *testing.T) {
	task := newTask("task-1", 1, "Delete me")
	project := newProject("p1", "Alpha", task)
	repo := newFakeRepo(project)
	service := newTestService(repo)
	plan := app.ActionPlan{
		Version: app.ActionPlanVersion,
		Summary: "Delete the task",
		Actions: []app.Action{{Type: app.ActionDeleteTask, TaskID: task.ID}},
	}

	preview, err := service.PreviewActionPlan(plan)
	if err != nil {
		t.Fatalf("PreviewActionPlan: %v", err)
	}
	if preview.Risk != app.ActionRiskDestructive || preview.DestructiveCount != 1 {
		t.Fatalf("preview risk = %#v, want one destructive action", preview)
	}
	_, err = service.ApplyActionPlan(context.Background(), plan, app.ApplyActionPlanOptions{ConfirmSafe: true})
	if !errors.Is(err, app.ErrActionPlanDestructiveConfirmationRequired) {
		t.Fatalf("ApplyActionPlan error = %v, want destructive confirmation", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("unconfirmed destructive plan committed %d times", len(repo.commits))
	}
	if project.GetTaskByID(task.ID) == nil {
		t.Fatal("unconfirmed destructive plan mutated the source snapshot")
	}
}

func TestActionPlanCommitsMultipleProjectsAsOneTransaction(t *testing.T) {
	t1 := newTask("task-1", 1, "First")
	t2 := newTask("task-2", 1, "Second")
	p1 := newProject("p1", "Alpha", t1)
	p2 := newProject("p2", "Beta", t2)
	repo := newFakeRepo(p1, p2)
	service := newTestService(repo)
	state := models.StateInProgress
	plan := app.ActionPlan{
		Version: app.ActionPlanVersion,
		Summary: "Update both tasks",
		Actions: []app.Action{
			{Type: app.ActionSetPriority, TaskID: t1.ID, Priority: intptr(1)},
			{Type: app.ActionSetState, TaskID: t2.ID, State: &state},
		},
	}

	result, applyErr := service.ApplyActionPlan(context.Background(), plan, app.ApplyActionPlanOptions{ConfirmSafe: true})
	if applyErr != nil {
		t.Fatalf("ApplyActionPlan: %v", applyErr)
	}
	if result.TransactionID != "txn-test" || result.UndoID != result.TransactionID {
		t.Fatalf("transaction/undo IDs = %q/%q", result.TransactionID, result.UndoID)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("commit shape = %d calls / %d changesets, want 1 / 2", len(repo.commits), len(repo.commits[0]))
	}
	if got := repo.commits[0][0].Project.GetTaskByID(t1.ID).Priority; got != 1 {
		t.Fatalf("priority in committed snapshot = %d, want 1", got)
	}
	if got := repo.commits[0][1].Project.GetTaskByID(t2.ID).State(); got != models.StateInProgress {
		t.Fatalf("state in committed snapshot = %s, want in_progress", got)
	}
}

func TestActionPlanDependencyIdentityMatchesLegacySlugAndStableID(t *testing.T) {
	for _, tc := range []struct {
		name       string
		actionType app.ActionType
		wantRefs   []string
	}{
		{name: "add deduplicates", actionType: app.ActionAddDependency, wantRefs: []string{"beta:blocker"}},
		{name: "remove matches alias", actionType: app.ActionRemoveDependency, wantRefs: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := newTask("task", 1, "Task")
			blocker := newTask("blocker", 1, "Blocker")
			task.DependsOn = []string{"beta:blocker"}
			project := newProject("p1", "Alpha", task)
			blockerProject := newProject("p2", "Beta", blocker)
			repo := newFakeRepo(project, blockerProject)
			service := newTestService(repo)
			plan := app.ActionPlan{
				Version: app.ActionPlanVersion,
				Summary: "Dependency identity",
				Actions: []app.Action{{
					Type:          tc.actionType,
					TaskID:        task.ID,
					BlockerTaskID: blocker.ID,
				}},
			}

			if _, err := service.ApplyActionPlan(context.Background(), plan, app.ApplyActionPlanOptions{ConfirmSafe: true}); err != nil {
				t.Fatalf("ApplyActionPlan: %v", err)
			}
			committed := repo.commits[0][0].Project.GetTaskByID(task.ID)
			if len(committed.DependsOn) != len(tc.wantRefs) {
				t.Fatalf("DependsOn = %v, want %v", committed.DependsOn, tc.wantRefs)
			}
			for i := range tc.wantRefs {
				if committed.DependsOn[i] != tc.wantRefs[i] {
					t.Fatalf("DependsOn = %v, want %v", committed.DependsOn, tc.wantRefs)
				}
			}
		})
	}
}

func TestActionPlanCancellationAndRevisionConflictCommitNothing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		setup  func(context.Context, *fakeRepo) context.Context
		isWant error
	}{
		{
			name: "canceled",
			setup: func(_ context.Context, _ *fakeRepo) context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			isWant: context.Canceled,
		},
		{
			name: "revision conflict",
			setup: func(ctx context.Context, repo *fakeRepo) context.Context {
				repo.preCommit = func() { repo.revisions["p1"] = "concurrent-revision" }
				return ctx
			},
			isWant: storage.ErrStaleRevision,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := newTask("task-1", 1, "Keep original")
			project := newProject("p1", "Alpha", task)
			repo := newFakeRepo(project)
			service := newTestService(repo)
			plan := app.ActionPlan{Version: 1, Actions: []app.Action{{Type: app.ActionEditTask, TaskID: task.ID, Title: strptr("Changed")}}}
			ctx := tc.setup(context.Background(), repo)

			_, err := service.ApplyActionPlan(ctx, plan, app.ApplyActionPlanOptions{ConfirmSafe: true})
			if !errors.Is(err, tc.isWant) {
				t.Fatalf("ApplyActionPlan error = %v, want %v", err, tc.isWant)
			}
			if len(repo.commits) != 0 {
				t.Fatalf("failed plan recorded %d commits", len(repo.commits))
			}
			if task.Title != "Keep original" {
				t.Fatalf("failed plan mutated source task title to %q", task.Title)
			}
		})
	}
}

func TestActionPlanRollbackThroughWorkspaceUndo(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	project := realTestProject(t, dataDir, "alpha", "Before")
	taskID := project.Tasks[0].ID
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)
	service := app.NewService(repo, nil, nil)
	plan := app.ActionPlan{Version: 1, Summary: "AI edit", Actions: []app.Action{{Type: app.ActionEditTask, TaskID: taskID, Title: strptr("After")}}}

	result, applyErr := service.ApplyActionPlan(context.Background(), plan, app.ApplyActionPlanOptions{ConfirmSafe: true})
	if applyErr != nil {
		t.Fatalf("ApplyActionPlan: %v", applyErr)
	}
	if result.TransactionID == "" || result.UndoID != result.TransactionID {
		t.Fatalf("missing transaction/undo identity: %#v", result)
	}
	reloaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("reload after apply: %v", err)
	}
	if got := reloaded.GetTaskByID(taskID).Title; got != "After" {
		t.Fatalf("title after apply = %q", got)
	}

	entry, ok, selectErr := ledger.SelectNewestUndo(storage.GlobalScope())
	if selectErr != nil || !ok {
		t.Fatalf("SelectNewestUndo: ok=%v err=%v", ok, selectErr)
	}
	if entry.TransactionID != result.UndoID {
		t.Fatalf("newest undo transaction %q, want %q", entry.TransactionID, result.UndoID)
	}
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	reloaded = &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("reload after undo: %v", err)
	}
	if got := reloaded.GetTaskByID(taskID).Title; got != "Before" {
		t.Fatalf("title after undo = %q, want Before", got)
	}
}

func TestActionPlanDeleteCleansDurableSubtreeRefsAndUndoRestoresThem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "Delete root")
	root := project.Tasks[0]
	root.Type = models.TaskTypeEpic
	child, err := models.NewTask("ai-delete-child", 2, models.TaskTypeTask, "Delete child")
	if err != nil {
		t.Fatalf("NewTask child: %v", err)
	}
	child.ParentID = strptr(root.ID)
	project.Tasks = append(project.Tasks, child)
	project.NextShortID = 3
	project.BuildTaskTree()

	runtime := config.EmptyRuntimeState(dataDir)
	planning := config.NewPlanningState()
	for _, id := range []string{root.ID, child.ID} {
		planning.FocusQueue = append(planning.FocusQueue, config.FocusQueueItem{TaskID: id, ProjectID: project.ID})
		planning.DailyPlan = append(planning.DailyPlan, config.DailyPlanItem{TaskID: id, ProjectID: project.ID})
		planning.FocusDismissedIDs = append(planning.FocusDismissedIDs, id)
	}
	if err := runtime.CommitPlanning(planning); err != nil {
		t.Fatalf("seed planning: %v", err)
	}

	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)
	service := app.NewService(repo, nil, nil)
	plan := app.ActionPlan{
		Version: app.ActionPlanVersion,
		Summary: "AI delete",
		Actions: []app.Action{{Type: app.ActionDeleteTask, TaskID: root.ID}},
	}
	var artifactIDs []string
	result, applyErr := service.ApplyActionPlan(context.Background(), plan, app.ApplyActionPlanOptions{
		ConfirmSafe:        true,
		ConfirmDestructive: true,
		DeletedTaskArtifacts: func(ids []string) ([]storage.ArtifactChange, error) {
			artifactIDs = append([]string(nil), ids...)
			domains, cloneErr := runtime.CloneDomains()
			if cloneErr != nil {
				return nil, cloneErr
			}
			deleted := make(map[string]bool, len(ids))
			for _, id := range ids {
				deleted[id] = true
			}
			if removed := domains.Planning.RemoveDurableTaskRefs(deleted); removed != 6 {
				return nil, fmt.Errorf("removed planning refs = %d, want 6", removed)
			}
			artifact, artifactErr := runtime.CheckedPlanningArtifact(domains.Planning)
			if artifactErr != nil {
				return nil, artifactErr
			}
			return []storage.ArtifactChange{{
				Path: artifact.Path, Data: artifact.Data, ExpectedRevision: artifact.ExpectedRevision,
			}}, nil
		},
	})
	if applyErr != nil {
		t.Fatalf("ApplyActionPlan delete: %v", applyErr)
	}
	for _, ids := range [][]string{artifactIDs, result.DeletedTaskIDs} {
		if len(ids) != 2 || !containsValue(ids, root.ID) || !containsValue(ids, child.ID) {
			t.Fatalf("deleted task IDs = %v, want root and child", ids)
		}
	}

	reloaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("reload after AI delete: %v", err)
	}
	if len(reloaded.Tasks) != 0 {
		t.Fatalf("tasks after AI delete = %d, want 0", len(reloaded.Tasks))
	}
	if err := runtime.RefreshAfterCommit(); err != nil {
		t.Fatalf("refresh planning after AI delete: %v", err)
	}
	if got := runtime.Planning(); len(got.FocusQueue) != 0 || len(got.DailyPlan) != 0 || len(got.FocusDismissedIDs) != 0 {
		t.Fatalf("planning refs survived AI delete: focus=%v daily=%v dismissed=%v", got.FocusQueue, got.DailyPlan, got.FocusDismissedIDs)
	}

	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo AI delete: %v", err)
	}
	reloaded = &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("reload after AI delete undo: %v", err)
	}
	if reloaded.GetTaskByID(root.ID) == nil || reloaded.GetTaskByID(child.ID) == nil {
		t.Fatalf("undo did not restore deleted subtree: %#v", reloaded.Tasks)
	}
	if err := runtime.Reload(); err != nil {
		t.Fatalf("reload planning after AI delete undo: %v", err)
	}
	if got := runtime.Planning(); len(got.FocusQueue) != 2 || len(got.DailyPlan) != 2 || len(got.FocusDismissedIDs) != 2 {
		t.Fatalf("undo did not restore planning refs: focus=%v daily=%v dismissed=%v", got.FocusQueue, got.DailyPlan, got.FocusDismissedIDs)
	}
}

func containsValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
