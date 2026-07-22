package app_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func strptr(value string) *string { return &value }
func intptr(value int) *int       { return &value }

func TestPreparedActionPlanSelectRisksUsesLocalClassificationAndOriginalIndexes(t *testing.T) {
	canceled := models.StateCanceled
	safeTask := newTask("safe", 1, "Safe")
	deleteTask := newTask("delete", 2, "Delete")
	cancelTask := newTask("cancel", 3, "Cancel")
	repo := newFakeRepo(newProject("p1", "Alpha", safeTask, deleteTask, cancelTask))
	service := newTestService(repo)
	plan := app.ActionPlan{
		Version: app.ActionPlanVersion,
		Summary: "mixed",
		Actions: []app.Action{
			{Type: app.ActionSetPriority, TaskID: "safe", Priority: intptr(1)},
			{Type: app.ActionDeleteTask, TaskID: "delete"},
			{Type: app.ActionSetState, TaskID: "cancel", State: &canceled},
		},
	}

	prepared, err := service.PrepareActionPlan(plan)
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	safe := prepared.SelectRisks(app.ActionRiskSafe).Preview()
	if safe.Summary != plan.Summary || len(safe.Actions) != 1 || safe.Actions[0].Index != 1 || safe.Actions[0].Risk != app.ActionRiskSafe {
		t.Fatalf("safe preview = %#v", safe)
	}
	destructive := prepared.SelectRisks(app.ActionRiskDestructive).Preview()
	if len(destructive.Actions) != 2 || destructive.Actions[0].Index != 2 || destructive.Actions[1].Index != 3 {
		t.Fatalf("destructive preview = %#v", destructive)
	}
	if _, err := service.ApplyPreparedActionPlan(context.Background(), prepared.SelectRisks(app.ActionRiskSafe), app.ApplyActionPlanOptions{ConfirmSafe: true}); err != nil {
		t.Fatalf("ApplyPreparedActionPlan safe selection: %v", err)
	}
	if got := repo.projects[0].GetTaskByID(safeTask.ID).Priority; got != 1 {
		t.Fatalf("safe priority = %d, want 1", got)
	}
	if repo.projects[0].GetTaskByID(deleteTask.ID) == nil || repo.projects[0].GetTaskByID(cancelTask.ID).State() == models.StateCanceled {
		t.Fatal("safe selection executed a declined destructive action")
	}
	if _, err := service.ApplyPreparedActionPlan(context.Background(), prepared.SelectRisks(app.ActionRiskDestructive), app.ApplyActionPlanOptions{ConfirmSafe: true, ConfirmDestructive: true}); !errors.Is(err, app.ErrPreparedActionPlanConsumed) {
		t.Fatalf("second risk selection apply error = %v, want consumed", err)
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

	prepared, err := service.PrepareActionPlan(plan)
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	preview := prepared.Preview()
	if preview.Risk != app.ActionRiskDestructive || preview.DestructiveCount != 1 {
		t.Fatalf("preview risk = %#v, want one destructive action", preview)
	}
	_, err = service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{ConfirmSafe: true})
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

func TestActionPlanDisplaySafetyRejectsUnsafeFieldsBeforePreview(t *testing.T) {
	task := newTask("task-1", 1, "Existing")
	project := newProject("p1", "Alpha", task)
	service := newTestService(newFakeRepo(project))

	unsafeTitle := "spoof\x1b[2J"
	unsafeDescription := "safe line\nsecond\u202e"
	for name, plan := range map[string]app.ActionPlan{
		"title": {
			Version: app.ActionPlanVersion,
			Actions: []app.Action{{Type: app.ActionCreateTask, ProjectID: project.ID, Title: &unsafeTitle}},
		},
		"multiline description": {
			Version: app.ActionPlanVersion,
			Actions: []app.Action{{Type: app.ActionEditTask, TaskID: task.ID, Description: &unsafeDescription}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.PrepareActionPlan(plan); err == nil || !strings.Contains(err.Error(), "unsafe terminal") {
				t.Fatalf("PrepareActionPlan error = %v, want display-safety rejection", err)
			}
		})
	}
}

func TestActionPlanDisplaySafetyPreservesMultilineMarkdownDescription(t *testing.T) {
	task := newTask("task-1", 1, "Existing")
	project := newProject("p1", "Alpha", task)
	service := newTestService(newFakeRepo(project))
	description := "## Notes\n\t- keep **Markdown**"

	prepared, err := service.PrepareActionPlan(app.ActionPlan{
		Version: app.ActionPlanVersion,
		Actions: []app.Action{{Type: app.ActionEditTask, TaskID: task.ID, Description: &description}},
	})
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	preview := prepared.Preview()
	if len(preview.Actions) != 1 || !strings.Contains(preview.Actions[0].Description, `## Notes\n\t- keep **Markdown**`) {
		t.Fatalf("preview = %#v, want preserved visible multiline description", preview)
	}
}

func TestPreparedActionPlanDefensivelyRetainsInputSnapshotAndPreview(t *testing.T) {
	task := newTask("task-1", 1, "Original")
	project := newProject("p1", "Alpha", task)
	repo := newFakeRepo(project)
	service := newTestService(repo)
	title := "Prepared title"
	plan := app.ActionPlan{
		Version: app.ActionPlanVersion,
		Summary: "Prepared summary",
		Actions: []app.Action{{Type: app.ActionEditTask, TaskID: task.ID, Title: &title}},
	}
	prepared, err := service.PrepareActionPlan(plan)
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	first := prepared.Preview()
	first.Summary = "caller mutation"
	first.Actions[0].Description = "caller mutation"
	if first.Summary != "caller mutation" {
		t.Fatalf("preview mutation did not update caller copy: %#v", first)
	}
	title = "pointer mutation"
	plan.Summary = "plan mutation"
	plan.Actions[0].Title = strptr("replacement action")
	project.Tasks[0].Title = "snapshot mutation"

	second := prepared.Preview()
	if second.Summary != "Prepared summary" || len(second.Actions) != 1 || !strings.Contains(second.Actions[0].Description, "Prepared title") {
		t.Fatalf("defensive preview = %#v", second)
	}
	result, err := service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{ConfirmSafe: true})
	if err != nil {
		t.Fatalf("ApplyPreparedActionPlan: %v", err)
	}
	if repo.snapshots != 1 {
		t.Fatalf("repository snapshots = %d, want exactly one prepare snapshot", repo.snapshots)
	}
	if got := repo.projects[0].GetTaskByID(task.ID).Title; got != "Prepared title" {
		t.Fatalf("committed title = %q, want retained prepared input", got)
	}
	if len(result.Actions) != 1 || !strings.Contains(result.Actions[0].Description, "Prepared title") {
		t.Fatalf("result actions = %#v, want retained preview", result.Actions)
	}
	if _, err := service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{ConfirmSafe: true}); !errors.Is(err, app.ErrPreparedActionPlanConsumed) {
		t.Fatalf("second apply error = %v, want consumed", err)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("single-use plan recorded %d commits, want 1", len(repo.commits))
	}
}

