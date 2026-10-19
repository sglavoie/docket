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
