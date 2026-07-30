package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCommitService_AttachmentMoveUndoSupportsRepeatedMoves(t *testing.T) {
	tests := []struct {
		name  string
		after func(t *testing.T, service *CommitService, ledger *WorkspaceLedger, src, dst *models.Project, task *models.Task)
	}{
		{
			name: "undo then repeat same move",
			after: func(t *testing.T, service *CommitService, ledger *WorkspaceLedger, src, dst *models.Project, task *models.Task) {
				if _, err := ledger.Undo(GlobalScope()); err != nil {
					t.Fatalf("Undo: %v", err)
				}
				assertTaskAttachmentLocation(t, src, dst, task, "payload.txt", "payload")
				commitAttachmentMove(t, service, src, dst, task)
				assertTaskAttachmentLocation(t, dst, src, task, "payload.txt", "payload")
			},
		},
		{
			name: "undo redo then reverse move",
			after: func(t *testing.T, service *CommitService, ledger *WorkspaceLedger, src, dst *models.Project, task *models.Task) {
				if _, err := ledger.Undo(GlobalScope()); err != nil {
					t.Fatalf("Undo: %v", err)
				}
				if _, err := ledger.Redo(GlobalScope()); err != nil {
					t.Fatalf("Redo: %v", err)
				}
				assertTaskAttachmentLocation(t, dst, src, task, "payload.txt", "payload")
				commitAttachmentMove(t, service, dst, src, task)
				assertTaskAttachmentLocation(t, src, dst, task, "payload.txt", "payload")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, ledger, dataDir := newAttachmentLedgerTestService(t)
			src := newCommitTestProject(t, dataDir, "src", "moving")
			dst := newCommitTestProject(t, dataDir, "dst", "existing")
			task := src.Tasks[0]
			seedTaskAttachment(t, src, task, "payload.txt", []byte("payload"))

			commitAttachmentMove(t, service, src, dst, task)
			assertTaskAttachmentLocation(t, dst, src, task, "payload.txt", "payload")
			tt.after(t, service, ledger, src, dst, task)
		})
	}
}

func TestCommitService_AttachmentCopyUndoPreservesDirectoryHistory(t *testing.T) {
	for _, tt := range []struct {
		name       string
		precreate  bool
		wantExists bool
	}{
		{name: "first copy removes new task directory", wantExists: false},
		{name: "copy preserves preexisting task directory", precreate: true, wantExists: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service, ledger, dataDir := newAttachmentLedgerTestService(t)
			project := newCommitTestProject(t, dataDir, "alpha", "copy")
			task := project.Tasks[0]
			taskDir := GetTaskAttachmentsDir(project, task)
			if tt.precreate {
				if err := os.MkdirAll(taskDir, DirPermissions); err != nil {
					t.Fatalf("precreate attachment directory: %v", err)
				}
			}
			source := filepath.Join(t.TempDir(), "first.txt")
			if err := os.WriteFile(source, []byte("first"), FilePermissions); err != nil {
				t.Fatalf("write attachment source: %v", err)
			}

			if _, err := service.Commit(ChangeSet{
				Project: project,
				AttachmentOps: []AttachmentOp{{
					Kind: AttachmentCopy, Task: task, DestProject: project, SourcePath: source,
				}},
			}); err != nil {
				t.Fatalf("Commit copy: %v", err)
			}
			if _, err := ledger.Undo(GlobalScope()); err != nil {
				t.Fatalf("Undo copy: %v", err)
			}
			requireAbsent(t, filepath.Join(taskDir, "first.txt"))
			_, err := os.Stat(taskDir)
			if tt.wantExists && err != nil {
				t.Fatalf("preexisting task directory removed: %v", err)
			}
			if !tt.wantExists && !os.IsNotExist(err) {
				t.Fatalf("new task directory survived undo: %v", err)
			}
		})
	}
}

func TestCommitService_AttachmentDeleteUndoRedoRestoresDirectoryTopology(t *testing.T) {
	service, ledger, dataDir := newAttachmentLedgerTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "delete")
	task := project.Tasks[0]
	taskDir := GetTaskAttachmentsDir(project, task)
	seedTaskAttachment(t, project, task, "a.txt", []byte("a"))
	seedTaskAttachment(t, project, task, "b.txt", []byte("b"))

	if _, err := service.Commit(ChangeSet{
		Project: project,
		AttachmentOps: []AttachmentOp{{
			Kind: AttachmentDelete, Task: task, SourceProject: project,
		}},
	}); err != nil {
		t.Fatalf("Commit delete: %v", err)
	}
	requireAbsent(t, taskDir)
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo delete: %v", err)
	}
	if got := readFile(t, filepath.Join(taskDir, "a.txt")); got != "a" {
		t.Fatalf("a.txt after undo = %q", got)
	}
	if got := readFile(t, filepath.Join(taskDir, "b.txt")); got != "b" {
		t.Fatalf("b.txt after undo = %q", got)
	}
	if _, err := ledger.Redo(GlobalScope()); err != nil {
		t.Fatalf("Redo delete: %v", err)
	}
	requireAbsent(t, taskDir)
}

