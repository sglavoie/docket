package syncmerge_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/bridge"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/synclog"
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
	weeklyNote  = ".tt/global-notes/2026-09-02-180000_4e5f6a7b-weekly-review.md"
	lensesFile  = ".tt/lenses.json"
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

// TestSyncRichArtifactsRoundTrip publishes each kind of rich content from one
// device and requires every other device, and observers that start from
// genesis, to hold it exactly: the bytes match and the live readers return the
// same fields.
func TestSyncRichArtifactsRoundTrip(t *testing.T) {
	t.Run("notes", testRichNotes)
	t.Run("comments", testRichComments)
	t.Run("checklists", testRichChecklists)
	t.Run("dependencies", testRichDependencies)
	t.Run("lenses", testRichLenses)
	t.Run("lenses_domain_save", testRichLensesDomainSave)
	t.Run("attachments", testRichAttachments)
	t.Run("concurrent_rich_edits", testRichConcurrentRichEdits)
	t.Run("mobile_edit_preserves_unexposed_fields", testRichMobileEditPreservesUnexposedFields)
}

// TestSyncExternalEdits changes the workspace outside every commit and
// requires valid changes to be published as changes and invalid ones to stay
// on disk, unpublished and never overwritten, until they are repaired.
func TestSyncExternalEdits(t *testing.T) {
	t.Run("valid_markdown_edit_is_published", testExternalValidMarkdownEditIsPublished)
	t.Run("invalid_markdown_edit_is_held", testExternalInvalidMarkdownEditIsHeld)
	t.Run("invalid_domain_edits_are_held", testExternalInvalidDomainEditsAreHeld)
	t.Run("external_files_are_published", testExternalExternalFilesArePublished)
	t.Run("domain_save_is_observed_before_incoming", testExternalDomainSaveIsObservedBeforeIncoming)
	t.Run("runtime_open_imports_external_edit", testExternalRuntimeOpenImportsExternalEdit)
}

