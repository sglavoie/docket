package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/sglavoie/docket/internal/synclog"
)

// devicePair is two devices that started from the same workspace bytes.
func devicePair(t *testing.T) (*syncWorkspace, *syncWorkspace) {
	t.Helper()
	rootA, rootB := t.TempDir(), t.TempDir()
	seedSyncWorkspace(t, rootA)
	copyTree(t, rootA, rootB)
	return openSyncWorkspace(t, rootA, "device-a"), openSyncWorkspace(t, rootB, "device-b")
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, DirPermissions)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, FilePermissions)
	})
	if err != nil {
		t.Fatalf("copy workspace: %v", err)
	}
}

// receiptState is everything that says whether device b acknowledged a
// transaction.
func receiptState(t *testing.T, w *syncWorkspace, txnID, author string) (known bool, received uint64) {
	t.Helper()
	_, known, err := w.history.Lookup(txnID)
	if err != nil {
		t.Fatal(err)
	}
	state, err := w.history.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if state != nil {
		received = state.Received[author]
	}
	return known, received
}

// TestSyncIncomingApplyAndReceiptAreAtomic applies device a's transaction on
// device b, stopping b at every point of the apply: after recovery b either
// holds the change and its receipt, or neither, and a transaction that was not
// acknowledged can still be applied afterwards while one that was is refused.
func TestSyncIncomingApplyAndReceiptAreAtomic(t *testing.T) {
	probeA, probeB := devicePair(t)
	if _, err := probeA.service.Commit(probeA.retitleAlpha("from device a")); err != nil {
		t.Fatal(err)
	}
	incoming := historyRecords(t, probeA.history)[0]
	opCount := len(crashDuring(t, func() error {
		_, err := probeB.service.CommitIncoming(incoming, probeA.history)
		return err
	}).Ops)

	for prefix := -1; prefix <= opCount; prefix++ {
		t.Run(fmt.Sprintf("crash_after_%02d", prefix), func(t *testing.T) {
			a, b := devicePair(t)
			if _, err := a.service.Commit(a.retitleAlpha("from device a")); err != nil {
				t.Fatal(err)
			}
			m := historyRecords(t, a.history)[0]
			before := syncedFiles(t, b.root)
			captured := crashDuring(t, func() error {
				_, err := b.service.CommitIncoming(m, a.history)
				return err
			})
			if len(captured.Ops) != opCount {
				t.Fatalf("apply has %d ops, the probe had %d", len(captured.Ops), opCount)
			}
			simulateCrashAfter(t, b.transactionsDir, captured, prefix)
			recoverSyncWorkspace(t, b)

			after := syncedFiles(t, b.root)
			known, received := receiptState(t, b, m.TxnID, "device-a")
			if prefix < 0 {
				if fmt.Sprint(after) != fmt.Sprint(before) {
					t.Fatal("an undurable apply changed the workspace")
				}
				if known || received != 0 {
					t.Fatalf("an unapplied transaction was acknowledged: known %v received %d", known, received)
				}
				if _, err := b.service.CommitIncoming(m, a.history); err != nil {
					t.Fatalf("applying the unacknowledged transaction again: %v", err)
				}
				after = syncedFiles(t, b.root)
				known, received = receiptState(t, b, m.TxnID, "device-a")
			}
			if fmt.Sprint(after) != fmt.Sprint(syncedFiles(t, a.root)) {
				t.Fatalf("device b does not hold device a's bytes after apply:\n a %v\n b %v", syncedFiles(t, a.root), after)
			}
			if !known || received != 1 {
				t.Fatalf("applied transaction not acknowledged: known %v received %d", known, received)
			}
			state, err := b.history.LoadState()
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(state.Heads) != fmt.Sprint([]string{m.TxnID}) || len(state.Outbox) != 0 {
				t.Fatalf("device b state heads %v outbox %v", state.Heads, state.Outbox)
			}
			stored, err := b.history.Transaction(m.TxnID)
			if err != nil || stored.TxnID != m.TxnID {
				t.Fatalf("receipt does not retain the manifest: %v", err)
			}
			if _, err := b.service.CommitIncoming(m, a.history); !errors.Is(err, synclog.ErrAlreadyApplied) {
				t.Fatalf("second apply error = %v, want ErrAlreadyApplied", err)
			}
			assertJournalClean(t, b.root)
		})
	}
}

