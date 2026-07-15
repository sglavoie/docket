package tui

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestConcurrentCLICommitAbortsStartupRepair(t *testing.T) {
	dataDir := t.TempDir()
	project := testutil.NewTestProject("TUI Startup Repair")
	project.Path = filepath.Join(dataDir, "projects", project.Slug)
	if err := os.MkdirAll(project.Path, storage.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	legacyTaskSectionField := "custom" + "_section"
	legacy := []byte("# Project: TUI Startup Repair\n" +
		"<!-- sections: Next;Sprint-1 -->\n\n" +
		"## Task: Loaded before CLI mutation\n" +
		"<!-- id: task-1, status: todo, priority: 1, section: Next, " + legacyTaskSectionField + ": Sprint-1 -->\n")
	if err := os.WriteFile(storage.GetTasksFilePath(project), legacy, storage.FilePermissions); err != nil {
		t.Fatalf("WriteFile(tasks.md): %v", err)
	}

	persister := NewProjectService()
	if err := persister.Load(project); err != nil {
		t.Fatalf("startup Load: %v", err)
	}
	if len(project.Warnings) == 0 {
		t.Fatal("startup load did not produce the expected repair warning")
	}

	// Advance the project through the same checked CommitService boundary used by
	// production CLI mutations after the TUI load but before its repair attempt.
	cliProject, err := models.NewProject(project.ID, project.Name, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	cliProject.Slug = project.Slug
	cliProject.Path = project.Path
	if err := storage.LoadProject(cliProject); err != nil {
		t.Fatalf("CLI LoadProject: %v", err)
	}
	base, err := storage.LoadProjectDocument(cliProject)
	if err != nil {
		t.Fatalf("CLI LoadProjectDocument: %v", err)
	}
	cliProject.Tasks[0].Title = "Committed by CLI"
	svc := storage.NewCommitService(config.TransactionsDir(dataDir), []string{dataDir})
	if _, err := svc.Commit(storage.ChangeSet{Project: cliProject, ExpectedRevision: base.Revision, Base: base}); err != nil {
		t.Fatalf("CLI Commit: %v", err)
	}
	cliBytes, err := os.ReadFile(storage.GetTasksFilePath(project))
	if err != nil {
		t.Fatalf("ReadFile(after CLI commit): %v", err)
	}

	model := &AppModel{
		runtime:        config.EmptyRuntimeState(dataDir),
		persister:      persister,
		currentProject: project,
	}
	err = persistStartupProjectRepair(model, project)
	if !errors.Is(err, storage.ErrStaleArtifact) {
		t.Fatalf("persistStartupProjectRepair error = %v, want ErrStaleArtifact", err)
	}
	after, err := os.ReadFile(storage.GetTasksFilePath(project))
	if err != nil {
		t.Fatalf("ReadFile(after startup repair): %v", err)
	}
	if !bytes.Equal(after, cliBytes) {
		t.Fatalf("startup repair overwrote CLI mutation\n got: %q\nwant: %q", after, cliBytes)
	}
}