func TestCommitService_AttachmentMoveDropsEmptySourceWithoutCreatingDestination(t *testing.T) {
	service, ledger, dataDir := newAttachmentLedgerTestService(t)
	src := newCommitTestProject(t, dataDir, "src", "moving empty")
	dst := newCommitTestProject(t, dataDir, "dst", "existing")
	task := src.Tasks[0]
	srcDir := GetTaskAttachmentsDir(src, task)
	dstDir := GetTaskAttachmentsDir(dst, task)
	if err := os.MkdirAll(srcDir, DirPermissions); err != nil {
		t.Fatalf("create empty source attachment directory: %v", err)
	}

	commitAttachmentMove(t, service, src, dst, task)
	requireAbsent(t, srcDir)
	requireAbsent(t, dstDir)
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo empty move: %v", err)
	}
	requireAbsent(t, srcDir)
	requireAbsent(t, dstDir)
}

func TestWorkspaceLedger_V2PersistsImageKinds(t *testing.T) {
	ledger, root := newTestLedger(t)
	filePath := filepath.Join(root, "projects", "alpha", "tasks.md")
	dirPath := filepath.Join(root, "projects", "alpha", attachmentsDirName, strings.Repeat("a", 64))
	if err := ledger.Record(LedgerRecord{
		TransactionID: "v2-kinds",
		Artifacts: []LedgerArtifact{
			{Path: filePath, AfterExists: true, After: []byte("tasks")},
			{Kind: ArtifactImageKindDirectory, Path: dirPath, AfterExists: true},
		},
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	data, err := os.ReadFile(ledger.ledgerPath())
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var persisted ledgerFile
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("unmarshal ledger: %v", err)
	}
	if persisted.SchemaVersion != UndoLedgerSchemaVersion {
		t.Fatalf("schema version = %d, want %d", persisted.SchemaVersion, UndoLedgerSchemaVersion)
	}
	if got := persisted.Entries[0].Artifacts[0].Kind; got != ArtifactImageKindFile {
		t.Fatalf("file image kind = %q", got)
	}
	if got := persisted.Entries[0].Artifacts[1].Kind; got != ArtifactImageKindDirectory {
		t.Fatalf("directory image kind = %q", got)
	}
}

func TestWorkspaceLedger_V1AttachmentCleanupIsConservativeAndSurvivesUpgrade(t *testing.T) {
	for _, tt := range []struct {
		name        string
		withSibling bool
	}{
		{name: "removes directory containing only restored deletion"},
		{name: "preserves directory containing unrelated sibling", withSibling: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ledger, root := newTestLedger(t)
			taskDir := filepath.Join(root, "projects", "alpha", attachmentsDirName, strings.Repeat("b", 64))
			tracked := filepath.Join(taskDir, "tracked.txt")
			writeFile(t, tracked, "tracked")
			if tt.withSibling {
				writeFile(t, filepath.Join(taskDir, "sibling.txt"), "sibling")
			}
			if err := ledger.Record(LedgerRecord{
				TransactionID: "legacy-copy",
				Artifacts:     []LedgerArtifact{{Path: tracked, AfterExists: true, After: []byte("tracked")}},
			}); err != nil {
				t.Fatalf("Record: %v", err)
			}
			downgradeLedgerToV1(t, ledger)

			if _, err := ledger.Undo(GlobalScope()); err != nil {
				t.Fatalf("v1 Undo: %v", err)
			}
			requireAbsent(t, tracked)
			if tt.withSibling {
				if got := readFile(t, filepath.Join(taskDir, "sibling.txt")); got != "sibling" {
					t.Fatalf("unrelated sibling = %q", got)
				}
			} else {
				requireAbsent(t, taskDir)
			}

			// Undo rewrites the ledger as v2. Redo and a second undo must retain
			// the v1 cleanup provenance rather than leaving an orphan directory.
			if _, err := ledger.Redo(GlobalScope()); err != nil {
				t.Fatalf("v1 Redo after upgrade: %v", err)
			}
			if got := readFile(t, tracked); got != "tracked" {
				t.Fatalf("tracked after redo = %q", got)
			}
			if _, err := ledger.Undo(GlobalScope()); err != nil {
				t.Fatalf("second v1 Undo after upgrade: %v", err)
			}
			if tt.withSibling {
				if got := readFile(t, filepath.Join(taskDir, "sibling.txt")); got != "sibling" {
					t.Fatalf("unrelated sibling after second undo = %q", got)
				}
			} else {
				requireAbsent(t, taskDir)
			}
		})
	}
}

