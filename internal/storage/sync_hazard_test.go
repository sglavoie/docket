package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/transaction"
)

// The tests in this file describe what happens to durable state when a workspace
// lives inside a directory a sync client copies between machines. None of them
// can be written as a scenario: the scenario driver has one app and one virtual
// clock, so it can express one machine's timeline but never two machines racing
// over a third party's file copies.
//
// The recurring mechanism is that advisory locking is machine-local. flock
// serializes two processes on one machine perfectly and says nothing at all about
// a second machine, which holds its own lock on its own copy of the lock file.
// Every hazard below follows from that.

// TestSyncedLedgerFileLosesEntriesButNeverCorrupts is the interleaved-undo hazard.
//
// The 50-entry ledger is one file, and both Record and restore load it, modify it,
// and write it back under the workspace commit lock. That lock makes the sequence
// atomic against every other process on this machine. Against a second machine it
// does nothing: both machines load the same ledger, both append, and the sync
// client keeps one file. The entry the loser appended is gone, its content blobs
// are still on disk, and Validate still passes — which is the part worth pinning
// down, because a silent loss is indistinguishable from a user having undone
// something.
func TestSyncedLedgerFileLosesEntriesButNeverCorrupts(t *testing.T) {
	ledger, root := newTestLedger(t)
	first := filepath.Join(root, "projects", "alpha", "tasks.md")
	second := filepath.Join(root, "projects", "beta", "tasks.md")

	mustRecord(t, ledger, "txn-shared", "alpha", first, "a1", "a2")

	// The bytes a second machine loaded before it appended anything. A sync client
	// that resolves in favor of that machine writes exactly this back.
	staleLedger, err := os.ReadFile(ledger.ledgerPath())
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}

	mustRecord(t, ledger, "txn-lost", "beta", second, "b1", "b2")
	if count, countErr := ledger.UndoCount(GlobalScope()); countErr != nil || count != 2 {
		t.Fatalf("UndoCount before the sync = (%d, %v), want (2, nil)", count, countErr)
	}
	lostBlob := blobPathFor(ledger, "b1")
	if _, statErr := os.Stat(lostBlob); statErr != nil {
		t.Fatalf("second entry's blob should exist before the sync: %v", statErr)
	}

	// The sync client wins the file.
	if writeErr := os.WriteFile(ledger.ledgerPath(), staleLedger, 0o600); writeErr != nil {
		t.Fatalf("apply synced ledger: %v", writeErr)
	}

	count, err := ledger.UndoCount(GlobalScope())
	if err != nil {
		t.Fatalf("UndoCount after the sync: %v", err)
	}
	if count != 1 {
		t.Fatalf("UndoCount after the sync = %d, want 1: the second machine's entry is lost", count)
	}
	// The loss is silent by construction: the file is well formed, every blob it
	// still references is present, and the orphaned blob is invisible to validation.
	if _, err := ledger.Validate(); err != nil {
		t.Fatalf("Validate after the sync = %v, want the truncated ledger to read as healthy", err)
	}
	if _, err := os.Stat(lostBlob); err != nil {
		t.Fatalf("the lost entry's blob should survive as garbage: %v", err)
	}
	// The surviving entry is still fully usable, which is why nothing reports the
	// loss: undo works, it just reaches one step less far back.
	writeFile(t, first, "a2")
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo of the surviving entry: %v", err)
	}
	if got := readFile(t, first); got != "a1" {
		t.Fatalf("after undo = %q, want a1", got)
	}
}

// TestRestoreFailsClosedWhenAnotherMachineChangedTheFile is the other half of the
// interleave: the ledger file can lose entries, but an entry that survives can
// never apply a stale image on top of another machine's write.
//
// preflightRestoreImage compares the on-disk content against the image the entry
// expects to find before it plans anything. The second machine's edit makes that
// comparison fail, so the restore is refused whole rather than overwriting work it
// has no record of, and the entry that can no longer apply is retired.
func TestRestoreFailsClosedWhenAnotherMachineChangedTheFile(t *testing.T) {
	ledger, root := newTestLedger(t)
	target := filepath.Join(root, "projects", "alpha", "tasks.md")
	writeFile(t, target, "a2")
	mustRecord(t, ledger, "txn-one", "alpha", target, "a1", "a2")

	// The other machine's edit arrives through the sync client. The ledger entry
	// still expects to find "a2".
	writeFile(t, target, "written by the other machine")

	_, err := ledger.Undo(GlobalScope())
	if err == nil {
		t.Fatal("Undo overwrote another machine's write instead of refusing")
	}
	if got := readFile(t, target); got != "written by the other machine" {
		t.Fatalf("target content = %q, want the other machine's write untouched", got)
	}
	// The ledger holds no record of the other machine's write, so the file can
	// never return to the image this entry expects. The refusal retires it rather
	// than leave it selected ahead of every older entry.
	var retired *RetiredEntryError
	if !errors.As(err, &retired) {
		t.Fatalf("Undo error = %v, want the entry retired", err)
	}
	if count, countErr := ledger.UndoCount(GlobalScope()); countErr != nil || count != 0 {
		t.Fatalf("UndoCount after the refusal = (%d, %v), want (0, nil)", count, countErr)
	}
}

