package bridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	weeklyReviewNote = "2026-09-02-180000_4e5f6a7b-weekly-review.md"
	releaseRunbook   = "2026-09-03-090000_0a1b2c3d-release-runbook.md"
	atlasSlug        = "atlas-launch"
)

// TestBridgeKeepsRichContentThroughTheSharedEngine drives comments, notes, and
// attachments through the bridge on one device, each outcome read back both
// through the bridge and from the workspace's files: what the phone writes is
// the record the desktop reads, a stale address is refused rather than
// redirected, and an attachment's bytes reach the workspace whole through one
// transaction while the temporary copy they came from is never recorded.
func TestBridgeKeepsRichContentThroughTheSharedEngine(t *testing.T) {
	cases := []struct {
		name string
		run  func(*testing.T, *harness)
	}{
		{"task_comments", richTaskComments},
		{"project_comments", richProjectComments},
		{"notes_lifecycle", richNotesLifecycle},
		{"stale_note_is_refused", richStaleNote},
		{"attachment_import", richAttachmentImport},
		{"attachment_kinds_and_removal", richAttachmentKinds},
		{"unavailable_attachment", richUnavailableAttachment},
		{"interrupted_import", richInterruptedImport},
		{"termination_and_reopen", richTerminationAndReopen},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, representativeFixture)
			h.openAs("rich-phone")
			c.run(t, h)
		})
	}
}

func (h *harness) atlasMarkdown() string {
	h.t.Helper()
	data, err := os.ReadFile(filepath.Join(h.dataDir, "projects", atlasSlug, "tasks.md"))
	if err != nil {
		h.t.Fatal(err)
	}
	return string(data)
}

func commentTexts(comments []commentView) []string {
	texts := make([]string, 0, len(comments))
	for _, comment := range comments {
		texts = append(texts, comment.Text)
	}
	return texts
}

func richTaskComments(t *testing.T, h *harness) {
	ship := task(atlasProject, shipBetaTask)
	read := h.detail(atlasProject, shipBetaTask)
	if got := commentTexts(read.Comments); !slices.Equal(got, []string{"Branch cut from main.", "Notarization failed once; retrying."}) {
		t.Fatalf("seeded comments %q", got)
	}
	var added commentResult
	h.ok(h.call("command.comment", map[string]any{"task": ship, "action": "add", "text": "Uploaded build 42.\nTestFlight review pending."}), &added)
	if len(added.Comments) != 3 || added.Committed.TransactionID == "" || added.Task == nil || added.Task.Comments != 3 {
		t.Fatalf("add answered %+v", added)
	}
	if !strings.Contains(h.atlasMarkdown(), "Uploaded build 42.\nTestFlight review pending.") {
		t.Fatal("the multiline comment is not in the task Markdown")
	}

	var edited commentResult
	h.ok(h.call("command.comment", map[string]any{
		"task": ship, "action": "edit", "index": 1, "original": "Notarization failed once; retrying.", "text": "Notarization passed on retry.",
	}), &edited)
	if edited.Comments[1].Text != "Notarization passed on retry." || !edited.Comments[1].CreatedAt.Equal(read.Comments[1].CreatedAt) {
		t.Fatalf("edit kept %+v, want the new text at the original instant", edited.Comments[1])
	}

	// The comment read before the edit no longer holds that text: refused.
	h.unchanged("a stale comment edit", func() {
		h.rejects(h.call("command.comment", map[string]any{
			"task": ship, "action": "delete", "index": 1, "original": "Notarization failed once; retrying.",
		}), CodeValidation)
	})
	h.unchanged("a comment index past the end", func() {
		h.rejects(h.call("command.comment", map[string]any{"task": ship, "action": "delete", "index": 9, "original": "x"}), CodeValidation)
	})
	h.unchanged("a blank comment", func() {
		h.rejects(h.call("command.comment", map[string]any{"task": ship, "action": "add", "text": "   "}), CodeValidation)
	})

	var removed commentResult
	h.ok(h.call("command.comment", map[string]any{"task": ship, "action": "delete", "index": 0, "original": "Branch cut from main."}), &removed)
	if got := commentTexts(removed.Comments); !slices.Equal(got, []string{"Notarization passed on retry.", "Uploaded build 42.\nTestFlight review pending."}) {
		t.Fatalf("after delete %q", got)
	}
	if strings.Contains(h.atlasMarkdown(), "Branch cut from main.") {
		t.Fatal("the deleted comment is still stored")
	}
}