// TestSyncUndo holds undo and redo to the sync contract: each is a new
// outgoing change every device applies, each is refused whole when another
// device's work has moved what it would restore, and the history behind them
// belongs to the device that made it.
func TestSyncUndo(t *testing.T) {
	t.Run("undo_and_redo_publish_new_transactions", testUndoAndRedoPublishNewTransactions)
	t.Run("remote_edit_refuses_stale_undo", testUndoRemoteEditRefusesStaleUndo)
	t.Run("remote_edit_refuses_stale_redo", testUndoRemoteEditRefusesStaleRedo)
	t.Run("unrecorded_change_refuses_undo", testUndoUnrecordedChangeRefusesUndo)
	t.Run("undo_history_is_device_local", testUndoHistoryIsDeviceLocal)
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

func testRichNotes(t *testing.T) {
	c := newCluster(t, "a", "b")
	a := c.r("a")
	projectNote := "projects/atlas-launch/notes/2026-10-06-120000_5e6f7a8b-sync-checklist.md"
	globalNote := ".tt/global-notes/2026-10-06-120000_6f7a8b9c-sync-retro.md"
	projectBody := []byte("---\ntitle: Sync checklist\ncreated: \"2026-10-06T12:00:00Z\"\nmodified: \"2026-10-06T12:00:00Z\"\n---\n\n- [ ] pair the phone\n")
	globalBody := []byte("---\ntitle: Sync retro\ncreated: \"2026-10-06T12:00:00Z\"\nmodified: \"2026-10-06T12:00:00Z\"\n---\n\nWhat went well.\n")
	runbook := append(a.read(runbookNote), []byte("4. Announce\n")...)
	a.commit(a.put(projectNote, projectBody), a.put(globalNote, globalBody), a.put(runbookNote, runbook))
	a.commit(storage.ArtifactChange{Path: a.path(weeklyNote), Delete: true})
	replicas := settleClean(t, c)
	requireFileEverywhere(t, replicas, projectNote, projectBody)
	requireFileEverywhere(t, replicas, globalNote, globalBody)
	requireFileEverywhere(t, replicas, runbookNote, runbook)
	requireAbsentEverywhere(t, replicas, weeklyNote)
}

func testRichComments(t *testing.T) {
	c := newCluster(t, "a", "b")
	a := c.r("a")
	a.editDoc(atlasSlug, func(doc *storage.ProjectDocument) {
		task := findTask(t, doc, shipBeta)
		task.Comments = append(task.Comments, &models.Comment{Text: "Uploaded to TestFlight.\nSecond line.", CreatedAt: richInstant})
		doc.Comments = append(doc.Comments, &models.ProjectComment{
			ID: "3c9b2f4e-5d6a-4b7c-8d9e-0f1a2b3c4d03", Text: "Beta shipped.", CreatedAt: richInstant,
		})
	})
	replicas := settleClean(t, c)
	requireTaskEverywhere(t, replicas, atlasSlug, shipBeta, a.task(atlasSlug, shipBeta))
	want := a.doc(atlasSlug).Comments
	for _, r := range replicas {
		if got := r.doc(atlasSlug).Comments; len(got) != 3 || fmt.Sprint(*got[2]) != fmt.Sprint(*want[2]) {
			t.Fatalf("%s holds project comments %+v", r.device, got)
		}
	}
}

func testRichChecklists(t *testing.T) {
	c := newCluster(t, "a", "b")
	a := c.r("a")
	a.editDoc(atlasSlug, func(doc *storage.ProjectDocument) {
		task := findTask(t, doc, notes)
		task.Checklist[1].Done = true
		task.Checklist = append(task.Checklist, models.ChecklistItem{Text: "Link the beta; then post"})
	})
	a.editDoc(homeSlug, func(doc *storage.ProjectDocument) {
		findTask(t, doc, homeTask1).Checklist = []models.ChecklistItem{{Text: "Measure"}, {Text: "Buy tiles", Done: true}}
	})
	replicas := settleClean(t, c)
	requireTaskEverywhere(t, replicas, atlasSlug, notes, a.task(atlasSlug, notes))
	requireTaskEverywhere(t, replicas, homeSlug, homeTask1, a.task(homeSlug, homeTask1))
	if got := len(replicas[len(replicas)-1].task(atlasSlug, notes).Checklist); got != 3 {
		t.Fatalf("checklist has %d items, want 3", got)
	}
}

func testRichDependencies(t *testing.T) {
	c := newCluster(t, "a", "b")
	a := c.r("a")
	a.editDoc(homeSlug, func(doc *storage.ProjectDocument) {
		findTask(t, doc, homeTask2).Dependencies = []models.TaskReference{{ProjectID: atlasID, TaskID: shipBeta}}
	})
	a.editDoc(atlasSlug, func(doc *storage.ProjectDocument) {
		task := findTask(t, doc, localize)
		task.Dependencies = append(task.Dependencies, models.TaskReference{ProjectID: atlasID, TaskID: awaitCert})
	})
	replicas := settleClean(t, c)
	requireTaskEverywhere(t, replicas, homeSlug, homeTask2, a.task(homeSlug, homeTask2))
	requireTaskEverywhere(t, replicas, atlasSlug, localize, a.task(atlasSlug, localize))
}

func testRichLenses(t *testing.T) {
	c := newCluster(t, "a", "b")
	a := c.r("a")
	lenses := lensDefinitions(t, a)
	lenses.Lenses = append(lenses.Lenses, config.NamedLens{
		ID: "9e8d7c6b-5a49-4382-a1b0-c9d8e7f6a503", Name: "Home waiting", Mode: "all", Scope: homeID, Query: "$waiting",
	})
	var env envelope
	if err := json.Unmarshal(a.read(lensesFile), &env); err != nil {
		t.Fatal(err)
	}
	a.commit(a.put(lensesFile, encodeDomain(t, env, lenses)))
	replicas := settleClean(t, c)
	for _, r := range replicas {
		if !hasLens(lensDefinitions(t, r), "Home waiting") {
			t.Fatalf("%s does not hold the new lens", r.device)
		}
	}
}

func testRichLensesDomainSave(t *testing.T) {
	c := newCluster(t, "a", "b")
	a := c.r("a")
	a.index()
	lenses := lensDefinitions(t, a)
	lenses.Lenses = append(lenses.Lenses, config.NamedLens{
		ID: "9e8d7c6b-5a49-4382-a1b0-c9d8e7f6a504", Name: "Deep work", Mode: "next",
	})
	// The checked domain save writes outside the journal; observation is
	// what publishes it.
	if err := config.NewDomainCoordinator(a.root).CommitLensDefinitions(lenses); err != nil {
		t.Fatal(err)
	}
	report := a.observe()
	if report.Imported == nil || report.Imported.Kind != synclog.KindExternal || len(report.Held) != 0 {
		t.Fatalf("the lens save was not imported: %+v", report)
	}
	if len(report.Imported.Effects) != 1 || report.Imported.Effects[0].Kind != synclog.EffectLenses {
		t.Fatalf("imported effects %+v, want the lens file alone", report.Imported.Effects)
	}
	replicas := settleClean(t, c)
	for _, r := range replicas {
		if !hasLens(lensDefinitions(t, r), "Deep work") {
			t.Fatalf("%s does not hold the lens saved through the domain store", r.device)
		}
	}
}

func testRichAttachments(t *testing.T) {
	c := newCluster(t, "a", "b")
	a := c.r("a")
	// Binary bytes: a zero, high bytes, and no trailing newline.
	icon := []byte{0x89, 'P', 'N', 'G', 0x00, 0xff, 0xfe, 0x10, 0x00}
	iconRel := attachmentRel(t, a, atlasSlug, darkIcons, "icons.png")
	doc := a.doc(atlasSlug)
	findTask(t, doc, darkIcons).Attachments = append(findTask(t, doc, darkIcons).Attachments, models.Attachment{
		Filename: "icons.png", OriginalPath: "/tmp/icons.png", AddedAt: richInstant,
	})
	a.commit(a.docArtifact(atlasSlug, doc), a.put(iconRel, icon))

	// Removing an attachment removes its metadata and its bytes together.
	checklistRel := attachmentRel(t, a, atlasSlug, shipBeta, "release-checklist.txt")
	if !a.exists(checklistRel) {
		t.Fatalf("fixture attachment %s is missing", checklistRel)
	}
	doc = a.doc(atlasSlug)
	task := findTask(t, doc, shipBeta)
	kept := task.Attachments[:0]
	for _, attachment := range task.Attachments {
		if attachment.Filename != "release-checklist.txt" {
			kept = append(kept, attachment)
		}
	}
	task.Attachments = kept
	a.commit(a.docArtifact(atlasSlug, doc), storage.ArtifactChange{Path: a.path(checklistRel), Delete: true})

	replicas := settleClean(t, c)
	requireFileEverywhere(t, replicas, iconRel, icon)
	requireAbsentEverywhere(t, replicas, checklistRel)
	requireTaskEverywhere(t, replicas, atlasSlug, darkIcons, a.task(atlasSlug, darkIcons))
	requireTaskEverywhere(t, replicas, atlasSlug, shipBeta, a.task(atlasSlug, shipBeta))
}

func testRichConcurrentRichEdits(t *testing.T) {
	c := newCluster(t, "a", "b")
	a, b := c.r("a"), c.r("b")
	a.editDoc(atlasSlug, func(doc *storage.ProjectDocument) {
		task := findTask(t, doc, shipBeta)
		task.Comments = append(task.Comments, &models.Comment{Text: "From the desk.", CreatedAt: richInstant})
	})
	b.editDoc(atlasSlug, func(doc *storage.ProjectDocument) {
		findTask(t, doc, shipBeta).Checklist = []models.ChecklistItem{{Text: "From the phone"}}
	})
	replicas := settleClean(t, c)
	for _, r := range replicas {
		task := r.task(atlasSlug, shipBeta)
		if len(task.Comments) != 3 || task.Comments[2].Text != "From the desk." || len(task.Checklist) != 1 {
			t.Fatalf("%s lost a concurrent rich edit: comments %d checklist %+v", r.device, len(task.Comments), task.Checklist)
		}
	}
}

func testRichMobileEditPreservesUnexposedFields(t *testing.T) {
	c := newCluster(t, "desk", "mobile")
	desk, mobile := c.r("desk"), c.r("mobile")
	before := desk.task(atlasSlug, shipBeta)

	// The phone changes the one thing its interface exposes — the state —
	// through the bridge over the shared engine.
	b := bridge.New(bridge.Options{})
	workspace := bridgeOpen(t, b, mobile.root, t.TempDir(), "mobile")
	bridgeCall(t, b, workspace, "command.transition", map[string]any{
		"task":  map[string]string{"project_id": atlasID, "task_id": shipBeta},
		"state": "waiting",
	})
	// Waiting leaves Today, so planning moves beside the document.
	edit := lastOutgoing(t, mobile)
	paths := map[string]bool{}
	for _, effect := range edit.Effects {
		paths[effect.Path] = true
	}
	if len(edit.Effects) != 2 || !paths[atlasDoc] || !paths[".tt/planning.json"] {
		t.Fatalf("the transition published %+v, want the project document and planning", edit.Effects)
	}

	replicas := settleClean(t, c)
	for _, r := range replicas {
		after := r.task(atlasSlug, shipBeta)
		if after.WorkflowState != models.StateWaiting {
			t.Fatalf("%s holds state %s", r.device, after.WorkflowState)
		}
		// Everything the phone does not show survives its edit untouched.
		expected := *before
		expected.WorkflowState = after.WorkflowState
		expected.StateChangedAt = after.StateChangedAt
		expected.UpdatedAt = after.UpdatedAt
		if got, want := taskJSON(t, after), taskJSON(t, &expected); got != want {
			t.Fatalf("%s lost a field the phone does not expose:\n got %s\nwant %s", r.device, got, want)
		}
	}
	for _, id := range []string{notes, proofread, awaitCert, localize, darkIcons} {
		requireTaskEverywhere(t, replicas, atlasSlug, id, findTask(t, mustParse(t, genesisDoc(t, c, atlasDoc)), id))
	}
}

func testExternalValidMarkdownEditIsPublished(t *testing.T) {
	c := newCluster(t, "a", "b")
	a := c.r("a")
	a.index()
	edited := bytes.Replace(a.read(atlasDoc), []byte("## Task: Ship beta build"), []byte("## Task: Ship beta build (rc1)"), 1)
	edited = bytes.Replace(edited, []byte("Cut the release branch."), []byte("Cut the release branch from main."), 1)
	a.write(atlasDoc, edited)

	report := a.observe()
	if report.Imported == nil || report.Imported.Kind != synclog.KindExternal || len(report.Held) != 0 {
		t.Fatalf("a valid external edit was not imported: %+v", report)
	}
	effect := report.Imported.Effects
	if len(effect) != 1 || effect[0].Path != atlasDoc || effect[0].Op != synclog.OpPut || effect[0].ProjectID != atlasID || effect[0].BaseBlob == nil {
		t.Fatalf("imported effects %+v", effect)
	}
	if again := a.observe(); again.Imported != nil || again.TransactionID != "" {
		t.Fatalf("observing an imported change again imported it twice: %+v", again)
	}
	replicas := settleClean(t, c)
	requireFileEverywhere(t, replicas, atlasDoc, edited)
	for _, r := range replicas {
		task := r.task(atlasSlug, shipBeta)
		if task.Title != "Ship beta build (rc1)" || !strings.HasPrefix(task.Description, "Cut the release branch from main.") {
			t.Fatalf("%s holds %q / %q", r.device, task.Title, task.Description)
		}
	}
}

func testExternalInvalidMarkdownEditIsHeld(t *testing.T) {
	c := newCluster(t, "a", "b")
	a, b := c.r("a"), c.r("b")
	a.index()
	original := a.read(atlasDoc)
	broken := bytes.Replace(original, []byte("<!-- id: 0a7e5d10-2c3b-4f6a-8e9d-000000000101,"), []byte("<!-- id: not-a-task-id,"), 1)
	if bytes.Equal(broken, original) {
		t.Fatal("the fixture did not contain the edited marker")
	}
	a.write(atlasDoc, broken)
	pending := len(a.outgoing())

	report := a.observe()
	if report.Imported != nil || len(report.Held) != 1 || report.Held[0].Path != atlasDoc || report.Held[0].Reason == "" {
		t.Fatalf("an invalid external edit was not held: %+v", report)
	}
	if len(a.outgoing()) != pending {
		t.Fatal("a held edit was published")
	}

	// Another device's change to the same document must not be reconciled
	// over the held bytes: it waits, and the bytes stay exactly as written.
	b.editDoc(atlasSlug, func(doc *storage.ProjectDocument) { findTask(t, doc, localize).Title = "Localize for 12 regions" })
	incoming := lastOutgoing(t, b)
	_, err := a.svc.CommitIncoming(incoming, b.history)
	if !errors.Is(err, synclog.ErrUnrecordedChange) {
		t.Fatalf("applying over a held edit: %v, want ErrUnrecordedChange", err)
	}
	if !bytes.Equal(a.read(atlasDoc), broken) {
		t.Fatal("the held edit was overwritten")
	}
	if _, known, lookupErr := a.history.Lookup(incoming.TxnID); lookupErr != nil || known {
		t.Fatalf("a refused transaction was acknowledged: known %v err %v", known, lookupErr)
	}

	// Repairing the file is a change like any other: it is imported, and
	// the waiting transaction then reconciles with it.
	repaired := bytes.Replace(original, []byte("## Task: Ship beta build"), []byte("## Task: Ship beta build (repaired)"), 1)
	a.write(atlasDoc, repaired)
	if fixed := a.observe(); fixed.Imported == nil || len(fixed.Held) != 0 {
		t.Fatalf("the repaired edit was not imported: %+v", fixed)
	}
	replicas := settleClean(t, c)
	for _, r := range replicas {
		if r.task(atlasSlug, shipBeta).Title != "Ship beta build (repaired)" || r.task(atlasSlug, localize).Title != "Localize for 12 regions" {
			t.Fatalf("%s lost the repair or the remote edit", r.device)
		}
	}
}

func testExternalInvalidDomainEditsAreHeld(t *testing.T) {
	c := newCluster(t, "a")
	a := c.r("a")
	a.index()
	planning := a.read(".tt/planning.json")
	a.write(".tt/planning.json", []byte("{ not json"))
	a.write(lensesFile, bytes.Replace(a.read(lensesFile), []byte(`"kind": "lenses"`), []byte(`"kind": "planning"`), 1))
	report := a.observe()
	if report.Imported != nil || len(report.Held) != 2 {
		t.Fatalf("invalid domain files were not held: %+v", report)
	}
	if !bytes.Equal(a.read(".tt/planning.json"), []byte("{ not json")) {
		t.Fatal("the held planning file was changed")
	}
	a.write(".tt/planning.json", planning)
	if report = a.observe(); len(report.Held) != 1 || report.Held[0].Path != lensesFile {
		t.Fatalf("after restoring planning: %+v", report)
	}
}

func testExternalExternalFilesArePublished(t *testing.T) {
	c := newCluster(t, "a", "b")
	a := c.r("a")
	a.index()
	note := "projects/atlas-launch/notes/2026-10-06-130000_7a8b9c0d-dropped-in.md"
	body := []byte("---\ntitle: Dropped in\n---\n\nWritten by another app.\n")
	attachment := attachmentRel(t, a, atlasSlug, shipBeta, "release-checklist.txt")
	attachmentBytes := []byte("updated outside tt\n")
	a.write(note, body)
	a.write(attachment, attachmentBytes)
	if err := removeFile(a.path(weeklyNote)); err != nil {
		t.Fatal(err)
	}
	// A stray file outside the synced layout is not part of the workspace.
	a.write("README.txt", []byte("not synced"))
	report := a.observe()
	if report.Imported == nil || len(report.Imported.Effects) != 3 || len(report.Held) != 0 {
		t.Fatalf("external files were not imported: %+v", report)
	}
	replicas := settleClean(t, c)
	requireFileEverywhere(t, replicas, note, body)
	requireFileEverywhere(t, replicas, attachment, attachmentBytes)
	requireAbsentEverywhere(t, replicas, weeklyNote)
	for _, r := range replicas[1:] {
		if r.exists("README.txt") {
			t.Fatalf("%s received an unsynced file", r.device)
		}
	}
}

func testExternalDomainSaveIsObservedBeforeIncoming(t *testing.T) {
	c := newCluster(t, "a", "b")
	a, b := c.r("a"), c.r("b")
	a.index()
	// Device a saves planning and a lens through the checked domain store,
	// which records nothing, while device b changes planning concurrently.
	planning := a.planning()
	planning.Next = append(planning.Next, config.ProjectTaskLocator{ProjectID: atlasID, TaskID: localize})
	coordinator := config.NewDomainCoordinator(a.root)
	if err := coordinator.CommitPlanning(planning); err != nil {
		t.Fatal(err)
	}
	lenses := lensDefinitions(t, a)
	lenses.Lenses = append(lenses.Lenses, config.NamedLens{ID: "9e8d7c6b-5a49-4382-a1b0-c9d8e7f6a505", Name: "Errands", Mode: "all"})
	if err := coordinator.CommitLensDefinitions(lenses); err != nil {
		t.Fatal(err)
	}
	b.commit(b.planningArtifact(func(p *config.PlanningState) {
		p.Next = append(p.Next, config.ProjectTaskLocator{ProjectID: atlasID, TaskID: darkIcons})
	}))

	// Receiving b's change imports a's saves first instead of refusing
	// them as unrecorded or reconciling them away.
	a.pull(b)
	imported := lastOutgoing(t, a)
	if imported.Kind != synclog.KindExternal || len(imported.Effects) != 2 {
		t.Fatalf("a's domain saves were not imported before the incoming change: %+v", imported)
	}
	replicas := settleClean(t, c)
	for _, r := range replicas {
		next := map[string]bool{}
		for _, item := range r.planning().Next {
			next[item.TaskID] = true
		}
		if !next[localize] || !next[darkIcons] {
			t.Fatalf("%s lost a planning change: %+v", r.device, r.planning().Next)
		}
		if !hasLens(lensDefinitions(t, r), "Errands") {
			t.Fatalf("%s lost the lens saved through the domain store", r.device)
		}
	}
}

func testExternalRuntimeOpenImportsExternalEdit(t *testing.T) {
	c := newCluster(t, "a")
	dataDir := c.r("a").root
	home := t.TempDir()
	open := func() {
		t.Helper()
		if _, err := engine.Open(engine.Config{DataDir: dataDir, Machine: engine.ExplicitMachine(home, "desk")}); err != nil {
			t.Fatal(err)
		}
	}
	open()
	edited := bytes.Replace(readFile(t, filepath.Join(dataDir, atlasDoc)), []byte("## Task: Ship beta build"), []byte("## Task: Ship beta build (edited in vim)"), 1)
	if err := writeFile(filepath.Join(dataDir, atlasDoc), edited); err != nil {
		t.Fatal(err)
	}
	open()
	history, err := synclog.New(synclog.Config{DataDir: dataDir, DeviceID: "desk", Schemas: replicaSchemas})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := history.Pending()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].Kind != synclog.KindExternal || pending[0].Effects[0].Path != atlasDoc {
		t.Fatalf("opening the runtime did not import the edit: %+v", pending)
	}
}