// TestConcurrentLedgerRecordsSerializeOnOneMachine is the control for the two
// tests above. On one machine the workspace commit lock is real, so the same
// interleave loses nothing: every entry survives and the file is never torn. Run
// under -race this is also the proof that two ledger handles over one directory
// share no mutable state.
func TestConcurrentLedgerRecordsSerializeOnOneMachine(t *testing.T) {
	ledger, root := newTestLedger(t)
	other := NewWorkspaceLedger(ledger.dir, ledger.roots)

	const rounds = 8
	var wg sync.WaitGroup
	errs := make(chan error, 2*rounds)
	record := func(l *WorkspaceLedger, prefix string) {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			path := filepath.Join(root, "projects", prefix, "tasks.md")
			if err := l.Record(LedgerRecord{
				TransactionID: prefix + string(rune('a'+i)),
				ProjectIDs:    []string{prefix},
				Artifacts: []LedgerArtifact{{
					Path: path, BeforeExists: true, Before: []byte(prefix), AfterExists: true,
					After: []byte(prefix + string(rune('a'+i))),
				}},
			}); err != nil {
				errs <- err
				return
			}
		}
	}
	wg.Add(2)
	go record(ledger, "alpha")
	go record(other, "beta")
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent ledger record: %v", err)
	}

	count, err := ledger.UndoCount(GlobalScope())
	if err != nil {
		t.Fatalf("UndoCount: %v", err)
	}
	if count != 2*rounds {
		t.Fatalf("UndoCount = %d, want %d: one machine's lock must lose nothing", count, 2*rounds)
	}
	if _, err := ledger.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// TestOlderBinaryRefusesNewerUndoLedger is the older-binary hazard for the undo
// ledger's own version guard.
//
// Two machines can run different builds against one synced workspace, and the
// ledger carries its own schema version independent of the catalog's. A newer
// build's ledger must be refused rather than read as if the fields the older build
// does not know about were absent — the alternative is an undo that restores a
// partial image. The guard is a refusal in every reader, including the read-only
// validation the doctor calls.
func TestOlderBinaryRefusesNewerUndoLedger(t *testing.T) {
	ledger, root := newTestLedger(t)
	target := filepath.Join(root, "projects", "alpha", "tasks.md")
	writeFile(t, target, "a2")
	mustRecord(t, ledger, "txn-one", "alpha", target, "a1", "a2")

	bumpLedgerSchemaVersion(t, ledger.ledgerPath(), UndoLedgerSchemaVersion+1)

	if _, err := ledger.Validate(); err == nil {
		t.Fatal("Validate accepted a ledger from a newer schema")
	} else if !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("Validate error = %v, want it to name the version mismatch", err)
	}
	if _, _, err := ledger.SelectNewestUndo(GlobalScope()); err == nil {
		t.Fatal("SelectNewestUndo accepted a ledger from a newer schema")
	}
	if _, err := ledger.Undo(GlobalScope()); err == nil {
		t.Fatal("Undo applied an entry from a ledger it cannot read")
	}
	if got := readFile(t, target); got != "a2" {
		t.Fatalf("target content = %q, want it untouched by the refused undo", got)
	}
	// A refusal must also not append: recording against an unreadable ledger would
	// rewrite it at this build's version and silently discard whatever the newer
	// build stored.
	if err := ledger.Record(LedgerRecord{
		TransactionID: "txn-two",
		ProjectIDs:    []string{"alpha"},
		Artifacts:     []LedgerArtifact{{Path: target, BeforeExists: true, Before: []byte("a2"), AfterExists: true, After: []byte("a3")}},
	}); err == nil {
		t.Fatal("Record rewrote a ledger written by a newer build")
	}
}

// bumpLedgerSchemaVersion rewrites only the version field, leaving every entry
// intact: the point is a file this build cannot vouch for, not a malformed one.
func bumpLedgerSchemaVersion(t *testing.T, path string, version int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var raw map[string]any
	if decodeErr := json.Unmarshal(data, &raw); decodeErr != nil {
		t.Fatalf("decode ledger: %v", decodeErr)
	}
	raw["schema_version"] = version
	next, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("encode ledger: %v", err)
	}
	if err := os.WriteFile(path, next, 0o600); err != nil {
		t.Fatalf("write ledger: %v", err)
	}
}

