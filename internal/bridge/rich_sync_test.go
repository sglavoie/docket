package bridge

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/syncexchange"
	"github.com/sglavoie/docket/internal/synclog"
)

// TestBridgeRichContentConvergesAcrossDevices carries comments, notes, and
// attachments between a phone, through the bridge, and a desktop, through the
// application calls its panels make. Work done offline on both sides crosses
// byte for byte; concurrent edits of one note, one comment, or one attachment
// name stay recoverable as durable conflicts on both devices rather than one
// side's bytes being dropped; and an attachment whose content has not arrived
// is reported unavailable and never half applied.
func TestBridgeRichContentConvergesAcrossDevices(t *testing.T) {
	t.Run("offline_edits_cross_byte_identical", syncRichOfflineEdits)
	t.Run("conflicting_note_edits", syncConflictingNoteEdits)
	t.Run("conflicting_comment_edits", syncConflictingCommentEdits)
	t.Run("conflicting_attachment_imports", syncConflictingAttachmentImports)
	t.Run("attachment_unavailable_until_its_bytes_arrive", syncAttachmentUnavailableUntilDelivered)
}

func (h *harness) blob(digest string) []byte {
	h.t.Helper()
	data, err := h.bridge.workspace.runtime.SyncCommits().SyncHistory().Blob(digest)
	if err != nil {
		h.t.Fatalf("blob %s is not recoverable: %v", digest, err)
	}
	return data
}

// recoverable reports whether content is the workspace's current bytes at
// path or the bytes of a candidate of an open conflict on it.
func (h *harness) recoverable(conflicts []synclog.Conflict, path string, holds func([]byte) bool) bool {
	h.t.Helper()
	if current, err := os.ReadFile(filepath.Join(h.dataDir, filepath.FromSlash(path))); err == nil && holds(current) {
		return true
	}
	for _, conflict := range conflicts {
		if conflict.Path != path {
			continue
		}
		for _, candidate := range conflict.Candidates {
			if candidate.Blob != nil && holds(h.blob(*candidate.Blob)) {
				return true
			}
		}
	}
	return false
}

func containing(text string) func([]byte) bool {
	return func(data []byte) bool { return bytes.Contains(data, []byte(text)) }
}

func equalTo(want []byte) func([]byte) bool {
	return func(data []byte) bool { return bytes.Equal(data, want) }
}

// desktopNote is the desktop's own read of a note, as its Notes panel holds it.
func (h *harness) desktopNote(location app.NoteLocation, filename string) *models.Note {
	h.t.Helper()
	notes, err := h.service().NotesAt(location)
	if err != nil {
		h.t.Fatal(err)
	}
	for _, note := range notes {
		if note.Filename == filename {
			return note
		}
	}
	h.t.Fatalf("the desktop holds no note %s", filename)
	return nil
}

func syncRichOfflineEdits(t *testing.T) {
	p := newPair(t)
	ship := task(atlasProject, shipBetaTask)
	notes := task(atlasProject, releaseNotesTask)

	// The phone works offline: a task comment, a project comment, a new global
	// note, an edit of a project note, and a photo.
	p.phone.ok(p.phone.call("command.comment", map[string]any{"task": ship, "action": "add", "text": "Checked on the train."}), nil)
	p.phone.ok(p.phone.call("command.comment", map[string]any{"project_id": atlasProject, "action": "add", "text": "Phone: beta date holds."}), nil)
	var created noteCommandResult
	p.phone.ok(p.phone.call("command.note", map[string]any{"action": "create", "location": globalNotes, "title": "Ideas", "body": "Café ☕ list\n\n1. tabs"}), &created)
	var runbook noteResult
	p.phone.ok(p.phone.call("query.note", map[string]any{"location": atlasNotes(), "filename": releaseRunbook}), &runbook)
	p.phone.ok(p.phone.call("command.note", map[string]any{
		"action": "update", "location": atlasNotes(), "filename": releaseRunbook, "revision": runbook.Note.Revision,
		"body": *runbook.Note.Body + "\n\nPhone: notarize before noon.",
	}), nil)
	photo := binaryContent(42, 512_000)
	path, digest := p.phone.stageImport("whiteboard.jpg", photo)
	imported := p.phone.attachFile(notes, path, "whiteboard.jpg", digest)

	// Meanwhile the desktop adds its own project comment and global note.
	if _, err := p.desktop.service().AddProjectComment(atlasProject, "Desktop: signing cert renewed."); err != nil {
		t.Fatal(err)
	}
	if _, err := p.desktop.service().CreateNote(app.NoteCreateRequest{Location: app.GlobalNoteLocation(), Note: &models.Note{Title: "Desk ideas", Body: "standing desk"}}); err != nil {
		t.Fatal(err)
	}

	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("independent rich edits conflicted: %+v", conflicts)
	}
	// The photo crossed whole: the desktop holds the exact bytes the phone
	// picked, at the path the phone's view names.
	desktopCopy, err := os.ReadFile(filepath.Join(p.desktop.dataDir, filepath.FromSlash(imported.Attachment.WorkspacePath)))
	if err != nil || !bytes.Equal(desktopCopy, photo) {
		t.Fatalf("the desktop's copy of the photo differs (%v)", err)
	}
	desktopView := p.desktop.detail(atlasProject, releaseNotesTask).Attachments
	if len(desktopView) != 1 || !desktopView[0].Available || desktopView[0].Name != "whiteboard.jpg" {
		t.Fatalf("the desktop reads the photo as %+v", desktopView)
	}
	var detail projectDetail
	p.desktop.ok(p.desktop.call("query.project", map[string]any{"project_id": atlasProject}), &detail)
	texts := commentTexts(detail.Comments)
	if !slices.Contains(texts, "Phone: beta date holds.") || !slices.Contains(texts, "Desktop: signing cert renewed.") {
		t.Fatalf("project comments after the exchange: %q", texts)
	}
	if !slices.Contains(commentTexts(p.desktop.detail(atlasProject, shipBetaTask).Comments), "Checked on the train.") {
		t.Fatal("the task comment did not cross")
	}
	titles := noteTitles(p.phone.notes(globalNotes))
	if !slices.Contains(titles, "Ideas") || !slices.Contains(titles, "Desk ideas") {
		t.Fatalf("global notes after the exchange: %q", titles)
	}
	if note := p.desktop.desktopNote(app.ProjectNoteLocation(atlasProject), releaseRunbook); !strings.Contains(note.Body, "Phone: notarize before noon.") {
		t.Fatalf("the desktop's runbook reads %q", note.Body)
	}
}