func testUndoAndRedoPublishNewTransactions(t *testing.T) {
	c := newCluster(t, "a", "b")
	a, b := c.r("a"), c.r("b")
	original := a.task(atlasSlug, shipBeta)
	a.commitUndoable(a.retitleArtifact(atlasSlug, shipBeta, "Ship beta build v2"))
	edited := a.task(atlasSlug, shipBeta)
	b.pull(a)
	b.commit(b.retitleArtifact(homeSlug, homeTask1, "Retile the kitchen"))
	a.pull(b)

	if _, err := a.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("undo: %v", err)
	}
	undo := lastOutgoing(t, a)
	if undo.Kind != synclog.KindUndo || len(undo.Effects) != 1 || undo.Effects[0].Path != atlasDoc {
		t.Fatalf("undo published %+v", undo)
	}
	heads := lastOutgoing(t, b).TxnID
	if !contains(undo.Parents, heads) {
		t.Fatalf("the undo does not descend from b's change it built on: parents %v", undo.Parents)
	}
	replicas := settleClean(t, c)
	requireTaskEverywhere(t, replicas, atlasSlug, shipBeta, original)
	for _, r := range replicas {
		if r.task(homeSlug, homeTask1).Title != "Retile the kitchen" {
			t.Fatalf("%s lost b's change across a's undo", r.device)
		}
	}

	if _, err := a.ledger.Redo(storage.GlobalScope()); err != nil {
		t.Fatalf("redo: %v", err)
	}
	if redo := lastOutgoing(t, a); redo.Kind != synclog.KindRedo {
		t.Fatalf("redo published %+v", redo)
	}
	replicas = settleClean(t, c)
	requireTaskEverywhere(t, replicas, atlasSlug, shipBeta, edited)
	all, _ := c.everything()
	requireOnlySyncedEffects(t, all)
}