func TestPreparedActionPlanGuardsRelevantButNotUnrelatedProjects(t *testing.T) {
	for _, tc := range []struct {
		name      string
		driftID   string
		wantStale bool
	}{
		{name: "target project", driftID: "p1", wantStale: true},
		{name: "blocker project", driftID: "p2", wantStale: true},
		{name: "unrelated project", driftID: "p3", wantStale: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := newTask("task", 1, "Task")
			blocker := newTask("blocker", 1, "Blocker")
			other := newTask("other", 1, "Other")
			repo := newFakeRepo(
				newProject("p1", "Alpha", task),
				newProject("p2", "Beta", blocker),
				newProject("p3", "Gamma", other),
			)
			service := newTestService(repo)
			plan := app.ActionPlan{Version: app.ActionPlanVersion, Actions: []app.Action{{
				Type: app.ActionAddDependency, TaskID: task.ID, BlockerTaskID: blocker.ID,
			}}}
			prepared, err := service.PrepareActionPlan(plan)
			if err != nil {
				t.Fatalf("PrepareActionPlan: %v", err)
			}
			repo.revisions[tc.driftID] = "concurrent-revision"
			_, err = service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{ConfirmSafe: true})
			if tc.wantStale {
				if !errors.Is(err, storage.ErrStaleRevision) || len(repo.commits) != 0 {
					t.Fatalf("ApplyPreparedActionPlan err=%v commits=%d, want stale and no commit", err, len(repo.commits))
				}
				return
			}
			if err != nil {
				t.Fatalf("ApplyPreparedActionPlan with unrelated drift: %v", err)
			}
			if len(repo.commits) != 1 {
				t.Fatalf("commits = %d, want 1", len(repo.commits))
			}
			guards := repo.commits[0][0].ProjectPreconditions
			if len(guards) != 2 || guards[0].ProjectID != "p1" || guards[1].ProjectID != "p2" {
				t.Fatalf("project preconditions = %#v, want target and blocker only", guards)
			}
		})
	}
}

