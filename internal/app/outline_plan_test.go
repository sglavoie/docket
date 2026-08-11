package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestOutlinePlanCommitsEditsCreatesAndReparentsAsOneTransaction(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	beta := newTask("t2", 2, "Beta")
	gamma := newTask("t3", 3, "Gamma")
	project := newProject("p1", "Alpha Project", alpha, beta, gamma)
	repo := newFakeRepo(project)
	planning := &config.PlanningState{}
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	result, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID, beta.ID, gamma.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: beta.ID, Depth: 0, Title: "Beta renamed", Priority: 1, Tags: []string{"work"}, Destination: app.CaptureDestinationDoing},
			{SourceLine: 2, TaskID: alpha.ID, Depth: 1, Title: "Alpha"},
			{SourceLine: 3, Depth: 1, Title: "Delta"},
			{SourceLine: 4, TaskID: gamma.ID, Depth: 0, Title: "Gamma"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}

	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d, want one atomic transaction", len(repo.commits))
	}
	changeset, ok := repo.lastCommit()
	if !ok {
		t.Fatal("transaction did not carry exactly one changeset")
	}
	if changeset.UndoAction != "Apply Outline" {
		t.Errorf("UndoAction = %q, want %q", changeset.UndoAction, "Apply Outline")
	}
	if changeset.UndoContext != result.Summary() {
		t.Errorf("UndoContext = %q, want the reported summary %q", changeset.UndoContext, result.Summary())
	}

	if beta.Title != "Beta renamed" || beta.Priority != 1 || beta.State() != models.StateDoing {
		t.Errorf("beta = %q/%d/%s, want the edited values", beta.Title, beta.Priority, beta.State())
	}
	if len(beta.Tags) != 1 || beta.Tags[0] != "work" {
		t.Errorf("beta tags = %v, want [work]", beta.Tags)
	}
	if alpha.ParentID == nil || *alpha.ParentID != beta.ID {
		t.Errorf("alpha parent = %v, want %s", alpha.ParentID, beta.ID)
	}
	if len(result.Created) != 1 {
		t.Fatalf("created = %d, want the one new line", len(result.Created))
	}
	delta := project.GetTaskByID(result.Created[0].TaskID)
	if delta == nil || delta.Title != "Delta" || delta.ParentID == nil || *delta.ParentID != beta.ID {
		t.Fatalf("created task = %+v, want Delta nested under beta", delta)
	}

	wantOrder := []string{beta.ID, alpha.ID, delta.ID, gamma.ID}
	gotOrder := make([]string, 0, len(project.Tasks))
	for _, task := range project.Tasks {
		gotOrder = append(gotOrder, task.ID)
	}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Errorf("task order = %v, want the buffer order %v", gotOrder, wantOrder)
	}

	updated := make([]string, 0, len(result.Updated))
	for _, item := range result.Updated {
		updated = append(updated, item.TaskID)
	}
	if strings.Join(updated, ",") != beta.ID+","+alpha.ID {
		t.Errorf("updated = %v, want only the two tasks the buffer changed", updated)
	}
	if got, want := result.Summary(), "Applied 3 changes"; got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

func TestOutlinePlanReconcilesPlanningOnce(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	project := newProject("p1", "Alpha Project", alpha)
	repo := newFakeRepo(project)
	planning := &config.PlanningState{}
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	result, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Depth: 0, Title: "Alpha", Destination: app.CaptureDestinationNext},
			{SourceLine: 2, Depth: 0, Title: "Beta", Destination: app.CaptureDestinationToday},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if len(result.Created) != 1 {
		t.Fatalf("created = %d, want one", len(result.Created))
	}
	beta := result.Created[0].TaskID

	if !planningHasLocator(planning.GetNext(), alpha.ID, project.ID) {
		t.Errorf("Next = %+v, want the edited task", planning.GetNext())
	}
	if !planningHasLocator(planning.GetToday(), beta, project.ID) {
		t.Errorf("Today = %+v, want the created task", planning.GetToday())
	}

	changeset, ok := repo.lastCommit()
	if !ok {
		t.Fatal("transaction did not carry exactly one changeset")
	}
	if len(changeset.Artifacts) != 1 {
		t.Fatalf("planning artifacts = %d, want exactly one reconciliation", len(changeset.Artifacts))
	}
	// memoryPlanningBuilder numbers each build, so a second reconciliation would
	// have produced planning-2 and overwritten the first.
	if got := string(changeset.Artifacts[0].Data); got != "planning-1" {
		t.Errorf("planning artifact = %q, want the single first build", got)
	}
}

