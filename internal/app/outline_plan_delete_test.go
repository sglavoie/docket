package app_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// This file covers deletion by omission: a task whose line the buffer no longer
// holds is removed in the same transaction and the same undo entry as the edits
// beside it. The ordinary apply path is in outline_plan_test.go and the
// refusals in outline_plan_validation_test.go.

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
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".docket", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir}, ledger)

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
