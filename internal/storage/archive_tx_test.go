package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// canonicalArchiveProject builds a project laid out at <data>/projects/<slug> so
// the transactional archive path (which derives the workspace data dir from that
// layout) and the workspace ledger operate over real, root-contained paths.
func canonicalArchiveProject(t *testing.T) (dataDir string, project *models.Project) {
	t.Helper()
	dataDir = t.TempDir()
	projectPath := filepath.Join(dataDir, "projects", "archive-tx")
	if err := os.MkdirAll(projectPath, DirPermissions); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	project = &models.Project{
		ID:   uuid.New().String(),
		Name: "Archive Tx",
		Slug: "archive-tx",
		Path: projectPath,
		Tasks: []*models.Task{
			{ID: "active-1", ShortID: 1, Title: "Still active", Type: models.TaskTypeTask, WorkflowState: models.StateReady, Priority: 3},
			{ID: "done-1", ShortID: 2, Title: "Completed task", Type: models.TaskTypeTask, WorkflowState: models.StateDone, Priority: 3, CompletedAt: &now},
			{ID: "done-2", ShortID: 3, Title: "Completed bug", Type: models.TaskTypeBug, WorkflowState: models.StateDone, Priority: 2, CompletedAt: &now},
		},
		NextShortID: 4,
	}
	project.BuildTaskTree()
	// Seed the canonical tasks.md so the commit's revision/activity read has a
	// prior document to diff against, mirroring a loaded project.
	if err := SaveProject(project); err != nil {
		t.Fatalf("seed SaveProject failed: %v", err)
	}
	return dataDir, project
}

func ledgerForDataDir(dataDir string) *WorkspaceLedger {
	return NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
}

func commitServiceForDataDir(dataDir string, ledger *WorkspaceLedger) *CommitService {
	return NewCommitServiceWithLedger(archiveTransactionsDir(dataDir), []string{dataDir}, ledger)
}

// activeTaskIDs reads the on-disk tasks.md and returns the set of task IDs still
// present as active (non-archived) tasks.
func activeTaskIDs(t *testing.T, project *models.Project) map[string]bool {
	t.Helper()
	content, err := os.ReadFile(tasksFilePath(project.Path))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}
		}
		t.Fatalf("read tasks.md: %v", err)
	}
	doc, err := ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse tasks.md: %v", err)
	}
	ids := make(map[string]bool, len(doc.Tasks))
	for _, task := range doc.Tasks {
		ids[task.ID] = true
	}
	return ids
}

// archivedTaskIDs reads the on-disk archive (via the derived summary index) and
// returns the set of archived task IDs.
func archivedTaskIDs(t *testing.T, project *models.Project) map[string]bool {
	t.Helper()
	summaries, err := LoadArchivedTaskSummaries(project)
	if err != nil {
		t.Fatalf("LoadArchivedTaskSummaries: %v", err)
	}
	ids := make(map[string]bool, len(summaries))
	for _, summary := range summaries {
		ids[summary.ID] = true
	}
	return ids
}

// archivedTaskIDsCanonical reads archive Markdown directly. Restore tests use it
// between undo and redo so merely asserting canonical state does not rebuild the
// journaled derived index and intentionally trigger the restore drift guard.
func archivedTaskIDsCanonical(project *models.Project) map[string]bool {
	tasks := LoadArchivedTasks(project)
	ids := make(map[string]bool, len(tasks))
	for _, task := range tasks {
		ids[task.ID] = true
	}
	return ids
}

// assertActiveXorArchived enforces the core invariant: no task ID may be both
// active (present in tasks.md) and archived (present in the archive) at once.
func assertActiveXorArchived(t *testing.T, project *models.Project, phase string) {
	t.Helper()
	active := activeTaskIDs(t, project)
	archived := archivedTaskIDs(t, project)
	for id := range active {
		if archived[id] {
			t.Fatalf("%s: task %s is simultaneously active and archived", phase, id)
		}
	}
}