func testUndoRemoteEditRefusesStaleUndo(t *testing.T) {
	c := newCluster(t, "a", "b")
	a, b := c.r("a"), c.r("b")
	a.commitUndoable(a.retitleArtifact(atlasSlug, shipBeta, "Ship beta build v2"))
	b.pull(a)
	b.commit(b.retitleArtifact(atlasSlug, localize, "Localize for 12 regions"))
	a.pull(b)

	current := a.read(atlasDoc)
	pending := len(a.outgoing())
	_, err := a.ledger.Undo(storage.GlobalScope())
	var conflict *storage.ArtifactConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("undo over a remote edit: %v, want an artifact conflict", err)
	}
	if !bytes.Equal(a.read(atlasDoc), current) {
		t.Fatal("a refused undo changed the document")
	}
	if len(a.outgoing()) != pending {
		t.Fatal("a refused undo published a transaction")
	}
	// The remote edit recorded no ledger entry, so nothing can bring the file
	// back to the image this one left: it is retired rather than left selected,
	// where it would wall off every older entry.
	var retired *storage.RetiredEntryError
	if !errors.As(err, &retired) {
		t.Fatalf("undo over a remote edit: %v, want the entry retired", err)
	}
	if _, ok, selectErr := a.ledger.SelectNewestUndo(storage.GlobalScope()); selectErr != nil || ok {
		t.Fatalf("a retired undo entry is still selected: %v %v", ok, selectErr)
	}
	replicas := settleClean(t, c)
	for _, r := range replicas {
		if r.task(atlasSlug, localize).Title != "Localize for 12 regions" || r.task(atlasSlug, shipBeta).Title != "Ship beta build v2" {
			t.Fatalf("%s lost an edit", r.device)
		}
	}
}

