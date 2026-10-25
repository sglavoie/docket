package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/storage"
)

// TestAddTaskAtomicArtifactUndoRedoAcceptance exercises the whole wired add path
// through the real transactional commit service and workspace undo ledger: an
// application-layer AddTask commits the new task AND an auxiliary durable artifact
// (standing in for the config store carrying a bumped template use-count) in ONE
// transaction, a workspace-ledger undo reverts BOTH the task and the artifact, and
// redo re-applies BOTH. This proves the task-add side effect is atomic with the
// task write and fully reversible through the same ledger the CLI adapter drives.
func TestAddTaskAtomicArtifactUndoRedoAcceptance(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "seed task")

	// A config-like artifact living inside the workspace root, so the commit
	// service accepts it and the ledger captures its before/after image.
	configPath := filepath.Join(dataDir, ".tt.json")
	if err := os.WriteFile(configPath, []byte(`{"use_count":0}`), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)

	appSvc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	res, err := appSvc.AddTask(app.AddTaskRequest{
		ProjectID: project.ID,
		Title:     "added via app",
		Artifacts: []storage.ArtifactChange{{Path: configPath, Data: []byte(`{"use_count":1}`)}},
	})
	if err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	// The task landed and the artifact was written in the same transaction.
	assertTaskTitlePresent(t, project.Path, "added via app", true)
	if got := readFileContent(t, configPath); got != `{"use_count":1}` {
		t.Fatalf("config artifact = %q, want bumped use_count", got)
	}
	if res.TaskID == "" {
		t.Fatal("expected a task id in the result")
	}

	// Undo reverts BOTH the task markdown and the config artifact atomically.
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	assertTaskTitlePresent(t, project.Path, "added via app", false)
	if got := readFileContent(t, configPath); got != `{"use_count":0}` {
		t.Fatalf("after undo config = %q, want restored use_count", got)
	}

	// Redo re-applies BOTH.
	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	assertTaskTitlePresent(t, project.Path, "added via app", true)
	if got := readFileContent(t, configPath); got != `{"use_count":1}` {
		t.Fatalf("after redo config = %q, want bumped use_count", got)
	}
}

// assertTaskTitlePresent parses a project's on-disk tasks.md and asserts whether a
// task with the given title is present.
func assertTaskTitlePresent(t *testing.T, projectPath, title string, want bool) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(projectPath, "tasks.md"))
	if err != nil {
		t.Fatalf("read tasks.md: %v", err)
	}
	doc, err := storage.ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse tasks.md: %v", err)
	}
	found := false
	for _, task := range doc.Tasks {
		if task.Title == title {
			found = true
		}
	}
	if found != want {
		t.Fatalf("task %q present=%v, want %v", title, found, want)
	}
}