func richProjectComments(t *testing.T, h *harness) {
	var detail projectDetail
	h.ok(h.call("query.project", map[string]any{"project_id": atlasProject}), &detail)
	if len(detail.Comments) != 2 || detail.Comments[1].Text != "Signing certificate is the long pole.\nTrack it weekly." {
		t.Fatalf("seeded project comments %+v", detail.Comments)
	}
	first := detail.Comments[0]

	var added commentResult
	h.ok(h.call("command.comment", map[string]any{"project_id": atlasProject, "action": "add", "text": "Beta slipped a week."}), &added)
	if added.CommentID == "" || len(added.Comments) != 3 || added.Committed.Projects[atlasProject] == "" {
		t.Fatalf("add answered %+v", added)
	}
	if !strings.Contains(h.atlasMarkdown(), "<!-- project-comment: "+added.CommentID+" ") {
		t.Fatal("the project comment is not stored under its id")
	}

	var edited commentResult
	h.ok(h.call("command.comment", map[string]any{
		"project_id": atlasProject, "action": "edit", "comment_id": first.ID, "original": first.Text, "text": "Kickoff agreed: beta in October.",
	}), &edited)
	for _, comment := range edited.Comments {
		if comment.ID == first.ID && (comment.Text != "Kickoff agreed: beta in October." || !comment.CreatedAt.Equal(first.CreatedAt)) {
			t.Fatalf("edit left %+v", comment)
		}
	}
	h.unchanged("a project comment edit against text no longer stored", func() {
		h.rejects(h.call("command.comment", map[string]any{
			"project_id": atlasProject, "action": "delete", "comment_id": first.ID, "original": first.Text,
		}), CodeValidation)
	})
	var removed commentResult
	h.ok(h.call("command.comment", map[string]any{
		"project_id": atlasProject, "action": "delete", "comment_id": added.CommentID, "original": "Beta slipped a week.",
	}), &removed)
	if len(removed.Comments) != 2 || strings.Contains(h.atlasMarkdown(), added.CommentID) {
		t.Fatalf("delete left %+v", removed.Comments)
	}
	h.rejects(h.call("command.comment", map[string]any{"task": task(atlasProject, shipBetaTask), "project_id": atlasProject, "action": "add", "text": "x"}), CodeInvalidRequest)
}

func noteTitles(notes []noteView) []string {
	titles := make([]string, 0, len(notes))
	for _, note := range notes {
		titles = append(titles, note.Title)
	}
	return titles
}

func (h *harness) notes(location any) []noteView {
	h.t.Helper()
	var listed notesResult
	payload := map[string]any{}
	if location != nil {
		payload["location"] = location
	}
	h.ok(h.call("query.notes", payload), &listed)
	return listed.Notes
}

var globalNotes = map[string]any{"scope": "global"}

func atlasNotes() map[string]any {
	return map[string]any{"scope": "project", "project_id": atlasProject}
}

