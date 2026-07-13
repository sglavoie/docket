package tui

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// newInboxCommitEnv seeds a single project with one processable inbox task on
// disk under a shared data-dir root, wiring the config the inbox processor uses
// to commit triage mutations through the app-layer transactional repository.
func newInboxCommitEnv(t *testing.T, dataDir string) (*config.Config, *config.Manager, *models.Project) {
	t.Helper()
	project, err := models.NewProject(uuid.New().String(), "inbox", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Name = "Inbox Project"
	project.Slug = "inbox"
	project.Path = filepath.Join(dataDir, "projects", "inbox")

	task, err := models.NewTask("inbox-1", 1, models.TaskTypeTask, "Triage me")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.SetSection(models.SectionInbox)
	task.Priority = 3
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.EnsureDefaultSections()
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	cfg := config.NewConfig()
	cfg.DataDir = dataDir
	cfg.Projects = []*models.Project{project}
	cfg.CurrentProject = project.ID
	mgr := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	return cfg, mgr, project
}

func reloadInboxProject(t *testing.T, p *models.Project) *models.Project {
	t.Helper()
	fresh := &models.Project{ID: p.ID, Name: p.Name, Slug: p.Slug, Path: p.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload: %v", err)
	}
	return fresh
}

// TestInboxProcessorCommitsTriageThroughTransaction verifies a single inbox
// triage mutation (priority change) commits through the app-layer transactional
// repository — not the retired saveProjectSync raw save — landing on disk and
// recording a workspace-ledger entry that undoes the change.
func TestInboxProcessorCommitsTriageThroughTransaction(t *testing.T) {
	dataDir := t.TempDir()
	cfg, mgr, project := newInboxCommitEnv(t, dataDir)

	m := NewInboxProcessorModel(project, cfg, mgr)
	cmd := m.changePriority(5)
	if cmd == nil {
		t.Fatal("changePriority should return a save command")
	}
	msg := cmd()
	saved, ok := msg.(InboxProjectSavedMsg)
	if !ok {
		t.Fatalf("expected InboxProjectSavedMsg, got %T", msg)
	}
	if saved.Error != nil {
		t.Fatalf("inbox commit failed: %v", saved.Error)
	}

	// The mutation landed on disk through the transactional commit.
	if got := reloadInboxProject(t, project).GetTaskByID("inbox-1"); got == nil || got.Priority != 5 {
		t.Fatalf("disk priority = %v, want 5", got)
	}

	// A workspace-ledger entry was recorded and undoes the triage change.
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), 0, []string{dataDir})
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	if got := reloadInboxProject(t, project).GetTaskByID("inbox-1"); got == nil || got.Priority != 3 {
		t.Fatalf("priority after undo = %v, want original 3", got)
	}
}

// TestInboxProcessorCommitFailureLeavesDiskUnchanged verifies a failed inbox
// commit (the project's tasks.md lives outside the approved data-dir root, which
// the transaction journal rejects) surfaces an error and leaves the on-disk task
// untouched — no partial mutation.
func TestInboxProcessorCommitFailureLeavesDiskUnchanged(t *testing.T) {
	dataDir := t.TempDir()
	outsideDir := t.TempDir()
	cfg, mgr, project := newInboxCommitEnv(t, outsideDir)
	// Approved root is dataDir, but the project's tasks.md lives under outsideDir,
	// so the transactional commit is rejected for escaping the root.
	cfg.DataDir = dataDir

	m := NewInboxProcessorModel(project, cfg, mgr)
	cmd := m.changePriority(5)
	if cmd == nil {
		t.Fatal("changePriority should return a save command")
	}
	saved, ok := cmd().(InboxProjectSavedMsg)
	if !ok {
		t.Fatalf("expected InboxProjectSavedMsg")
	}
	if saved.Error == nil {
		t.Fatal("expected a commit failure for the out-of-root project")
	}

	if got := reloadInboxProject(t, project).GetTaskByID("inbox-1"); got == nil || got.Priority != 3 {
		t.Fatalf("disk priority = %v, want unchanged 3 after failed commit", got)
	}
}
