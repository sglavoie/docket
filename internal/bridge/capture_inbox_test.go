package bridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	shared.URLs = []string{"https://example.com/release?ref=share"}
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
	if len(read.Attachments) != 2 || read.Attachments[0].URL != "https://example.com/release?ref=share" || read.Attachments[1].Name != "scan.pdf" {
		t.Fatalf("the shared link and file are %+v", read.Attachments)
	}
	if !bytes.Equal(h.attachmentBytes(&read.Attachments[1]), scan) {
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

func captureTerminationAfterCommit(t *testing.T, h *captureHarness) {
	envelope := h.envelope(captureOne)
	envelope.Title = "Renew passport"
	photo := binaryContent(9, 20_000)
	h.save(envelope, sharedFile{"photo.jpg", photo})
	h.interrupted("created")
	if n := h.titled("Renew passport"); n != 1 || !h.entryExists(captureOne) || h.receiptExists(captureOne) {
		t.Fatalf("after the termination: %d tasks, entry %v, receipt %v", n, h.entryExists(captureOne), h.receiptExists(captureOne))
	}
	result := h.importAll()
	if len(result.Imported) != 1 || result.Imported[0].Outcome != "resumed" {
		t.Fatalf("the retry answered %+v", result)
	}
	if n := h.titled("Renew passport"); n != 1 {
		t.Fatalf("%d tasks titled Renew passport after the retry, want 1", n)
	}
	if names := h.fileNames(h.unfiled().ID, capturedTaskID(captureOne, 0)); !slices.Equal(names, []string{"photo.jpg"}) {
		t.Fatalf("attachments after the retry: %q", names)
	}
	if h.entryExists(captureOne) || !h.receiptExists(captureOne) {
		t.Fatal("the resumed capture was not receipted and removed")
	}
}

func captureTerminationBetweenFiles(t *testing.T, h *captureHarness) {
	envelope := h.envelope(captureOne)
	envelope.Title = "Tax documents"
	h.save(envelope, sharedFile{"w2.pdf", binaryContent(1, 9_000)}, sharedFile{"1099.pdf", binaryContent(2, 7_000)})
	h.interrupted("attached")
	target := capturedTaskID(captureOne, 0)
	if names := h.fileNames(h.unfiled().ID, target); !slices.Equal(names, []string{"w2.pdf"}) {
		t.Fatalf("after the termination the task holds %q", names)
	}
	result := h.importAll()
	if len(result.Imported) != 1 || result.Imported[0].Outcome != "resumed" || len(result.TransactionIDs) != 1 {
		t.Fatalf("the retry answered %+v", result)
	}
	if names := h.fileNames(h.unfiled().ID, target); !slices.Equal(names, []string{"w2.pdf", "1099.pdf"}) {
		t.Fatalf("after the retry the task holds %q, want each file once", names)
	}
}

func captureTerminationAfterReceipt(t *testing.T, h *captureHarness) {
	envelope := h.envelope(captureOne)
	envelope.Title = "Water the plants"
	h.save(envelope)
	h.interrupted("receipted")
	if !h.entryExists(captureOne) || !h.receiptExists(captureOne) {
		t.Fatal("the termination should leave the entry and its receipt")
	}
	result := h.importAll()
	if len(result.Imported) != 1 || result.Imported[0].Outcome != "already_imported" || len(result.TransactionIDs) != 0 {
		t.Fatalf("the retry answered %+v", result)
	}
	if n := h.titled("Water the plants"); n != 1 || h.entryExists(captureOne) {
		t.Fatalf("%d tasks, entry left %v", n, h.entryExists(captureOne))
	}
}

func captureInterruptedRemoval(t *testing.T, h *captureHarness) {
	envelope := h.envelope(captureOne)
	envelope.Title = "Return the library book"
	h.save(envelope)
	h.importAll()
	// A removal cut short after its rename leaves the entry under staging.
	leftover := filepath.Join(h.inbox, captureStagingDir, capturedRemovalPrefix+captureOne, "files")
	if err := os.MkdirAll(leftover, 0o755); err != nil {
		t.Fatal(err)
	}
	if result := h.importAll(); len(result.Imported)+len(result.Pending) != 0 {
		t.Fatalf("the leftover read as a capture: %+v", result)
	}
	if _, err := os.Stat(filepath.Dir(leftover)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the interrupted removal was not finished (stat err %v)", err)
	}
}

func captureExtensionTerminationAndRetry(t *testing.T, h *captureHarness) {
	envelope := h.envelope(captureOne)
	envelope.Title = "Compare flight prices"
	envelope.URLs = []string{"https://flights.example/search?from=YUL"}
	// The extension was terminated while writing: its staging directory holds
	// the file but no rename published it. Nothing imports and nothing is
	// pending.
	h.stage(envelope, sharedFile{"itinerary.pdf", binaryContent(4, 5_000)})
	if result := h.importAll(); len(result.Imported)+len(result.Pending) != 0 {
		t.Fatalf("an unpublished capture was read: %+v", result)
	}
	if n := h.titled("Compare flight prices"); n != 0 {
		t.Fatalf("an unpublished capture created %d tasks", n)
	}
	// The retry rewrites the same capture and publishes it once.
	h.save(envelope, sharedFile{"itinerary.pdf", binaryContent(4, 5_000)})
	result := h.importAll()
	if len(result.Imported) != 1 || result.Imported[0].Outcome != "imported" {
		t.Fatalf("the retried capture answered %+v", result)
	}
	if n := h.titled("Compare flight prices"); n != 1 {
		t.Fatalf("%d tasks after the retry, want 1", n)
	}
	// A staging copy no writer can still be finishing is swept a day later.
	abandoned := h.envelope(captureTwo)
	abandoned.Title = "Never published"
	path := h.stage(abandoned)
	old := h.clock.Now().Add(-2 * abandonedStagingAge)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	h.importAll()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an abandoned staging copy was kept (stat err %v)", err)
	}
}

