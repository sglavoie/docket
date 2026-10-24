package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// seedTaskAttachment creates the on-disk attachments directory for a task under a
// project and writes a single file with the given bytes, returning the file's path.
func seedTaskAttachment(t *testing.T, project *models.Project, task *models.Task, name string, data []byte) string {
	t.Helper()
	dir := GetTaskAttachmentsDir(project, task)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		t.Fatalf("mkdir attachments: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	return path
}

// newCommitTestProject builds a minimal single-task project rooted under a
// canonical <data>/projects/<slug> layout inside dataDir.
func newCommitTestProject(t *testing.T, dataDir, slug, title string) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	p.Slug = slug
	p.Path = filepath.Join(dataDir, "projects", slug)
	task, err := models.NewGenericTask(uuid.New().String(), 1, title)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	p.Tasks = []*models.Task{task}
	p.NextShortID = 2
	p.BuildTaskTree()
	return p
}

func newCommitTestService(t *testing.T) (*CommitService, string) {
	t.Helper()
	dataDir := t.TempDir()
	svc := NewCommitService(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir})
	return svc, dataDir
}

// onDiskRevision parses a project's committed tasks.md and returns its canonical
// revision.
func onDiskRevision(t *testing.T, project *models.Project) string {
	t.Helper()
	content, err := os.ReadFile(tasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read tasks file: %v", err)
	}
	doc, err := ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse committed doc: %v", err)
	}
	return doc.Revision
}

// TestCommitService_WritesProjectAndArtifact confirms a changeset commits the
// project markdown and an associated configuration artifact atomically, and
// reports the new revision.
func TestCommitResultTransactionIDAndWritesProjectAndArtifact(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "write me")

	artifactPath := filepath.Join(dataDir, ".tt", "planning.json")
	result, err := svc.Commit(ChangeSet{
		Project:   project,
		Artifacts: []ArtifactChange{{Path: artifactPath, Data: []byte(`{"queue":[]}`)}},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if result.Revision == "" {
		t.Fatal("expected a non-empty revision")
	}
	if result.TransactionID == "" {
		t.Fatal("expected a non-empty transaction ID")
	}

	// The project markdown was written and parses back to the committed task.
	loaded, err := models.NewProject(project.ID, project.Slug, "")
	if err != nil {
		t.Fatal(err)
	}
	loaded.Path = project.Path
	if loadErr := LoadProject(loaded); loadErr != nil {
		t.Fatalf("LoadProject: %v", loadErr)
	}
	if len(loaded.Tasks) != 1 || loaded.Tasks[0].Title != "write me" {
		t.Fatalf("loaded tasks = %+v, want single 'write me'", loaded.Tasks)
	}

	// The configuration artifact was committed in the same transaction.
	got, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(got) != `{"queue":[]}` {
		t.Fatalf("artifact content = %q", got)
	}

	// The journal is clean: no leftover manifests after a successful commit.
	entries, _ := os.ReadDir(filepath.Join(dataDir, ".tt", "transactions"))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			t.Fatalf("unexpected leftover manifest: %s", e.Name())
		}
	}
}

// TestCommitService_PreservesLosslessBaseAndReturnsReloadRevision proves the
// transactional user-mutation path writes the schema-v5 envelope from the exact
// loaded base and reports the revision an immediate reload observes. It repeats
// the mutation within one persisted timestamp minute to guard the historical
// in-memory/reload revision divergence.
func TestCommitService_PreservesLosslessBaseAndReturnsReloadRevision(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "lossless", "before")
	project.Tasks[0].CreatedAt = time.Date(2026, 7, 13, 10, 15, 5, 0, time.Local)
	project.Tasks[0].UpdatedAt = time.Date(2026, 7, 13, 10, 15, 5, 0, time.Local)

	seed := ProjectDocumentFromBase(project, nil)
	if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(tasksFilePath(project.Path), []byte(mustWriteCurrentDocument(t, seed)), FilePermissions); err != nil {
		t.Fatalf("seed tasks: %v", err)
	}
	base, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("load base: %v", err)
	}

	project.Tasks[0].Title = "first mutation"
	project.Tasks[0].UpdatedAt = time.Date(2026, 7, 13, 10, 15, 40, 0, time.Local)
	first, err := svc.Commit(ChangeSet{Project: project, ExpectedRevision: base.Revision, Base: base})
	if err != nil {
		t.Fatalf("first Commit: %v", err)
	}
	firstBytes, err := os.ReadFile(tasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read first commit: %v", err)
	}
	firstContent := string(firstBytes)
	if !strings.Contains(firstContent, schemaMarkerV20) {
		t.Fatalf("committed content missing schema marker:\n%s", firstContent)
	}
	if strings.Contains(firstContent, "<!-- future-feature: keep-exactly -->") {
		t.Fatalf("committed schema-7 content retained opaque legacy block:\n%s", firstContent)
	}
	if !strings.Contains(firstContent, "updated:") {
		t.Fatalf("schema-7 commit omitted required updated timestamp:\n%s", firstContent)
	}
	firstReload, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("reload first commit: %v", err)
	}
	if first.Revision != firstReload.Revision {
		t.Fatalf("first result revision %s != reload %s", first.Revision, firstReload.Revision)
	}
	if strings.Contains(firstContent, "<!-- sections:") {
		t.Fatalf("canonical commit retained legacy section metadata:\n%s", firstContent)
	}
	project.Tasks[0].Title = "second mutation"
	project.Tasks[0].UpdatedAt = time.Date(2026, 7, 13, 10, 15, 55, 0, time.Local)
	second, err := svc.Commit(ChangeSet{Project: project, ExpectedRevision: firstReload.Revision, Base: firstReload})
	if err != nil {
		t.Fatalf("second Commit: %v", err)
	}
	secondReload, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("reload second commit: %v", err)
	}
	if second.Revision != secondReload.Revision {
		t.Fatalf("second result revision %s != reload %s", second.Revision, secondReload.Revision)
	}
}

