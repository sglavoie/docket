package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/synclog"
	"github.com/sglavoie/tt/internal/transaction"
)

// errInjectedCrash stands in for the process dying between journal
// preparation and application.
var errInjectedCrash = errors.New("injected crash")

const syncTestWorkspaceID = "4d9a0c1e-0000-4000-8000-00000000c0de"

var syncTestSchemas = map[string]int{
	synclog.SchemaDomainEnvelope: 1,
	synclog.SchemaCatalog:        21,
	synclog.SchemaTaskMarkdown:   CurrentTaskSchemaVersion,
}

// syncWorkspace is one device's workspace with an attached sync history.
type syncWorkspace struct {
	root            string
	transactionsDir string
	ledger          *WorkspaceLedger
	history         *synclog.History
	service         *CommitService
	alpha           *models.Project
	beta            *models.Project
}

func syncCatalog(projects map[string]string) []byte {
	type project struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}
	slugs := make([]string, 0, len(projects))
	for slug := range projects {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	list := make([]project, 0, len(slugs))
	for _, slug := range slugs {
		list = append(list, project{ID: projects[slug], Slug: slug})
	}
	data, _ := json.MarshalIndent(map[string]any{
		"schema_version": 1,
		"workspace_id":   syncTestWorkspaceID,
		"kind":           "workspace",
		"data":           map[string]any{"workspace_id": syncTestWorkspaceID, "projects": list},
	}, "", "  ")
	return data
}

// seedSyncWorkspace writes the shared starting workspace with no history: a
// catalog naming alpha and beta, both projects' tasks.md, and a beta note.
func seedSyncWorkspace(t *testing.T, root string) {
	t.Helper()
	writeFile(t, filepath.Join(root, ".tt", "workspace.json"), string(syncCatalog(map[string]string{"alpha": "project-alpha", "beta": "project-beta"})))
	writeFile(t, filepath.Join(root, ".tt", "planning.json"), `{"today":[]}`)
	seed := NewCommitService(filepath.Join(root, ".tt", "transactions"), []string{root})
	for slug, id := range map[string]string{"alpha": "project-alpha", "beta": "project-beta"} {
		project := newCommitTestProject(t, root, slug, slug+" task")
		project.ID = id
		if _, err := seed.Commit(ChangeSet{Project: project}); err != nil {
			t.Fatalf("seed %s: %v", slug, err)
		}
	}
	writeFile(t, filepath.Join(root, "projects", "beta", "notes", "plan.md"), "beta plan")
}

