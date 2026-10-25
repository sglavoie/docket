package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

// opsRepo wraps fakeRepo and applies planned lifecycle operations directly on a
// temporary filesystem. An injected commitErr simulates transaction failure.
type opsRepo struct {
	*fakeRepo
	commitErr          error
	committed          [][]transaction.Op
	committedArtifacts [][]storage.ArtifactChange
}

// journalOpsRepo keeps the app test snapshot in memory while delegating the
// lifecycle persistence boundary to the production CommitService and journal.
type journalOpsRepo struct {
	*fakeRepo
	commit *storage.CommitService
}

func (r *journalOpsRepo) CommitOps(ops []transaction.Op, artifacts []storage.ArtifactChange) error {
	return r.commit.CommitOpsWithArtifacts(ops, artifacts)
}

func (r *journalOpsRepo) CommitOpsTransaction(request storage.OpsCommitRequest) (string, error) {
	return r.commit.CommitOpsTransaction(request)
}

func (r *opsRepo) CommitOps(ops []transaction.Op, artifacts []storage.ArtifactChange) error {
	if r.commitErr != nil {
		return r.commitErr
	}
	r.committed = append(r.committed, ops)
	r.committedArtifacts = append(r.committedArtifacts, artifacts)
	for _, op := range ops {
		if err := applyLifecycleOp(op); err != nil {
			return err
		}
	}
	for _, artifact := range artifacts {
		op := transaction.Op{Kind: transaction.OpWrite, Target: artifact.Path, Data: artifact.Data, Mode: artifact.Mode}
		if artifact.Delete {
			op.Kind = transaction.OpDelete
		}
		if err := applyLifecycleOp(op); err != nil {
			return err
		}
	}
	return nil
}

func applyLifecycleOp(op transaction.Op) error {
	switch op.Kind {
	case transaction.OpWrite:
		if err := os.MkdirAll(filepath.Dir(op.Target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(op.Target, op.Data, 0o600)
	case transaction.OpMove:
		if err := os.MkdirAll(filepath.Dir(op.Target), 0o755); err != nil {
			return err
		}
		return os.Rename(op.Source, op.Target)
	case transaction.OpDelete:
		if err := os.Remove(op.Target); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return nil
}

// seedProjectDir creates a project physically on disk under <dataDir>/projects/<slug>.
func seedProjectDir(t *testing.T, dataDir, slug, name string) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	p.Slug = slug
	p.Name = name
	p.Path = filepath.Join(dataDir, "projects", slug)
	if err := os.MkdirAll(p.Path, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, "tasks.md"), []byte("# "+name+"\n"), 0o600); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}
	return p
}

func configArtifact(dataDir string, data string) storage.ArtifactChange {
	return storage.ArtifactChange{Path: filepath.Join(dataDir, "config.json"), Data: []byte(data)}
}

func tasksExpectedRevision(t *testing.T, project *models.Project) *string {
	t.Helper()
	data, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read tasks revision: %v", err)
	}
	revision := storage.RawArtifactRevision(data)
	return &revision
}

func TestProtectedSystemInboxRenameRejectedBeforeCommit(t *testing.T) {
	dataDir := t.TempDir()
	project := seedProjectDir(t, dataDir, "inbox", "Inbox")
	repo := &opsRepo{fakeRepo: newFakeRepo()}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	_, renameErr := svc.RenameProjectResult(app.RenameProjectRequest{
		Project:                project,
		SystemUnfiledProjectID: project.ID,
		DataDir:                dataDir,
		OldSlug:                "inbox",
		NewSlug:                "renamed",
	})
	if !errors.Is(renameErr, app.ErrProtectedSystemUnfiled) {
		t.Fatalf("RenameProject(system Inbox) error = %v, want ErrProtectedSystemUnfiled", renameErr)
	}
	if len(repo.committed) != 0 {
		t.Fatalf("protected lifecycle request reached persistence: %#v", repo.committed)
	}
	if _, err := os.Stat(project.Path); err != nil {
		t.Fatalf("protected project directory changed: %v", err)
	}
}

