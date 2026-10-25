package syncmerge_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/bridge"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/synclog"
)

// Session 6: every rich artifact a workspace holds survives the round trip
// through sync; changes made outside a commit are imported when they are valid
// and held, intact and unpublished, when they are not; and undo is a
// device-local, revision-checked new change that can never restore an old
// image over another device's work.

const (
	atlasSlug   = "atlas-launch"
	homeSlug    = "home-renovation"
	atlasDoc    = "projects/atlas-launch/tasks.md"
	runbookNote = "projects/atlas-launch/notes/2026-09-03-090000_0a1b2c3d-release-runbook.md"
	weeklyNote  = ".docket/global-notes/2026-09-02-180000_4e5f6a7b-weekly-review.md"
	lensesFile  = ".docket/lenses.json"
)

var richInstant = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

// index has the device index its workspace, as engine.Open does on a
// device's first launch, so later observation compares against it.
func (r *replica) index() {
	r.t.Helper()
	report, err := r.svc.CommitExternalChanges()
	if err != nil {
		r.t.Fatalf("%s: index: %v", r.device, err)
	}
	if !report.Baselined || report.Imported != nil {
		r.t.Fatalf("%s: a fresh device imported instead of indexing: %+v", r.device, report)
	}
}

// observe imports the device's external changes.
func (r *replica) observe() *storage.ExternalChanges {
	r.t.Helper()
	report, err := r.svc.CommitExternalChanges()
	if err != nil {
		r.t.Fatalf("%s: observe: %v", r.device, err)
	}
	return report
}

// write changes a workspace file outside any commit, as an editor would.
func (r *replica) write(rel string, data []byte) {
	r.t.Helper()
	if err := writeFile(r.path(rel), data); err != nil {
		r.t.Fatalf("%s: write %s: %v", r.device, rel, err)
	}
}

// commitUndoable commits artifacts as an undoable user transaction.
func (r *replica) commitUndoable(artifacts ...storage.ArtifactChange) {
	r.t.Helper()
	_, err := r.svc.CommitArtifactTransaction(storage.ArtifactCommitRequest{
		Artifacts:  artifacts,
		UndoAction: "Edit Task",
		ProjectIDs: []string{atlasID},
	})
	if err != nil {
		r.t.Fatalf("%s: undoable commit: %v", r.device, err)
	}
}

func (r *replica) retitleArtifact(slug, id, title string) storage.ArtifactChange {
	r.t.Helper()
	doc := r.doc(slug)
	findTask(r.t, doc, id).Title = title
	return r.docArtifact(slug, doc)
}

func (r *replica) task(slug, id string) *models.Task {
	r.t.Helper()
	return findTask(r.t, r.doc(slug), id)
}

// settleClean delivers everything everywhere, including to two observers that
// start from genesis, and requires identical, valid content and no conflict.
func settleClean(t *testing.T, c *cluster) []*replica {
	t.Helper()
	replicas := c.settle(2)
	if conflicts := requireConverged(t, replicas); len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %s", conflictsJSON(t, replicas[0]))
	}
	return replicas
}