func richNotesLifecycle(t *testing.T, h *harness) {
	all := h.notes(nil)
	if got := noteTitles(all); !slices.Contains(got, "Weekly review") || !slices.Contains(got, "Release runbook") {
		t.Fatalf("every reachable collection lists %q", got)
	}
	var read noteResult
	h.ok(h.call("query.note", map[string]any{"location": globalNotes, "filename": weeklyReviewNote}), &read)
	if read.Note.Body == nil || !strings.Contains(*read.Note.Body, "Look at Waiting and check-ins first.") || read.Note.Revision == "" {
		t.Fatalf("read %+v", read.Note)
	}

	body := "# Packing\n\n- passport\n- charger\n\nUnicode stays: café, 日本, ✓"
	var created noteCommandResult
	h.ok(h.call("command.note", map[string]any{"action": "create", "location": globalNotes, "title": "Trip prep", "body": body}), &created)
	if created.Note == nil || created.Committed.TransactionID == "" || *created.Note.Body != body {
		t.Fatalf("create answered %+v", created)
	}
	stored, err := os.ReadFile(filepath.Join(h.dataDir, ".tt", "global-notes", created.Note.Filename))
	if err != nil || !strings.Contains(string(stored), body) {
		t.Fatalf("the global note file holds %q (%v)", stored, err)
	}

	var updated noteCommandResult
	h.ok(h.call("command.note", map[string]any{
		"action": "update", "location": globalNotes, "filename": created.Note.Filename, "revision": created.Note.Revision,
		"body": body + "\n- adapter\n\n",
	}), &updated)
	if updated.Note.Revision == created.Note.Revision || *updated.Note.Body != body+"\n- adapter" || updated.Note.Title != "Trip prep" {
		t.Fatalf("update answered %+v", updated.Note)
	}

	var renamed noteCommandResult
	h.ok(h.call("command.note", map[string]any{
		"action": "rename", "location": globalNotes, "filename": updated.Note.Filename, "revision": updated.Note.Revision, "title": "Trip checklist",
	}), &renamed)
	if renamed.Note.Title != "Trip checklist" || renamed.Note.Filename == updated.Note.Filename {
		t.Fatalf("rename answered %+v", renamed.Note)
	}

	var moved noteCommandResult
	h.ok(h.call("command.note", map[string]any{
		"action": "move", "location": globalNotes, "filename": renamed.Note.Filename, "revision": renamed.Note.Revision, "destination": atlasNotes(),
	}), &moved)
	if moved.Note.Location.ProjectID != atlasProject || moved.Note.ProjectName != "Atlas Launch" {
		t.Fatalf("move answered %+v", moved.Note)
	}
	if _, err := os.Stat(filepath.Join(h.dataDir, "projects", atlasSlug, "notes", moved.Note.Filename)); err != nil {
		t.Fatalf("the moved note is not in the project: %v", err)
	}
	if slices.Contains(noteTitles(h.notes(globalNotes)), "Trip checklist") {
		t.Fatal("the moved note is still global")
	}

	var deleted noteCommandResult
	h.ok(h.call("command.note", map[string]any{
		"action": "delete", "location": atlasNotes(), "filename": moved.Note.Filename, "revision": moved.Note.Revision,
	}), &deleted)
	if deleted.Note != nil || deleted.Committed.TransactionID == "" || slices.Contains(noteTitles(h.notes(atlasNotes())), "Trip checklist") {
		t.Fatalf("delete answered %+v", deleted)
	}

	// Archived projects' notes are out of reach, as on the desktop.
	h.rejects(h.call("query.notes", map[string]any{"location": map[string]any{"scope": "project", "project_id": oldWebsiteProject}}), CodeNotFound)
	h.rejects(h.call("command.note", map[string]any{"action": "create", "location": globalNotes, "title": "  "}), CodeValidation)
}

func richStaleNote(t *testing.T, h *harness) {
	var read noteResult
	h.ok(h.call("query.note", map[string]any{"location": atlasNotes(), "filename": releaseRunbook}), &read)
	// Another writer changes the note under the phone, as an import would.
	path := filepath.Join(h.dataDir, "projects", atlasSlug, "notes", releaseRunbook)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("\nStep 4: staple the ticket.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	for _, action := range []string{"update", "delete"} {
		refusal := h.refused(h.call("command.note", map[string]any{
			"action": action, "location": atlasNotes(), "filename": releaseRunbook, "revision": read.Note.Revision, "body": "overwritten",
		}), CodeStaleRevision)
		if refusal.Details["current_revision"] == nil {
			t.Fatalf("%s refusal names no current revision: %+v", action, refusal)
		}
		if now, _ := os.ReadFile(path); !bytes.Equal(now, after) {
			t.Fatalf("a stale %s wrote the note", action)
		}
	}
}

// stageImport copies bytes where the app stages a picked item: under the
// device's runtime directory.
func (h *harness) stageImport(name string, data []byte) (string, string) {
	h.t.Helper()
	dir := filepath.Join(h.runtimeDir, "attachment-imports", strings.ReplaceAll(name, ".", "-"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		h.t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return path, "sha256:" + hex.EncodeToString(sum[:])
}

// binaryContent is bytes no text round trip preserves: every byte value,
// NULs, and invalid UTF-8.
func binaryContent(seed byte, size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i*31) ^ seed
	}
	return data
}

