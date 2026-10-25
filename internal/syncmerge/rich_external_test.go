package syncmerge_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/synclog"
)

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
