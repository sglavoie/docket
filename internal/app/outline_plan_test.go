package app_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/textgrammar"
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

// TestOutlinePlanDeletesAbsentTasksInTheSameTransaction is the contract that
// makes `O` a bulk-delete tool: the removals ride the edits beside them, so both
// land or neither does, and one undo entry covers the pair.
func TestOutlinePlanDeletesAbsentTasksInTheSameTransaction(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	beta := newTask("t2", 2, "Beta")
	gamma := newTask("t3", 3, "Gamma")
	gamma.Attachments = []models.Attachment{{Filename: "spec.pdf"}}
	project := newProject("p1", "Alpha Project", alpha, beta, gamma)
	repo := newFakeRepo(project)
	planning := &config.PlanningState{}
	planning.SetToday([]config.ProjectTaskLocator{{TaskID: gamma.ID, ProjectID: project.ID}})
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	result, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID, beta.ID, gamma.ID},
		DeleteTaskIDs:    []string{gamma.ID},
		ConfirmDeletions: true,
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Depth: 0, Title: "Alpha edited"},
			{SourceLine: 2, TaskID: beta.ID, Depth: 0, Title: "Beta"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if project.GetTaskByID(gamma.ID) != nil {
		t.Fatal("a task the buffer no longer holds survived")
	}
	if alpha.Title != "Alpha edited" {
		t.Errorf("alpha title = %q, want the edit that rode along", alpha.Title)
	}
	if len(result.Deleted) != 1 || result.Deleted[0].TaskID != gamma.ID {
		t.Fatalf("deleted = %+v, want only gamma", result.Deleted)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d, want one atomic transaction", len(repo.commits))
	}
	changeset, ok := repo.lastCommit()
	if !ok {
		t.Fatal("transaction did not carry exactly one changeset")
	}
	if changeset.UndoAction != "Apply Outline" {
		t.Errorf("UndoAction = %q, want the shared outline entry", changeset.UndoAction)
	}
	if len(changeset.AttachmentOps) != 1 {
		t.Fatalf("AttachmentOps = %d, want one AttachmentDelete for the removed task", len(changeset.AttachmentOps))
	}
	op := changeset.AttachmentOps[0]
	if op.Kind != storage.AttachmentDelete || op.Task != gamma || op.SourceProject != project {
		t.Errorf("attachment op = %+v, want a delete of gamma in this project", op)
	}
	if len(planning.GetToday()) != 0 {
		t.Errorf("Today = %+v, want the deleted task's ref cleaned in the same commit", planning.GetToday())
	}
	if len(changeset.Artifacts) != 1 {
		t.Fatalf("planning artifacts = %d, want exactly one reconciliation", len(changeset.Artifacts))
	}
	for _, item := range result.Updated {
		if item.TransactionID != result.Deleted[0].TransactionID {
			t.Errorf("edit transaction %q differs from the deletion's %q", item.TransactionID, result.Deleted[0].TransactionID)
		}
	}
	if got, want := result.Summary(), "Applied 1 change · 1 task deleted"; got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

// TestOutlinePlanEmptiesAProjectWhenEveryLineIsGone is the boundary of
// absence-deletes: a buffer holding no lines removes the whole scope, so the
// empty-buffer refusal has to be about deleting nothing, not about holding
// nothing.
func TestOutlinePlanEmptiesAProjectWhenEveryLineIsGone(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	beta := newTask("t2", 2, "Beta")
	beta.ParentID = &alpha.ID
	project := newProject("p1", "Alpha Project", alpha, beta)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID, beta.ID},
		DeleteTaskIDs:    []string{alpha.ID, beta.ID},
		ConfirmDeletions: true,
	})
	if err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if len(project.Tasks) != 0 {
		t.Fatalf("project holds %d tasks, want none", len(project.Tasks))
	}
	if len(result.Deleted) != 2 {
		t.Fatalf("deleted = %d, want both tasks", len(result.Deleted))
	}
	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d, want one atomic transaction", len(repo.commits))
	}
	if got, want := result.Summary(), "Applied 0 changes · 2 tasks deleted"; got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

// TestOutlinePlanRefusesUnconfirmedDeletions holds the outline apply to the same
// destructive-intent rule as Delete and BulkDelete: an adapter that skipped the
// prompt cannot remove a task by omission.
func TestOutlinePlanRefusesUnconfirmedDeletions(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	beta := newTask("t2", 2, "Beta")
	project := newProject("p1", "Alpha Project", alpha, beta)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID, beta.ID},
		DeleteTaskIDs:    []string{beta.ID},
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Depth: 0, Title: "Alpha"},
		},
	})
	if !errors.Is(err, app.ErrDestructiveConfirmationRequired) {
		t.Fatalf("err = %v, want ErrDestructiveConfirmationRequired", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("an unconfirmed deletion committed %d times, want 0", len(repo.commits))
	}
	if project.GetTaskByID(beta.ID) == nil {
		t.Error("an unconfirmed deletion removed the task anyway")
	}
}