func TestPreparedActionPlanGuardsTransitiveDependencyReadProjects(t *testing.T) {
	task := newTask("task", 1, "Task")
	blocker := newTask("blocker", 1, "Blocker")
	transitive := newTask("transitive", 1, "Transitive blocker")
	blocker.DependsOn = []string{"p3:" + transitive.ID}
	repo := newFakeRepo(
		newProject("p1", "Alpha", task),
		newProject("p2", "Beta", blocker),
		newProject("p3", "Gamma", transitive),
	)
	service := newTestService(repo)
	prepared, err := service.PrepareActionPlan(app.ActionPlan{Version: app.ActionPlanVersion, Actions: []app.Action{{
		Type: app.ActionAddDependency, TaskID: task.ID, BlockerTaskID: blocker.ID,
	}}})
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	repo.revisions["p3"] = "concurrent-transitive-change"
	_, err = service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{ConfirmSafe: true})
	if !errors.Is(err, storage.ErrStaleRevision) || len(repo.commits) != 0 {
		t.Fatalf("ApplyPreparedActionPlan err=%v commits=%d, want transitive stale guard", err, len(repo.commits))
	}
}

func TestPreparedDeleteBindsExactSubtreeAndGuardsEveryLoadedProject(t *testing.T) {
	root := newTask("root", 1, "Root")
	root.Type = models.TaskTypeEpic
	child := newTask("child", 2, "Child")
	child.ParentID = strptr(root.ID)
	alpha := newProject("p1", "Alpha", root, child)
	beta := newProject("p2", "Beta", newTask("beta", 1, "Beta"))
	repo := newFakeRepo(alpha, beta)
	service := newTestService(repo)
	prepared, err := service.PrepareActionPlan(app.ActionPlan{
		Version: app.ActionPlanVersion,
		Actions: []app.Action{{Type: app.ActionDeleteTask, TaskID: root.ID}},
	})
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	preview := prepared.Preview()
	if len(preview.Actions) != 1 || !strings.Contains(preview.Actions[0].Description, "1 descendant") {
		t.Fatalf("delete preview = %#v, want exact descendant count", preview)
	}

	// A child/reference/edit/rename/removal changes a loaded project's canonical
	// revision. Even drift in an otherwise untouched project must expire a delete,
	// because deletion scans every project for references to the retained subtree.
	repo.revisions[beta.ID] = "concurrent-reference-change"
	_, err = service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{
		ConfirmSafe:        true,
		ConfirmDestructive: true,
		DeletedTaskArtifacts: func([]string) ([]storage.ArtifactChange, error) {
			return nil, nil
		},
	})
	if !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("ApplyPreparedActionPlan error = %v, want stale revision", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("stale delete commits=%d, want no effects", len(repo.commits))
	}
}

