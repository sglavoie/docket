package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/transaction"
)

// TestCommitService_AttachmentCopyCommitsWithProject confirms an attachment copy
// is staged and committed in the same transaction as the project write, landing the
// ingested bytes atomically with the tasks.md that references them.
func TestCommitService_AttachmentCopyCommitsWithProject(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "has attachment")
	task := project.Tasks[0]

	srcPath := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(srcPath, []byte("pdf-bytes"), 0o600); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	if _, err := svc.Commit(ChangeSet{
		Project:       project,
		AttachmentOps: []AttachmentOp{{Kind: AttachmentCopy, Task: task, DestProject: project, SourcePath: srcPath}},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if !commitProjectExists(project) {
		t.Fatal("project tasks.md not written")
	}
	got, err := os.ReadFile(GetAttachmentPath(project, task, "report.pdf"))
	if err != nil {
		t.Fatalf("read committed attachment: %v", err)
	}
	if string(got) != "pdf-bytes" {
		t.Fatalf("attachment content = %q, want %q", got, "pdf-bytes")
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentCopyDuplicateNameAbortsAll confirms copying a file
// whose name already exists in the task's directory fails, and — because attachment
// resolution runs before any bytes are staged — leaves the project unwritten.
func TestCommitService_AttachmentCopyDuplicateNameAbortsAll(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "dup name")
	task := project.Tasks[0]
	seedTaskAttachment(t, project, task, "dup.txt", []byte("existing"))

	srcPath := filepath.Join(t.TempDir(), "dup.txt")
	if err := os.WriteFile(srcPath, []byte("incoming"), 0o600); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	_, err := svc.Commit(ChangeSet{
		Project:       project,
		AttachmentOps: []AttachmentOp{{Kind: AttachmentCopy, Task: task, DestProject: project, SourcePath: srcPath}},
	})
	if err == nil {
		t.Fatal("expected duplicate-name copy to be rejected")
	}
	if commitProjectExists(project) {
		t.Fatal("project written despite rejected attachment copy")
	}
	// The pre-existing file is untouched.
	got, _ := os.ReadFile(GetAttachmentPath(project, task, "dup.txt"))
	if string(got) != "existing" {
		t.Fatalf("existing attachment changed to %q", got)
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentCopyMissingSourceAbortsAll confirms a copy whose
// source file is missing fails and prevents the project write.
func TestCommitService_AttachmentCopyMissingSourceAbortsAll(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "missing source")
	task := project.Tasks[0]

	_, err := svc.Commit(ChangeSet{
		Project:       project,
		AttachmentOps: []AttachmentOp{{Kind: AttachmentCopy, Task: task, DestProject: project, SourcePath: filepath.Join(t.TempDir(), "nope.txt")}},
	})
	if err == nil {
		t.Fatal("expected missing-source copy to be rejected")
	}
	if commitProjectExists(project) {
		t.Fatal("project written despite missing attachment source")
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentMoveAcrossProjects moves a task's whole attachments
// directory — including a large file — from one project to another in the same
// transaction as both projects' tasks.md writes, confirming the destination takes
// ownership of the bytes and the source directory is gone.
func TestCommitService_AttachmentMoveAcrossProjects(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	src := newCommitTestProject(t, dataDir, "src", "moving")
	dst := newCommitTestProject(t, dataDir, "dst", "existing")
	task := src.Tasks[0]

	large := make([]byte, 5<<20) // 5 MiB
	for i := range large {
		large[i] = byte(i % 251)
	}
	seedTaskAttachment(t, src, task, "small.txt", []byte("small"))
	seedTaskAttachment(t, src, task, "large.bin", large)

	if _, err := svc.CommitMulti([]ChangeSet{
		{Project: src, AttachmentOps: []AttachmentOp{{Kind: AttachmentMove, Task: task, SourceProject: src, DestProject: dst}}},
		{Project: dst},
	}); err != nil {
		t.Fatalf("CommitMulti: %v", err)
	}

	if _, err := os.Stat(GetTaskAttachmentsDir(src, task)); !os.IsNotExist(err) {
		t.Fatalf("source attachments dir should be gone, stat err = %v", err)
	}
	if got, err := os.ReadFile(GetAttachmentPath(dst, task, "small.txt")); err != nil || string(got) != "small" {
		t.Fatalf("moved small file = %q err = %v", got, err)
	}
	movedLarge, err := os.ReadFile(GetAttachmentPath(dst, task, "large.bin"))
	if err != nil {
		t.Fatalf("read moved large file: %v", err)
	}
	if len(movedLarge) != len(large) || movedLarge[0] != large[0] || movedLarge[len(large)-1] != large[len(large)-1] {
		t.Fatalf("moved large file corrupted: len=%d want=%d", len(movedLarge), len(large))
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentMoveMissingSourceIsNoop confirms moving a task that
// has no attachments directory is a no-op that still commits the project writes.
func TestCommitService_AttachmentMoveMissingSourceIsNoop(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	src := newCommitTestProject(t, dataDir, "src", "no attachments")
	dst := newCommitTestProject(t, dataDir, "dst", "dst")
	task := src.Tasks[0]

	if _, err := svc.CommitMulti([]ChangeSet{
		{Project: src, AttachmentOps: []AttachmentOp{{Kind: AttachmentMove, Task: task, SourceProject: src, DestProject: dst}}},
		{Project: dst},
	}); err != nil {
		t.Fatalf("CommitMulti: %v", err)
	}
	if !commitProjectExists(src) || !commitProjectExists(dst) {
		t.Fatal("both projects should commit even with no attachments to move")
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentMoveDestExistsAbortsAll confirms a move onto an
// already-occupied destination directory fails and prevents all writes.
func TestCommitService_AttachmentMoveDestExistsAbortsAll(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	src := newCommitTestProject(t, dataDir, "src", "moving")
	dst := newCommitTestProject(t, dataDir, "dst", "existing")
	task := src.Tasks[0]
	seedTaskAttachment(t, src, task, "a.txt", []byte("a"))
	seedTaskAttachment(t, dst, task, "collide.txt", []byte("already here"))

	_, err := svc.CommitMulti([]ChangeSet{
		{Project: src, AttachmentOps: []AttachmentOp{{Kind: AttachmentMove, Task: task, SourceProject: src, DestProject: dst}}},
		{Project: dst},
	})
	if err == nil {
		t.Fatal("expected move onto occupied destination to be rejected")
	}
	if commitProjectExists(src) || commitProjectExists(dst) {
		t.Fatal("projects written despite rejected attachment move")
	}
	// The source attachment is left in place for a retry.
	if !AttachmentExists(src, task, "a.txt") {
		t.Fatal("source attachment should be untouched after rejected move")
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentDeleteRemovesDir confirms an attachment delete removes
// every file and the task's directory in the same transaction as the project write.
func TestCommitService_AttachmentDeleteRemovesDir(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "deleting")
	task := project.Tasks[0]
	seedTaskAttachment(t, project, task, "a.txt", []byte("a"))
	seedTaskAttachment(t, project, task, "b.txt", []byte("b"))

	if _, err := svc.Commit(ChangeSet{
		Project:       project,
		AttachmentOps: []AttachmentOp{{Kind: AttachmentDelete, Task: task, SourceProject: project}},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, err := os.Stat(GetTaskAttachmentsDir(project, task)); !os.IsNotExist(err) {
		t.Fatalf("task attachments dir should be gone, stat err = %v", err)
	}
	if !commitProjectExists(project) {
		t.Fatal("project tasks.md not written")
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentMoveRecoversWithMetadata proves the crash-safety
// guarantee: a transaction that renames a task's attachments directory together
// with both projects' tasks.md writes, interrupted after Prepare but before Commit,
// is rolled forward by recovery so the attachment bytes and the task metadata are
// restored together — never one without the other.
func TestCommitService_AttachmentMoveRecoversWithMetadata(t *testing.T) {
	_, dataDir := newCommitTestService(t)
	src := newCommitTestProject(t, dataDir, "src", "moving")
	dst := newCommitTestProject(t, dataDir, "dst", "existing")
	task := src.Tasks[0]

	seedTaskAttachment(t, src, task, "payload.bin", []byte("attachment-bytes"))
	srcDir := GetTaskAttachmentsDir(src, task)
	destDir := GetTaskAttachmentsDir(dst, task)
	if err := os.MkdirAll(filepath.Dir(destDir), DirPermissions); err != nil {
		t.Fatalf("mkdir dest parent: %v", err)
	}

	// Perform the metadata move in memory so the staged tasks.md reflects it.
	moved := src.Tasks[0]
	src.Tasks = nil
	src.BuildTaskTree()
	moved.ShortID = dst.GetNextShortID()
	dst.Tasks = append(dst.Tasks, moved)
	dst.NextShortID = moved.ShortID + 1
	dst.BuildTaskTree()

	srcContent := mustWriteCurrentProject(t, src)
	dstContent := mustWriteCurrentProject(t, dst)

	txnDir := filepath.Join(dataDir, ".docket", "transactions")
	journal := transaction.NewJournal(txnDir, []string{dataDir}, "")
	if _, err := journal.Prepare([]transaction.Op{
		{Kind: transaction.OpWrite, Target: tasksFilePath(src.Path), Data: []byte(srcContent), Mode: FilePermissions},
		{Kind: transaction.OpWrite, Target: tasksFilePath(dst.Path), Data: []byte(dstContent), Mode: FilePermissions},
		{Kind: transaction.OpMove, Source: srcDir, Target: destDir},
	}); err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	// Simulate a crash: the manifest is prepared but never committed. A fresh
	// journal (as a later process would construct) recovers it.
	report, err := transaction.NewJournal(txnDir, []string{dataDir}, "").Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if len(report.RolledForward) != 1 {
		t.Fatalf("RolledForward = %v, want exactly one transaction", report.RolledForward)
	}

	// Attachment bytes followed the task to the destination.
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Fatalf("source attachments dir should be gone, stat err = %v", err)
	}
	if got, err := os.ReadFile(GetAttachmentPath(dst, task, "payload.bin")); err != nil || string(got) != "attachment-bytes" {
		t.Fatalf("recovered attachment = %q err = %v", got, err)
	}

	// Metadata landed together with the bytes: dst owns the task, src is empty.
	loadedDst, _ := models.NewProject(uuid.New().String(), "dst", "")
	loadedDst.Path = dst.Path
	if err := LoadProject(loadedDst); err != nil {
		t.Fatalf("LoadProject dst: %v", err)
	}
	if len(loadedDst.Tasks) != 2 {
		t.Fatalf("dst has %d tasks after recovery, want 2", len(loadedDst.Tasks))
	}
	loadedSrc, _ := models.NewProject(uuid.New().String(), "src", "")
	loadedSrc.Path = src.Path
	if err := LoadProject(loadedSrc); err != nil {
		t.Fatalf("LoadProject src: %v", err)
	}
	if len(loadedSrc.Tasks) != 0 {
		t.Fatalf("src has %d tasks after recovery, want 0", len(loadedSrc.Tasks))
	}
	assertJournalClean(t, dataDir)
}