func TestOutlinePlanReportsAbsentTasksUnchangedInsteadOfDeleting(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	beta := newTask("t2", 2, "Beta")
	gamma := newTask("t3", 3, "Gamma")
	project := newProject("p1", "Alpha Project", alpha, beta, gamma)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID, beta.ID, gamma.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Depth: 0, Title: "Alpha edited"},
			{SourceLine: 2, TaskID: beta.ID, Depth: 0, Title: "Beta"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if project.GetTaskByID(gamma.ID) == nil {
		t.Fatal("a task missing from the buffer was deleted")
	}
	if gamma.Title != "Gamma" {
		t.Errorf("gamma title = %q, want the untouched value", gamma.Title)
	}
	if len(result.Unchanged) != 1 || result.Unchanged[0].TaskID != gamma.ID {
		t.Fatalf("unchanged = %+v, want only gamma", result.Unchanged)
	}
	if got, want := result.Summary(), "Applied 1 change · 1 task not in buffer, unchanged"; got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

func TestOutlinePlanOmittedDestinationPreservesState(t *testing.T) {
	waiting := newTask("t1", 1, "Blocked")
	waiting.ApplyWorkflowState(models.StateWaiting)
	project := newProject("p1", "Alpha Project", waiting)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	if _, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{waiting.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: waiting.ID, Depth: 0, Title: "Blocked, renamed"},
		},
	}); err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if waiting.State() != models.StateWaiting {
		t.Errorf("state = %s, want the preserved Waiting state", waiting.State())
	}
}

func TestOutlinePlanRejectsUnknownAndCrossProjectTasks(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	foreign := newTask("t9", 1, "Foreign")
	project := newProject("p1", "Alpha Project", alpha)
	other := newProject("p2", "Other Project", foreign)
	repo := newFakeRepo(project, other)
	svc := newTestService(repo)

	_, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		Lines:            []app.OutlinePlanLine{{SourceLine: 4, TaskID: "missing", Depth: 0, Title: "Alpha"}},
	})
	if !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("unknown task error = %v, want ErrTaskNotFound", err)
	}
	if !strings.HasPrefix(err.Error(), "line 4: ") {
		t.Errorf("error = %q, want the buffer line named first", err)
	}

	_, err = svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		Lines:            []app.OutlinePlanLine{{SourceLine: 1, TaskID: foreign.ID, Depth: 0, Title: "Foreign"}},
	})
	if !errors.Is(err, app.ErrInvalidInput) || !strings.Contains(err.Error(), "another project") {
		t.Fatalf("cross-project error = %v, want a refusal naming the other project", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("commits = %d, want none", len(repo.commits))
	}
}

func TestOutlinePlanRejectsDoneWithNonterminalChild(t *testing.T) {
	parent := newTask("t1", 1, "Parent")
	child := newTask("t2", 2, "Child")
	project := newProject("p1", "Alpha Project", parent, child)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{parent.ID, child.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: parent.ID, Depth: 0, Title: "Parent", Destination: app.CaptureDestinationDone},
			{SourceLine: 2, TaskID: child.ID, Depth: 1, Title: "Child"},
		},
	})
	var blocked *app.ChildrenBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("error = %v, want a ChildrenBlockedError from the reindented tree", err)
	}
	assertOutlinePlanRolledBack(t, repo, project, 2)
	if parent.State() == models.StateDone {
		t.Error("parent was completed despite the blocking child")
	}
}

