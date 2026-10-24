package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestFingerprintTracksTasksAndDomains pins the signal a focus reload compares:
// stable while nothing on disk moves, and different once a project's tasks.md
// appears or a `.tt` domain file is touched.
func TestFingerprintTracksTasksAndDomains(t *testing.T) {
	dataDir := t.TempDir()
	domains := filepath.Join(dataDir, ".tt")
	if err := os.MkdirAll(domains, 0o755); err != nil {
		t.Fatal(err)
	}
	catalog := filepath.Join(domains, "workspace.json")
	if err := os.WriteFile(catalog, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := &models.Project{ID: "alpha", Path: filepath.Join(dataDir, "projects", "alpha")}
	projects := []*models.Project{project, nil}

	if Fingerprint("", projects) != "" {
		t.Fatal("an empty data dir produced a fingerprint")
	}
	first := Fingerprint(dataDir, projects)
	if Fingerprint(dataDir, projects) != first {
		t.Fatal("the fingerprint moved with nothing on disk changing")
	}

	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storage.TasksFilePath(project.Path), []byte("# Alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterTasks := Fingerprint(dataDir, projects)
	if afterTasks == first {
		t.Fatal("writing tasks.md left the fingerprint unchanged")
	}

	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(catalog, later, later); err != nil {
		t.Fatal(err)
	}
	if Fingerprint(dataDir, projects) == afterTasks {
		t.Fatal("touching a domain file left the fingerprint unchanged")
	}
}

func TestFingerprintTracksImportedNotesThroughSyncState(t *testing.T) {
	dataDir := t.TempDir()
	before := Fingerprint(dataDir, nil)
	state := filepath.Join(dataDir, ".tt", "sync", "devices", "desktop", "state.json")
	if err := os.MkdirAll(filepath.Dir(state), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, []byte(`{"received":{"phone":1}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	firstImport := Fingerprint(dataDir, nil)
	if firstImport == before {
		t.Fatal("first import left the fingerprint unchanged")
	}
	later := time.Now().Add(time.Second)
	if err := os.Chtimes(state, later, later); err != nil {
		t.Fatal(err)
	}
	if Fingerprint(dataDir, nil) == firstImport {
		t.Fatal("later import left the fingerprint unchanged")
	}
}

// TestChangedElsewhereIgnoresOwnCommits pins the polling rule: what a commit
// recorded in the registry wrote is not a change made elsewhere, while a commit
// by a service sharing no registry, and a write after the recorded one, are.
func TestChangedElsewhereIgnoresOwnCommits(t *testing.T) {
	dataDir := t.TempDir()
	project := &models.Project{ID: "alpha", Path: filepath.Join(dataDir, "projects", "alpha")}
	projects := []*models.Project{project}
	tasks := storage.TasksFilePath(project.Path)
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tasks, []byte("# Alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	own := storage.NewOwnWrites()
	transactions := filepath.Join(dataDir, ".tt", "transactions")
	svc := storage.NewCommitService(transactions, []string{dataDir}).WithOwnWrites(own)

	if (Stamps(nil)).ChangedElsewhere(nil, own) {
		t.Fatal("no stamps changed against no stamps")
	}
	baseline := StampWorkspace(dataDir, projects)
	if baseline.ChangedElsewhere(StampWorkspace(dataDir, projects), own) {
		t.Fatal("an untouched workspace changed elsewhere")
	}

	if _, err := svc.CommitArtifacts([]storage.ArtifactChange{
		{Path: filepath.Join(dataDir, ".tt", "workspace.json"), Data: []byte("{}")},
		{Path: tasks, Data: []byte("# Alpha\n\n## Task\n")},
	}); err != nil {
		t.Fatal(err)
	}
	committed := StampWorkspace(dataDir, projects)
	if committed.Fingerprint() == baseline.Fingerprint() {
		t.Fatal("the commit left the fingerprint unchanged")
	}
	if baseline.ChangedElsewhere(committed, own) {
		t.Fatal("a recorded commit read as a change made elsewhere")
	}
	if !baseline.ChangedElsewhere(committed, nil) {
		t.Fatal("with no registry, the commit did not read as a change made elsewhere")
	}

	unrelated := storage.NewCommitService(transactions, []string{dataDir})
	if _, err := unrelated.CommitArtifacts([]storage.ArtifactChange{
		{Path: filepath.Join(dataDir, ".tt", "planning.json"), Data: []byte("{}")},
	}); err != nil {
		t.Fatal(err)
	}
	if !committed.ChangedElsewhere(StampWorkspace(dataDir, projects), own) {
		t.Fatal("a commit by a service sharing no registry did not read as a change made elsewhere")
	}

	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(tasks, later, later); err != nil {
		t.Fatal(err)
	}
	if !baseline.ChangedElsewhere(StampWorkspace(dataDir, projects), own) {
		t.Fatal("a write after the commit did not read as a change made elsewhere")
	}
}
