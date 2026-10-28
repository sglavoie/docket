package bridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestBridgeImportsSavedCapturesOnce drives the capture inbox a share
// extension and a shortcut write into: the engine imports each saved capture
// through the composer's own validation, exactly once whatever is delivered
// twice and wherever the extension or the app was terminated, and a capture
// it cannot import stays pending with its reason beside it.
func TestBridgeImportsSavedCapturesOnce(t *testing.T) {
	cases := []struct {
		name string
		run  func(*testing.T, *captureHarness)
	}{
		{"text_link_and_file_import", captureImportsEveryKind},
		{"duplicate_delivery", captureDuplicateDelivery},
		{"termination_after_commit", captureTerminationAfterCommit},
		{"termination_between_files", captureTerminationBetweenFiles},
		{"termination_after_receipt", captureTerminationAfterReceipt},
		{"interrupted_removal_is_finished", captureInterruptedRemoval},
		{"extension_termination_and_retry", captureExtensionTerminationAndRetry},
		{"refused_capture_stays_pending", captureRefusedStaysPending},
		{"damaged_file_imports_nothing", captureDamagedFile},
		{"other_workspace_is_left_alone", captureOtherWorkspace},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, representativeFixture)
			h.openAs("capture-phone")
			c.run(t, &captureHarness{harness: h, inbox: filepath.Join(filepath.Dir(h.dataDir), "app-group", "captures")})
		})
	}
}

type captureHarness struct {
	*harness
	inbox string
}

// sharedFile is one file a capture carries.
type sharedFile struct {
	name string
	data []byte
}

func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// save writes a capture the way the extension does: everything under
// .staging/<id>, the envelope last, then one rename into the inbox.
func (h *captureHarness) save(envelope captureEnvelope, files ...sharedFile) string {
	h.t.Helper()
	staging := h.stage(envelope, files...)
	final := filepath.Join(h.inbox, envelope.ID)
	if err := os.Rename(staging, final); err != nil {
		h.t.Fatal(err)
	}
	return final
}

// stage writes a capture's staging directory without publishing it.
func (h *captureHarness) stage(envelope captureEnvelope, files ...sharedFile) string {
	h.t.Helper()
	staging := filepath.Join(h.inbox, captureStagingDir, envelope.ID)
	if err := os.MkdirAll(filepath.Join(staging, "files"), 0o755); err != nil {
		h.t.Fatal(err)
	}
	for index, file := range files {
		relative := filepath.Join("files", strings.Repeat("0", 1)+string(rune('1'+index))+"-"+file.name)
		if err := os.WriteFile(filepath.Join(staging, relative), file.data, 0o644); err != nil {
			h.t.Fatal(err)
		}
		envelope.Files = append(envelope.Files, captureFile{Path: filepath.ToSlash(relative), Filename: file.name, Digest: digestOf(file.data), Size: int64(len(file.data))})
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, captureEnvelopeName), data, 0o644); err != nil {
		h.t.Fatal(err)
	}
	return staging
}

func (h *captureHarness) envelope(id string) captureEnvelope {
	return captureEnvelope{
		Format: CaptureFormat, ID: id, Workspace: h.workspace, Source: "share",
		CreatedAt: time.Date(2026, time.October, 6, 8, 0, 0, 0, time.UTC),
	}
}

func (h *captureHarness) importAll() importCapturesResult {
	h.t.Helper()
	var result importCapturesResult
	h.ok(h.call("capture.import", map[string]any{"captures_dir": h.inbox}), &result)
	return result
}

// interrupted runs one import that a termination cuts short at stage.
func (h *captureHarness) interrupted(stage string) {
	h.t.Helper()
	h.bridge.options.captureFault = func(at, _ string) error {
		if at == stage {
			return errCaptureFault
		}
		return nil
	}
	reply := h.call("capture.import", map[string]any{"captures_dir": h.inbox})
	if reply.OK {
		h.t.Fatalf("the import interrupted at %s answered ok: %s", stage, reply.Result)
	}
	// The process is gone: a new bridge, with no memory of the requests the
	// old one served, opens the same workspace.
	h.reopenAs("capture-phone")
}

func (h *harness) reopenAs(device string) {
	h.t.Helper()
	h.bridge = New(Options{Clock: h.clock, IDs: &sequentialIDs{next: 900}})
	h.openAs(device)
}

// titled counts the tasks across the workspace's Markdown carrying title.
func (h *captureHarness) titled(title string) int {
	h.t.Helper()
	matches, err := filepath.Glob(filepath.Join(h.dataDir, "projects", "*", "tasks.md"))
	if err != nil {
		h.t.Fatal(err)
	}
	count := 0
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			h.t.Fatal(err)
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			if line == "## Task: "+title {
				count++
			}
		}
	}
	return count
}

func (h *captureHarness) entryExists(id string) bool {
	_, err := os.Stat(filepath.Join(h.inbox, id))
	return err == nil
}

func (h *captureHarness) receiptExists(id string) bool {
	_, err := os.Stat(filepath.Join(h.runtimeDir, captureReceiptsDir, id+".json"))
	return err == nil
}