// TestSyncIncomingRefusalsAcknowledgeNothing covers every refusal: nothing is
// applied, no receipt is written, and the tracking state is byte-identical.
func TestSyncIncomingRefusalsAcknowledgeNothing(t *testing.T) {
	reseal := func(t *testing.T, m synclog.Manifest, edit func(*synclog.Manifest)) *synclog.Manifest {
		t.Helper()
		edit(&m)
		id, err := m.ComputeTxnID()
		if err != nil {
			t.Fatal(err)
		}
		m.TxnID = id
		return &m
	}
	type refusal struct {
		name  string
		want  error
		setup func(t *testing.T, a, b *syncWorkspace) (*synclog.Manifest, synclog.BlobSource)
	}
	first := func(t *testing.T, a *syncWorkspace) *synclog.Manifest {
		t.Helper()
		if _, err := a.service.Commit(a.retitleAlpha("from device a")); err != nil {
			t.Fatal(err)
		}
		return historyRecords(t, a.history)[0]
	}
	refusals := []refusal{
		{"held_capability", synclog.ErrHeld, func(t *testing.T, a, b *syncWorkspace) (*synclog.Manifest, synclog.BlobSource) {
			m := reseal(t, *first(t, a), func(m *synclog.Manifest) { m.Requires = append(m.Requires, "zz-future.v9") })
			return m, a.history
		}},
		{"held_schema", synclog.ErrHeld, func(t *testing.T, a, b *syncWorkspace) (*synclog.Manifest, synclog.BlobSource) {
			m := reseal(t, *first(t, a), func(m *synclog.Manifest) {
				m.Schemas = map[string]int{synclog.SchemaDomainEnvelope: 1, synclog.SchemaCatalog: 22, synclog.SchemaTaskMarkdown: CurrentTaskSchemaVersion}
			})
			return m, a.history
		}},
		{"sequence_gap", synclog.ErrIncomplete, func(t *testing.T, a, b *syncWorkspace) (*synclog.Manifest, synclog.BlobSource) {
			first(t, a)
			if _, err := a.service.Commit(a.retitleAlpha("second")); err != nil {
				t.Fatal(err)
			}
			return historyRecords(t, a.history)[1], a.history
		}},
		{"missing_parent", synclog.ErrIncomplete, func(t *testing.T, a, b *syncWorkspace) (*synclog.Manifest, synclog.BlobSource) {
			m := reseal(t, *first(t, a), func(m *synclog.Manifest) { m.Parents = []string{synclog.Digest([]byte("never delivered"))} })
			return m, a.history
		}},
		{"missing_blob", synclog.ErrIncomplete, func(t *testing.T, a, b *syncWorkspace) (*synclog.Manifest, synclog.BlobSource) {
			return first(t, a), emptyBlobs{}
		}},
		{"concurrent_local_edit", synclog.ErrNeedsMerge, func(t *testing.T, a, b *syncWorkspace) (*synclog.Manifest, synclog.BlobSource) {
			if _, err := b.service.Commit(b.retitleAlpha("from device b")); err != nil {
				t.Fatal(err)
			}
			return first(t, a), a.history
		}},
		{"foreign_workspace", synclog.ErrForeignWorkspace, func(t *testing.T, a, b *syncWorkspace) (*synclog.Manifest, synclog.BlobSource) {
			return reseal(t, *first(t, a), func(m *synclog.Manifest) { m.WorkspaceID = "another-workspace" }), a.history
		}},
		{"tampered", synclog.ErrInvalidManifest, func(t *testing.T, a, b *syncWorkspace) (*synclog.Manifest, synclog.BlobSource) {
			m := *first(t, a)
			m.Kind = synclog.KindRepair
			return &m, a.history
		}},
		{"own_transaction", synclog.ErrOwnTransaction, func(t *testing.T, a, b *syncWorkspace) (*synclog.Manifest, synclog.BlobSource) {
			if _, err := b.service.Commit(b.retitleAlpha("by device b")); err != nil {
				t.Fatal(err)
			}
			return historyRecords(t, b.history)[0], b.history
		}},
	}
	for _, r := range refusals {
		t.Run(r.name, func(t *testing.T) {
			a, b := devicePair(t)
			m, blobs := r.setup(t, a, b)
			before := syncedFiles(t, b.root)
			stateBefore, _ := os.ReadFile(b.history.StatePath())
			if _, err := b.service.CommitIncoming(m, blobs); !errors.Is(err, r.want) {
				t.Fatalf("CommitIncoming error = %v, want %v", err, r.want)
			}
			if fmt.Sprint(syncedFiles(t, b.root)) != fmt.Sprint(before) {
				t.Fatal("a refused transaction changed the workspace")
			}
			stateAfter, _ := os.ReadFile(b.history.StatePath())
			if string(stateAfter) != string(stateBefore) {
				t.Fatal("a refused transaction changed the tracking state")
			}
			if r.name != "own_transaction" {
				if known, received := receiptState(t, b, m.TxnID, "device-a"); known || received != 0 {
					t.Fatalf("a refused transaction was acknowledged: known %v received %d", known, received)
				}
			}
			assertJournalClean(t, b.root)
		})
	}
}

