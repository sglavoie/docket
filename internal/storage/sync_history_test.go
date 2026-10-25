package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/synclog"
	"github.com/sglavoie/docket/internal/transaction"
)

// localMutation is one entry point of the common commit boundary.
type localMutation struct {
	name string
	kind synclog.Kind
	// prepare runs before the measured mutation, outside crash injection.
	prepare func(t *testing.T, w *syncWorkspace)
	run     func(w *syncWorkspace) error
}

func localMutations() []localMutation {
	return []localMutation{
		{
			name: "commit_multi", kind: synclog.KindUser,
			run: func(w *syncWorkspace) error {
				cs := w.retitleAlpha("alpha retitled")
				cs.Artifacts = []ArtifactChange{{Path: w.path("projects/alpha/notes/new.md"), Data: []byte("fresh note")}}
				beta := w.beta
				beta.Tasks[0].Title = "beta retitled"
				_, err := w.service.CommitMulti([]ChangeSet{cs, {Project: beta}})
				return err
			},
		},
		{
			name: "artifact_transaction", kind: synclog.KindUser,
			run: func(w *syncWorkspace) error {
				_, err := w.service.CommitArtifactTransaction(ArtifactCommitRequest{
					Artifacts: []ArtifactChange{
						{Path: w.path(".tt/planning.json"), Data: []byte(`{"today":["x"]}`), BackupPath: w.path(".tt/planning.json.bak")},
						{Path: w.path("projects/beta/notes/plan.md"), Delete: true},
						{Path: w.path(".tt/global-notes/inbox.md"), Data: []byte("global")},
					},
					ProjectIDs: []string{"project-beta"},
					UndoAction: "artifacts",
				})
				return err
			},
		},
		{
			name: "config_artifacts", kind: synclog.KindUser,
			run: func(w *syncWorkspace) error {
				_, err := w.service.CommitArtifacts([]ArtifactChange{{
					Path: w.path(".tt/lenses.json"), Data: []byte(`{"lenses":[]}`),
				}})
				return err
			},
		},
		{
			name: "ops_transaction_rename", kind: synclog.KindUser,
			run: func(w *syncWorkspace) error {
				_, err := w.service.CommitOpsTransaction(OpsCommitRequest{
					Ops: []transaction.Op{{Kind: transaction.OpMove, Source: w.path("projects/beta"), Target: w.path("projects/gamma")}},
					Artifacts: []ArtifactChange{{
						Path: w.path(".tt/workspace.json"),
						Data: syncCatalog(map[string]string{"alpha": "project-alpha", "gamma": "project-beta"}),
					}},
					ProjectIDs: []string{"project-beta"},
					UndoAction: "rename",
				})
				return err
			},
		},
		{
			name: "undo", kind: synclog.KindUndo,
			prepare: func(t *testing.T, w *syncWorkspace) {
				if _, err := w.service.Commit(w.retitleAlpha("before undo")); err != nil {
					t.Fatalf("seed undoable commit: %v", err)
				}
			},
			run: func(w *syncWorkspace) error {
				_, err := w.ledger.Undo(GlobalScope())
				return err
			},
		},
	}
}

// TestSyncLocalCommitAndOutboxAreAtomic drives every commit-boundary entry
// point, stops the process at every point of its journal transaction, and
// recovers: the user change and its outgoing record land together or not at
// all, and once landed the record describes exactly what changed.
func TestSyncLocalCommitAndOutboxAreAtomic(t *testing.T) {
	for _, mutation := range localMutations() {
		t.Run(mutation.name, func(t *testing.T) {
			probe := newSyncWorkspace(t)
			if mutation.prepare != nil {
				mutation.prepare(t, probe)
			}
			opCount := len(crashDuring(t, func() error { return mutation.run(probe) }).Ops)

			for prefix := -1; prefix <= opCount; prefix++ {
				t.Run(fmt.Sprintf("crash_after_%02d", prefix), func(t *testing.T) {
					w := newSyncWorkspace(t)
					if mutation.prepare != nil {
						mutation.prepare(t, w)
					}
					beforeRecords := len(historyRecords(t, w.history))
					before := syncedFiles(t, w.root)
					captured := crashDuring(t, func() error { return mutation.run(w) })
					if len(captured.Ops) != opCount {
						t.Fatalf("manifest has %d ops, the probe had %d", len(captured.Ops), opCount)
					}
					simulateCrashAfter(t, w.transactionsDir, captured, prefix)
					recoverSyncWorkspace(t, w)

					after := syncedFiles(t, w.root)
					records := historyRecords(t, w.history)
					if prefix < 0 {
						if fmt.Sprint(after) != fmt.Sprint(before) {
							t.Fatalf("an undurable transaction changed the workspace")
						}
						if len(records) != beforeRecords {
							t.Fatalf("an undurable transaction left %d records, want %d", len(records), beforeRecords)
						}
						return
					}
					if len(records) != beforeRecords+1 {
						t.Fatalf("recovered commit has %d records, want %d", len(records), beforeRecords+1)
					}
					record := records[len(records)-1]
					if record.Kind != mutation.kind || record.DeviceID != "device-a" || record.WorkspaceID != syncTestWorkspaceID {
						t.Fatalf("record = kind %s device %s workspace %s", record.Kind, record.DeviceID, record.WorkspaceID)
					}
					if record.LocalTxnID != captured.ID {
						t.Fatalf("record cites journal %s, the commit was %s", record.LocalTxnID, captured.ID)
					}
					if int(record.DeviceSeq) != beforeRecords+1 {
						t.Fatalf("device_seq = %d, want %d", record.DeviceSeq, beforeRecords+1)
					}
					assertHistoryExplains(t, w, before, after, record)
				})
			}
		})
	}
}

