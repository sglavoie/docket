package storage

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/synclog"
	"github.com/sglavoie/docket/internal/transaction"
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
	writeFile(t, filepath.Join(root, ".docket", "workspace.json"), string(syncCatalog(map[string]string{"alpha": "project-alpha", "beta": "project-beta"})))
	writeFile(t, filepath.Join(root, ".docket", "planning.json"), `{"today":[]}`)
	seed := NewCommitService(filepath.Join(root, ".docket", "transactions"), []string{root})
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
	transactionsDir := filepath.Join(root, ".docket", "transactions")
	ledger := NewWorkspaceLedger(filepath.Join(root, ".docket", "undo"), []string{root})
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