// TestRenameProjectCommitsAtomically proves a slug change moves the project
// directory, rewrites its tasks.md at the new path, and applies the config
// rewrite — all in ONE committed op set — and that the use case reports and
// records the new path.
func TestRenameProjectCommitsAtomically(t *testing.T) {
	dataDir := t.TempDir()
	project := seedProjectDir(t, dataDir, "alpha", "Alpha")
	repo := &opsRepo{fakeRepo: newFakeRepo()}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	// Rename Alpha -> Beta in memory, then commit through the use case.
	project.Name = "Beta"
	project.Slug = "beta"
	newTasks := []byte("# Beta\n<!-- renamed -->\n")
	expected := tasksExpectedRevision(t, project)
	_, err := svc.RenameProjectResult(app.RenameProjectRequest{
		Project:               project,
		DataDir:               dataDir,
		OldSlug:               "alpha",
		NewSlug:               "beta",
		TasksMarkdown:         newTasks,
		TasksExpectedRevision: expected,
		Config:                []storage.ArtifactChange{configArtifact(dataDir, `{"slug":"beta"}`)},
	})
	if err != nil {
		t.Fatalf("RenameProject: %v", err)
	}
	newPath := filepath.Join(dataDir, "projects", "beta")
	if project.Path != newPath {
		t.Errorf("project.Path = %q, want %q", project.Path, newPath)
	}
	if len(repo.committed) != 1 {
		t.Fatalf("expected exactly one atomic commit, got %d", len(repo.committed))
	}
	if len(repo.committedArtifacts) != 1 || len(repo.committedArtifacts[0]) != 2 {
		t.Fatalf("expected tasks and config to remain typed artifacts, got %#v", repo.committedArtifacts)
	}
	if repo.committedArtifacts[0][0].ExpectedRevision == nil {
		t.Fatal("expected renamed tasks artifact to retain its revision precondition")
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, "projects", "alpha")); !os.IsNotExist(statErr) {
		t.Errorf("old project dir should be gone, stat err = %v", statErr)
	}
	data, readErr := os.ReadFile(filepath.Join(newPath, "tasks.md"))
	if readErr != nil {
		t.Fatalf("read renamed tasks.md: %v", readErr)
	}
	if string(data) != string(newTasks) {
		t.Errorf("renamed tasks.md = %q, want %q", data, newTasks)
	}
	if got, _ := os.ReadFile(filepath.Join(dataDir, "config.json")); string(got) != `{"slug":"beta"}` {
		t.Errorf("config after rename = %q", got)
	}
}

func TestRenameProjectRejectsUncheckedTasksRewrite(t *testing.T) {
	dataDir := t.TempDir()
	project := seedProjectDir(t, dataDir, "alpha", "Alpha")
	repo := &opsRepo{fakeRepo: newFakeRepo()}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	_, err := svc.RenameProjectResult(app.RenameProjectRequest{
		Project:       project,
		DataDir:       dataDir,
		OldSlug:       "alpha",
		NewSlug:       "beta",
		TasksMarkdown: []byte("# Beta\n"),
	})
	if err == nil || !strings.Contains(err.Error(), "expected revision") {
		t.Fatalf("RenameProject unchecked tasks err = %v, want expected-revision error", err)
	}
	if len(repo.committed) != 0 {
		t.Fatalf("unchecked rename reached persistence: %#v", repo.committed)
	}
}