// TestRecoverWorkspaceLeavesAnotherMachinesLiveTransactionAlone is the storage
// half of the sync-copy-mid-transaction hazard, same-path form: both machines
// keep the workspace at the same absolute path, so the staged bytes and the
// manifest the sync client copies name paths this machine would happily write.
//
// The commit service must not roll them forward. The transaction is not
// interrupted, it is in progress somewhere else, and finishing it here would apply
// bytes whose owner may still abort.
func TestRecoverWorkspaceLeavesAnotherMachinesLiveTransactionAlone(t *testing.T) {
	root := t.TempDir()
	transactionsDir := filepath.Join(root, ".docket", "transactions")
	target := filepath.Join(root, "projects", "alpha", "tasks.md")

	owner := transaction.NewJournal(transactionsDir, []string{root}, "machine-a")
	manifest, err := owner.Prepare([]transaction.Op{{
		Kind: transaction.OpWrite, Target: target, Data: []byte("owner bytes"), Mode: FilePermissions,
	}})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	here := NewCommitService(transactionsDir, []string{root}).OnMachine("machine-b")
	report, err := here.RecoverWorkspace()
	if err != nil {
		t.Fatalf("RecoverWorkspace: %v", err)
	}
	if len(report.ForeignLive) != 1 || report.ForeignLive[0].Machine != "machine-a" {
		t.Fatalf("ForeignLive = %+v, want the one transaction owned by machine-a", report.ForeignLive)
	}
	if len(report.RolledForward) != 0 {
		t.Fatalf("RolledForward = %v, want nothing applied", report.RolledForward)
	}
	if report.Blocked() {
		t.Fatalf("another machine's live transaction must not block this one: %+v", report.Quarantined)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target stat = %v, want it never written", err)
	}
	// The owner can still finish, which is the whole reason for leaving it alone.
	if err := owner.Commit(manifest); err != nil {
		t.Fatalf("owner Commit: %v", err)
	}
	if got := readFile(t, target); got != "owner bytes" {
		t.Fatalf("target content = %q, want the owner's bytes", got)
	}
}

// TestRecoverWorkspaceIgnoresLiveForeignTransactionFromAnotherPath is the
// different-path form. The other machine keeps the workspace somewhere this
// machine's approved roots do not cover, so every path in its manifest is one this
// machine must refuse to touch.
//
// A stale transaction like that is quarantined, and quarantine blocks every
// mutation in the workspace. A live one must not be, or the second machine would be
// frozen for as long as the first machine is working — which is most of the time.
func TestRecoverWorkspaceIgnoresLiveForeignTransactionFromAnotherPath(t *testing.T) {
	root := t.TempDir()
	elsewhere := t.TempDir()
	transactionsDir := filepath.Join(root, ".docket", "transactions")

	// The other machine's journal writes its manifest into the synced transactions
	// directory while staging bytes beside its own copy of the workspace.
	foreign := transaction.NewJournal(transactionsDir, []string{elsewhere}, "machine-a")
	if _, err := foreign.Prepare([]transaction.Op{{
		Kind:   transaction.OpWrite,
		Target: filepath.Join(elsewhere, "projects", "alpha", "tasks.md"),
		Data:   []byte("other path"),
		Mode:   FilePermissions,
	}}); err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	here := NewCommitService(transactionsDir, []string{root}).OnMachine("machine-b")
	report, err := here.RecoverWorkspace()
	if err != nil {
		t.Fatalf("RecoverWorkspace: %v", err)
	}
	if report.Blocked() {
		t.Fatalf("a live foreign transaction must not quarantine: %+v", report.Quarantined)
	}
	if len(report.ForeignLive) != 1 {
		t.Fatalf("ForeignLive = %+v, want one entry", report.ForeignLive)
	}
	// Mutations still work here, which is the property being protected.
	if _, err := here.CommitArtifacts([]ArtifactChange{{
		Path: filepath.Join(root, "projects", "alpha", "notes.md"), Data: []byte("local work"), Mode: FilePermissions,
	}}); err != nil {
		t.Fatalf("commit while another machine is mid-transaction: %v", err)
	}
}

