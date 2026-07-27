package app_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestResetSandboxCommitsProjectAndRevisionCheckedNotesTogether(t *testing.T) {
	project := sandboxProjectWithMetadata(t)
	repo := newFakeRepo(project)
	repo.validateChangeArtifacts = true
	port := repo.artifactPort()
	seedFakeSandboxNote(t, repo, port, project, "first.md", "First", "first body")
	seedFakeSandboxNote(t, repo, port, project, "second.md", "Second", "second body")
	svc := app.NewServiceWithArtifacts(repo, nil, nil, port, port)

	preview, err := svc.PreviewSandboxReset(project.ID)
	if err != nil {
		t.Fatalf("PreviewSandboxReset: %v", err)
	}
	if got, want := preview.Counts, (app.SandboxResetCounts{Tasks: 1, Notes: 2, Tags: 2, PinnedTags: 1, ShortIDCounter: 1}); got != want {
		t.Fatalf("preview counts = %+v, want %+v", got, want)
	}

	result, err := svc.ResetSandbox(project.ID)
	if err != nil {
		t.Fatalf("ResetSandbox: %v", err)
	}
	if result.Cleared != preview.Counts || result.TransactionID == "" || result.Revision == "" {
		t.Fatalf("result = %+v, want committed preview counts and ids", result)
	}
	if len(repo.files) != 0 {
		t.Fatalf("note artifacts remain after reset: %v", repo.files)
	}
	change, ok := repo.lastCommit()
	if !ok {
		t.Fatal("reset did not emit exactly one ChangeSet")
	}
	if change.UndoAction != "Reset Sandbox" || len(change.Artifacts) != 2 {
		t.Fatalf("reset ChangeSet = %+v, want one reset with two note deletes", change)
	}
	assertSandboxProjectCleared(t, change.Project)
	if len(project.Tasks) != 1 {
		t.Fatal("application service mutated the snapshot object supplied by its repository")
	}
	for _, artifact := range change.Artifacts {
		if !artifact.Delete || artifact.ExpectedRevision == nil || *artifact.ExpectedRevision == "" {
			t.Fatalf("note delete is not revision checked: %+v", artifact)
		}
	}
}

func TestResetSandboxDoesNotSkipNonDefaultStateWhenItemCountsAreZero(t *testing.T) {
	project := newProject("sandbox", "Sandbox")
	project.Sandbox = true
	project.NextShortID = 9
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	preview, err := svc.PreviewSandboxReset(project.ID)
	if err != nil {
		t.Fatalf("PreviewSandboxReset: %v", err)
	}
	if preview.Counts.Empty() || preview.Counts.ShortIDCounter != 1 {
		t.Fatalf("metadata-only reset preview = %+v, want counter reset", preview.Counts)
	}
	result, err := svc.ResetSandbox(project.ID)
	if err != nil {
		t.Fatalf("ResetSandbox: %v", err)
	}
	if result.Cleared != preview.Counts {
		t.Fatalf("committed counts = %+v, want preview %+v", result.Cleared, preview.Counts)
	}
	change, ok := repo.lastCommit()
	if !ok {
		t.Fatal("metadata-only reset did not commit")
	}
	assertSandboxProjectCleared(t, change.Project)
}

func TestResetSandboxNoteRevisionRaceAbortsProjectAndEveryNote(t *testing.T) {
	project := sandboxProjectWithMetadata(t)
	repo := newFakeRepo(project)
	repo.validateChangeArtifacts = true
	port := repo.artifactPort()
	firstPath := seedFakeSandboxNote(t, repo, port, project, "first.md", "First", "first body")
	secondPath := seedFakeSandboxNote(t, repo, port, project, "second.md", "Second", "second body")
	svc := app.NewServiceWithArtifacts(repo, nil, nil, port, port)

	concurrent := []byte("concurrent exact bytes")
	repo.preCommit = func() { repo.files[filepath.Clean(secondPath)] = concurrent }
	_, err := svc.ResetSandbox(project.ID)
	if !errors.Is(err, storage.ErrStaleArtifact) {
		t.Fatalf("ResetSandbox error = %v, want ErrStaleArtifact", err)
	}
	if len(repo.commits) != 0 {
		t.Fatal("stale note reset recorded a project commit")
	}
	if len(project.Tasks) != 1 || project.NextShortID != 2 || len(project.Tags) == 0 {
		t.Fatalf("project cache changed after rejected reset: %+v", project)
	}
	if _, ok := repo.files[filepath.Clean(firstPath)]; !ok {
		t.Fatal("unrelated note was deleted by rejected reset")
	}
	if got := repo.files[filepath.Clean(secondPath)]; string(got) != string(concurrent) {
		t.Fatalf("concurrent note bytes = %q, want %q", got, concurrent)
	}
}