// TestRenameProjectCommitsThroughRealJournal supplements the operation-recording
// test with the production CommitService. It regresses the failure where Prepare
// staged tasks.md inside beta/ before trying to rename alpha/ onto that occupied
// destination.
func TestRenameProjectCommitsThroughRealJournal(t *testing.T) {
	dataDir := t.TempDir()
	project := seedProjectDir(t, dataDir, "alpha", "Alpha")
	repo := &journalOpsRepo{
		fakeRepo: newFakeRepo(),
		commit: storage.NewCommitService(
			filepath.Join(dataDir, ".tt", "transactions"),
			[]string{dataDir},
		),
	}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	project.Name = "Beta"
	project.Slug = "beta"
	newTasks := []byte("# Beta\n<!-- real journal -->\n")
	expected := tasksExpectedRevision(t, project)
	_, err := svc.RenameProjectResult(app.RenameProjectRequest{
		Project:               project,
		DataDir:               dataDir,
		OldSlug:               "alpha",
		NewSlug:               "beta",
		TasksMarkdown:         newTasks,
		TasksExpectedRevision: expected,
		Config:                []storage.ArtifactChange{configArtifact(dataDir, `{"slug":"beta"}`)},
	})
	if err != nil {
		t.Fatalf("RenameProject through CommitService: %v", err)
	}

	newPath := filepath.Join(dataDir, "projects", "beta")
	if project.Path != newPath {
		t.Fatalf("project path = %q, want %q", project.Path, newPath)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, "projects", "alpha")); !os.IsNotExist(statErr) {
		t.Fatalf("old project directory remains, stat err = %v", statErr)
	}
	if got, readErr := os.ReadFile(filepath.Join(newPath, "tasks.md")); readErr != nil || string(got) != string(newTasks) {
		t.Fatalf("destination tasks = %q, %v; want %q", got, readErr, newTasks)
	}
	if got, readErr := os.ReadFile(filepath.Join(dataDir, "config.json")); readErr != nil || string(got) != `{"slug":"beta"}` {
		t.Fatalf("catalog artifact = %q, %v", got, readErr)
	}
	entries, readErr := os.ReadDir(filepath.Join(dataDir, ".tt", "transactions"))
	if readErr != nil {
		t.Fatalf("read transactions dir: %v", readErr)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			t.Fatalf("completed rename left transaction manifest %q", entry.Name())
		}
	}
}

// TestRenameProjectResultCorrelatesLedgerTransactionID proves the identity a
// rename returns is the durable workspace-ledger transaction, not an empty
// fallback: recovery feedback advertises undo only on exact equality with the
// newest global entry, so an empty or mismatched ID silently drops that control.
func TestRenameProjectResultCorrelatesLedgerTransactionID(t *testing.T) {
	dataDir := t.TempDir()
	project := seedProjectDir(t, dataDir, "alpha", "Alpha")
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	repo := &journalOpsRepo{
		fakeRepo: newFakeRepo(),
		commit: storage.NewCommitServiceWithLedger(
			filepath.Join(dataDir, ".tt", "transactions"),
			[]string{dataDir},
			ledger,
		),
	}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	project.Name = "Beta"
	project.Slug = "beta"
	expected := tasksExpectedRevision(t, project)
	transactionID, err := svc.RenameProjectResult(app.RenameProjectRequest{
		Project:               project,
		DataDir:               dataDir,
		OldSlug:               "alpha",
		NewSlug:               "beta",
		TasksMarkdown:         []byte("# Beta\n"),
		TasksExpectedRevision: expected,
		Config:                []storage.ArtifactChange{configArtifact(dataDir, `{"slug":"beta"}`)},
	})
	if err != nil {
		t.Fatalf("RenameProjectResult: %v", err)
	}
	if transactionID == "" {
		t.Fatal("RenameProjectResult returned an empty transaction ID through a ledger-aware repository")
	}

	entry, ok, selectErr := ledger.SelectNewestUndo(storage.GlobalScope())
	if selectErr != nil {
		t.Fatalf("SelectNewestUndo: %v", selectErr)
	}
	if !ok {
		t.Fatal("rename recorded no workspace ledger entry")
	}
	if entry.TransactionID != transactionID {
		t.Fatalf("newest ledger transaction = %q, want returned %q", entry.TransactionID, transactionID)
	}
	if entry.Action != "Edit Project" || entry.Context != "Beta" {
		t.Fatalf("ledger entry = %q / %q, want %q / %q", entry.Action, entry.Context, "Edit Project", "Beta")
	}
	if len(entry.ProjectIDs) != 1 || entry.ProjectIDs[0] != project.ID {
		t.Fatalf("ledger project ids = %v, want [%s]", entry.ProjectIDs, project.ID)
	}
}