func captureRefusedStaysPending(t *testing.T, h *captureHarness) {
	envelope := h.envelope(captureOne)
	envelope.Source = "shortcut"
	envelope.Text = "Plan the offsite $ready"
	dir := h.save(envelope)
	before := h.revisions()
	result := h.importAll()
	if len(result.Imported) != 0 || len(result.Pending) != 1 {
		t.Fatalf("a refused capture answered %+v", result)
	}
	held := result.Pending[0]
	if held.Reason != "refused" || held.Code != CodeValidation || len(held.Lines) != 1 || held.Lines[0].Line != 1 {
		t.Fatalf("the refusal is %+v", held)
	}
	if after := h.revisions(); before != after {
		t.Fatal("a refused capture moved a revision")
	}
	var status captureStatus
	data, err := os.ReadFile(filepath.Join(dir, captureStatusName))
	if err != nil {
		t.Fatalf("no status beside the refused capture: %v", err)
	}
	if err := json.Unmarshal(data, &status); err != nil || status.State != "refused" || status.Message != held.Message {
		t.Fatalf("status %s (%v)", data, err)
	}
	// It is retried, and still refused, by the next import.
	if again := h.importAll(); len(again.Pending) != 1 || again.Pending[0].Reason != "refused" {
		t.Fatalf("the retry answered %+v", again)
	}
	// Corrected in place, it imports.
	envelope.Text = "Plan the offsite $next"
	h.rewrite(dir, envelope)
	if fixed := h.importAll(); len(fixed.Imported) != 1 || fixed.Imported[0].Tasks[0].Title != "Plan the offsite" {
		t.Fatalf("the corrected capture answered %+v", fixed)
	}
	if h.entryExists(captureOne) {
		t.Fatal("the corrected capture stayed pending")
	}
}

func (h *captureHarness) rewrite(dir string, envelope captureEnvelope) {
	h.t.Helper()
	data, err := json.Marshal(envelope)
	if err != nil {
		h.t.Fatal(err)
	}
	if err := writeFileAtomically(filepath.Join(dir, captureEnvelopeName), data); err != nil {
		h.t.Fatal(err)
	}
}

