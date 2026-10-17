package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReceiptUndoRestoresPlanningAndReplaysWithoutUndoingAnotherAction(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	var captured, completed mutationResult
	h.ok(h.call("command.capture", map[string]any{"title": "Receipt task", "destination": "today"}), &captured)
	target := locator{captured.Task.ProjectID, captured.Task.TaskID}
	h.ok(h.call("command.transition", map[string]any{"task": target, "state": "done"}), &completed)
	request := map[string]any{"transaction_id": completed.Committed.TransactionID}
	h.ok(h.call("command.undo", request, withRequestID("undo-receipt")), nil)
	var read taskResult
	h.ok(h.call("query.task", map[string]any{"task": target}), &read)
	if read.Task.State != "backlog" || !read.Today {
		t.Fatalf("undo did not restore status and planning: %+v", read)
	}
	h.ok(h.call("command.capture", map[string]any{"title": "Later action"}), nil)
	before := h.revisions()
	h.ok(h.call("command.undo", request, withRequestID("undo-receipt")), nil)
	if h.revisions() != before {
		t.Fatal("replayed undo changed another action")
	}
	h.rejects(h.call("command.undo", request), CodeRefused)
	if h.revisions() != before {
		t.Fatal("superseded receipt changed another action")
	}
	h.rejects(h.call("command.undo", nil), CodeInvalidRequest)
}

func TestReceiptUndoSurvivesReopeningAndRefusesStaleArtifacts(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	var captured mutationResult
	h.ok(h.call("command.capture", map[string]any{"title": "Receipt task"}), &captured)
	h.ok(h.call("workspace.close", nil), nil)
	h.open(false)
	h.ok(h.call("command.undo", map[string]any{"transaction_id": captured.Committed.TransactionID}), nil)
	h.rejects(h.call("query.task", map[string]any{"task": locator{captured.Task.ProjectID, captured.Task.TaskID}}), CodeNotFound)
	h.ok(h.call("command.capture", map[string]any{"title": "Local task"}), &captured)
	path := filepath.Join(h.dataDir, "projects", h.unfiled().Slug, "tasks.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.ReplaceAll(string(data), "Local task", "External edit")
	if err = os.WriteFile(path, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	h.rejects(h.call("command.undo", map[string]any{"transaction_id": captured.Committed.TransactionID}), CodeStaleRevision)
	after, err := os.ReadFile(path)
	if err != nil || string(after) != changed {
		t.Fatalf("undo overwrote an external edit: %s, %v", after, err)
	}
}

func TestReceiptUndoSyncsAndCannotOverwriteIncomingWork(t *testing.T) {
	p := newPair(t)
	target := task(atlasProject, releaseNotesTask)
	var changed mutationResult
	p.phone.ok(p.phone.call("command.plan", map[string]any{"task": target, "action": "add_next"}), &changed)
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("planning conflicts: %+v", conflicts)
	}
	p.phone.ok(p.phone.call("command.undo", map[string]any{"transaction_id": changed.Committed.TransactionID}), nil)
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("undo conflicts: %+v", conflicts)
	}
	if p.desktop.detail(atlasProject, releaseNotesTask).Next {
		t.Fatal("undo did not reach the desktop")
	}
	p.phone.ok(p.phone.call("command.transition", map[string]any{"task": target, "state": "doing"}), &changed)
	p.settle()
	p.desktop.ok(p.desktop.call("command.edit", map[string]any{"task": target, "title": "An incoming edit"}), nil)
	p.settle()
	p.phone.rejects(p.phone.call("command.undo", map[string]any{"transaction_id": changed.Committed.TransactionID}), CodeStaleRevision)
	if got := p.phone.detail(atlasProject, releaseNotesTask).Task.Title; got != "An incoming edit" {
		t.Fatalf("undo overwrote incoming work: %q", got)
	}
}
