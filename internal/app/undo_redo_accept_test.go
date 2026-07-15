package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// storageRepo is a real storage-backed app.Repository for acceptance testing: it
// drives mutations through the transactional commit service with the workspace
// undo ledger attached, so a delete committed through the application layer can be
// undone and redone through the same ledger the production adapter (later session)
// will use. It tracks each project's revision from the commit results, exactly as
// the storage-backed repository must.
type storageRepo struct {
	svc       *storage.CommitService
	projects  []*models.Project
	revisions map[string]string
}

func newStorageRepo(svc *storage.CommitService, projects ...*models.Project) *storageRepo {
	return &storageRepo{svc: svc, projects: projects, revisions: make(map[string]string)}
}

// seed writes the initial project state to disk so tasks.md exists and each
// project has a known revision before the use case under test runs.
func (r *storageRepo) seed(t *testing.T) {
	t.Helper()
	changesets := make([]storage.ChangeSet, len(r.projects))
	for i, p := range r.projects {
		changesets[i] = storage.ChangeSet{Project: p, UndoAction: "seed"}
	}
	results, err := r.svc.CommitMulti(changesets)
	if err != nil {
		t.Fatalf("seed commit: %v", err)
	}
	for i, p := range r.projects {
		r.revisions[p.ID] = results[i].Revision
	}
}

func (r *storageRepo) Snapshot() (app.Snapshot, error) {
	revs := make(map[string]string, len(r.revisions))
	docs := make(map[string]*storage.ProjectDocument, len(r.projects))
	for _, project := range r.projects {
		doc, err := storage.LoadProjectDocument(project)
		if err != nil {
			return nil, err
		}
		docs[project.ID] = doc
		revs[project.ID] = doc.Revision
	}
	return app.NewSnapshot(r.projects, revs, docs), nil
}

func (r *storageRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	results, err := r.svc.CommitMulti(changesets)
	if err != nil {
		return nil, err
	}
	for i := range changesets {
		r.revisions[changesets[i].Project.ID] = results[i].Revision
	}
	return results, nil
}

func (r *storageRepo) CommitArtifactTransaction(request storage.ArtifactCommitRequest) (string, error) {
	return r.svc.CommitArtifactTransaction(request)
}

// RawSaveProject implements app.MaintenanceRepository: it rewrites a project's
// tasks.md outside the transactional commit boundary, the deliberate repair-only
// escape hatch a doctor command uses.
func (r *storageRepo) RawSaveProject(project *models.Project) error {
	return storage.SaveProject(project)
}

// TestDeleteUndoRedoAcceptance exercises the whole wired path: an application-layer
// Delete commits a transaction that removes a task and its attachment bytes, the
// workspace ledger undo restores both the task markdown and the attachment bytes,
// and redo re-applies the delete. This proves destructive application use cases are
// fully reversible through the transactional undo ledger.
func TestDeleteUndoRedoAcceptance(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "delete me")
	task := project.Tasks[0]
	attachPath := seedRealAttachment(t, project, task, "note.txt", []byte("attachment bytes"))

	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)

	appSvc := app.NewService(repo, nil, nil)
	res, err := appSvc.Delete(app.DeleteRequest{TaskID: task.ID, Confirm: true})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(res.RemovedTaskIDs) != 1 {
		t.Fatalf("RemovedTaskIDs = %v, want 1", res.RemovedTaskIDs)
	}
	requireAbsentFile(t, attachPath)

	// Undo the delete: the task markdown and attachment bytes captured before the
	// commit come back through the ledger.
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := readFileContent(t, attachPath); got != "attachment bytes" {
		t.Fatalf("restored attachment = %q, want %q", got, "attachment bytes")
	}

	// Redo re-applies the delete: the attachment is removed again.
	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	requireAbsentFile(t, attachPath)
}

// TestDeleteConfigArtifactUndoRedoAcceptance proves a durable config artifact folded
// into a delete transaction commits with the removal, is restored byte-for-byte by
// the workspace ledger undo, and is re-applied by redo. This is the deletion analog
// of the move command's durable planning-ref cleanup riding the same transaction.
func TestDeleteConfigArtifactUndoRedoAcceptance(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "delete me")
	task := project.Tasks[0]

	// A durable config file inside the approved data-dir root holds a planning ref to
	// the task about to be deleted.
	configPath := filepath.Join(dataDir, ".tt.json")
	before := []byte(`{"focus_queue":[{"task_id":"` + task.ID + `","project_id":"` + project.ID + `"}]}`)
	if err := os.WriteFile(configPath, before, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)

	appSvc := app.NewService(repo, nil, nil)
	after := []byte(`{"focus_queue":[]}`)
	art := storage.ArtifactChange{Path: configPath, Data: after}
	if _, err := appSvc.Delete(app.DeleteRequest{TaskID: task.ID, Confirm: true}, art); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := readFileContent(t, configPath); got != string(after) {
		t.Fatalf("config after delete = %q, want the cleaned bytes", got)
	}

	// Undo restores the pre-delete config bytes: the planning ref comes back.
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := readFileContent(t, configPath); got != string(before) {
		t.Fatalf("restored config = %q, want %q", got, string(before))
	}

	// Redo re-applies the cleanup.
	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if got := readFileContent(t, configPath); got != string(after) {
		t.Fatalf("redone config = %q, want the cleaned bytes", got)
	}
}

// TestDeleteRollsBackWhenArtifactEscapesRoot injects a partial-write failure: an
// artifact whose target escapes the approved data-dir root aborts the whole delete
// during transaction preparation, so nothing is written — the task markdown and its
// attachment bytes survive intact.
func TestDeleteRollsBackWhenArtifactEscapesRoot(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "keep me")
	task := project.Tasks[0]
	attachPath := seedRealAttachment(t, project, task, "note.txt", []byte("attachment bytes"))

	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)

	appSvc := app.NewService(repo, nil, nil)
	escape := storage.ArtifactChange{Path: filepath.Join(t.TempDir(), "outside.json"), Data: []byte("x")}
	if _, err := appSvc.Delete(app.DeleteRequest{TaskID: task.ID, Confirm: true}, escape); err == nil {
		t.Fatal("expected delete to fail when an artifact escapes the approved root")
	}

	// The attachment bytes survive: the attachment-delete op never applied.
	if got := readFileContent(t, attachPath); got != "attachment bytes" {
		t.Fatalf("attachment mutated by aborted delete: %q", got)
	}
	// The on-disk task markdown is unchanged: the task is still present after reload.
	reloaded := &models.Project{ID: project.ID, Name: "Alpha", Slug: "alpha", Path: filepath.Join(dataDir, "projects", "alpha")}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if reloaded.GetTaskByID(task.ID) == nil {
		t.Fatalf("task %s removed from disk despite aborted delete", task.ID)
	}
}

// realTestProject builds a single-task project rooted under a canonical
// <data>/projects/<slug> layout so the commit service can write it within roots.
func realTestProject(t *testing.T, dataDir, slug, title string) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	p.Slug = slug
	p.Path = filepath.Join(dataDir, "projects", slug)
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	p.Tasks = []*models.Task{task}
	p.NextShortID = 2
	p.EnsureDefaultSections()
	p.BuildTaskTree()
	return p
}

// seedRealAttachment writes a file into a task's on-disk attachments directory,
// returning its path.
func seedRealAttachment(t *testing.T, project *models.Project, task *models.Task, name string, data []byte) string {
	t.Helper()
	dir := storage.GetTaskAttachmentsDir(project, task)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir attachments: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	return path
}

func readFileContent(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func requireAbsentFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, stat err = %v", path, err)
	}
}