func TestResetSandboxJournalFailureAtEveryOperationPreservesState(t *testing.T) {
	for failAt := 0; failAt < 3; failAt++ { // tasks.md followed by two note deletes
		t.Run(fmt.Sprintf("operation-%d", failAt), func(t *testing.T) {
			project := sandboxProjectWithMetadata(t)
			base := newFakeRepo(project)
			port := base.artifactPort()
			firstPath := seedFakeSandboxNote(t, base, port, project, "first.md", "First", "first body")
			secondPath := seedFakeSandboxNote(t, base, port, project, "second.md", "Second", "second body")
			repo := &sandboxJournalFailureRepo{fakeRepo: base, failAt: failAt}
			svc := app.NewServiceWithArtifacts(repo, nil, nil, port, port)

			if _, err := svc.ResetSandbox(project.ID); err == nil {
				t.Fatal("ResetSandbox error = nil, want injected journal failure")
			}
			if got, want := len(repo.attempted), failAt+1; got != want {
				t.Fatalf("journal attempts = %d, want failure after operation %d", got, want)
			}
			if len(project.Tasks) != 1 || project.NextShortID != 2 || len(project.Tags) == 0 {
				t.Fatalf("project changed at failed journal position %d", failAt)
			}
			for _, path := range []string{firstPath, secondPath} {
				if _, ok := base.files[filepath.Clean(path)]; !ok {
					t.Fatalf("note %s disappeared at failed journal position %d", path, failAt)
				}
			}
		})
	}
}

func TestResetSandboxUndoRedoRestoresExactProjectAndNoteBytes(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	project := realTestProject(t, dataDir, "_sandbox", "reset me")
	project.Sandbox = true
	project.Tags = []string{"catalog"}
	project.PinnedTags = []string{"catalog"}
	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)

	notesDir := storage.NotesDir(project)
	if err := os.MkdirAll(notesDir, 0o700); err != nil {
		t.Fatalf("MkdirAll notes: %v", err)
	}
	valid := models.NewNote("Valid")
	valid.Filename = "valid.md"
	valid.Body = "valid body\nwith exact spacing"
	if err := storage.SaveNote(project, valid); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	brokenPath := filepath.Join(notesDir, "broken.md")
	brokenBytes := []byte("---\ntitle: [\n---\nraw malformed body\n")
	if err := os.WriteFile(brokenPath, brokenBytes, 0o600); err != nil {
		t.Fatalf("write broken note: %v", err)
	}
	validPath := filepath.Join(notesDir, valid.Filename)
	validBytes, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatalf("read valid note: %v", err)
	}
	beforeTasks, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read tasks before reset: %v", err)
	}

	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir), nil)
	result, err := svc.ResetSandbox(project.ID)
	if err != nil {
		t.Fatalf("ResetSandbox: %v", err)
	}
	if result.Cleared.Notes != 2 || result.Cleared.Tasks != 1 {
		t.Fatalf("cleared counts = %+v, want one task and two notes", result.Cleared)
	}
	afterTasks, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read tasks after reset: %v", err)
	}
	assertFileAbsent(t, validPath)
	assertFileAbsent(t, brokenPath)

	if _, err := ledger.Undo(storage.ProjectScope(project.ID)); err != nil {
		t.Fatalf("Undo reset: %v", err)
	}
	assertExactFile(t, storage.TasksFilePath(project.Path), beforeTasks)
	assertExactFile(t, validPath, validBytes)
	assertExactFile(t, brokenPath, brokenBytes)

	if _, err := ledger.Redo(storage.ProjectScope(project.ID)); err != nil {
		t.Fatalf("Redo reset: %v", err)
	}
	assertExactFile(t, storage.TasksFilePath(project.Path), afterTasks)
	assertFileAbsent(t, validPath)
	assertFileAbsent(t, brokenPath)
}

type sandboxJournalFailureRepo struct {
	*fakeRepo
	failAt    int
	attempted []string
}

func (r *sandboxJournalFailureRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	if len(changesets) != 1 {
		return nil, fmt.Errorf("injected journal expected one changeset")
	}
	operations := []string{"tasks.md"}
	for _, artifact := range changesets[0].Artifacts {
		operations = append(operations, filepath.Base(artifact.Path))
	}
	for i, operation := range operations {
		r.attempted = append(r.attempted, operation)
		if i == r.failAt {
			return nil, fmt.Errorf("injected journal failure at operation %d (%s)", i, operation)
		}
	}
	return r.fakeRepo.Commit(changesets...)
}

func sandboxProjectWithMetadata(t *testing.T) *models.Project {
	t.Helper()
	project := newProject("sandbox", "Sandbox", newTask("task-1", 1, "Reset me"))
	project.Sandbox = true
	project.Tags = []string{"catalog"}
	project.PinnedTags = []string{"catalog"}
	project.Tasks[0].Tags = []string{"task-tag"}
	return project
}

func seedFakeSandboxNote(t *testing.T, repo *fakeRepo, port *memArtifactPort, project *models.Project, filename, title, body string) string {
	t.Helper()
	note := &models.Note{
		Filename:   filename,
		Title:      title,
		Body:       body,
		CreatedAt:  time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC),
		ModifiedAt: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC),
	}
	change, err := port.EncodeNoteSave(app.ProjectNoteLocation(project.ID), project, note)
	if err != nil {
		t.Fatalf("EncodeNoteSave: %v", err)
	}
	repo.files[filepath.Clean(change.Path)] = append([]byte(nil), change.Data...)
	return change.Path
}

func assertSandboxProjectCleared(t *testing.T, project *models.Project) {
	t.Helper()
	if len(project.Tasks) != 0 || project.NextShortID != 1 {
		t.Fatalf("sandbox tasks not cleared: tasks=%d next=%d", len(project.Tasks), project.NextShortID)
	}
	if len(project.Tags) != 0 || len(project.PinnedTags) != 0 {
		t.Fatalf("sandbox metadata not cleared: tags=%v pins=%v", project.Tags, project.PinnedTags)
	}
}

func assertExactFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s bytes = %q, want %q", path, got, want)
	}
}

func assertFileAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists after reset, stat err=%v", path, err)
	}
}
