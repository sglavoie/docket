package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/storage"
)

// seedProjectContent fills a seeded project directory with the three
// populations one delete has to take together: task Markdown, a note, and an
// attachment. Each returns its path and bytes so the undo assertion can compare
// them without re-deriving the layout.
func seedProjectContent(t *testing.T, projectPath string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{
		filepath.Join(projectPath, "tasks.md"):                       []byte("# Alpha\n\n## Task\n"),
		filepath.Join(projectPath, "notes", "design.md"):             []byte("# Design\n\nnote body\n"),
		filepath.Join(projectPath, ".attachments", "spec-draft.txt"): []byte("attached bytes\n"),
	}
	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return files
}

func deleteLedgerRepo(t *testing.T, dataDir string) (*journalOpsRepo, *storage.WorkspaceLedger) {
	t.Helper()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	return &journalOpsRepo{
		fakeRepo: newFakeRepo(),
		commit: storage.NewCommitServiceWithLedger(
			filepath.Join(dataDir, ".tt", "transactions"),
			[]string{dataDir},
			ledger,
		),
	}, ledger
}

// TestDeleteProjectRefusesTheSystemUnfiledProject proves the protection is an
// application-layer precondition rather than a TUI courtesy: the refusal lands
// before any op is planned, so no caller can route around it.
func TestDeleteProjectRefusesTheSystemUnfiledProject(t *testing.T) {
	dataDir := t.TempDir()
	project := seedProjectDir(t, dataDir, "unfiled", "Unfiled")
	repo := &opsRepo{fakeRepo: newFakeRepo()}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	_, err := svc.DeleteProjectResult(app.DeleteProjectRequest{
		Project:                project,
		SystemUnfiledProjectID: project.ID,
		DataDir:                dataDir,
	})
	if !errors.Is(err, app.ErrProtectedSystemUnfiled) {
		t.Fatalf("DeleteProjectResult(system Unfiled) error = %v, want ErrProtectedSystemUnfiled", err)
	}
	if len(repo.committed) != 0 {
		t.Fatalf("protected delete reached persistence: %#v", repo.committed)
	}
	if _, statErr := os.Stat(project.Path); statErr != nil {
		t.Fatalf("protected project directory changed: %v", statErr)
	}
}

// TestDeleteProjectRemovesEverythingInOneTransaction drives the production
// commit boundary. Tasks, notes, and attachments leave together with the
// catalog and planning rewrites, and the whole removal records exactly one
// workspace ledger entry — the unit `u` later undoes.
func TestDeleteProjectRemovesEverythingInOneTransaction(t *testing.T) {
	dataDir := t.TempDir()
	project := seedProjectDir(t, dataDir, "alpha", "Alpha")
	seedProjectContent(t, project.Path)
	repo, ledger := deleteLedgerRepo(t, dataDir)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	catalog := filepath.Join(dataDir, ".tt", "workspace.json")
	planning := filepath.Join(dataDir, ".tt", "planning.json")
	transactionID, err := svc.DeleteProjectResult(app.DeleteProjectRequest{
		Project: project,
		DataDir: dataDir,
		Config: []storage.ArtifactChange{
			{Path: catalog, Data: []byte(`{"projects":[]}`)},
			{Path: planning, Data: []byte(`{"today":[],"next":[]}`)},
		},
	})
	if err != nil {
		t.Fatalf("DeleteProjectResult: %v", err)
	}
	if transactionID == "" {
		t.Fatal("DeleteProjectResult returned an empty transaction ID through a ledger-aware repository")
	}
	if _, statErr := os.Stat(project.Path); !os.IsNotExist(statErr) {
		t.Fatalf("project directory survives the delete, stat err = %v", statErr)
	}
	// The catalog and planning rewrites are part of the same commit, so a
	// project can never be gone from disk while still named in either file.
	if got, readErr := os.ReadFile(catalog); readErr != nil || string(got) != `{"projects":[]}` {
		t.Fatalf("catalog = %q, %v", got, readErr)
	}
	if got, readErr := os.ReadFile(planning); readErr != nil || string(got) != `{"today":[],"next":[]}` {
		t.Fatalf("planning = %q, %v", got, readErr)
	}

	count, countErr := ledger.UndoCount(storage.GlobalScope())
	if countErr != nil {
		t.Fatalf("UndoCount: %v", countErr)
	}
	if count != 1 {
		t.Fatalf("undo entries = %d, want exactly one for a whole-project delete", count)
	}
	entry, ok, selectErr := ledger.SelectNewestUndo(storage.GlobalScope())
	if selectErr != nil {
		t.Fatalf("SelectNewestUndo: %v", selectErr)
	}
	if !ok {
		t.Fatal("delete recorded no workspace ledger entry")
	}
	if entry.TransactionID != transactionID {
		t.Fatalf("newest ledger transaction = %q, want returned %q", entry.TransactionID, transactionID)
	}
	if entry.Action != "Delete Project" || entry.Context != "Alpha" {
		t.Fatalf("ledger entry = %q / %q, want %q / %q", entry.Action, entry.Context, "Delete Project", "Alpha")
	}
	if len(entry.ProjectIDs) != 1 || entry.ProjectIDs[0] != project.ID {
		t.Fatalf("ledger project ids = %v, want [%s]", entry.ProjectIDs, project.ID)
	}
}

// TestDeleteProjectUndoRestoresTasksNotesAndAttachments is the reason the
// delete stays one transaction: undo returns every byte the cascade took,
// including the notes and attachments no task record names.
func TestDeleteProjectUndoRestoresTasksNotesAndAttachments(t *testing.T) {
	dataDir := t.TempDir()
	project := seedProjectDir(t, dataDir, "alpha", "Alpha")
	files := seedProjectContent(t, project.Path)
	repo, ledger := deleteLedgerRepo(t, dataDir)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	catalog := filepath.Join(dataDir, ".tt", "workspace.json")
	if err := os.MkdirAll(filepath.Dir(catalog), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(catalog, []byte(`{"projects":["alpha"]}`), 0o600); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
	if _, err := svc.DeleteProjectResult(app.DeleteProjectRequest{
		Project: project,
		DataDir: dataDir,
		Config:  []storage.ArtifactChange{{Path: catalog, Data: []byte(`{"projects":[]}`)}},
	}); err != nil {
		t.Fatalf("DeleteProjectResult: %v", err)
	}

	if _, statErr := os.Stat(project.Path); !os.IsNotExist(statErr) {
		t.Fatalf("project directory survives the delete, stat err = %v", statErr)
	}
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	for path, want := range files {
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("undo did not restore %s: %v", path, readErr)
			continue
		}
		if string(got) != string(want) {
			t.Errorf("restored %s = %q, want %q", path, got, want)
		}
	}
	if got, readErr := os.ReadFile(catalog); readErr != nil || string(got) != `{"projects":["alpha"]}` {
		t.Errorf("undo left catalog = %q, %v; want the pre-delete bytes", got, readErr)
	}
}
