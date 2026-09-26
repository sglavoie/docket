package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// TestConflictListJSONIsAnEmptyArrayWithoutConflicts keeps a clean workspace
// iterable by a script: no conflicts is [], never null.
func TestConflictListJSONIsAnEmptyArrayWithoutConflicts(t *testing.T) {
	setupConflictTest(t)
	conflictListJSON = true
	t.Cleanup(func() { conflictListJSON = false })

	stdout, _, err := captureDoctorOutput(t, func() error { return runConflictList(conflictListCmd, nil) })
	if err != nil {
		t.Fatalf("runConflictList --json: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != "[]" {
		t.Fatalf("conflict list --json with no conflicts = %q, want []", got)
	}
}

// TestConflictShowUnknownIDNamesTheListCommand points an unmatched id at the
// command that prints the real ones.
func TestConflictShowUnknownIDNamesTheListCommand(t *testing.T) {
	setupConflictTest(t)

	err := runConflictShow(conflictShowCmd, []string{"ffffffff"})
	if err == nil {
		t.Fatal("expected an error for an unknown conflict id")
	}
	for _, want := range []string{`no conflict matches "ffffffff"`, "run 'tt conflict list' to see IDs"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
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

// retireCLIBundleDocuments rewrites a recorded bundle's documents as schema-2
// markdown, the state an old bundle reaches on its own: nothing migrates
// `.tt/conflicts/` when the schema advances.
func retireCLIBundleDocuments(t *testing.T, store *storage.ConflictStore, id string) {
	t.Helper()
	retired := "# Alpha\n\n<!-- tt-schema: 2 -->\n\n## Task: base title\n\n- state: backlog\n"
	for _, name := range []string{"base.md", "local.md", "remote.md"} {
		if err := os.WriteFile(filepath.Join(store.Dir(), id, name), []byte(retired), 0o600); err != nil {
			t.Fatalf("rewriting %s: %v", name, err)
		}
	}
}

// TestConflictShowUnreadableDocuments verifies show still renders a bundle whose
// documents outlived their schema — the command previously died before printing
// anything, hiding the record it was the only way to inspect.
func TestConflictShowUnreadableDocuments(t *testing.T) {
	store, project := setupConflictTest(t)
	id := recordCLIConflict(t, store, project)
	retireCLIBundleDocuments(t, store, id)

	if err := runConflictShow(conflictShowCmd, []string{shortConflictID(id)}); err != nil {
		t.Fatalf("runConflictShow on an unreadable bundle: %v", err)
	}
}

// TestConflictResolveUnreadableLocalGuidance verifies the CLI refusal names the
// cause and points at the resolutions that still work.
func TestConflictResolveUnreadableLocalGuidance(t *testing.T) {
	store, project := setupConflictTest(t)
	id := recordCLIConflict(t, store, project)
	retireCLIBundleDocuments(t, store, id)

	conflictResolveUse = "local"
	conflictResolveFile = ""
	t.Cleanup(func() { conflictResolveUse = ""; conflictResolveFile = "" })

	err := runConflictResolve(conflictResolveCmd, []string{id})
	if !errors.Is(err, storage.ErrConflictDocumentUnreadable) {
		t.Fatalf("err = %v, want ErrConflictDocumentUnreadable", err)
	}
	for _, want := range []string{"schema 2", "--use remote", "--use file"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q missing %q", err.Error(), want)
		}
	}
	if records, _ := store.List(); len(records) != 1 {
		t.Fatalf("refused resolution must leave the bundle intact, %d remain", len(records))
	}
}

// TestConflictResolveUnreadableUseRemote verifies the resolution that reads no
// stored document clears an ancient bundle, which is what unstrands the row the
// status strip draws from it.
func TestConflictResolveUnreadableUseRemote(t *testing.T) {
	store, project := setupConflictTest(t)
	id := recordCLIConflict(t, store, project)
	retireCLIBundleDocuments(t, store, id)

	conflictResolveUse = "remote"
	conflictResolveFile = ""
	t.Cleanup(func() { conflictResolveUse = ""; conflictResolveFile = "" })

	if err := runConflictResolve(conflictResolveCmd, []string{id}); err != nil {
		t.Fatalf("runConflictResolve(remote): %v", err)
	}
	doc, err := storage.LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if doc.Tasks[0].Title != "remote title" {
		t.Fatalf("on-disk title = %q, want the untouched remote title", doc.Tasks[0].Title)
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

// TestConflictResolveRefusesFileWithoutUseFile keeps --file from being silently
// ignored beside another strategy, and leaves the conflict recorded.
func TestConflictResolveRefusesFileWithoutUseFile(t *testing.T) {
	store, project := setupConflictTest(t)
	id := recordCLIConflict(t, store, project)

	for _, use := range []string{"local", "remote"} {
		conflictResolveUse = use
		conflictResolveFile = "merged.md"
		t.Cleanup(func() { conflictResolveUse = ""; conflictResolveFile = "" })

		err := runConflictResolve(conflictResolveCmd, []string{id})
		if err == nil || err.Error() != "--file only applies with --use file" {
			t.Fatalf("--use %s --file error = %v, want the --file refusal", use, err)
		}
	}
	if _, err := store.Load(id); err != nil {
		t.Fatalf("a refused resolution removed the conflict: %v", err)
	}
}
