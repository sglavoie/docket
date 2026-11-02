package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (h *harness) history() historyResult {
	h.t.Helper()
	var result historyResult
	h.ok(h.call("query.history", nil), &result)
	return result
}

func TestChangeHistoryPersistsDetailsAndRecoveryOrderAcrossRestart(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	if got := h.history(); len(got.Entries) != 0 || got.NextUndo != "" || got.NextRedo != "" {
		t.Fatalf("new device history: %+v", got)
	}
	target := task(atlasProject, releaseNotesTask)
	original := h.detail(atlasProject, releaseNotesTask).Task.Title
	var first, second mutationResult
	h.ok(h.call("command.edit", map[string]any{"task": target, "title": "Accidental iPhone edit"}), &first)
	h.ok(h.call("command.capture", map[string]any{"title": "A second change"}), &second)
	h.ok(h.call("workspace.close", nil), nil)
	h.open(false)
	history := h.history()
	if len(history.Entries) != 2 || history.NextUndo != second.Committed.TransactionID || history.Entries[1].TransactionID != first.Committed.TransactionID || history.Entries[1].Undone || len(history.Entries[1].Projects) == 0 || history.Entries[1].At.IsZero() {
		t.Fatalf("history after restart: %+v", history)
	}
	var details struct {
		Changes []struct{ Entity, Field, Before, After string }
	}
	before := h.revisions()
	h.ok(h.call("query.history_detail", map[string]any{"transaction_id": first.Committed.TransactionID}), &details)
	found := false
	for _, change := range details.Changes {
		if change.Field == "Title" && change.Before == original && change.After == "Accidental iPhone edit" {
			found = true
		}
	}
	if !found || before != h.revisions() {
		t.Fatalf("inspection should show the retained edit without mutating: %+v", details)
	}
	// Inspecting the older row never selects it as the recovery target.
	h.rejects(h.call("command.undo", map[string]any{"transaction_id": first.Committed.TransactionID}), CodeRefused)
	for _, id := range []string{second.Committed.TransactionID, first.Committed.TransactionID} {
		h.ok(h.call("command.undo", map[string]any{"transaction_id": id}), nil)
	}
	history = h.history()
	if history.NextRedo != first.Committed.TransactionID || history.NextUndo != "" || !history.Entries[0].Undone || history.Entries[0].TransactionID != second.Committed.TransactionID {
		t.Fatalf("redo order must differ from newest-first timeline: %+v", history)
	}
	h.rejects(h.call("command.redo", map[string]any{"transaction_id": second.Committed.TransactionID}), CodeRefused)
	h.ok(h.call("command.redo", map[string]any{"transaction_id": first.Committed.TransactionID}, withRequestID("redo-first")), nil)
	history = h.history()
	if history.NextUndo != first.Committed.TransactionID || history.NextRedo != second.Committed.TransactionID || history.Entries[1].Undone {
		t.Fatalf("redo snapshot: %+v", history)
	}
	h.ok(h.call("command.redo", map[string]any{"transaction_id": second.Committed.TransactionID}), nil)
	before = h.revisions()
	h.ok(h.call("command.redo", map[string]any{"transaction_id": first.Committed.TransactionID}, withRequestID("redo-first")), nil)
	if h.revisions() != before {
		t.Fatal("replayed redo changed a different transaction")
	}
	h.ok(h.call("command.undo", map[string]any{"transaction_id": second.Committed.TransactionID}), nil)
	h.ok(h.call("command.capture", map[string]any{"title": "New forward change clears redo"}), nil)
	if history = h.history(); history.NextRedo != "" {
		t.Fatalf("new change retained redo: %+v", history)
	}
	h.rejects(h.call("query.history_detail", nil), CodeInvalidRequest)
	h.rejects(h.call("command.redo", nil), CodeInvalidRequest)
	h.rejects(h.call("query.history", map[string]any{"unexpected": true}), CodeInvalidRequest)
	h.rejects(h.call("query.history_detail", map[string]any{"transaction_id": second.Committed.TransactionID}), CodeRefused)
}

func TestChangeHistoryRecoveryAfterRestartPreservesPeerEdits(t *testing.T) {
	p := newPair(t)
	target := task(atlasProject, releaseNotesTask)
	other := task(atlasProject, screenshotsTask)
	original := p.phone.detail(atlasProject, releaseNotesTask).Task.Title
	var edited mutationResult
	p.phone.ok(p.phone.call("command.edit", map[string]any{"task": target, "title": "Accidental phone edit"}), &edited)
	p.settle()
	p.desktop.ok(p.desktop.call("command.edit", map[string]any{"task": other, "title": "Unrelated Mac work"}), nil)
	p.settle()
	deviceID := p.phone.bridge.workspace.deviceID
	p.phone.ok(p.phone.call("workspace.close", nil), nil)
	p.phone.openAs(deviceID)
	history := p.phone.history()
	if len(history.Entries) != 1 || history.NextUndo != edited.Committed.TransactionID {
		t.Fatalf("history should contain only this device's change: %+v", history)
	}
	request := map[string]any{"transaction_id": history.NextUndo}
	p.phone.ok(p.phone.call("command.undo", request), nil)
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("undo conflicts: %+v", conflicts)
	}
	requireTitle(t, p, target, original)
	requireTitle(t, p, other, "Unrelated Mac work")
	p.phone.ok(p.phone.call("command.redo", request), nil)
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("redo conflicts: %+v", conflicts)
	}
	requireTitle(t, p, target, "Accidental phone edit")
	requireTitle(t, p, other, "Unrelated Mac work")
	// A peer overwriting the same field must refuse rather than overwrite it.
	p.desktop.ok(p.desktop.call("command.edit", map[string]any{"task": target, "title": "Intentional Mac correction"}), nil)
	p.settle()
	p.phone.rejects(p.phone.call("command.undo", request), CodeStaleRevision)
	requireTitle(t, p, target, "Intentional Mac correction")
}

func TestChangeHistoryUnreadableImagesKeepTheEntry(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	var created mutationResult
	h.ok(h.call("command.capture", map[string]any{"title": "Retained image"}), &created)
	// Corrupt retained bytes only: inspection must fail, without pruning history.
	root := filepath.Join(h.dataDir, ".docket", "undo")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && !strings.HasSuffix(path, ".json") && !strings.HasSuffix(path, ".lock") {
			return os.WriteFile(path, []byte("corrupt retained image"), 0600)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	h.rejects(h.call("query.history_detail", map[string]any{"transaction_id": created.Committed.TransactionID}), CodeRefused)
	if got := h.history(); len(got.Entries) != 1 || got.NextUndo != created.Committed.TransactionID {
		t.Fatalf("inspection retired an entry: %+v", got)
	}
}