func TestOutlinePlanRejectsNestingBeyondTheDepthCap(t *testing.T) {
	t.Run("within the buffer", func(t *testing.T) {
		project := newProject("p1", "Alpha Project")
		repo := newFakeRepo(project)
		svc := newTestService(repo)

		lines := make([]app.OutlinePlanLine, 0, models.MaxNestingDepth+2)
		for depth := 0; depth <= models.MaxNestingDepth+1; depth++ {
			lines = append(lines, app.OutlinePlanLine{SourceLine: depth + 1, Depth: depth, Title: "Level"})
		}
		_, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
			ProjectID:        project.ID,
			ExpectedRevision: repo.revisions[project.ID],
			Lines:            lines,
		})
		if !errors.Is(err, app.ErrInvalidInput) || !strings.Contains(err.Error(), "maximum of 6 levels") {
			t.Fatalf("error = %v, want a depth-cap refusal", err)
		}
		assertOutlinePlanRolledBack(t, repo, project, 0)
	})

	t.Run("through a descendant the buffer never showed", func(t *testing.T) {
		chain := make([]*models.Task, 0, 5)
		for i := range 5 {
			task := newTask("c"+string(rune('1'+i)), i+1, "Chain")
			if i > 0 {
				parentID := chain[i-1].ID
				task.ParentID = &parentID
			}
			chain = append(chain, task)
		}
		project := newProject("p1", "Alpha Project", chain...)
		repo := newFakeRepo(project)
		svc := newTestService(repo)

		_, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
			ProjectID:        project.ID,
			ExpectedRevision: repo.revisions[project.ID],
			ScopeTaskIDs:     []string{chain[0].ID},
			Lines: []app.OutlinePlanLine{
				{SourceLine: 1, Depth: 0, Title: "New root"},
				{SourceLine: 2, Depth: 1, Title: "New child"},
				{SourceLine: 3, Depth: 2, Title: "New grandchild"},
				{SourceLine: 4, TaskID: chain[0].ID, Depth: 3, Title: "Chain"},
			},
		})
		if !errors.Is(err, app.ErrInvalidInput) || !strings.Contains(err.Error(), "maximum of 6 levels") {
			t.Fatalf("error = %v, want a depth-cap refusal covering the hidden subtree", err)
		}
		assertOutlinePlanRolledBack(t, repo, project, 5)
		if chain[0].ParentID != nil {
			t.Errorf("chain root parent = %v, want nil after rollback", chain[0].ParentID)
		}
	})
}

func TestOutlinePlanRejectsMalformedLines(t *testing.T) {
	cases := []struct {
		name string
		line app.OutlinePlanLine
		want string
	}{
		{"blank title", app.OutlinePlanLine{SourceLine: 2, Depth: 0, Title: "   "}, "line 2: "},
		{"priority out of range", app.OutlinePlanLine{SourceLine: 2, Depth: 0, Title: "Task", Priority: 4}, "priority"},
		{"malformed tag", app.OutlinePlanLine{SourceLine: 2, Depth: 0, Title: "Task", Tags: []string{strings.Repeat("a", models.MaxTagLength+1)}}, "tag"},
		{"retired destination", app.OutlinePlanLine{SourceLine: 2, Depth: 0, Title: "Task", Destination: "ready"}, "$backlog"},
		{"indentation without a parent", app.OutlinePlanLine{SourceLine: 2, Depth: 2, Title: "Task"}, "has no parent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			alpha := newTask("t1", 1, "Alpha")
			project := newProject("p1", "Alpha Project", alpha)
			repo := newFakeRepo(project)
			svc := newTestService(repo)

			_, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
				ProjectID:        project.ID,
				ExpectedRevision: repo.revisions[project.ID],
				Lines: []app.OutlinePlanLine{
					{SourceLine: 1, TaskID: alpha.ID, Depth: 0, Title: "Alpha"},
					tc.line,
				},
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want one containing %q", err, tc.want)
			}
			assertOutlinePlanRolledBack(t, repo, project, 1)
		})
	}
}

func TestOutlinePlanRejectsDuplicateEmptyAndUnknownScopes(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	project := newProject("p1", "Alpha Project", alpha)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Depth: 0, Title: "Alpha"},
			{SourceLine: 2, TaskID: alpha.ID, Depth: 0, Title: "Alpha again"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "more than one line") {
		t.Fatalf("duplicate error = %v, want a refusal naming the repeat", err)
	}

	if _, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
	}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("empty buffer error = %v, want ErrInvalidInput", err)
	}

	if _, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        "missing",
		ExpectedRevision: repo.revisions[project.ID],
		Lines:            []app.OutlinePlanLine{{SourceLine: 1, Depth: 0, Title: "Task"}},
	}); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("unknown project error = %v, want ErrProjectNotFound", err)
	}

	assertOutlinePlanRolledBack(t, repo, project, 1)
}