func (h *harness) attachFile(target locator, path, name, digest string) attachmentResult {
	h.t.Helper()
	var result attachmentResult
	h.ok(h.call("command.attachment", map[string]any{"task": target, "action": "add_file", "path": path, "filename": name, "digest": digest}), &result)
	return result
}

func (h *harness) attachmentBytes(view *attachmentView) []byte {
	h.t.Helper()
	if view == nil || !view.Available || view.WorkspacePath == "" {
		h.t.Fatalf("attachment not available: %+v", view)
	}
	data, err := os.ReadFile(filepath.Join(h.dataDir, filepath.FromSlash(view.WorkspacePath)))
	if err != nil {
		h.t.Fatal(err)
	}
	return data
}

func richAttachmentImport(t *testing.T, h *harness) {
	notes := task(atlasProject, releaseNotesTask)
	photo := binaryContent(7, 300_000)
	path, digest := h.stageImport("IMG_0001.HEIC", photo)
	added := h.attachFile(notes, path, "IMG_0001.HEIC", digest)
	if added.Outcome != "added" || added.Committed.TransactionID == "" || added.Attachment == nil || added.Attachment.Name != "IMG_0001.HEIC" {
		t.Fatalf("import answered %+v", added)
	}
	if !bytes.Equal(h.attachmentBytes(added.Attachment), photo) {
		t.Fatal("the workspace copy is not byte-identical to the picked item")
	}
	if added.Attachment.Size != int64(len(photo)) || added.Attachment.LocalPath == "" {
		t.Fatalf("attachment view %+v", added.Attachment)
	}
	md := h.atlasMarkdown()
	if strings.Contains(md, "attachment-imports") || strings.Contains(md, h.runtimeDir) || strings.Contains(md, url.QueryEscape(h.runtimeDir)) {
		t.Fatal("the task Markdown records the temporary staging path")
	}
	if !strings.Contains(taskLine(t, h.dataDir, atlasSlug, releaseNotesTask), "file|IMG_0001.HEIC||0|") {
		t.Fatalf("the stored attachment keeps an original path: %s", taskLine(t, h.dataDir, atlasSlug, releaseNotesTask))
	}

	// The same bytes again are already attached: nothing commits.
	h.unchanged("re-importing the same bytes", func() {
		again := h.attachFile(notes, path, "IMG_0001.HEIC", digest)
		if again.Outcome != "present" || again.Committed.TransactionID != "" || len(again.Attachments) != 1 {
			t.Fatalf("re-import answered %+v", again)
		}
	})

	// Different bytes under the same picked name keep both.
	other := binaryContent(9, 1000)
	otherPath, otherDigest := h.stageImport("IMG_0001.HEIC", other)
	second := h.attachFile(notes, otherPath, "IMG_0001.HEIC", otherDigest)
	if second.Attachment.Name != "IMG_0001-2.HEIC" || !bytes.Equal(h.attachmentBytes(second.Attachment), other) {
		t.Fatalf("a second item under one name answered %+v", second.Attachment)
	}
	if !bytes.Equal(h.attachmentBytes(&second.Attachments[0]), photo) {
		t.Fatal("the first item's bytes changed")
	}

	// A name a provider may hand over is made storable, never refused.
	odd, oddDigest := h.stageImport("odd.txt", []byte("plain"))
	named := h.attachFile(notes, odd, "notes; v2|final.txt", oddDigest)
	if named.Attachment.Name != "notes- v2-final.txt" {
		t.Fatalf("an unstorable name became %q", named.Attachment.Name)
	}

	// Only a copy staged under the device's runtime directory is accepted.
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("outside"))
	h.unchanged("a path outside runtime_dir", func() {
		h.rejects(h.call("command.attachment", map[string]any{
			"task": notes, "action": "add_file", "path": outside, "filename": "outside.txt", "digest": "sha256:" + hex.EncodeToString(sum[:]),
		}), CodeInvalidRequest)
	})
	escape := filepath.Join(h.runtimeDir, "attachment-imports", "link")
	if err := os.Symlink(outside, escape); err != nil {
		t.Fatal(err)
	}
	h.unchanged("a symlink out of runtime_dir", func() {
		h.rejects(h.call("command.attachment", map[string]any{
			"task": notes, "action": "add_file", "path": escape, "filename": "outside.txt", "digest": "sha256:" + hex.EncodeToString(sum[:]),
		}), CodeInvalidRequest)
	})
}