func TestWorkspaceLedger_NewV2RecordOnV1LedgerIsNotMarkedLegacy(t *testing.T) {
	ledger, root := newTestLedger(t)
	first := filepath.Join(root, "first.txt")
	second := filepath.Join(root, "second.txt")
	if err := ledger.Record(LedgerRecord{
		TransactionID: "legacy", Artifacts: []LedgerArtifact{{Path: first, AfterExists: true, After: []byte("first")}},
	}); err != nil {
		t.Fatalf("Record legacy seed: %v", err)
	}
	downgradeLedgerToV1(t, ledger)
	if err := ledger.Record(LedgerRecord{
		TransactionID: "current", Artifacts: []LedgerArtifact{{Path: second, AfterExists: true, After: []byte("second")}},
	}); err != nil {
		t.Fatalf("Record current entry: %v", err)
	}
	lf, err := ledger.load()
	if err != nil {
		t.Fatalf("load mixed ledger: %v", err)
	}
	if got := lf.Entries[0].LegacySchemaVersion; got != undoLedgerSchemaVersionV1 {
		t.Fatalf("legacy entry provenance = %d", got)
	}
	if got := lf.Entries[1].LegacySchemaVersion; got != 0 {
		t.Fatalf("new v2 entry incorrectly marked legacy: %d", got)
	}
}

func TestWorkspaceLedger_DirectoryPreflightRejectsTypeDriftAtomically(t *testing.T) {
	ledger, root := newTestLedger(t)
	taskDir := filepath.Join(root, "projects", "alpha", attachmentsDirName, strings.Repeat("c", 64))
	attachment := filepath.Join(taskDir, "payload.txt")
	other := filepath.Join(root, "other.txt")
	writeFile(t, other, "after")
	if err := ledger.Record(LedgerRecord{
		TransactionID: "directory-drift",
		Artifacts: []LedgerArtifact{
			{Kind: ArtifactImageKindDirectory, Path: taskDir, BeforeExists: true},
			{Path: attachment, BeforeExists: true, Before: []byte("payload")},
			{Path: other, BeforeExists: true, Before: []byte("before"), AfterExists: true, After: []byte("after")},
		},
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(taskDir), DirPermissions); err != nil {
		t.Fatalf("create attachment parent: %v", err)
	}
	if err := os.WriteFile(taskDir, []byte("not a directory"), FilePermissions); err != nil {
		t.Fatalf("write type drift: %v", err)
	}

	if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("Undo error = %v, want ErrStaleArtifact", err)
	}
	if got := readFile(t, other); got != "after" {
		t.Fatalf("unrelated file changed despite failed preflight: %q", got)
	}
	assertLedgerRestoreStacks(t, ledger, 1, 0)
}

func TestLedgerDirectoryImagesParticipateInArtifactScope(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "alpha", attachmentsDirName, strings.Repeat("d", 64))
	descendant := filepath.Join(dir, "payload.txt")
	directoryEntry := LedgerEntry{Artifacts: []ArtifactImage{{
		Kind: ArtifactImageKindDirectory, Path: dir, AfterExists: true,
	}}}
	if !entryMatchesScope(directoryEntry, ArtifactRootScope(descendant)) {
		t.Fatal("directory image did not match a descendant artifact scope")
	}
	sibling := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-other", "payload.txt")
	if entryMatchesScope(directoryEntry, ArtifactRootScope(sibling)) {
		t.Fatal("directory image matched a similarly prefixed sibling scope")
	}
}

func newAttachmentLedgerTestService(t *testing.T) (*CommitService, *WorkspaceLedger, string) {
	t.Helper()
	dataDir := t.TempDir()
	ledger := NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	service := NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	return service, ledger, dataDir
}

func commitAttachmentMove(t *testing.T, service *CommitService, src, dst *models.Project, task *models.Task) {
	t.Helper()
	if _, err := service.CommitMulti([]ChangeSet{
		{Project: src, AttachmentOps: []AttachmentOp{{Kind: AttachmentMove, Task: task, SourceProject: src, DestProject: dst}}},
		{Project: dst},
	}); err != nil {
		t.Fatalf("CommitMulti attachment move %s -> %s: %v", src.Slug, dst.Slug, err)
	}
}

func assertTaskAttachmentLocation(t *testing.T, present, absent *models.Project, task *models.Task, name, content string) {
	t.Helper()
	if got := readFile(t, GetAttachmentPath(present, task, name)); got != content {
		t.Fatalf("attachment at %s = %q, want %q", present.Slug, got, content)
	}
	requireAbsent(t, GetTaskAttachmentsDir(absent, task))
}

func downgradeLedgerToV1(t *testing.T, ledger *WorkspaceLedger) {
	t.Helper()
	lf, err := ledger.load()
	if err != nil {
		t.Fatalf("load ledger for downgrade: %v", err)
	}
	lf.SchemaVersion = undoLedgerSchemaVersionV1
	for _, stack := range [][]LedgerEntry{lf.Entries, lf.RedoEntries} {
		for entryIdx := range stack {
			stack[entryIdx].LegacySchemaVersion = 0
			for imageIdx := range stack[entryIdx].Artifacts {
				stack[entryIdx].Artifacts[imageIdx].Kind = ""
			}
		}
	}
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		t.Fatalf("marshal v1 ledger: %v", err)
	}
	if err := os.WriteFile(ledger.ledgerPath(), data, FilePermissions); err != nil {
		t.Fatalf("write v1 ledger: %v", err)
	}
}