func syncConflictingNoteEdits(t *testing.T) {
	p := newPair(t)
	var read noteResult
	p.phone.ok(p.phone.call("query.note", map[string]any{"location": atlasNotes(), "filename": releaseRunbook}), &read)
	p.phone.ok(p.phone.call("command.note", map[string]any{
		"action": "update", "location": atlasNotes(), "filename": releaseRunbook, "revision": read.Note.Revision,
		"body": "Phone rewrite: run the release from the CI job.",
	}), nil)
	desk := p.desktop.desktopNote(app.ProjectNoteLocation(atlasProject), releaseRunbook)
	desk.Body = "Desktop rewrite: run the release by hand."
	desk.ModifiedAt = p.desktop.clock.Now()
	if _, err := p.desktop.service().UpdateNote(app.NoteUpdateRequest{Location: app.ProjectNoteLocation(atlasProject), Note: desk}); err != nil {
		t.Fatal(err)
	}
	conflicts := p.settle()
	path := "projects/atlas-launch/notes/" + releaseRunbook
	conflictOn(t, conflicts, path)
	for _, h := range []*harness{p.phone, p.desktop} {
		for _, text := range []string{"Phone rewrite", "Desktop rewrite"} {
			if !h.recoverable(conflicts, path, containing(text)) {
				t.Fatalf("%q is lost on one device", text)
			}
		}
	}
	// The phone's revision from before the exchange no longer holds.
	p.phone.unchanged("a note edit against the pre-exchange revision", func() {
		p.phone.rejects(p.phone.call("command.note", map[string]any{
			"action": "update", "location": atlasNotes(), "filename": releaseRunbook, "revision": read.Note.Revision, "body": "late",
		}), CodeStaleRevision)
	})
}

func syncConflictingCommentEdits(t *testing.T) {
	p := newPair(t)
	ship := task(atlasProject, shipBetaTask)
	p.phone.ok(p.phone.call("command.comment", map[string]any{
		"task": ship, "action": "edit", "index": 0, "original": "Branch cut from main.", "text": "Branch cut from main at 4f2c (phone).",
	}), nil)
	if _, err := p.desktop.service().EditComment(ship.app(), 0, "Branch cut from release/1.0 (desktop)."); err != nil {
		t.Fatal(err)
	}
	var project projectDetail
	p.phone.ok(p.phone.call("query.project", map[string]any{"project_id": atlasProject}), &project)
	first := project.Comments[0]
	p.phone.ok(p.phone.call("command.comment", map[string]any{
		"project_id": atlasProject, "action": "edit", "comment_id": first.ID, "original": first.Text, "text": "Kickoff: beta Oct 1 (phone).",
	}), nil)
	if _, err := p.desktop.service().EditProjectComment(app.ProjectCommentTarget{ProjectID: atlasProject, CommentID: first.ID, Original: first.Text}, "Kickoff: beta Oct 8 (desktop)."); err != nil {
		t.Fatal(err)
	}
	conflicts := p.settle()
	path := "projects/atlas-launch/tasks.md"
	conflict := conflictOn(t, conflicts, path)
	if len(conflict.Details) == 0 {
		t.Fatalf("the conflict names nothing it judged: %+v", conflict)
	}
	for _, h := range []*harness{p.phone, p.desktop} {
		for _, text := range []string{"(phone)", "(desktop)", "Branch cut from main at 4f2c", "Branch cut from release/1.0", "beta Oct 1", "beta Oct 8"} {
			if !h.recoverable(conflicts, path, containing(text)) {
				t.Fatalf("%q is lost on one device", text)
			}
		}
	}
	// The phone's stale address refuses after the merge rather than landing
	// on whichever comment now sits there.
	if read := p.phone.detail(atlasProject, shipBetaTask).Comments[0].Text; read != "Branch cut from main at 4f2c (phone)." {
		p.phone.unchanged("a comment edit against text the merge replaced", func() {
			p.phone.rejects(p.phone.call("command.comment", map[string]any{
				"task": ship, "action": "delete", "index": 0, "original": "Branch cut from main at 4f2c (phone).",
			}), CodeValidation)
		})
	}
}

