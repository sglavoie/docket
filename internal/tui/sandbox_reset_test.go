package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestSandboxResetTUISuccessReloadsCacheOnlyAfterDoneMessage(t *testing.T) {
	shell, project, notePath := newSandboxResetTUIFixture(t, nil)

	msg := shell.executeSandboxReset()()
	done, ok := msg.(SandboxResetDoneMsg)
	if !ok {
		t.Fatalf("executeSandboxReset message = %T, want SandboxResetDoneMsg", msg)
	}
	if done.Result.Cleared.Tasks != 1 || done.Result.Cleared.Notes != 1 {
		t.Fatalf("committed reset result = %+v, want one task and one note", done.Result.Cleared)
	}

	// The worker has committed to disk, but it must not touch the model pointer
	// owned by Bubble Tea before Update receives the result.
	assertSandboxResetTUICacheIntact(t, project)
	assertSandboxResetTUIDiskCleared(t, project, notePath)

	handled, cmd := shell.handleProjectMessages(done)
	if !handled || cmd == nil {
		t.Fatalf("done handled=%v cmd=%v, want handled status timeout", handled, cmd)
	}
	if shell.pendingSandboxReset || shell.pendingSandboxProjectID != "" || shell.pendingSandboxTaskCount != 0 {
		t.Fatalf("pending reset state not cleared: pending=%v id=%q count=%d", shell.pendingSandboxReset, shell.pendingSandboxProjectID, shell.pendingSandboxTaskCount)
	}
	if len(project.Tasks) != 0 || project.NextShortID != 1 || len(project.Tags) != 0 || len(project.PinnedTags) != 0 {
		t.Fatalf("cache was not reloaded to committed reset: %+v", project)
	}
	if shell.statusMessage != "Cleared 7 item(s) from sandbox" {
		t.Fatalf("status = %q, want committed count", shell.statusMessage)
	}
}

func TestSandboxResetTUIFailurePreservesDiskAndCache(t *testing.T) {
	injected := errors.New("injected reset commit failure")
	shell, project, notePath := newSandboxResetTUIFixture(t, injected)
	beforeTasks, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read tasks before reset: %v", err)
	}
	beforeNote, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("read note before reset: %v", err)
	}

	msg := shell.executeSandboxReset()()
	failed, ok := msg.(SandboxResetFailedMsg)
	if !ok {
		t.Fatalf("executeSandboxReset message = %T, want SandboxResetFailedMsg", msg)
	}
	if !errors.Is(failed.Err, injected) {
		t.Fatalf("failure = %v, want injected commit error", failed.Err)
	}
	assertSandboxResetTUICacheIntact(t, project)
	if got, err := os.ReadFile(storage.TasksFilePath(project.Path)); err != nil || string(got) != string(beforeTasks) {
		t.Fatalf("tasks bytes after failed reset = %q, err=%v; want %q", got, err, beforeTasks)
	}
	if got, err := os.ReadFile(notePath); err != nil || string(got) != string(beforeNote) {
		t.Fatalf("note bytes after failed reset = %q, err=%v; want %q", got, err, beforeNote)
	}
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("LoadProject after failed reset: %v", err)
	}
	assertSandboxResetTUICacheIntact(t, fresh)

	handled, cmd := shell.handleProjectMessages(failed)
	if !handled || cmd == nil {
		t.Fatalf("failure handled=%v cmd=%v, want handled status timeout", handled, cmd)
	}
	if shell.err == nil || !errors.Is(shell.err, injected) {
		t.Fatalf("shell error = %v, want injected error", shell.err)
	}
	assertSandboxResetTUICacheIntact(t, project)
}

type sandboxResetFailRepo struct {
	*scenarioRepo
	err error
}

func (r *sandboxResetFailRepo) Commit(_ ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	return nil, r.err
}

func newSandboxResetTUIFixture(t *testing.T, commitErr error) (*AppModel, *models.Project, string) {
	t.Helper()
	dataDir := t.TempDir()
	project, err := models.NewProject("sandbox-project", "Sandbox", filepath.Join(dataDir, "projects", "sandbox"))
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Sandbox = true
	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Reset me")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}
	task.Tags = []string{"task-tag"}
	project.NextShortID = 2
	project.Tags = []string{"catalog"}
	project.PinnedTags = []string{"catalog"}
	project.SectionColors = map[string]string{models.SectionNext: "blue"}

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	base := newScenarioRepo(commitSvc, project)
	base.seed(t)

	note := models.NewNote("Sandbox note")
	note.Body = "preserve these exact note bytes on failure"
	if saveErr := storage.SaveNote(project, note); saveErr != nil {
		t.Fatalf("SaveNote: %v", saveErr)
	}
	notePath, err := storage.NoteFilePath(project, note.Filename)
	if err != nil {
		t.Fatalf("NoteFilePath: %v", err)
	}

	var repo app.Repository = base
	if commitErr != nil {
		repo = &sandboxResetFailRepo{scenarioRepo: base, err: commitErr}
	}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir), nil)
	shell := newTestAppModel()
	setTestAppProjects(shell, project)
	shell.currentProject = project
	shell.appService = svc
	shell.pendingSandboxReset = true
	shell.pendingSandboxProjectID = project.ID
	shell.pendingSandboxTaskCount = 7
	return shell, project, notePath
}

func assertSandboxResetTUICacheIntact(t *testing.T, project *models.Project) {
	t.Helper()
	if len(project.Tasks) != 1 || project.NextShortID != 2 || len(project.Tags) != 1 || len(project.PinnedTags) != 1 || len(project.SectionColors) != 1 {
		t.Fatalf("sandbox state changed before successful Update reload: %+v", project)
	}
}

func assertSandboxResetTUIDiskCleared(t *testing.T, project *models.Project, notePath string) {
	t.Helper()
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("LoadProject after reset: %v", err)
	}
	if len(fresh.Tasks) != 0 || fresh.NextShortID != 1 || len(fresh.Tags) != 0 || len(fresh.PinnedTags) != 0 || len(fresh.SectionColors) != 0 {
		t.Fatalf("committed sandbox was not cleared: %+v", fresh)
	}
	if _, err := os.Stat(notePath); !os.IsNotExist(err) {
		t.Fatalf("sandbox note exists after reset, stat error=%v", err)
	}
}