// TestSyncedProjectLockDoesNotWedgeTheSecondMachine covers the lock files a sync
// client copies between machines.
//
// flock state is kernel state, not file content, so a lock file that arrives
// through a sync client is an ordinary unlocked file no matter what the machine it
// came from was doing with it. The file's presence and its modification time carry
// no authority, and nothing may treat either as a held lock — otherwise a synced
// workspace would arrive permanently unwritable. The lock file is also never
// removed, so a project that has ever been written owns one forever.
func TestSyncedProjectLockDoesNotWedgeTheSecondMachine(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "projects", "alpha")
	writeFile(t, filepath.Join(projectDir, "tasks.md"), "# Tasks\n")

	// The lock file as it arrives from the other machine: present, and stamped long
	// before this machine ever opened the workspace.
	lockPath := filepath.Join(projectDir, LockFileName)
	writeFile(t, lockPath, "")
	stale := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(lockPath, stale, stale); err != nil {
		t.Fatalf("age the synced lock file: %v", err)
	}

	transactionsDir := filepath.Join(root, ".docket", "transactions")
	service := NewCommitService(transactionsDir, []string{root}).OnMachine("machine-b")
	if _, err := service.CommitArtifacts([]ArtifactChange{{
		Path: filepath.Join(projectDir, "tasks.md"), Data: []byte("# Tasks\n\n## Task\n"), Mode: FilePermissions,
	}}); err != nil {
		t.Fatalf("commit with a synced lock file present: %v", err)
	}
	if got := readFile(t, filepath.Join(projectDir, "tasks.md")); !strings.Contains(got, "## Task") {
		t.Fatalf("tasks.md = %q, want the commit applied", got)
	}
	// The lock file survives the commit: locking never removes it, which is why a
	// diagnostic reports its presence as information rather than as a defect.
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file should still exist after the commit: %v", err)
	}
}

// TestAttachmentSkewIsReportedInBothDirections is the attachment hazard. The two
// halves of an attachment are stored apart — the metadata entry inside tasks.md,
// the bytes in a per-task directory beside it — so a sync client that has copied
// one but not the other leaves them disagreeing in whichever direction it got to
// first.
//
// Both directions have to be visible and they must not be conflated: metadata with
// no bytes is dead to every reader, while bytes with no metadata may be the only
// surviving copy of the file and are never a thing to delete.
func TestAttachmentSkewIsReportedInBothDirections(t *testing.T) {
	root := t.TempDir()
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: filepath.Join(root, "projects", "alpha")}
	present := newAuditTask(t, "11111111-1111-4111-8111-111111111111", 1, "has bytes")
	missing := newAuditTask(t, "22222222-2222-4222-8222-222222222222", 2, "has metadata only")
	project.Tasks = []*models.Task{present, missing}

	// tasks.md arrived for both tasks. Only the first task's bytes did.
	present.Attachments = []models.Attachment{{Filename: "diagram.png", OriginalPath: "/tmp/diagram.png"}}
	missing.Attachments = []models.Attachment{{Filename: "spec.pdf", OriginalPath: "/tmp/spec.pdf"}}
	if err := EnsureTaskAttachmentsDir(project, present); err != nil {
		t.Fatalf("EnsureTaskAttachmentsDir: %v", err)
	}
	presentDir, err := ResolveTaskAttachmentsDir(project, present)
	if err != nil {
		t.Fatalf("ResolveTaskAttachmentsDir: %v", err)
	}
	writeFile(t, filepath.Join(presentDir, "diagram.png"), "png bytes")
	// Bytes for a file no metadata entry names: the other machine added the
	// attachment and this machine's tasks.md has not caught up.
	writeFile(t, filepath.Join(presentDir, "arrived-early.png"), "png bytes")
	// A whole directory for a task this machine's tasks.md no longer holds.
	orphanDir := filepath.Join(project.Path, ".attachments", "0"+strings.Repeat("f", 63))
	writeFile(t, filepath.Join(orphanDir, "deleted-task.txt"), "bytes")

	findings, err := InspectProjectAttachments(project)
	if err != nil {
		t.Fatalf("InspectProjectAttachments: %v", err)
	}

	var missingBytes, orphanedBytes []string
	for _, finding := range findings {
		if finding.Orphaned {
			orphanedBytes = append(orphanedBytes, finding.Detail)
		} else {
			missingBytes = append(missingBytes, finding.Detail)
		}
	}
	if len(missingBytes) != 1 || !strings.Contains(missingBytes[0], "spec.pdf") {
		t.Fatalf("metadata-without-bytes findings = %v, want exactly the spec.pdf entry", missingBytes)
	}
	joinedOrphans := strings.Join(orphanedBytes, "|")
	if !strings.Contains(joinedOrphans, "arrived-early.png") {
		t.Fatalf("orphan findings = %v, want the unnamed file", orphanedBytes)
	}
	if !strings.Contains(joinedOrphans, "deleted-task.txt") && !strings.Contains(joinedOrphans, "1 file") {
		t.Fatalf("orphan findings = %v, want the unowned directory", orphanedBytes)
	}
	// The attachment that is intact must not be reported at all, or every synced
	// workspace would read as damaged.
	if strings.Contains(strings.Join(append(missingBytes, orphanedBytes...), "|"), "diagram.png") {
		t.Fatalf("findings named the healthy attachment: %v", findings)
	}
}

// newAuditTask builds a valid task with a stable ID, so the attachment directory
// name a test computes is the one the production resolver computes.
func newAuditTask(t *testing.T, id string, shortID int, title string) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatalf("NewGenericTask(%q): %v", title, err)
	}
	return task
}
