package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"unicode"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestPrintActionPlanPreviewDisplaySafety(t *testing.T) {
	var out bytes.Buffer
	printActionPlanPreview(&out, app.PlanPreview{
		Summary: "clear\x1b[2J\u009bspoof\r\u202e\n\ttail",
		Actions: []app.ActionPreview{{
			Index:       1,
			Risk:        app.ActionRiskSafe,
			Description: "title\x1b]0;spoof\x07\u009dhidden\u009c\u2066\n\tmarkdown",
		}},
	})

	got := out.String()
	for _, r := range got {
		if (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.Is(unicode.Cf, r) {
			t.Fatalf("preview contains raw terminal control U+%04X: %q", r, got)
		}
	}
	for _, want := range []string{`\x1b[2J`, `\u009bspoof`, `\r`, `\u202e`, `\n\ttail`, `\x1b]0;spoof\a`, `\u009dhidden\u009c`, `\u2066`} {
		if !strings.Contains(got, want) {
			t.Fatalf("preview = %q, want visible escape %q", got, want)
		}
	}
	if !strings.Contains(got, "\\u2066\n\tmarkdown") {
		t.Fatalf("preview = %q, want multiline description layout preserved", got)
	}
}

func TestApplyCLIActionPlanCleansDurableAndMachineDeleteRefs(t *testing.T) {
	deps, project := setupStateTest(t)
	project.BuildTaskTree()

	var root *models.Task
	for _, task := range project.Tasks {
		if len(task.Children) > 0 {
			root = task
			break
		}
	}
	if root == nil {
		t.Fatal("test project has no task subtree")
	}
	deleted := collectTaskDescendantIDs(project.Tasks, map[string]bool{root.ID: true})
	var survivor *models.Task
	for _, task := range project.Tasks {
		if !deleted[task.ID] {
			survivor = task
			break
		}
	}
	if survivor == nil {
		t.Fatal("test project has no task outside deleted subtree")
	}
	for id := range deleted {
		deps.Config.Planning().FocusQueue = append(deps.Config.Planning().FocusQueue, config.FocusQueueItem{TaskID: id, ProjectID: project.ID})
		deps.Config.Planning().DailyPlan = append(deps.Config.Planning().DailyPlan, config.DailyPlanItem{TaskID: id, ProjectID: project.ID})
		deps.Config.Planning().FocusDismissedIDs = append(deps.Config.Planning().FocusDismissedIDs, id)
		deps.Config.Machine().RecentTaskVisits = append(deps.Config.Machine().RecentTaskVisits, config.RecentTaskVisit{TaskID: id, ProjectID: project.ID})
	}
	deps.Config.Planning().FocusQueue = append(deps.Config.Planning().FocusQueue, config.FocusQueueItem{TaskID: survivor.ID, ProjectID: project.ID})
	deps.Config.Planning().DailyPlan = append(deps.Config.Planning().DailyPlan, config.DailyPlanItem{TaskID: survivor.ID, ProjectID: project.ID})
	deps.Config.Planning().FocusDismissedIDs = append(deps.Config.Planning().FocusDismissedIDs, survivor.ID)
	deps.Config.Machine().RecentTaskVisits = append(deps.Config.Machine().RecentTaskVisits, config.RecentTaskVisit{TaskID: survivor.ID, ProjectID: project.ID})
	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

	workspace, err := LoadCLIWorkspace()
	testutil.AssertNoError(t, err)
	service := newWorkspaceAppService(workspace)
	prepared, err := service.PrepareActionPlan(app.ActionPlan{
		Version: app.ActionPlanVersion,
		Summary: "Delete subtree",
		Actions: []app.Action{{Type: app.ActionDeleteTask, TaskID: root.ID}},
	})
	testutil.AssertNoError(t, err)
	result, warning, applyErr := applyCLIActionPlan(
		context.Background(),
		workspace.Runtime,
		service,
		prepared,
		app.ApplyActionPlanOptions{ConfirmSafe: true, ConfirmDestructive: true},
	)
	testutil.AssertNoError(t, applyErr)
	if warning != nil {
		t.Fatalf("post-commit reference warning: %v", warning)
	}
	if len(result.DeletedTaskIDs) != len(deleted) {
		t.Fatalf("deleted IDs = %v, want %v", result.DeletedTaskIDs, deleted)
	}
	if len(result.DeletedTaskLocators) != len(deleted) {
		t.Fatalf("deleted locators = %v, want %d qualified tasks", result.DeletedTaskLocators, len(deleted))
	}
	for _, id := range result.DeletedTaskIDs {
		if !deleted[id] {
			t.Fatalf("unexpected deleted task ID %q", id)
		}
	}

	reloaded, loadErr := manager.LoadRuntimeState()
	testutil.AssertNoError(t, loadErr)
	if got := reloaded.Planning(); len(got.FocusQueue) != 1 || got.FocusQueue[0].TaskID != survivor.ID ||
		len(got.DailyPlan) != 1 || got.DailyPlan[0].TaskID != survivor.ID ||
		len(got.FocusDismissedIDs) != len(deleted)+1 {
		t.Fatalf("CLI AI delete cleaned the wrong planning refs: focus=%v daily=%v dismissed=%v", got.FocusQueue, got.DailyPlan, got.FocusDismissedIDs)
	}
	if got := reloaded.Machine().RecentTaskVisits; len(got) != 1 || got[0].TaskID != survivor.ID {
		t.Fatalf("CLI AI delete cleaned the wrong machine visits: %v", got)
	}
	stored := deps.Repository.Projects[project.ID]
	for id := range deleted {
		if stored.GetTaskByID(id) != nil {
			t.Fatalf("deleted subtree task %q survived in project", id)
		}
	}
	if stored.GetTaskByID(survivor.ID) == nil {
		t.Fatalf("task %q outside reviewed deletion subtree was removed", survivor.ID)
	}
}

func TestSelectCLIActionPlanPreservesPreparedIndexes(t *testing.T) {
	_, project := setupStateTest(t)
	project.BuildTaskTree()
	var root *models.Task
	for _, task := range project.Tasks {
		if len(task.Children) > 0 {
			root = task
			break
		}
	}
	if root == nil {
		t.Fatal("test project has no task subtree")
	}
	workspace, err := LoadCLIWorkspace()
	testutil.AssertNoError(t, err)
	service := newWorkspaceAppService(workspace)
	title := "Create after destructive action"
	prepared, err := service.PrepareActionPlan(app.ActionPlan{
		Version: app.ActionPlanVersion,
		Actions: []app.Action{
			{Type: app.ActionDeleteTask, TaskID: root.ID},
			{Type: app.ActionCreateTask, ProjectID: project.ID, Title: &title},
		},
	})
	testutil.AssertNoError(t, err)

	for _, tc := range []struct {
		name        string
		safe        bool
		destructive bool
		wantIndexes []int
	}{
		{name: "safe-only", safe: true, wantIndexes: []int{2}},
		{name: "destructive-only", destructive: true, wantIndexes: []int{1}},
		{name: "mixed", safe: true, destructive: true, wantIndexes: []int{1, 2}},
		{name: "declined", wantIndexes: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected := selectCLIActionPlan(prepared, tc.safe, tc.destructive).Preview()
			if len(selected.Actions) != len(tc.wantIndexes) {
				t.Fatalf("selected actions = %#v, want indexes %v", selected.Actions, tc.wantIndexes)
			}
			for i, want := range tc.wantIndexes {
				if selected.Actions[i].Index != want {
					t.Fatalf("selected action %d index = %d, want original index %d", i, selected.Actions[i].Index, want)
				}
			}
		})
	}
}

