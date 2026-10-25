package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/storage"
)

func TestAttachURLRecordsMetadataOnly(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.AttachURL(repo.loc(task.ID), "https://example.com/x"); err != nil {
		t.Fatalf("AttachURL: %v", err)
	}
	if len(task.Attachments) != 1 || !task.Attachments[0].IsURL {
		t.Fatalf("expected one URL attachment, got %+v", task.Attachments)
	}
	cs, ok := repo.lastCommit()
	if !ok {
		t.Fatal("expected one committed changeset")
	}
	if cs.UndoAction != "Attach URL" {
		t.Errorf("UndoAction = %q, want Attach URL", cs.UndoAction)
	}
	if len(cs.AttachmentOps) != 0 {
		t.Errorf("URL attach must carry no attachment file ops, got %d", len(cs.AttachmentOps))
	}
}

func TestAttachURLRejectsReservedDelimiter(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	// A URL carrying the reserved ";" attachment delimiter is rejected by the model.
	if _, err := svc.AttachURL(repo.loc(task.ID), "https://example.com/a;b"); err == nil {
		t.Fatal("expected AttachURL to reject a reserved delimiter")
	}
	if len(repo.commits) != 0 {
		t.Errorf("expected no commit on rejected input, got %d", len(repo.commits))
	}
}

func TestAttachURLRejectsNonWebOrRelativeValues(t *testing.T) {
	for _, value := range []string{"example.com/file", "file:///tmp/file", "https:///missing-host"} {
		task := newTask("t1", 1, "Task")
		repo := newFakeRepo(newProject("p1", "Alpha", task))
		svc := newTestService(repo)
		if _, err := svc.AttachURL(repo.loc(task.ID), value); !errors.Is(err, app.ErrInvalidInput) {
			t.Errorf("AttachURL(%q) error = %v, want ErrInvalidInput", value, err)
		}
		if len(repo.commits) != 0 {
			t.Errorf("AttachURL(%q) committed invalid URL", value)
		}
	}
}

func TestAttachCodeReferenceRecordsMetadataOnly(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.AttachCodeReference(repo.loc(task.ID), "src/main.go", 42); err != nil {
		t.Fatalf("AttachCodeReference: %v", err)
	}
	if len(task.Attachments) != 1 || !task.Attachments[0].IsCodeRef || task.Attachments[0].LineNumber != 42 {
		t.Fatalf("expected one code-ref attachment at line 42, got %+v", task.Attachments)
	}
	if cs, _ := repo.lastCommit(); cs.UndoAction != "Attach Code Ref" || len(cs.AttachmentOps) != 0 {
		t.Errorf("unexpected changeset: action=%q ops=%d", cs.UndoAction, len(cs.AttachmentOps))
	}
}

func TestAttachCodeReferenceRejectsBadLine(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.AttachCodeReference(repo.loc(task.ID), "src/main.go", 0); err == nil {
		t.Fatal("expected AttachCodeReference to reject a non-positive line")
	}
	if len(repo.commits) != 0 {
		t.Errorf("expected no commit, got %d", len(repo.commits))
	}
}

func TestAttachFileEmitsCopyOp(t *testing.T) {
	task := newTask("t1", 1, "Task")
	project := newProject("p1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.AttachFile(repo.loc(task.ID), "/external/dir/report.pdf")
	if err != nil {
		t.Fatalf("AttachFile: %v", err)
	}
	if len(task.Attachments) != 1 || task.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("expected metadata for report.pdf, got %+v", task.Attachments)
	}
	cs, ok := repo.lastCommit()
	if !ok {
		t.Fatal("expected one committed changeset")
	}
	if cs.UndoAction != "Attach File" {
		t.Errorf("UndoAction = %q, want Attach File", cs.UndoAction)
	}
	if len(cs.AttachmentOps) != 1 {
		t.Fatalf("expected one attachment op, got %d", len(cs.AttachmentOps))
	}
	op := cs.AttachmentOps[0]
	if op.Kind != storage.AttachmentCopy || op.SourcePath != "/external/dir/report.pdf" || op.Filename != "report.pdf" {
		t.Errorf("unexpected copy op: %+v", op)
	}
	if op.DestProject != project {
		t.Error("copy op destination project mismatch")
	}
}

func TestAttachFileMissingSourceRejected(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.AttachFile(repo.loc(task.ID), ""); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("AttachFile(empty) error = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("expected no commit, got %d", len(repo.commits))
	}
}

func TestAttachFileRejectsUnsafeDestinationNameBeforeMutation(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.AttachFile(repo.loc(task.ID), ".."); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("AttachFile(..) error = %v, want ErrInvalidInput", err)
	}
	if len(task.Attachments) != 0 || len(repo.commits) != 0 {
		t.Fatalf("unsafe filename mutated state: attachments=%v commits=%d", task.Attachments, len(repo.commits))
	}
}