func testUndoRemoteEditRefusesStaleRedo(t *testing.T) {
	c := newCluster(t, "a", "b")
	a, b := c.r("a"), c.r("b")
	a.commitUndoable(a.retitleArtifact(atlasSlug, shipBeta, "Ship beta build v2"))
	if _, err := a.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatal(err)
	}
	b.pull(a)
	b.commit(b.retitleArtifact(atlasSlug, shipBeta, "Ship beta build, phone title"))
	a.pull(b)

	current := a.read(atlasDoc)
	_, err := a.ledger.Redo(storage.GlobalScope())
	var conflict *storage.ArtifactConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("redo over a remote edit: %v, want an artifact conflict", err)
	}
	if !bytes.Equal(a.read(atlasDoc), current) || a.task(atlasSlug, shipBeta).Title != "Ship beta build, phone title" {
		t.Fatal("a refused redo overwrote the remote edit")
	}
	settleClean(t, c)
}

func testUndoUnrecordedChangeRefusesUndo(t *testing.T) {
	c := newCluster(t, "a")
	a := c.r("a")
	a.commitUndoable(a.retitleArtifact(atlasSlug, shipBeta, "Ship beta build v2"))
	edited := bytes.Replace(a.read(atlasDoc), []byte("Cut the release branch."), []byte("Cut it."), 1)
	a.write(atlasDoc, edited)
	var conflict *storage.ArtifactConflictError
	if _, err := a.ledger.Undo(storage.GlobalScope()); !errors.As(err, &conflict) {
		t.Fatalf("undo over an external edit: %v, want an artifact conflict", err)
	}
	if !bytes.Equal(a.read(atlasDoc), edited) {
		t.Fatal("undo overwrote an external edit")
	}
}

