package syncmerge_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sglavoie/docket/internal/bridge"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/synclog"
)

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

func testRichNotes(t *testing.T) {
	c := newCluster(t, "a", "b")
	a := c.r("a")
	projectNote := "projects/atlas-launch/notes/2026-10-06-120000_5e6f7a8b-sync-checklist.md"
	globalNote := ".docket/global-notes/2026-10-06-120000_6f7a8b9c-sync-retro.md"
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
	if len(edit.Effects) != 2 || !paths[atlasDoc] || !paths[".docket/planning.json"] {
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