// TestRenameProjectRealCommitRejectsConcurrentTaskEdit proves the renamed
// project's own full-file rewrite validates the old-path tasks.md revision under
// the production commit boundary. A task edit after planning aborts the complete
// lifecycle transaction before the directory move or heading rewrite begins.
func TestRenameProjectRealCommitRejectsConcurrentTaskEdit(t *testing.T) {
	dataDir := t.TempDir()
	project := seedProjectDir(t, dataDir, "alpha", "Alpha")
	expected := tasksExpectedRevision(t, project)
	repo := &journalOpsRepo{
		fakeRepo: newFakeRepo(),
		commit: storage.NewCommitService(
			filepath.Join(dataDir, ".tt", "transactions"),
			[]string{dataDir},
		),
	}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	project.Name = "Beta"
	project.Slug = "beta"
	concurrent := []byte("# Alpha\n<!-- concurrent task edit -->\n")
	if err := os.WriteFile(storage.TasksFilePath(project.Path), concurrent, 0o600); err != nil {
		t.Fatalf("write concurrent tasks: %v", err)
	}

	_, err := svc.RenameProjectResult(app.RenameProjectRequest{
		Project:               project,
		DataDir:               dataDir,
		OldSlug:               "alpha",
		NewSlug:               "beta",
		TasksMarkdown:         []byte("# Beta\n"),
		TasksExpectedRevision: expected,
	})
	if !errors.Is(err, storage.ErrStaleArtifact) {
		t.Fatalf("RenameProject err = %v, want ErrStaleArtifact", err)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, "projects", "alpha")); statErr != nil {
		t.Fatalf("source project moved despite stale tasks: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, "projects", "beta")); !os.IsNotExist(statErr) {
		t.Fatalf("rename destination exists after aborted commit: %v", statErr)
	}
	if got, readErr := os.ReadFile(filepath.Join(dataDir, "projects", "alpha", "tasks.md")); readErr != nil || string(got) != string(concurrent) {
		t.Fatalf("source tasks = %q, %v; want concurrent bytes %q", got, readErr, concurrent)
	}
}

// TestRenameProjectSlugUnchangedRewritesInPlace proves a name change that keeps
// the slug commits no directory move but still rewrites tasks.md at the existing
// path.
func TestRenameProjectSlugUnchangedRewritesInPlace(t *testing.T) {
	dataDir := t.TempDir()
	project := seedProjectDir(t, dataDir, "alpha", "Alpha")
	repo := &opsRepo{fakeRepo: newFakeRepo()}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	newTasks := []byte("# Alpha Renamed\n")
	expected := tasksExpectedRevision(t, project)
	_, err := svc.RenameProjectResult(app.RenameProjectRequest{
		Project:               project,
		DataDir:               dataDir,
		OldSlug:               "alpha",
		NewSlug:               "alpha",
		TasksMarkdown:         newTasks,
		TasksExpectedRevision: expected,
	})
	if err != nil {
		t.Fatalf("RenameProject: %v", err)
	}
	// The single commit carries no raw move and one revision-checked tasks artifact.
	if len(repo.committed) != 1 || len(repo.committed[0]) != 0 {
		t.Fatalf("expected one commit with no raw ops, got %#v", repo.committed)
	}
	if len(repo.committedArtifacts) != 1 || len(repo.committedArtifacts[0]) != 1 || repo.committedArtifacts[0][0].ExpectedRevision == nil {
		t.Fatalf("expected one checked tasks artifact, got %#v", repo.committedArtifacts)
	}
	data, _ := os.ReadFile(filepath.Join(dataDir, "projects", "alpha", "tasks.md"))
	if string(data) != string(newTasks) {
		t.Errorf("tasks.md = %q, want %q", data, newTasks)
	}
}

// TestRenameProjectFailsClosedWithoutOpsCommitter proves the use case refuses to
// run when the repository cannot commit raw ops.
func TestRenameProjectFailsClosedWithoutOpsCommitter(t *testing.T) {
	svc := newTestService(newFakeRepo())
	_, err := svc.RenameProjectResult(app.RenameProjectRequest{
		Project: &models.Project{ID: uuid.New().String(), Slug: "beta"},
		DataDir: t.TempDir(),
		OldSlug: "alpha",
		NewSlug: "beta",
	})
	if err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("expected fail-closed error, got %v", err)
	}
}