func TestAttachFileReservedFilenameRejectedBeforeCommit(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	// A source basename containing the reserved ";" delimiter fails metadata
	// validation, which must abort BEFORE any copy op is committed — no orphan.
	if _, err := svc.AttachFile(repo.loc(task.ID), "/external/bad;name.txt"); err == nil {
		t.Fatal("expected AttachFile to reject a reserved filename")
	}
	if len(repo.commits) != 0 {
		t.Errorf("expected no commit so no copy runs, got %d", len(repo.commits))
	}
	if len(task.Attachments) != 0 {
		t.Errorf("expected no metadata added on rejection, got %d", len(task.Attachments))
	}
}

func TestRemoveAttachmentFileEmitsDeleteOp(t *testing.T) {
	task := newTask("t1", 1, "Task")
	if err := task.AddAttachment("keep.txt", false, "/orig/keep.txt"); err != nil {
		t.Fatal(err)
	}
	if err := task.AddAttachment("gone.txt", false, "/orig/gone.txt"); err != nil {
		t.Fatal(err)
	}
	project := newProject("p1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.RemoveAttachment(repo.loc(task.ID), 1, true)
	if err != nil {
		t.Fatalf("RemoveAttachment: %v", err)
	}
	if len(task.Attachments) != 1 || task.Attachments[0].Filename != "keep.txt" {
		t.Fatalf("expected only keep.txt to remain, got %+v", task.Attachments)
	}
	cs, _ := repo.lastCommit()
	if len(cs.AttachmentOps) != 1 {
		t.Fatalf("expected one delete op, got %d", len(cs.AttachmentOps))
	}
	op := cs.AttachmentOps[0]
	if op.Kind != storage.AttachmentDelete || op.Filename != "gone.txt" {
		t.Errorf("unexpected delete op: %+v", op)
	}
}

func TestRemoveURLAttachmentCarriesNoFileOp(t *testing.T) {
	task := newTask("t1", 1, "Task")
	if err := task.AddAttachment("https://example.com", true, "https://example.com"); err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	_, err := svc.RemoveAttachment(repo.loc(task.ID), 0, true)
	if err != nil {
		t.Fatalf("RemoveAttachment: %v", err)
	}
	if cs, _ := repo.lastCommit(); len(cs.AttachmentOps) != 0 {
		t.Errorf("expected no file op for URL removal, got %d", len(cs.AttachmentOps))
	}
}

func TestRemoveAttachmentKeepFileCarriesNoFileOp(t *testing.T) {
	task := newTask("t1", 1, "Task")
	if err := task.AddAttachment("keep.txt", false, "/orig/keep.txt"); err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	_, err := svc.RemoveAttachment(repo.loc(task.ID), 0, false)
	if err != nil {
		t.Fatalf("RemoveAttachment: %v", err)
	}
	if cs, _ := repo.lastCommit(); len(cs.AttachmentOps) != 0 {
		t.Errorf("expected no file op for keep-file removal, got %d", len(cs.AttachmentOps))
	}
}

func TestRemoveAttachmentIndexOutOfRange(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	if _, err := svc.RemoveAttachment(repo.loc(task.ID), 0, true); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("RemoveAttachment(out of range) error = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("expected no commit, got %d", len(repo.commits))
	}
}

func TestRemoveAttachmentCommitErrorSurfaces(t *testing.T) {
	task := newTask("t1", 1, "Task")
	if err := task.AddAttachment("gone.txt", false, "/orig/gone.txt"); err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	sentinel := errors.New("injected commit failure")
	repo.commitErr = sentinel
	svc := newTestService(repo)

	if _, err := svc.RemoveAttachment(repo.loc(task.ID), 0, true); !errors.Is(err, sentinel) {
		t.Fatalf("RemoveAttachment error = %v, want injected failure", err)
	}
	if len(task.Attachments) != 1 || task.Attachments[0].Filename != "gone.txt" {
		t.Fatalf("failed removal commit left metadata mutated: %+v", task.Attachments)
	}
}

func TestAttachFileCommitErrorRestoresMetadata(t *testing.T) {
	task := newTask("t1", 1, "Task")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	repo.commitErr = errors.New("commit failed")
	svc := newTestService(repo)

	if _, err := svc.AttachFile(repo.loc(task.ID), "/external/report.pdf"); err == nil {
		t.Fatal("AttachFile unexpectedly succeeded")
	}
	if len(task.Attachments) != 0 {
		t.Fatalf("failed attachment commit left metadata mutated: %+v", task.Attachments)
	}
}

// TestAttachFileUndoRedoAcceptance drives the whole wired path against a real
// transactional commit service with the workspace ledger: AttachFile ingests an
// external file into the task's attachments directory in one transaction, ledger
// undo removes the ingested bytes, and redo re-ingests them.
func TestAttachFileUndoRedoAcceptance(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "attach me")
	task := project.Tasks[0]

	src := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(src, []byte("pdf bytes"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)

	appSvc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	_, err := appSvc.AttachFile(repo.loc(task.ID), src)
	if err != nil {
		t.Fatalf("AttachFile: %v", err)
	}
	dest, err := storage.ResolveAttachmentPath(project, task, filepath.Base(src))
	if err != nil {
		t.Fatalf("ResolveAttachmentPath: %v", err)
	}
	if got := readFileContent(t, dest); got != "pdf bytes" {
		t.Fatalf("ingested attachment = %q, want %q", got, "pdf bytes")
	}

	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	requireAbsentFile(t, dest)

	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if got := readFileContent(t, dest); got != "pdf bytes" {
		t.Fatalf("redone attachment = %q, want %q", got, "pdf bytes")
	}
}

// TestRemoveAttachmentUndoRedoAcceptance proves a single-attachment removal commits
// the metadata removal and the byte delete together, that ledger undo restores the
// deleted bytes, and redo removes them again — while a sibling attachment in the
// same directory is never touched.
func TestRemoveAttachmentUndoRedoAcceptance(t *testing.T) {
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project := realTestProject(t, dataDir, "alpha", "has attachments")
	task := project.Tasks[0]
	if err := task.AddAttachment("gone.txt", false, "/orig/gone.txt"); err != nil {
		t.Fatal(err)
	}
	if err := task.AddAttachment("keep.txt", false, "/orig/keep.txt"); err != nil {
		t.Fatal(err)
	}
	gonePath := seedRealAttachment(t, project, task, "gone.txt", []byte("gone bytes"))
	keepPath := seedRealAttachment(t, project, task, "keep.txt", []byte("keep bytes"))

	repo := newStorageRepo(commitSvc, project)
	repo.seed(t)
	// Re-derive the on-disk revision so the optimistic-concurrency check matches what
	// the commit service reads back: attachment AddedAt precision is lost in the
	// markdown round-trip, so the seeded in-memory revision diverges from disk. This
	// mirrors the production adapter's projectDiskRevision (LoadProjectDocument).
	doc, err := storage.LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	repo.revisions[project.ID] = doc.Revision

	appSvc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	if _, err := appSvc.RemoveAttachment(repo.loc(task.ID), 0, true); err != nil {
		t.Fatalf("RemoveAttachment: %v", err)
	}
	requireAbsentFile(t, gonePath)
	if got := readFileContent(t, keepPath); got != "keep bytes" {
		t.Fatalf("sibling attachment mutated: %q", got)
	}

	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := readFileContent(t, gonePath); got != "gone bytes" {
		t.Fatalf("restored attachment = %q, want %q", got, "gone bytes")
	}
	if got := readFileContent(t, keepPath); got != "keep bytes" {
		t.Fatalf("sibling attachment changed by undo: %q", got)
	}

	if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	requireAbsentFile(t, gonePath)
	if got := readFileContent(t, keepPath); got != "keep bytes" {
		t.Fatalf("sibling attachment removed by redo: %q", got)
	}
}

// TestImportAttachmentRecordsNoTemporaryPath holds a staged import to the
// workspace bytes alone: the copy op reads the staged file, the attachment is
// named as the person picked it, and the temporary staging path is recorded
// nowhere, so nothing points at storage that will be swept.
func TestImportAttachmentRecordsNoTemporaryPath(t *testing.T) {
	task := newTask("t1", 1, "Task")
	project := newProject("p1", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	staged := "/device/runtime/attachment-imports/4f0c/IMG_0001.HEIC"
	if _, err := svc.ImportAttachment(repo.loc(task.ID), staged, "Whiteboard.HEIC"); err != nil {
		t.Fatalf("ImportAttachment: %v", err)
	}
	if len(task.Attachments) != 1 {
		t.Fatalf("expected one attachment, got %+v", task.Attachments)
	}
	attachment := task.Attachments[0]
	if attachment.Filename != "Whiteboard.HEIC" || attachment.OriginalPath != "" || attachment.IsURL || attachment.IsCodeRef {
		t.Fatalf("the import recorded %+v", attachment)
	}
	cs, ok := repo.lastCommit()
	if !ok || len(cs.AttachmentOps) != 1 {
		t.Fatalf("expected one committed copy op, got %+v", cs)
	}
	op := cs.AttachmentOps[0]
	if op.Kind != storage.AttachmentCopy || op.SourcePath != staged || op.Filename != "Whiteboard.HEIC" || op.DestProject != project {
		t.Fatalf("unexpected copy op: %+v", op)
	}
	if cs.UndoAction != "Attach File" {
		t.Errorf("UndoAction = %q, want Attach File", cs.UndoAction)
	}
}

// TestImportAttachmentCommitsNothingWhenRefused: a name the canonical writer
// would refuse aborts before any metadata or byte is staged.
func TestImportAttachmentCommitsNothingWhenRefused(t *testing.T) {
	for _, name := range []string{"", "..", "a/b.txt", "bad;name.txt"} {
		task := newTask("t1", 1, "Task")
		repo := newFakeRepo(newProject("p1", "Alpha", task))
		svc := newTestService(repo)
		if _, err := svc.ImportAttachment(repo.loc(task.ID), "/device/runtime/attachment-imports/x/file", name); err == nil {
			t.Errorf("ImportAttachment(%q) succeeded", name)
		}
		if len(task.Attachments) != 0 || len(repo.commits) != 0 {
			t.Errorf("ImportAttachment(%q) left attachments=%v commits=%d", name, task.Attachments, len(repo.commits))
		}
	}
}