func TestOutlinePlanEditsDueDatesAndTags(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	alpha.Tags = []string{"stale"}
	due := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	project := newProject("p1", "Alpha Project", alpha)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	if _, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Depth: 0, Title: "Alpha", Tags: []string{"fresh"}, Due: &due},
		},
	}); err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if len(alpha.Tags) != 1 || alpha.Tags[0] != "fresh" {
		t.Errorf("tags = %v, want the buffer's complete set", alpha.Tags)
	}
	if alpha.DueDate == nil || !alpha.DueDate.Equal(due) {
		t.Errorf("due = %v, want %v", alpha.DueDate, due)
	}
}

// TestOutlinePlanRejectsAStaleDumpRevisionBeforeMutating is the concurrency
// contract: a writer that lands between the dump and the apply must lose the
// buffer, not the edits it just committed.
func TestOutlinePlanRejectsAStaleDumpRevisionBeforeMutating(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	beta := newTask("t2", 2, "Beta")
	project := newProject("p1", "Alpha Project", alpha, beta)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	dumped := repo.revisions[project.ID]

	// A concurrent writer commits while the editor is open.
	repo.revisions[project.ID] = dumped + "-moved"

	_, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: dumped,
		ScopeTaskIDs:     []string{alpha.ID, beta.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Depth: 0, Title: "Alpha renamed"},
			{SourceLine: 2, TaskID: beta.ID, Depth: 1, Title: "Beta reparented"},
			{SourceLine: 3, Depth: 0, Title: "Gamma created"},
		},
	})
	if !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("error = %v, want a stale-revision refusal", err)
	}
	assertOutlinePlanRolledBack(t, repo, project, 2)
	if alpha.Title != "Alpha" || beta.Title != "Beta" || beta.ParentID != nil {
		t.Errorf("tasks were mutated before the revision was checked: %q / %q parent %v", alpha.Title, beta.Title, beta.ParentID)
	}
}

// TestOutlinePlanRequiresTheDumpRevision keeps the binding mandatory: a caller
// that cannot name the tree its buffer came from has nothing to be atomic
// against, so the request is refused rather than applied against whatever is
// current.
func TestOutlinePlanRequiresTheDumpRevision(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	project := newProject("p1", "Alpha Project", alpha)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:    project.ID,
		ScopeTaskIDs: []string{alpha.ID},
		Lines:        []app.OutlinePlanLine{{SourceLine: 1, TaskID: alpha.ID, Depth: 0, Title: "Alpha renamed"}},
	})
	if !errors.Is(err, app.ErrInvalidInput) || !strings.Contains(err.Error(), "revision") {
		t.Fatalf("error = %v, want a refusal naming the missing revision", err)
	}
	if alpha.Title != "Alpha" {
		t.Errorf("title = %q, want the untouched value", alpha.Title)
	}
	assertOutlinePlanRolledBack(t, repo, project, 1)
}

// TestOutlineProjectAtRevisionReadsBothFromOneSnapshot is what makes the dump
// atomic at the other end: the tree an adapter renders and the revision it
// carries back must come from the same read.
func TestOutlineProjectAtRevisionReadsBothFromOneSnapshot(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	project := newProject("p1", "Alpha Project", alpha)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	loaded, revision, err := svc.ProjectAtRevision(project.ID)
	if err != nil {
		t.Fatalf("ProjectAtRevision: %v", err)
	}
	if repo.snapshots != 1 {
		t.Errorf("snapshots = %d, want the project and its revision from one read", repo.snapshots)
	}
	if loaded == nil || loaded.ID != project.ID {
		t.Fatalf("project = %v, want %s", loaded, project.ID)
	}
	if revision != repo.revisions[project.ID] {
		t.Errorf("revision = %q, want %q", revision, repo.revisions[project.ID])
	}
	if _, _, err := svc.ProjectAtRevision("missing"); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("unknown project error = %v, want ErrProjectNotFound", err)
	}
}

// assertOutlinePlanRolledBack checks that a rejected buffer left no commit and
// no partially applied task behind.
func assertOutlinePlanRolledBack(t *testing.T, repo *fakeRepo, project *models.Project, wantTasks int) {
	t.Helper()
	if len(repo.commits) != 0 {
		t.Fatalf("commits = %d, want none after a rejected buffer", len(repo.commits))
	}
	if len(project.Tasks) != wantTasks {
		t.Fatalf("tasks = %d, want %d after rollback", len(project.Tasks), wantTasks)
	}
}