// TestOutlinePlanRefusesADeleteListThatDisagreesWithTheBuffer is what proves the
// user was shown exactly what gets removed: the confirmed list and the buffer's
// absences must name the same set, in either direction, or nothing is written.
func TestOutlinePlanRefusesADeleteListThatDisagreesWithTheBuffer(t *testing.T) {
	tests := []struct {
		name    string
		deletes []string
		lines   []app.OutlinePlanLine
		message string
	}{
		{
			name:    "absent but unconfirmed",
			deletes: nil,
			lines: []app.OutlinePlanLine{
				{SourceLine: 1, TaskID: "t1", Depth: 0, Title: "Alpha"},
			},
			message: "task t2 is missing from the buffer but was not confirmed for deletion",
		},
		{
			name:    "confirmed but still present",
			deletes: []string{"t2"},
			lines: []app.OutlinePlanLine{
				{SourceLine: 1, TaskID: "t1", Depth: 0, Title: "Alpha"},
				{SourceLine: 2, TaskID: "t2", Depth: 0, Title: "Beta"},
			},
			message: "task t2 is on line 2 and cannot also be deleted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alpha := newTask("t1", 1, "Alpha")
			beta := newTask("t2", 2, "Beta")
			project := newProject("p1", "Alpha Project", alpha, beta)
			repo := newFakeRepo(project)
			svc := newTestService(repo)

			_, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
				ProjectID:        project.ID,
				ExpectedRevision: repo.revisions[project.ID],
				ScopeTaskIDs:     []string{alpha.ID, beta.ID},
				DeleteTaskIDs:    tt.deletes,
				ConfirmDeletions: true,
				Lines:            tt.lines,
			})
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("err = %v, want it to name %q", err, tt.message)
			}
			if len(repo.commits) != 0 {
				t.Errorf("a disagreeing request committed %d times, want 0", len(repo.commits))
			}
			if project.GetTaskByID(beta.ID) == nil {
				t.Error("a refused request still removed the task")
			}
		})
	}
}

// TestOutlinePlanReparentsASurvivorWhoseParentLineWasDeleted is why the removal
// runs before parentage is applied: indentation stays the only parentage
// channel, so a child whose own line survived attaches wherever the buffer put
// it rather than being cascaded away with its old parent.
func TestOutlinePlanReparentsASurvivorWhoseParentLineWasDeleted(t *testing.T) {
	alpha := newTask("t1", 1, "Alpha")
	beta := newTask("t2", 2, "Beta")
	child := newTask("t3", 3, "Child")
	parentID := beta.ID
	child.ParentID = &parentID
	project := newProject("p1", "Alpha Project", alpha, beta, child)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	if _, err := svc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{alpha.ID, beta.ID, child.ID},
		DeleteTaskIDs:    []string{beta.ID},
		ConfirmDeletions: true,
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: alpha.ID, Depth: 0, Title: "Alpha"},
			{SourceLine: 2, TaskID: child.ID, Depth: 1, Title: "Child"},
		},
	}); err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	if project.GetTaskByID(beta.ID) != nil {
		t.Fatal("the deleted parent survived")
	}
	survivor := project.GetTaskByID(child.ID)
	if survivor == nil {
		t.Fatal("a child whose own line survived was cascaded away with its parent")
	}
	if survivor.ParentID == nil || *survivor.ParentID != alpha.ID {
		t.Errorf("survivor parent = %v, want the buffer's indentation to name %s", survivor.ParentID, alpha.ID)
	}
}

// TestOutlinePlanDeletionUndoRestoresTheProject drives the wired path: one undo
// puts back both the removed task and the edit that committed beside it,
// because the whole buffer is a single ledger entry.
func TestOutlinePlanDeletionUndoRestoresTheProject(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "Keep me")
	kept := project.Tasks[0]
	doomed, err := models.NewGenericTask(uuid.New().String(), 2, "Delete me")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	project.Tasks = append(project.Tasks, doomed)
	project.NextShortID = 3
	project.BuildTaskTree()

	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)
	tasksPath := storage.TasksFilePath(project.Path)

	appSvc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	if _, err := appSvc.ApplyOutlinePlan(app.OutlinePlanRequest{
		ProjectID:        project.ID,
		ExpectedRevision: repo.revisions[project.ID],
		ScopeTaskIDs:     []string{kept.ID, doomed.ID},
		DeleteTaskIDs:    []string{doomed.ID},
		ConfirmDeletions: true,
		Lines: []app.OutlinePlanLine{
			{SourceLine: 1, TaskID: kept.ID, Depth: 0, Title: "Keep me, renamed"},
		},
	}); err != nil {
		t.Fatalf("ApplyOutlinePlan: %v", err)
	}
	committed := readFileContent(t, tasksPath)
	if strings.Contains(committed, "Delete me") || !strings.Contains(committed, "Keep me, renamed") {
		t.Fatalf("committed tasks.md did not apply both halves:\n%s", committed)
	}

	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	restored := readFileContent(t, tasksPath)
	if !strings.Contains(restored, "Delete me") {
		t.Errorf("undo did not restore the deleted task:\n%s", restored)
	}
	if strings.Contains(restored, "Keep me, renamed") {
		t.Errorf("undo restored the deletion but kept the edit:\n%s", restored)
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