func TestArchiveTasksTxCommitsOneTransaction(t *testing.T) {
	dataDir, project := canonicalArchiveProject(t)
	svc := commitServiceForDataDir(dataDir, ledgerForDataDir(dataDir))

	// Give an archived task an attachment so retention rides the transaction.
	attachDir, err := resolveTaskAttachmentsDir(project, project.Tasks[1])
	if err != nil {
		t.Fatalf("resolve attachments dir: %v", err)
	}
	if err = os.MkdirAll(attachDir, DirPermissions); err != nil {
		t.Fatalf("mkdir attachments: %v", err)
	}
	attachFile := filepath.Join(attachDir, "note.txt")
	if err = os.WriteFile(attachFile, []byte("attached"), FilePermissions); err != nil {
		t.Fatalf("write attachment: %v", err)
	}

	count, err := ArchiveTasksTx(svc, project)
	if err != nil {
		t.Fatalf("ArchiveTasksTx failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("archived count = %d, want 2", count)
	}
	if len(project.Tasks) != 1 || project.Tasks[0].ID != "active-1" {
		t.Fatalf("remaining tasks = %+v, want only active-1", project.Tasks)
	}

	assertActiveXorArchived(t, project, "after commit")
	if archived := archivedTaskIDs(t, project); !archived["done-1"] || !archived["done-2"] {
		t.Fatalf("archived set missing completed tasks: %v", archived)
	}

	// The index committed with the archive must be recognized as fresh (no rebuild
	// churn) even though it was written inside the transaction without a mtime.
	status, err := InspectArchiveIndex(project)
	if err != nil {
		t.Fatalf("InspectArchiveIndex: %v", err)
	}
	if !status.Fresh || status.NeedsRebuild {
		t.Fatalf("index status after transactional archive = %+v, want fresh", status)
	}

	// Attachment retention committed with the transaction.
	if _, err := os.Stat(attachFile); !os.IsNotExist(err) {
		t.Fatalf("archived task attachment should be removed, stat err=%v", err)
	}
}

func TestArchiveTasksTxUndoRestoresActiveAndRemovesArchivedCopy(t *testing.T) {
	dataDir, project := canonicalArchiveProject(t)
	ledger := ledgerForDataDir(dataDir)
	svc := commitServiceForDataDir(dataDir, ledger)

	if _, err := ArchiveTasksTx(svc, project); err != nil {
		t.Fatalf("ArchiveTasksTx failed: %v", err)
	}
	assertActiveXorArchived(t, project, "after commit")

	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo failed: %v", err)
	}

	// Undo restores the active tasks and removes the archived copy entirely.
	active := activeTaskIDs(t, project)
	if !active["done-1"] || !active["done-2"] {
		t.Fatalf("undo did not restore active tasks: %v", active)
	}
	archived := archivedTaskIDsCanonical(project)
	if len(archived) != 0 {
		t.Fatalf("undo left archived copies behind: %v", archived)
	}
	for id := range active {
		if archived[id] {
			t.Fatalf("after undo: task %s is simultaneously active and archived", id)
		}
	}

	if _, err := ledger.Redo(GlobalScope()); err != nil {
		t.Fatalf("Redo failed: %v", err)
	}

	// Redo re-archives: the tasks leave tasks.md and reappear in the archive.
	active = activeTaskIDs(t, project)
	if active["done-1"] || active["done-2"] {
		t.Fatalf("redo left completed tasks active: %v", active)
	}
	archived = archivedTaskIDs(t, project)
	if !archived["done-1"] || !archived["done-2"] {
		t.Fatalf("redo did not restore archived copies: %v", archived)
	}
	assertActiveXorArchived(t, project, "after redo")
}

func TestArchiveTasksTxRotatesWithinTransaction(t *testing.T) {
	dataDir, project := canonicalArchiveProject(t)
	svc := commitServiceForDataDir(dataDir, ledgerForDataDir(dataDir))

	// Seed a completed.md large enough to force rotation on the next archive.
	seed := archiveIndexTestTask("old-archived", 9, models.TaskTypeTask, "Previously archived", time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC))
	seed.Description = strings.Repeat("archived description line\n", int(archiveRotationThreshold/25)+1)
	if err := os.WriteFile(filepath.Join(project.Path, archiveFileName), []byte(WriteArchivedTasks(project.Name, []*models.Task{seed})), FilePermissions); err != nil {
		t.Fatalf("seed large archive: %v", err)
	}

	if _, err := ArchiveTasksTx(svc, project); err != nil {
		t.Fatalf("ArchiveTasksTx failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(project.Path, "completed.1.md")); err != nil {
		t.Fatalf("rotation should have produced completed.1.md: %v", err)
	}
	archived := archivedTaskIDs(t, project)
	if !archived["old-archived"] || !archived["done-1"] || !archived["done-2"] {
		t.Fatalf("archive after rotation missing tasks: %v", archived)
	}
	assertActiveXorArchived(t, project, "after rotation")

	status, err := InspectArchiveIndex(project)
	if err != nil {
		t.Fatalf("InspectArchiveIndex: %v", err)
	}
	if !status.Fresh || status.SourceCount != 2 {
		t.Fatalf("post-rotation index status = %+v, want fresh with 2 sources", status)
	}
}

func TestArchiveTasksTxRefusesUnparseableExistingArchive(t *testing.T) {
	dataDir, project := canonicalArchiveProject(t)
	svc := commitServiceForDataDir(dataDir, ledgerForDataDir(dataDir))

	// Corrupt the existing archive so it cannot be parsed; the append would grow it
	// but the transactional path must refuse to archive into a corrupt source.
	badContent := "### Task: Bad archive\n" + strings.Repeat("x", scannerMaxBufSize+1)
	if err := os.WriteFile(filepath.Join(project.Path, archiveFileName), []byte(badContent), FilePermissions); err != nil {
		t.Fatalf("write corrupt archive: %v", err)
	}

	if _, err := ArchiveTasksTx(svc, project); err == nil {
		t.Fatal("expected ArchiveTasksTx to refuse archiving into an unparseable archive")
	}

	// The project must remain fully un-archived: completed tasks still present.
	active := activeTaskIDs(t, project)
	if !active["done-1"] || !active["done-2"] {
		t.Fatalf("failed archive should leave completed tasks in tasks.md: %v", active)
	}
}