// TestCommitService_StaleRevisionConflict confirms a changeset whose expected
// revision no longer matches the on-disk revision is aborted with a
// stale-revision conflict and does not overwrite the concurrent change.
func TestCommitService_StaleRevisionConflict(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "original")

	if _, err := svc.Commit(ChangeSet{Project: project}); err != nil {
		t.Fatalf("initial Commit: %v", err)
	}
	staleRevision := onDiskRevision(t, project)

	// A concurrent change advances the on-disk revision.
	extra, err := models.NewGenericTask(uuid.New().String(), 2, "added out of band")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, extra)
	project.NextShortID = 3
	project.BuildTaskTree()
	if _, commitErr := svc.Commit(ChangeSet{Project: project}); commitErr != nil {
		t.Fatalf("concurrent Commit: %v", commitErr)
	}
	currentRevision := onDiskRevision(t, project)
	if currentRevision == staleRevision {
		t.Fatal("expected the concurrent commit to change the on-disk revision")
	}

	// Committing against the now-stale revision must conflict before writing.
	_, err = svc.Commit(ChangeSet{Project: project, ExpectedRevision: staleRevision})
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("Commit err = %v, want ErrStaleRevision", err)
	}
	var conflict *RevisionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected *RevisionConflictError, got %T", err)
	}
	if conflict.Expected != staleRevision || conflict.Actual != currentRevision {
		t.Fatalf("conflict revisions = (%s, %s), want (%s, %s)", conflict.Expected, conflict.Actual, staleRevision, currentRevision)
	}

	// The on-disk file is untouched by the rejected commit.
	if got := onDiskRevision(t, project); got != currentRevision {
		t.Fatalf("on-disk revision changed to %s after rejected commit", got)
	}
}

// TestCommitService_ExpectedRevisionMatchCommits confirms that supplying the
// correct current revision lets the commit proceed.
func TestCommitService_ExpectedRevisionMatchCommits(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "v1")

	if _, err := svc.Commit(ChangeSet{Project: project}); err != nil {
		t.Fatalf("initial Commit: %v", err)
	}
	current := onDiskRevision(t, project)

	extra, err := models.NewGenericTask(uuid.New().String(), 2, "v2 task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, extra)
	project.NextShortID = 3
	project.BuildTaskTree()

	if _, err := svc.Commit(ChangeSet{Project: project, ExpectedRevision: current}); err != nil {
		t.Fatalf("Commit with matching revision: %v", err)
	}
}

// TestCommitService_BlockedByQuarantine confirms a quarantined transaction blocks
// the commit and leaves the project unwritten.
func TestCommitService_BlockedByQuarantine(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "should not write")

	// Plant a corrupt manifest so startup recovery quarantines it.
	txnDir := filepath.Join(dataDir, ".tt", "transactions")
	if err := os.MkdirAll(txnDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(txnDir, "txn-corrupt.json"), []byte("{ not valid"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Commit(ChangeSet{Project: project})
	if !errors.Is(err, ErrCommitBlocked) {
		t.Fatalf("Commit err = %v, want ErrCommitBlocked", err)
	}
	if _, statErr := os.Stat(tasksFilePath(project.Path)); !os.IsNotExist(statErr) {
		t.Fatalf("project written despite blocked commit: stat err = %v", statErr)
	}
}

// commitProjectExists reports whether a project's tasks.md is present on disk.
func commitProjectExists(project *models.Project) bool {
	_, err := os.Stat(tasksFilePath(project.Path))
	return err == nil
}

// assertJournalClean fails if any manifest is left in the transactions directory,
// which would mean a commit did not reclaim its transaction.
func assertJournalClean(t *testing.T, dataDir string) {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(dataDir, ".tt", "transactions"))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			t.Fatalf("unexpected leftover manifest: %s", e.Name())
		}
	}
}