func richAttachmentKinds(t *testing.T, h *harness) {
	ship := task(atlasProject, shipBetaTask)
	read := h.detail(atlasProject, shipBetaTask)
	kinds := make([]string, 0, len(read.Attachments))
	for _, attachment := range read.Attachments {
		kinds = append(kinds, attachment.Kind)
	}
	if !slices.Equal(kinds, []string{"file", "url", "code"}) {
		t.Fatalf("seeded attachment kinds %v", kinds)
	}
	if !read.Attachments[0].Available || read.Attachments[1].URL != "https://example.com/atlas/release" ||
		read.Attachments[2].CodePath != "cmd/release.go" || read.Attachments[2].Line != 42 || read.Attachments[2].Available {
		t.Fatalf("seeded attachments %+v", read.Attachments)
	}
	if read.AttachmentDir == "" || !strings.HasPrefix(read.Attachments[0].WorkspacePath, read.AttachmentDir+"/") {
		t.Fatalf("attachment dir %q, file at %q", read.AttachmentDir, read.Attachments[0].WorkspacePath)
	}

	var link attachmentResult
	h.ok(h.call("command.attachment", map[string]any{"task": ship, "action": "add_url", "url": "https://example.com/beta/notes"}), &link)
	if link.Attachment == nil || link.Attachment.Kind != "url" || len(link.Attachments) != 4 {
		t.Fatalf("add_url answered %+v", link)
	}
	h.unchanged("a URL that is not http", func() {
		h.rejects(h.call("command.attachment", map[string]any{"task": ship, "action": "add_url", "url": "file:///etc/passwd"}), CodeValidation)
	})
	h.unchanged("removing by a stale name", func() {
		h.rejects(h.call("command.attachment", map[string]any{"task": ship, "action": "remove", "index": 0, "original": "other.txt"}), CodeValidation)
	})

	bytesPath := filepath.Join(h.dataDir, filepath.FromSlash(read.Attachments[0].WorkspacePath))
	var removed attachmentResult
	h.ok(h.call("command.attachment", map[string]any{"task": ship, "action": "remove", "index": 0, "original": "release-checklist.txt"}), &removed)
	if removed.Outcome != "removed" || len(removed.Attachments) != 3 || removed.Attachments[0].Kind != "url" {
		t.Fatalf("remove answered %+v", removed)
	}
	if _, err := os.Stat(bytesPath); !os.IsNotExist(err) {
		t.Fatalf("the removed file's bytes remain (%v)", err)
	}
	// The code reference survives every other change whole.
	if code := removed.Attachments[1]; code.Kind != "code" || code.CodePath != "cmd/release.go" || code.Line != 42 {
		t.Fatalf("the code reference became %+v", code)
	}
}

func richUnavailableAttachment(t *testing.T, h *harness) {
	read := h.detail(atlasProject, shipBetaTask)
	path := filepath.Join(h.dataDir, filepath.FromSlash(read.Attachments[0].WorkspacePath))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	read = h.detail(atlasProject, shipBetaTask)
	missing := read.Attachments[0]
	if missing.Available || missing.LocalPath != "" || missing.Size != 0 || missing.WorkspacePath == "" || missing.Name != "release-checklist.txt" {
		t.Fatalf("missing bytes read as %+v", missing)
	}
	// Metadata alone still edits: the record can be removed.
	var removed attachmentResult
	h.ok(h.call("command.attachment", map[string]any{
		"task": task(atlasProject, shipBetaTask), "action": "remove", "index": 0, "original": "release-checklist.txt",
	}), &removed)
	if len(removed.Attachments) != 2 {
		t.Fatalf("remove answered %+v", removed.Attachments)
	}
}

