package app_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/textgrammar"
)

// This file covers the buffers the apply refuses: unresolvable targets, illegal
// shapes, and a revision that no longer names the tree the dump came from. Every
// case asserts through assertOutlinePlanRolledBack, which lives beside the
// ordinary apply path in outline_plan_test.go; deletion by omission is in
// outline_plan_delete_test.go.

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
			{SourceLine: 1, TaskID: parent.ID, Depth: 0, Title: "Parent", Destination: textgrammar.DestinationDone},
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
