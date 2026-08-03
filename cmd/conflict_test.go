package cmd

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// setupConflictTest wires a workspace so the conflict subcommands resolve their
// store and commit service against a real on-disk data dir, and returns the
// store, the project on disk, and its data dir.
//
//nolint:govet // The test-local data directory is intentionally independent of the command flag.
func setupConflictTest(t *testing.T) (*storage.ConflictStore, *models.Project) {
	t.Helper()
	deps := setupTestDeps(t)
	manager := config.NewManager(deps.ConfigPath)
	if err := saveTestRuntime(manager, deps.Config); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dataDir := config.WorkspaceRootFromConfigPath(deps.ConfigPath)
	setCommandDataDir(t, dataDir)
	store := storage.NewConflictStore(config.ConflictsDir(dataDir))
	svc := storage.NewCommitService(config.TransactionsDir(dataDir), []string{dataDir})

	project := &models.Project{ID: "p1", Name: "Alpha", Slug: "alpha", Path: filepath.Join(dataDir, "projects", "alpha")}
	task, err := models.NewGenericTask(uuid.NewString(), 1, "remote title")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	project.BuildTaskTree()
	if _, err := svc.Commit(storage.ChangeSet{Project: project}); err != nil {
		t.Fatalf("commit remote: %v", err)
	}
	return store, project
}

// editDocTitle copies a document with its first task's title changed and its
// revision recomputed.
func editDocTitle(doc *storage.ProjectDocument, title string) *storage.ProjectDocument {
	copied := *doc
	tasks := make([]*models.Task, len(doc.Tasks))
	for i, tk := range doc.Tasks {
		c := *tk
		tasks[i] = &c
	}
	copied.Tasks = tasks
	copied.Tasks[0].Title = title
	copied.Revision = copied.ComputeRevision()
	return &copied
}

func recordCLIConflict(t *testing.T, store *storage.ConflictStore, project *models.Project) string {
	t.Helper()
	remote, err := storage.LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("load remote doc: %v", err)
	}
	base := editDocTitle(remote, "base title")
	local := editDocTitle(remote, "local title")
	id, err := storage.RecordMergeConflict(store, project, base, local, remote,
		[]storage.MergeConflict{{Kind: storage.MergeConflictFieldEdit, Field: "title", Base: "base title", Local: "local title", Current: "remote title"}},
		"task edit", "editing title")
	if err != nil {
		t.Fatalf("record conflict: %v", err)
	}
	return id
}

// TestConflictListAndShow verifies the list and show commands run against a
// recorded conflict without error and that a short id selects it.
func TestConflictListAndShow(t *testing.T) {
	store, project := setupConflictTest(t)
	id := recordCLIConflict(t, store, project)

	if err := runConflictList(conflictListCmd, nil); err != nil {
		t.Fatalf("runConflictList: %v", err)
	}
	if err := runConflictShow(conflictShowCmd, []string{shortConflictID(id)}); err != nil {
		t.Fatalf("runConflictShow with short id: %v", err)
	}
}

// TestConflictResolveUseLocal verifies the resolve command commits the local
// edits and clears the conflict.
func TestConflictResolveUseLocal(t *testing.T) {
	store, project := setupConflictTest(t)
	id := recordCLIConflict(t, store, project)

	conflictResolveUse = "local"
	conflictResolveFile = ""
	t.Cleanup(func() { conflictResolveUse = ""; conflictResolveFile = "" })

	if err := runConflictResolve(conflictResolveCmd, []string{id}); err != nil {
		t.Fatalf("runConflictResolve: %v", err)
	}

	doc, err := storage.LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if doc.Tasks[0].Title != "local title" {
		t.Fatalf("on-disk title = %q, want local title", doc.Tasks[0].Title)
	}
	if records, _ := store.List(); len(records) != 0 {
		t.Fatalf("expected the conflict to be cleared, %d remain", len(records))
	}
}

// TestConflictResolveRequiresUse verifies an unset or invalid --use is rejected.
func TestConflictResolveRequiresUse(t *testing.T) {
	store, project := setupConflictTest(t)
	id := recordCLIConflict(t, store, project)

	conflictResolveUse = ""
	conflictResolveFile = ""
	t.Cleanup(func() { conflictResolveUse = ""; conflictResolveFile = "" })

	if err := runConflictResolve(conflictResolveCmd, []string{id}); err == nil {
		t.Fatal("expected an error when --use is missing")
	}

	conflictResolveUse = "bogus"
	if err := runConflictResolve(conflictResolveCmd, []string{id}); err == nil {
		t.Fatal("expected an error for an unknown --use value")
	}
}
