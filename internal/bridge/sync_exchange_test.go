package bridge

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/syncexchange"
)

// openAs opens the harness workspace under a named device.
func (h *harness) openAs(deviceID string) {
	h.t.Helper()
	var status statusResult
	h.ok(h.call("workspace.open", map[string]any{
		"data_dir": h.dataDir, "runtime_dir": h.runtimeDir, "device_id": deviceID,
	}), &status)
	h.workspace = status.Workspace
}

func (h *harness) exportPending() exportResult {
	h.t.Helper()
	var exported exportResult
	h.ok(h.call("sync.export_pending", nil), &exported)
	return exported
}

func (h *harness) importInbox(inbox string) syncexchange.Report {
	h.t.Helper()
	var report syncexchange.Report
	h.ok(h.call("sync.import", map[string]any{"inbox_dir": inbox}), &report)
	return report
}

func (h *harness) syncStatus() syncStatusResult {
	h.t.Helper()
	var status syncStatusResult
	h.ok(h.call("sync.status", nil), &status)
	return status
}

func (h *harness) hasTask(title string) bool {
	h.t.Helper()
	var all tasksResult
	h.ok(h.call("query.tasks", map[string]any{"mode": "all"}), &all)
	for _, item := range all.Items {
		if item.Task.Title == title {
			return true
		}
	}
	return false
}