// TestSyncLocalRecordsNameProjectsAndAncestry checks the record content the
// atomicity test does not: project identities follow the catalog across a
// rename, every transaction descends from the one before, and an identical
// rewrite records nothing.
func TestSyncLocalRecordsNameProjectsAndAncestry(t *testing.T) {
	w := newSyncWorkspace(t)
	if _, err := w.service.Commit(w.retitleAlpha("first")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.service.Commit(ChangeSet{Project: w.alpha}); err != nil {
		t.Fatalf("identical rewrite: %v", err)
	}
	if _, err := w.service.CommitOpsTransaction(OpsCommitRequest{
		Ops: []transaction.Op{{Kind: transaction.OpMove, Source: w.path("projects/beta"), Target: w.path("projects/gamma")}},
		Artifacts: []ArtifactChange{{
			Path: w.path(".tt/workspace.json"),
			Data: syncCatalog(map[string]string{"alpha": "project-alpha", "gamma": "project-beta"}),
		}},
		UndoAction: "rename",
	}); err != nil {
		t.Fatal(err)
	}
	records := historyRecords(t, w.history)
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2 (the identical rewrite records nothing)", len(records))
	}
	first, rename := records[0], records[1]
	if len(first.Parents) != 0 || first.DeviceSeq != 1 {
		t.Fatalf("first record parents %v seq %d", first.Parents, first.DeviceSeq)
	}
	if len(rename.Parents) != 1 || rename.Parents[0] != first.TxnID || rename.DeviceSeq != 2 {
		t.Fatalf("rename record parents %v seq %d, want [%s] 2", rename.Parents, rename.DeviceSeq, first.TxnID)
	}
	if first.Effects[0].Kind != synclog.EffectProjectDocument || first.Effects[0].ProjectID != "project-alpha" {
		t.Fatalf("first effect = %+v", first.Effects[0])
	}
	moves := 0
	for _, effect := range rename.Effects {
		if effect.Op == synclog.OpMove {
			moves++
			if effect.ProjectID != "project-beta" {
				t.Fatalf("moved %s names project %q, want the renamed project's identity", effect.Path, effect.ProjectID)
			}
		}
	}
	if moves != 2 {
		t.Fatalf("rename moved %d files, want tasks.md and the note: %+v", moves, rename.Effects)
	}
	wantRequires := []string{fmt.Sprintf("catalog.v%d", 21), "notes.v1", fmt.Sprintf("project-document.v%d", CurrentTaskSchemaVersion)}
	if fmt.Sprint(rename.Requires) != fmt.Sprint(wantRequires) {
		t.Fatalf("requires = %v, want %v", rename.Requires, wantRequires)
	}
	data, err := os.ReadFile(w.history.IdentityPath())
	if err != nil {
		t.Fatalf("device identity not recorded: %v", err)
	}
	var identity map[string]any
	if err := json.Unmarshal(data, &identity); err != nil || identity["device_id"] != "device-a" || identity["workspace_id"] != syncTestWorkspaceID {
		t.Fatalf("device identity = %s (%v)", data, err)
	}
}

// TestSyncLocalCommitRefusesUnclassifiedPaths keeps the complete-effects rule:
// a commit whose change the history cannot name is refused whole.
func TestSyncLocalCommitRefusesUnclassifiedPaths(t *testing.T) {
	w := newSyncWorkspace(t)
	before := syncedFiles(t, w.root)
	cs := w.retitleAlpha("never lands")
	cs.Artifacts = []ArtifactChange{{Path: w.path("stray.txt"), Data: []byte("?")}}
	if _, err := w.service.Commit(cs); !errors.Is(err, synclog.ErrUnclassifiedPath) {
		t.Fatalf("Commit error = %v, want ErrUnclassifiedPath", err)
	}
	if fmt.Sprint(syncedFiles(t, w.root)) != fmt.Sprint(before) {
		t.Fatal("a refused commit changed the workspace")
	}
	if _, err := os.Stat(w.path("stray.txt")); !os.IsNotExist(err) {
		t.Fatalf("refused artifact landed: %v", err)
	}
	if records := historyRecords(t, w.history); len(records) != 0 {
		t.Fatalf("a refused commit left %d records", len(records))
	}
}

// TestSyncPublicationConfirmationDrainsTheOutbox shows outgoing tracking is
// durable and idempotent: confirming removes a record from the outbox once,
// and the record itself stays retained.
func TestSyncPublicationConfirmationDrainsTheOutbox(t *testing.T) {
	w := newSyncWorkspace(t)
	for _, title := range []string{"one", "two"} {
		if _, err := w.service.Commit(w.retitleAlpha(title)); err != nil {
			t.Fatal(err)
		}
	}
	records := historyRecords(t, w.history)
	if len(records) != 2 {
		t.Fatalf("outbox = %d, want 2", len(records))
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := w.service.CommitSyncPublished([]string{records[0].TxnID}); err != nil {
			t.Fatalf("CommitSyncPublished: %v", err)
		}
	}
	remaining := historyRecords(t, w.history)
	if len(remaining) != 1 || remaining[0].TxnID != records[1].TxnID {
		t.Fatalf("outbox after confirmation = %v", remaining)
	}
	if _, err := w.history.Transaction(records[0].TxnID); err != nil {
		t.Fatalf("published record is no longer retained: %v", err)
	}
	assertJournalClean(t, w.root)
}