func testUndoHistoryIsDeviceLocal(t *testing.T) {
	c := newCluster(t, "a")
	a := c.r("a")
	a.commitUndoable(a.retitleArtifact(atlasSlug, shipBeta, "Ship beta build v2"))
	entry, ok, err := a.ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok || entry.Device != "a" {
		t.Fatalf("the entry is not stamped with its device: %+v %v %v", entry, ok, err)
	}

	// A copy of the workspace, ledger included, opened as another device
	// cannot replay a's history.
	copied := filepath.Join(t.TempDir(), "copy")
	copyTree(t, a.root, copied)
	other := c.open("c", copied)
	if count, countErr := other.ledger.UndoCount(storage.GlobalScope()); countErr != nil || count != 0 {
		t.Fatalf("another device sees %d undo entries (%v)", count, countErr)
	}
	if _, err := other.ledger.Undo(storage.GlobalScope()); !errors.Is(err, storage.ErrNoUndoHistory) {
		t.Fatalf("another device undid a's entry: %v", err)
	}
	if other.task(atlasSlug, shipBeta).Title != "Ship beta build v2" {
		t.Fatal("another device's refused undo changed the workspace")
	}
	if count, countErr := a.ledger.UndoCount(storage.GlobalScope()); countErr != nil || count != 1 {
		t.Fatalf("device a sees %d undo entries (%v)", count, countErr)
	}
	if _, err := a.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("device a cannot undo its own entry: %v", err)
	}
}