// stage copies one published file into an inbox, as a transport delivers it.
func stage(t *testing.T, inbox string, file syncexchange.File, data []byte) {
	t.Helper()
	target := filepath.Join(inbox, filepath.FromSlash(file.Path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if data == nil {
		var err error
		if data, err = os.ReadFile(file.Source); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func onlyResult(t *testing.T, report syncexchange.Report) syncexchange.Result {
	t.Helper()
	if len(report.Results) != 1 {
		t.Fatalf("want one staged manifest, got %+v", report.Results)
	}
	return report.Results[0]
}

// TestBridgeSyncExchangeImportsOnlyVerifiedContent drives the engine half of
// the exchange contract through the bridge between two devices that start
// from the same workspace: export lists every file to publish, an import
// applies a manifest only once every blob it names is staged and intact, and
// missing content is reported as unavailable while the local files stay.
// exchangeFixture is one Mac capture exported for a phone that has not seen
// it: the phone's tasks.md as it was, and an empty inbox to stage into.
type exchangeFixture struct {
	mac, phone *harness
	outgoing   syncexchange.Outgoing
	inbox      string
	tasksPath  string
	before     []byte
}

func newExchangeFixture(t *testing.T) exchangeFixture {
	t.Helper()
	mac := newHarness(t, representativeFixture)
	phone := newHarness(t, representativeFixture)
	mac.openAs("mac-device")
	phone.openAs("phone-device")
	if mac.workspace != phone.workspace {
		t.Fatalf("the two copies name different workspaces")
	}
	unfiled := mac.unfiled()
	mac.ok(mac.call("command.capture", map[string]any{"project_id": unfiled.ID, "title": "From the Mac"}), nil)

	exported := mac.exportPending()
	if exported.DeviceID != "mac-device" || len(exported.Pending) != 1 {
		t.Fatalf("export %+v", exported)
	}
	outgoing := exported.Pending[0]
	if want := syncexchange.ManifestPath("mac-device", outgoing.DeviceSeq, outgoing.TxnID); outgoing.Manifest.Path != want {
		t.Fatalf("manifest path %s, want %s", outgoing.Manifest.Path, want)
	}
	if len(outgoing.Blobs) < 2 {
		t.Fatalf("a task edit publishes its base and new bytes: %+v", outgoing.Blobs)
	}
	if status := mac.syncStatus(); len(status.Outbox) != 1 || status.Outbox[0].TxnID != outgoing.TxnID {
		t.Fatalf("status outbox %+v", status.Outbox)
	}

	inbox := filepath.Join(t.TempDir(), "inbox")
	tasksPath := filepath.Join(phone.dataDir, "projects", "unfiled", "tasks.md")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	return exchangeFixture{mac: mac, phone: phone, outgoing: outgoing, inbox: inbox, tasksPath: tasksPath, before: before}
}

func TestBridgeSyncExchangeImportsOnlyVerifiedContent(t *testing.T) {
	f := newExchangeFixture(t)
	mac, phone, outgoing, inbox, tasksPath, before := f.mac, f.phone, f.outgoing, f.inbox, f.tasksPath, f.before

	t.Run("manifest_without_content_waits", func(t *testing.T) {
		stage(t, inbox, outgoing.Manifest, nil)
		result := onlyResult(t, phone.importInbox(inbox))
		if result.Outcome != syncexchange.Incomplete || len(result.Missing) == 0 {
			t.Fatalf("result %+v", result)
		}
		if !slices.Contains(result.Unavailable, "projects/unfiled/tasks.md") {
			t.Fatalf("the waiting content is not reported unavailable: %+v", result.Unavailable)
		}
	})
	t.Run("damaged_content_waits", func(t *testing.T) {
		for _, blob := range outgoing.Blobs {
			data, err := os.ReadFile(blob.Source)
			if err != nil {
				t.Fatal(err)
			}
			stage(t, inbox, blob, data[:len(data)/2])
		}
		result := onlyResult(t, phone.importInbox(inbox))
		if result.Outcome != syncexchange.Incomplete {
			t.Fatalf("a truncated transfer was imported: %+v", result)
		}
		after, err := os.ReadFile(tasksPath)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("an incomplete import changed the workspace (err %v)", err)
		}
		if phone.hasTask("From the Mac") {
			t.Fatal("the task appeared before its content was verified")
		}
	})
	t.Run("verified_content_imports_once", func(t *testing.T) {
		for _, blob := range outgoing.Blobs {
			stage(t, inbox, blob, nil)
		}
		result := onlyResult(t, phone.importInbox(inbox))
		if result.Outcome != syncexchange.Imported || result.LocalTxnID == "" {
			t.Fatalf("result %+v", result)
		}
		if !phone.hasTask("From the Mac") {
			t.Fatal("imported task is not readable")
		}
		if again := onlyResult(t, phone.importInbox(inbox)); again.Outcome != syncexchange.Known {
			t.Fatalf("a redelivered manifest was not recognized: %+v", again)
		}
		if status := phone.syncStatus(); status.Received["mac-device"] != outgoing.DeviceSeq {
			t.Fatalf("receipt %+v", status.Received)
		}
	})
	t.Run("misnamed_manifest_is_invalid", func(t *testing.T) {
		other := filepath.Join(t.TempDir(), "inbox")
		misnamed := outgoing.Manifest
		misnamed.Path = syncexchange.ManifestPath("mac-device", outgoing.DeviceSeq+1, outgoing.TxnID)
		stage(t, other, misnamed, nil)
		if result := onlyResult(t, phone.importInbox(other)); result.Outcome != syncexchange.Invalid {
			t.Fatalf("result %+v", result)
		}
	})
	t.Run("acknowledge_empties_the_outbox", func(t *testing.T) {
		mac.ok(mac.call("sync.acknowledge", map[string]any{"txn_ids": []string{outgoing.TxnID}}), nil)
		if status := mac.syncStatus(); len(status.Outbox) != 0 {
			t.Fatalf("outbox %+v", status.Outbox)
		}
		if again := mac.exportPending(); len(again.Pending) != 0 {
			t.Fatalf("export after acknowledge %+v", again.Pending)
		}
	})
	t.Run("missing_attachment_is_unavailable_not_deleted", func(t *testing.T) {
		rel := "projects/atlas-launch/.attachments/session-8-evidence.txt"
		if err := os.WriteFile(filepath.Join(mac.dataDir, filepath.FromSlash(rel)), []byte("attachment bytes\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		exported := mac.exportPending()
		if exported.External == nil || exported.External.Imported == nil || len(exported.Pending) != 1 {
			t.Fatalf("an external attachment was not published: %+v", exported)
		}
		attachment := exported.Pending[0]
		box := filepath.Join(t.TempDir(), "inbox")
		stage(t, box, attachment.Manifest, nil)
		result := onlyResult(t, phone.importInbox(box))
		if result.Outcome != syncexchange.Incomplete || !slices.Contains(result.Unavailable, rel) {
			t.Fatalf("result %+v", result)
		}
		if _, err := os.Stat(filepath.Join(phone.dataDir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("unavailable content was materialized (err %v)", err)
		}
		for _, blob := range attachment.Blobs {
			stage(t, box, blob, nil)
		}
		if result := onlyResult(t, phone.importInbox(box)); result.Outcome != syncexchange.Imported {
			t.Fatalf("result %+v", result)
		}
		data, err := os.ReadFile(filepath.Join(phone.dataDir, filepath.FromSlash(rel)))
		if err != nil || string(data) != "attachment bytes\n" {
			t.Fatalf("attachment after import %q (err %v)", data, err)
		}
	})
}