// reopen is a terminated process starting again: a new bridge over the same
// directories, with no memory of the previous one.
func (h *harness) reopen() {
	h.t.Helper()
	h.bridge = New(Options{Clock: h.clock, IDs: &phoneIDs{next: 500}})
	h.openAs("rich-phone")
}

func richInterruptedImport(t *testing.T, h *harness) {
	notes := task(atlasProject, releaseNotesTask)
	scan := binaryContent(3, 64_000)

	// The staging copy was cut short by the termination: refused, nothing
	// written.
	path, digest := h.stageImport("scan.pdf", scan)
	if err := os.WriteFile(path, scan[:len(scan)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	h.unchanged("a truncated staging copy", func() {
		refusal := h.refused(h.call("command.attachment", map[string]any{
			"task": notes, "action": "add_file", "path": path, "filename": "scan.pdf", "digest": digest,
		}), CodeValidation)
		if refusal.Details["field"] != "attachment" {
			t.Fatalf("refusal %+v", refusal)
		}
	})
	// A staged copy that vanished is named, not imported.
	h.unchanged("a vanished staging copy", func() {
		h.rejects(h.call("command.attachment", map[string]any{
			"task": notes, "action": "add_file", "path": filepath.Join(filepath.Dir(path), "gone.pdf"), "filename": "gone.pdf", "digest": digest,
		}), CodeNotFound)
	})

	// Terminated before the commit: the next process finds nothing attached
	// and the resumed import commits once.
	if err := os.WriteFile(path, scan, 0o644); err != nil {
		t.Fatal(err)
	}
	h.reopen()
	if got := h.detail(atlasProject, releaseNotesTask).Attachments; len(got) != 0 {
		t.Fatalf("an uncommitted import left %+v", got)
	}
	first := h.attachFile(notes, path, "scan.pdf", digest)
	if first.Outcome != "added" {
		t.Fatalf("resumed import answered %+v", first)
	}

	// Terminated after the commit but before the app forgot its staging
	// record: resuming answers present and commits nothing more.
	h.reopen()
	h.unchanged("resuming an import that already committed", func() {
		again := h.attachFile(notes, path, "scan.pdf", digest)
		if again.Outcome != "present" || len(again.Attachments) != 1 || !bytes.Equal(h.attachmentBytes(again.Attachment), scan) {
			t.Fatalf("resuming answered %+v", again)
		}
	})
}

func richTerminationAndReopen(t *testing.T, h *harness) {
	ship := task(atlasProject, shipBetaTask)
	h.ok(h.call("command.comment", map[string]any{"task": ship, "action": "add", "text": "Written offline."}), nil)
	var created noteCommandResult
	h.ok(h.call("command.note", map[string]any{"action": "create", "location": atlasNotes(), "title": "Offline draft", "body": "line one\n\nline two"}), &created)
	audio := binaryContent(11, 120_000)
	path, digest := h.stageImport("memo.m4a", audio)
	h.attachFile(ship, path, "memo.m4a", digest)
	before := syncedFiles(t, h.dataDir)

	h.reopen()
	after := syncedFiles(t, h.dataDir)
	for name, data := range before {
		if !bytes.Equal(after[name], data) {
			t.Fatalf("%s changed across the restart", name)
		}
	}
	read := h.detail(atlasProject, shipBetaTask)
	if !slices.Contains(commentTexts(read.Comments), "Written offline.") {
		t.Fatalf("the comment did not survive: %q", commentTexts(read.Comments))
	}
	last := read.Attachments[len(read.Attachments)-1]
	if last.Name != "memo.m4a" || !bytes.Equal(h.attachmentBytes(&last), audio) {
		t.Fatalf("the attachment did not survive: %+v", last)
	}
	var note noteResult
	h.ok(h.call("query.note", map[string]any{"location": atlasNotes(), "filename": created.Note.Filename}), &note)
	if *note.Note.Body != "line one\n\nline two" || note.Note.Revision != created.Note.Revision {
		t.Fatalf("the note did not survive: %+v", note.Note)
	}
	// Everything written offline is waiting to publish.
	if pending := h.exportPending().Pending; len(pending) < 3 {
		t.Fatalf("offline work queued %d transactions to publish", len(pending))
	}
}