func TestPreparedActionPlanMultiProjectPreconditionFailureIsAtomic(t *testing.T) {
	alphaTask := newTask("alpha-task", 1, "Alpha")
	betaTask := newTask("beta-task", 1, "Beta")
	repo := newFakeRepo(newProject("p1", "Alpha", alphaTask), newProject("p2", "Beta", betaTask))
	service := newTestService(repo)
	plan := app.ActionPlan{Version: app.ActionPlanVersion, Actions: []app.Action{
		{Type: app.ActionEditTask, TaskID: alphaTask.ID, Title: strptr("Alpha changed")},
		{Type: app.ActionEditTask, TaskID: betaTask.ID, Title: strptr("Beta changed")},
	}}
	prepared, err := service.PrepareActionPlan(plan)
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	repo.preCommit = func() { repo.revisions["p2"] = "concurrent-beta" }
	_, err = service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{ConfirmSafe: true})
	if !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("ApplyPreparedActionPlan error = %v, want stale revision", err)
	}
	if len(repo.commits) != 0 || alphaTask.Title != "Alpha" || betaTask.Title != "Beta" {
		t.Fatalf("failed multi-project plan was not atomic: commits=%d alpha=%q beta=%q", len(repo.commits), alphaTask.Title, betaTask.Title)
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

	prepared, err := service.PrepareActionPlan(plan)
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	result, applyErr := service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{ConfirmSafe: true})
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

			prepared, err := service.PrepareActionPlan(plan)
			if err != nil {
				t.Fatalf("PrepareActionPlan: %v", err)
			}
			if _, err := service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{ConfirmSafe: true}); err != nil {
				t.Fatalf("ApplyPreparedActionPlan: %v", err)
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
			prepared, prepareErr := service.PrepareActionPlan(plan)
			if prepareErr != nil {
				t.Fatalf("PrepareActionPlan: %v", prepareErr)
			}
			ctx := tc.setup(context.Background(), repo)

			_, err := service.ApplyPreparedActionPlan(ctx, prepared, app.ApplyActionPlanOptions{ConfirmSafe: true})
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
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	project := realTestProject(t, dataDir, "alpha", "Before")
	taskID := project.Tasks[0].ID
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)
	service := app.NewService(repo, nil, nil)
	plan := app.ActionPlan{Version: 1, Summary: "AI edit", Actions: []app.Action{{Type: app.ActionEditTask, TaskID: taskID, Title: strptr("After")}}}

	prepared, err := service.PrepareActionPlan(plan)
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	result, applyErr := service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{ConfirmSafe: true})
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
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
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
	if catalogErr := runtime.CommitCatalog(runtime.Catalog()); catalogErr != nil {
		t.Fatalf("seed catalog: %v", catalogErr)
	}
	planning := config.NewPlanningState()
	for _, id := range []string{root.ID, child.ID} {
		planning.FocusQueue = append(planning.FocusQueue, config.FocusQueueItem{TaskID: id, ProjectID: project.ID})
		planning.DailyPlan = append(planning.DailyPlan, config.DailyPlanItem{TaskID: id, ProjectID: project.ID})
		planning.FocusDismissedIDs = append(planning.FocusDismissedIDs, id)
	}
	if planningErr := runtime.CommitPlanning(planning); planningErr != nil {
		t.Fatalf("seed planning: %v", planningErr)
	}

	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)
	service := app.NewService(repo, nil, nil)
	plan := app.ActionPlan{
		Version: app.ActionPlanVersion,
		Summary: "AI delete",
		Actions: []app.Action{{Type: app.ActionDeleteTask, TaskID: root.ID}},
	}
	var artifactLocators []app.TaskLocator
	prepared, err := service.PrepareActionPlan(plan)
	if err != nil {
		t.Fatalf("PrepareActionPlan: %v", err)
	}
	result, applyErr := service.ApplyPreparedActionPlan(context.Background(), prepared, app.ApplyActionPlanOptions{
		ConfirmSafe:        true,
		ConfirmDestructive: true,
		DeletedTaskLocatorArtifacts: func(locators []app.TaskLocator) ([]storage.ArtifactChange, error) {
			artifactLocators = append([]app.TaskLocator(nil), locators...)
			domains, cloneErr := runtime.CloneDomains()
			if cloneErr != nil {
				return nil, cloneErr
			}
			qualified := make([]config.ProjectTaskLocator, 0, len(locators))
			for _, locator := range locators {
				qualified = append(qualified, config.ProjectTaskLocator{ProjectID: locator.ProjectID, TaskID: locator.TaskID})
			}
			if removed := domains.Planning.RemoveDurableTaskLocators(qualified); removed != 4 {
				return nil, fmt.Errorf("removed planning refs = %d, want 4", removed)
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
		t.Fatalf("ApplyPreparedActionPlan delete: %v", applyErr)
	}
	if len(artifactLocators) != 2 || len(result.DeletedTaskLocators) != 2 {
		t.Fatalf("deleted task locators = callback %v result %v, want root and child", artifactLocators, result.DeletedTaskLocators)
	}
	if len(result.DeletedTaskIDs) != 2 || !containsValue(result.DeletedTaskIDs, root.ID) || !containsValue(result.DeletedTaskIDs, child.ID) {
		t.Fatalf("deleted task IDs = %v, want root and child", result.DeletedTaskIDs)
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
	if got := runtime.Planning(); len(got.FocusQueue) != 0 || len(got.DailyPlan) != 0 || len(got.FocusDismissedIDs) != 2 {
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