func (h *captureHarness) fileNames(projectID, taskID string) []string {
	h.t.Helper()
	var names []string
	for _, attachment := range h.detail(projectID, taskID).Attachments {
		names = append(names, attachment.Name)
	}
	return names
}

const (
	captureOne   = "6f1d2c3b-4a5e-4f60-8172-839405a6b7c8"
	captureTwo   = "7a2e3d4c-5b6f-4071-8283-94a516b7c8d9"
	captureThree = "8b3f4e5d-6c70-4182-9394-a5b627c8d9ea"
)

func captureImportsEveryKind(t *testing.T, h *captureHarness) {
	shortcut := h.envelope(captureOne)
	shortcut.Source = "shortcut"
	shortcut.Text = "Call the venue !1 #events\nBook the caterer"
	h.save(shortcut)

	shared := h.envelope(captureTwo)
	shared.CreatedAt = shortcut.CreatedAt.Add(time.Minute)
	shared.Title = "Read: release checklist"
	shared.Note = "Shared from Safari"
	shared.URLs = []string{"https://example.com/release?ref=share", "message://%3Cabc@example.com%3E"}
	scan := binaryContent(5, 40_000)
	h.save(shared, sharedFile{"scan.pdf", scan})

	result := h.importAll()
	if len(result.Imported) != 2 || len(result.Pending) != 0 {
		t.Fatalf("import answered %+v", result)
	}
	if result.Imported[0].CaptureID != captureOne || result.Imported[1].CaptureID != captureTwo {
		t.Fatalf("captures did not import oldest first: %+v", result.Imported)
	}
	first := result.Imported[0]
	if first.Outcome != "imported" || len(first.Tasks) != 2 {
		t.Fatalf("the shortcut capture answered %+v", first)
	}
	unfiled := h.unfiled()
	for index, view := range first.Tasks {
		if view.TaskID != capturedTaskID(captureOne, index) || view.ProjectID != unfiled.ID || view.State != "inbox" {
			t.Fatalf("task %d is %+v, want the derived identity in the Unfiled inbox", index, view)
		}
	}
	if call := first.Tasks[0]; call.Title != "Call the venue" || call.Priority != 1 || !slices.Equal(call.Tags, []string{"events"}) {
		t.Fatalf("the grammar was not read as the composer reads it: %+v", call)
	}

	read := h.detail(unfiled.ID, capturedTaskID(captureTwo, 0))
	if read.Task.Title != "Read: release checklist" || read.Task.Description != "Shared from Safari" {
		t.Fatalf("the shared capture is %+v", read.Task)
	}
	if len(read.Attachments) != 3 || read.Attachments[0].URL != "https://example.com/release?ref=share" ||
		read.Attachments[1].URL != "message://%3Cabc@example.com%3E" || read.Attachments[2].Name != "scan.pdf" {
		t.Fatalf("the shared links and file are %+v", read.Attachments)
	}
	if !bytes.Equal(h.attachmentBytes(&read.Attachments[2]), scan) {
		t.Fatal("the shared file's bytes did not reach the workspace whole")
	}
	if strings.Contains(h.unfiledMarkdown(), h.inbox) {
		t.Fatal("the workspace recorded the inbox path the file came from")
	}
	for _, id := range []string{captureOne, captureTwo} {
		if h.entryExists(id) || !h.receiptExists(id) {
			t.Fatalf("capture %s: entry left %v, receipt written %v", id, h.entryExists(id), h.receiptExists(id))
		}
	}
	if len(result.TransactionIDs) != 3 {
		t.Fatalf("transactions %q, want the two captures and the file", result.TransactionIDs)
	}
}

func (h *captureHarness) unfiledMarkdown() string {
	h.t.Helper()
	data, err := os.ReadFile(filepath.Join(h.dataDir, "projects", h.unfiled().Slug, "tasks.md"))
	if err != nil {
		h.t.Fatal(err)
	}
	return string(data)
}

func captureDuplicateDelivery(t *testing.T, h *captureHarness) {
	envelope := h.envelope(captureOne)
	envelope.Title = "Pay the invoice"
	h.save(envelope)
	if result := h.importAll(); len(result.Imported) != 1 || result.Imported[0].Outcome != "imported" {
		t.Fatalf("first delivery answered %+v", result)
	}
	// The extension delivers the same capture again — a retry after it was
	// imported, or a restored container — and a second import runs.
	h.save(envelope)
	result := h.importAll()
	if len(result.Imported) != 1 || result.Imported[0].Outcome != "already_imported" || len(result.TransactionIDs) != 0 {
		t.Fatalf("second delivery answered %+v", result)
	}
	if len(result.Imported[0].Tasks) != 1 || result.Imported[0].Tasks[0].Title != "Pay the invoice" {
		t.Fatalf("second delivery should name the task already imported: %+v", result.Imported[0])
	}
	if h.entryExists(captureOne) {
		t.Fatal("the duplicate stayed in the inbox")
	}
	// An empty inbox imports nothing at all.
	if result := h.importAll(); len(result.Imported)+len(result.Pending) != 0 {
		t.Fatalf("an empty inbox answered %+v", result)
	}
	if n := h.titled("Pay the invoice"); n != 1 {
		t.Fatalf("%d tasks titled Pay the invoice, want 1", n)
	}
}
