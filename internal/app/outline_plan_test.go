package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// This file covers the buffer the apply accepts: what one transaction writes,
// what planning reconciliation it triggers, and the snapshot the dump itself is
// read from. It also holds assertOutlinePlanRolledBack, which the refusals in
// outline_plan_validation_test.go assert through. Deletion by omission lives in
// outline_plan_delete_test.go.

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
			{SourceLine: 1, TaskID: beta.ID, Depth: 0, Title: "Beta renamed", Priority: 1, Tags: []string{"work"}, Context: " Client-Acme ", Destination: textgrammar.DestinationDoing},
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
	if beta.Context != "client-acme" {
		t.Errorf("beta context = %q, want client-acme", beta.Context)
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

func TestOutlinePlanCountsAContextOnlyEdit(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	alpha.Context = "legacy"
	project := newProject("p1", "Alpha Project", alpha)
	repo := newFakeRepo(project)

	result, err := newTestService(repo).ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Depth: 0, Title: alpha.Title, Context: "platform"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if alpha.Context != "platform" || len(result.Updated) != 1 || result.Updated[0].TaskID != alpha.ID {
		t.Fatalf("context-only result = context %q updated %+v", alpha.Context, result.Updated)
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