func taskJSON(t *testing.T, task *models.Task) string {
	t.Helper()
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// requireTaskEverywhere holds every replica's copy of a task to want.
func requireTaskEverywhere(t *testing.T, replicas []*replica, slug, id string, want *models.Task) {
	t.Helper()
	wantJSON := taskJSON(t, want)
	for _, r := range replicas {
		if got := taskJSON(t, r.task(slug, id)); got != wantJSON {
			t.Fatalf("%s holds task %s as\n%s\nwant\n%s", r.device, id, got, wantJSON)
		}
	}
}

func requireFileEverywhere(t *testing.T, replicas []*replica, rel string, want []byte) {
	t.Helper()
	for _, r := range replicas {
		if got := r.read(rel); !bytes.Equal(got, want) {
			t.Fatalf("%s holds %s as %q, want %q", r.device, rel, got, want)
		}
	}
}

func requireAbsentEverywhere(t *testing.T, replicas []*replica, rel string) {
	t.Helper()
	for _, r := range replicas {
		if r.exists(rel) {
			t.Fatalf("%s still holds %s", r.device, rel)
		}
	}
}

// requireOnlySyncedEffects holds every record to the exchange's boundary: no
// effect ever names the journal, the ledger, or the sync history itself.
func requireOnlySyncedEffects(t *testing.T, manifests []*synclog.Manifest) {
	t.Helper()
	for _, m := range manifests {
		for _, effect := range m.Effects {
			for _, rel := range []string{effect.Path, effect.TargetPath} {
				if rel == "" {
					continue
				}
				if _, _, local, err := synclog.Classify(rel); err != nil || local {
					t.Fatalf("%s txn %s publishes device-local path %s", m.Kind, m.TxnID, rel)
				}
			}
		}
	}
}

func lastOutgoing(t *testing.T, r *replica) *synclog.Manifest {
	t.Helper()
	out := r.outgoing()
	if len(out) == 0 {
		t.Fatalf("%s has no outgoing transaction", r.device)
	}
	return out[len(out)-1]
}

func lensDefinitions(t *testing.T, r *replica) *config.WorkspaceLensDefinitions {
	t.Helper()
	lenses, _, err := config.NewDomainFileStore(r.root).LoadLensDefinitionsChecked()
	if err != nil {
		t.Fatalf("%s: lenses: %v", r.device, err)
	}
	return lenses
}

func hasLens(lenses *config.WorkspaceLensDefinitions, name string) bool {
	for _, lens := range lenses.Lenses {
		if lens.Name == name {
			return true
		}
	}
	return false
}

func attachmentRel(t *testing.T, r *replica, slug, taskID, filename string) string {
	t.Helper()
	project := &models.Project{Path: r.path("projects/" + slug)}
	abs, err := storage.ResolveAttachmentPath(project, &models.Task{ID: taskID}, filename)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(r.root, abs)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel)
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// bridgeOpen opens dataDir through the bridge as device and returns the
// workspace handle later requests name.
func bridgeOpen(t *testing.T, b *bridge.Bridge, dataDir, runtimeDir, device string) string {
	t.Helper()
	var status struct {
		Workspace string `json:"workspace"`
	}
	bridgeDecode(t, bridgeRaw(t, b, "", "workspace.open", map[string]any{
		"data_dir": dataDir, "runtime_dir": runtimeDir, "device_id": device,
	}), &status)
	return status.Workspace
}

func bridgeCall(t *testing.T, b *bridge.Bridge, workspace, op string, payload any) {
	t.Helper()
	bridgeDecode(t, bridgeRaw(t, b, workspace, op, payload), &json.RawMessage{})
}

var bridgeRequests int

func bridgeRaw(t *testing.T, b *bridge.Bridge, workspace, op string, payload any) string {
	t.Helper()
	bridgeRequests++
	envelope := map[string]any{
		"bridge_version": bridge.Version,
		"request_id":     fmt.Sprintf("rich-%d", bridgeRequests),
		"op":             op,
		"payload":        payload,
	}
	if workspace != "" {
		envelope["workspace"] = workspace
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return b.Call(string(data))
}

func bridgeDecode(t *testing.T, reply string, into any) {
	t.Helper()
	var decoded struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal([]byte(reply), &decoded); err != nil {
		t.Fatalf("bridge reply is not JSON: %v\n%s", err, reply)
	}
	if !decoded.OK {
		t.Fatalf("bridge refused: %s", decoded.Error)
	}
	if err := json.Unmarshal(decoded.Result, into); err != nil {
		t.Fatalf("decode bridge result: %v", err)
	}
}

func genesisDoc(t *testing.T, c *cluster, rel string) []byte {
	t.Helper()
	return readFile(t, filepath.Join(c.genesis, filepath.FromSlash(rel)))
}

func mustParse(t *testing.T, data []byte) *storage.ProjectDocument {
	t.Helper()
	doc, err := storage.ParseProjectDocument(string(data))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}