func openSyncWorkspace(t *testing.T, root, device string) *syncWorkspace {
	t.Helper()
	history, err := synclog.New(synclog.Config{
		DataDir:  root,
		DeviceID: device,
		Schemas:  syncTestSchemas,
		Now:      func() time.Time { return time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("synclog.New: %v", err)
	}
	transactionsDir := filepath.Join(root, ".tt", "transactions")
	ledger := NewWorkspaceLedger(filepath.Join(root, ".tt", "undo"), []string{root})
	service := NewCommitServiceWithLedger(transactionsDir, []string{root}, ledger).WithSyncHistory(history)
	w := &syncWorkspace{root: root, transactionsDir: transactionsDir, ledger: ledger, history: history, service: service}
	w.alpha = w.loadProject(t, "alpha", "project-alpha")
	w.beta = w.loadProject(t, "beta", "project-beta")
	return w
}

func newSyncWorkspace(t *testing.T) *syncWorkspace {
	t.Helper()
	root := t.TempDir()
	seedSyncWorkspace(t, root)
	return openSyncWorkspace(t, root, "device-a")
}

func (w *syncWorkspace) loadProject(t *testing.T, slug, id string) *models.Project {
	t.Helper()
	project, err := models.NewProject(id, slug, "")
	if err != nil {
		t.Fatal(err)
	}
	project.Slug = slug
	project.Path = filepath.Join(w.root, "projects", slug)
	if err := LoadProject(project); err != nil {
		t.Fatalf("LoadProject %s: %v", slug, err)
	}
	return project
}

func (w *syncWorkspace) path(rel string) string {
	return filepath.Join(w.root, filepath.FromSlash(rel))
}

func (w *syncWorkspace) retitleAlpha(title string) ChangeSet {
	project := w.alpha
	project.Tasks[0].Title = title
	return ChangeSet{Project: project, UndoAction: "retitle"}
}

// syncedFiles is the digest of every synced file in the workspace, by
// workspace-relative path: what another device would have to agree with.
func syncedFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		_, _, local, classifyErr := synclog.Classify(rel)
		if classifyErr != nil {
			return classifyErr
		}
		if local {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		out[rel] = synclog.Digest(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk synced files: %v", err)
	}
	return out
}

// historyRecords lists every transaction the device has recorded, oldest
// first, from its tracking state's view of the outbox.
func historyRecords(t *testing.T, h *synclog.History) []*synclog.Manifest {
	t.Helper()
	pending, err := h.Pending()
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	return pending
}

// assertHistoryExplains holds the central invariant: the change between before
// and after is exactly what the newest record's effects describe, every digest
// it names is retained, and the tracking state agrees with the workspace.
func assertHistoryExplains(t *testing.T, w *syncWorkspace, before, after map[string]string, record *synclog.Manifest) {
	t.Helper()
	changed := map[string]bool{}
	for rel, digest := range after {
		if before[rel] != digest {
			changed[rel] = true
		}
	}
	for rel := range before {
		if _, ok := after[rel]; !ok {
			changed[rel] = true
		}
	}
	described := map[string]bool{}
	for _, effect := range record.Effects {
		described[effect.Path] = true
		switch effect.Op {
		case synclog.OpPut:
			if after[effect.Path] != *effect.Blob {
				t.Fatalf("put %s names %s, workspace holds %s", effect.Path, *effect.Blob, after[effect.Path])
			}
		case synclog.OpDelete:
			if _, ok := after[effect.Path]; ok {
				t.Fatalf("delete %s, but the workspace still holds it", effect.Path)
			}
		case synclog.OpMove:
			described[effect.TargetPath] = true
			if after[effect.TargetPath] != *effect.Blob {
				t.Fatalf("move to %s names %s, workspace holds %s", effect.TargetPath, *effect.Blob, after[effect.TargetPath])
			}
		}
		if effect.BaseBlob != nil {
			if before[effect.Path] != *effect.BaseBlob {
				t.Fatalf("%s base %s, but the replaced bytes were %s", effect.Path, *effect.BaseBlob, before[effect.Path])
			}
			if _, err := w.history.Blob(*effect.BaseBlob); err != nil {
				t.Fatalf("base blob of %s not retained: %v", effect.Path, err)
			}
		} else if _, existed := before[effect.Path]; existed {
			t.Fatalf("%s existed before but its effect names no base", effect.Path)
		}
		if effect.Blob != nil {
			if _, err := w.history.Blob(*effect.Blob); err != nil {
				t.Fatalf("blob of %s not retained: %v", effect.Path, err)
			}
		}
	}
	for rel := range changed {
		if !described[rel] {
			t.Fatalf("committed change to %s has no effect in record %s", rel, record.TxnID)
		}
	}
	for rel := range described {
		if !changed[rel] {
			t.Fatalf("record %s describes %s, which did not change", record.TxnID, rel)
		}
	}
	state, err := w.history.LoadState()
	if err != nil || state == nil {
		t.Fatalf("LoadState = %v, %v", state, err)
	}
	for rel := range changed {
		if after[rel] != state.Tracked[rel] {
			t.Fatalf("tracked %s = %q, workspace %q", rel, state.Tracked[rel], after[rel])
		}
	}
	if len(state.Heads) != 1 || state.Heads[0] != record.TxnID {
		t.Fatalf("heads = %v, want the newest record %s", state.Heads, record.TxnID)
	}
}

// crashDuring runs fn with commitJournal replaced, so the commit fn makes
// stops after its manifest is durable and before any operation applies, and
// returns that manifest.
func crashDuring(t *testing.T, fn func() error) *transaction.Manifest {
	t.Helper()
	var captured *transaction.Manifest
	previous := commitJournal
	commitJournal = func(_ *transaction.Journal, manifest *transaction.Manifest) error {
		captured = manifest
		return errInjectedCrash
	}
	err := fn()
	commitJournal = previous
	if !errors.Is(err, errInjectedCrash) {
		t.Fatalf("mutation error = %v, want the injected crash", err)
	}
	return captured
}

// simulateCrashAfter leaves the transaction as a process killed after its
// first prefix operations would: prefix < 0 never made the manifest durable,
// 0 left it prepared, and n > 0 marked it committing and applied n operations.
func simulateCrashAfter(t *testing.T, transactionsDir string, manifest *transaction.Manifest, prefix int) {
	t.Helper()
	path := filepath.Join(transactionsDir, manifest.ID+".json")
	if prefix < 0 {
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove manifest: %v", err)
		}
		return
	}
	if prefix == 0 {
		return
	}
	manifest.State = transaction.StateCommitting
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, FilePermissions); err != nil {
		t.Fatalf("write committing manifest: %v", err)
	}
	for i := 0; i < prefix; i++ {
		op := manifest.Ops[i]
		var applyErr error
		switch op.Kind {
		case transaction.OpWrite:
			applyErr = os.Rename(op.Staged, op.Target)
		case transaction.OpMove:
			applyErr = os.Rename(op.Source, op.Target)
		case transaction.OpDelete:
			if applyErr = os.Remove(op.Target); os.IsNotExist(applyErr) {
				applyErr = nil
			}
		}
		if applyErr != nil {
			t.Fatalf("apply op %d (%s %s): %v", i, op.Kind, op.Target, applyErr)
		}
	}
}

func recoverSyncWorkspace(t *testing.T, w *syncWorkspace) {
	t.Helper()
	report, err := w.service.RecoverWorkspace()
	if err != nil {
		t.Fatalf("RecoverWorkspace: %v", err)
	}
	if report.Blocked() {
		t.Fatalf("recovery quarantined: %+v", report.Quarantined)
	}
	assertJournalClean(t, w.root)
}

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