func captureDamagedFile(t *testing.T, h *captureHarness) {
	envelope := h.envelope(captureOne)
	envelope.Title = "Insurance claim"
	dir := h.save(envelope, sharedFile{"claim.pdf", binaryContent(6, 8_000)})
	matches, _ := filepath.Glob(filepath.Join(dir, "files", "*claim.pdf"))
	if len(matches) != 1 {
		t.Fatalf("staged files %q", matches)
	}
	if err := os.WriteFile(matches[0], []byte("torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := h.importAll()
	if len(result.Pending) != 1 || result.Pending[0].Reason != "refused" || !strings.Contains(result.Pending[0].Message, "damaged") {
		t.Fatalf("a damaged capture answered %+v", result)
	}
	if n := h.titled("Insurance claim"); n != 0 {
		t.Fatalf("a damaged capture created %d tasks", n)
	}
}

func captureOtherWorkspace(t *testing.T, h *captureHarness) {
	envelope := h.envelope(captureOne)
	envelope.Title = "Belongs elsewhere"
	envelope.Workspace = "00000000-0000-4000-8000-0000000000ff"
	dir := h.save(envelope)
	result := h.importAll()
	if len(result.Pending) != 1 || result.Pending[0].Reason != "other_workspace" || len(result.Imported) != 0 {
		t.Fatalf("another workspace's capture answered %+v", result)
	}
	if _, err := os.Stat(filepath.Join(dir, captureStatusName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("another workspace's capture was written to")
	}
	if n := h.titled("Belongs elsewhere"); n != 0 {
		t.Fatalf("another workspace's capture created %d tasks", n)
	}
}

// TestBridgeAnswersAWidgetSnapshot holds the cache the widgets draw to Today
// and Inbox as the app evaluates them, with each row's identities resolving
// to the task, and to the day it was true for.
func TestBridgeAnswersAWidgetSnapshot(t *testing.T) {
	h := newHarness(t, representativeFixture)
	var status statusResult
	h.ok(h.call("workspace.open", map[string]any{
		"data_dir": h.dataDir, "runtime_dir": h.runtimeDir, "device_id": "widget-phone", "time_zone": "America/Toronto",
	}), &status)
	h.workspace = status.Workspace

	h.ok(h.call("command.capture", map[string]any{"text": "Overdue invoice @2026-10-01"}), nil)

	var snapshot widgetSnapshotResult
	h.ok(h.call("query.widget_snapshot", map[string]any{"limit": 3}), &snapshot)
	if snapshot.Format != WidgetSnapshotFormat || snapshot.Workspace != status.Workspace {
		t.Fatalf("snapshot header %+v", snapshot)
	}
	// 09:00 UTC is 05:00 in Toronto: the day is the sixth, and it ends at the
	// next local midnight.
	if snapshot.Day != "2026-10-06" || !snapshot.GeneratedAt.Equal(h.clock.Now()) ||
		!snapshot.ValidUntil.Equal(time.Date(2026, time.October, 7, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("snapshot freshness: day %s generated %s valid until %s", snapshot.Day, snapshot.GeneratedAt, snapshot.ValidUntil)
	}
	for _, view := range []struct {
		mode string
		list widgetList
	}{{"today", snapshot.Today}, {"inbox", snapshot.Inbox}} {
		var tasks tasksResult
		h.ok(h.call("query.tasks", map[string]any{"mode": view.mode, "sort": map[string]any{"field": "manual"}}), &tasks)
		if view.list.Count != len(tasks.Items) || len(view.list.Tasks) != min(3, len(tasks.Items)) {
			t.Fatalf("%s: count %d rows %d, the tab holds %d", view.mode, view.list.Count, len(view.list.Tasks), len(tasks.Items))
		}
		for index, row := range view.list.Tasks {
			want := tasks.Items[index].Task
			if row.TaskID != want.TaskID || row.ProjectID != want.ProjectID || row.Title != want.Title {
				t.Fatalf("%s row %d is %+v, the tab's is %+v", view.mode, index, row, want)
			}
			read := h.detail(row.ProjectID, row.TaskID)
			if read.Task.Title != row.Title || read.Project.Name != row.ProjectName {
				t.Fatalf("%s row %d does not link to its task: %+v", view.mode, index, read.Task)
			}
		}
	}
	if snapshot.Inbox.Count == 0 {
		t.Fatal("the fixture's inbox is empty, so the overdue mark is not exercised")
	}
	var overdue *widgetTask
	var all widgetSnapshotResult
	h.ok(h.call("query.widget_snapshot", map[string]any{"limit": maxWidgetLimit}), &all)
	for index := range all.Inbox.Tasks {
		if all.Inbox.Tasks[index].Title == "Overdue invoice" {
			overdue = &all.Inbox.Tasks[index]
		}
	}
	if overdue == nil || !overdue.Overdue {
		t.Fatalf("the overdue capture is %+v", overdue)
	}

	// After midnight the snapshot describes the new day.
	h.clock.advance(20 * time.Hour)
	h.ok(h.call("query.widget_snapshot", nil), &snapshot)
	if snapshot.Day != "2026-10-07" || !snapshot.ValidUntil.Equal(time.Date(2026, time.October, 8, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("the next day's snapshot: day %s valid until %s", snapshot.Day, snapshot.ValidUntil)
	}
	h.rejects(h.call("query.widget_snapshot", map[string]any{"limit": maxWidgetLimit + 1}), CodeInvalidRequest)
}