type emptyBlobs struct{}

func (emptyBlobs) Blob(digest string) ([]byte, error) { return nil, os.ErrNotExist }

// TestSyncIncomingKeepsCausalOrderAndConcurrentHeads applies a chain in order,
// then shows a concurrent local commit and a disjoint incoming one leave two
// heads that the next local transaction names as its parents.
func TestSyncIncomingKeepsCausalOrderAndConcurrentHeads(t *testing.T) {
	a, b := devicePair(t)
	if _, err := a.service.Commit(a.retitleAlpha("a one")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.service.Commit(a.retitleAlpha("a two")); err != nil {
		t.Fatal(err)
	}
	chain := historyRecords(t, a.history)
	for _, m := range chain {
		if _, err := b.service.CommitIncoming(m, a.history); err != nil {
			t.Fatalf("apply %d: %v", m.DeviceSeq, err)
		}
	}
	if _, err := b.service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts: []ArtifactChange{{Path: b.path("projects/beta/notes/plan.md"), Data: []byte("b edit")}},
	}); err != nil {
		t.Fatal(err)
	}
	local := historyRecords(t, b.history)[0]
	if fmt.Sprint(local.Parents) != fmt.Sprint([]string{chain[1].TxnID}) {
		t.Fatalf("device b's commit parents = %v, want device a's head", local.Parents)
	}
	if _, err := a.service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts: []ArtifactChange{{Path: a.path(".tt/planning.json"), Data: []byte(`{"today":["a"]}`)}},
	}); err != nil {
		t.Fatal(err)
	}
	concurrent := historyRecords(t, a.history)[2]
	if _, err := b.service.CommitIncoming(concurrent, a.history); err != nil {
		t.Fatalf("apply disjoint concurrent transaction: %v", err)
	}
	state, err := b.history.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	wantHeads := []string{local.TxnID, concurrent.TxnID}
	sort.Strings(wantHeads)
	if fmt.Sprint(state.Heads) != fmt.Sprint(wantHeads) {
		t.Fatalf("heads = %v, want both concurrent transactions %v", state.Heads, wantHeads)
	}
	if _, err := b.service.Commit(b.retitleAlpha("joins both")); err != nil {
		t.Fatal(err)
	}
	joined := historyRecords(t, b.history)[1]
	if fmt.Sprint(joined.Parents) != fmt.Sprint(wantHeads) {
		t.Fatalf("joining commit parents = %v, want %v", joined.Parents, wantHeads)
	}
}