func TestApplyCLIActionPlanExpiresAfterConfirmationTimeDrift(t *testing.T) {
	project, err := models.NewProject("cli-drift-project", "CLI Drift", "")
	testutil.AssertNoError(t, err)
	project.Path = t.TempDir()
	task, err := models.NewTask("cli-drift-task", 1, models.TaskTypeTask, "Before")
	testutil.AssertNoError(t, err)
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.BuildTaskTree()

	repo := newCLIActionPlanTestRepo(project)
	service := app.NewService(repo, nil, nil)
	after := "After"
	prepared, err := service.PrepareActionPlan(app.ActionPlan{
		Version: app.ActionPlanVersion,
		Actions: []app.Action{{Type: app.ActionEditTask, TaskID: task.ID, Title: &after}},
	})
	testutil.AssertNoError(t, err)

	repo.revisions[project.ID] = "changed-while-confirming"
	_, _, applyErr := applyCLIActionPlan(
		context.Background(),
		config.EmptyRuntimeState(""),
		service,
		prepared,
		app.ApplyActionPlanOptions{ConfirmSafe: true},
	)
	if !errors.Is(applyErr, storage.ErrStaleRevision) {
		t.Fatalf("apply error = %v, want ErrStaleRevision", applyErr)
	}
	if repo.commits != 0 {
		t.Fatalf("stale prepared plan committed %d time(s)", repo.commits)
	}
}

type cliActionPlanTestRepo struct {
	projects  []*models.Project
	revisions map[string]string
	documents map[string]*storage.ProjectDocument
	commits   int
}

func newCLIActionPlanTestRepo(projects ...*models.Project) *cliActionPlanTestRepo {
	repo := &cliActionPlanTestRepo{
		projects:  projects,
		revisions: make(map[string]string, len(projects)),
		documents: make(map[string]*storage.ProjectDocument, len(projects)),
	}
	for _, project := range projects {
		doc, err := storage.CanonicalProjectDocument(project, nil)
		if err != nil {
			panic(err)
		}
		repo.revisions[project.ID] = doc.Revision
		repo.documents[project.ID] = doc
	}
	return repo
}

func (r *cliActionPlanTestRepo) Snapshot() (app.Snapshot, error) {
	revisions := make(map[string]string, len(r.revisions))
	for id, revision := range r.revisions {
		revisions[id] = revision
	}
	documents := make(map[string]*storage.ProjectDocument, len(r.documents))
	for id, document := range r.documents {
		documents[id] = document
	}
	return app.NewSnapshot(r.projects, revisions, documents), nil
}

func (r *cliActionPlanTestRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	for _, changeset := range changesets {
		if actual := r.revisions[changeset.Project.ID]; changeset.ExpectedRevision != actual {
			return nil, &storage.RevisionConflictError{
				ProjectID:   changeset.Project.ID,
				ProjectName: changeset.Project.Name,
				Expected:    changeset.ExpectedRevision,
				Actual:      actual,
			}
		}
		for _, precondition := range changeset.ProjectPreconditions {
			if actual := r.revisions[precondition.ProjectID]; precondition.ExpectedRevision != actual {
				return nil, &storage.RevisionConflictError{
					ProjectID:   precondition.ProjectID,
					ProjectName: precondition.ProjectName,
					Expected:    precondition.ExpectedRevision,
					Actual:      actual,
				}
			}
		}
	}
	r.commits++
	results := make([]*storage.CommitResult, len(changesets))
	for i := range results {
		results[i] = &storage.CommitResult{TransactionID: "cli-test-transaction"}
	}
	return results, nil
}