func syncConflictingAttachmentImports(t *testing.T) {
	p := newPair(t)
	notes := task(atlasProject, releaseNotesTask)
	phonePhoto := binaryContent(1, 200_000)
	desktopPhoto := binaryContent(2, 150_000)
	path, digest := p.phone.stageImport("photo.jpg", phonePhoto)
	phoneView := p.phone.attachFile(notes, path, "photo.jpg", digest)
	source := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(source, desktopPhoto, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.desktop.service().AttachFile(notes.app(), source); err != nil {
		t.Fatal(err)
	}
	conflicts := p.settle()
	bytesPath := phoneView.Attachment.WorkspacePath
	conflictOn(t, conflicts, bytesPath)
	for _, h := range []*harness{p.phone, p.desktop} {
		if !h.recoverable(conflicts, bytesPath, equalTo(phonePhoto)) || !h.recoverable(conflicts, bytesPath, equalTo(desktopPhoto)) {
			t.Fatal("one device lost one side's photo bytes")
		}
	}
	// Whichever bytes the merge holds, both devices hold the same, and the
	// phone reads them as available.
	read := p.phone.detail(atlasProject, releaseNotesTask).Attachments
	if len(read) == 0 || !read[0].Available {
		t.Fatalf("after the conflict the phone reads %+v", read)
	}
}

func syncAttachmentUnavailableUntilDelivered(t *testing.T) {
	p := newPair(t)
	notes := task(atlasProject, releaseNotesTask)
	scan := binaryContent(77, 90_000)
	path, digest := p.phone.stageImport("receipt.pdf", scan)
	view := p.phone.attachFile(notes, path, "receipt.pdf", digest).Attachment

	// The transport delivers the manifest but the photo's blob is still a
	// placeholder: the desktop applies nothing and names the attachment.
	exported := p.phone.exportPending()
	if len(exported.Pending) != 1 {
		t.Fatalf("pending %+v", exported.Pending)
	}
	inbox := filepath.Join(t.TempDir(), "inbox")
	stage(t, inbox, exported.Pending[0].Manifest, nil)
	var withheld syncexchange.File
	for _, blob := range exported.Pending[0].Blobs {
		if data, _ := os.ReadFile(blob.Source); bytes.Equal(data, scan) {
			withheld = blob
			continue
		}
		stage(t, inbox, blob, nil)
	}
	if withheld.Path == "" {
		t.Fatal("no published blob carries the attachment's bytes")
	}
	before := syncedFiles(t, p.desktop.dataDir)
	result := onlyResult(t, p.desktop.importInbox(inbox))
	if result.Outcome != syncexchange.Incomplete || !slices.Contains(result.Unavailable, view.WorkspacePath) {
		t.Fatalf("an import without the attachment's bytes answered %+v", result)
	}
	after := syncedFiles(t, p.desktop.dataDir)
	if len(after) != len(before) {
		t.Fatal("an incomplete import changed the desktop's files")
	}
	for name, data := range before {
		if !bytes.Equal(after[name], data) {
			t.Fatalf("an incomplete import changed %s", name)
		}
	}
	if got := p.desktop.detail(atlasProject, releaseNotesTask).Attachments; len(got) != 0 {
		t.Fatalf("the desktop shows an attachment whose bytes never arrived: %+v", got)
	}

	// A truncated copy of the bytes is still not the content.
	stage(t, inbox, withheld, scan[:1000])
	if result := onlyResult(t, p.desktop.importInbox(inbox)); result.Outcome == syncexchange.Imported {
		t.Fatalf("a truncated blob was imported: %+v", result)
	}
	stage(t, inbox, withheld, nil)
	if result := onlyResult(t, p.desktop.importInbox(inbox)); result.Outcome != syncexchange.Imported {
		t.Fatalf("the complete delivery answered %+v", result)
	}
	got := p.desktop.detail(atlasProject, releaseNotesTask).Attachments
	if len(got) != 1 || !got[0].Available || !bytes.Equal(p.desktop.attachmentBytes(&got[0]), scan) {
		t.Fatalf("after delivery the desktop reads %+v", got)
	}
}
