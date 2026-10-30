package bridge

import (
	"fmt"
	"testing"

	"github.com/sglavoie/docket/internal/storage"
)

func TestCreationUndoRedoPreservesOtherTasksAcrossDevices(t *testing.T) {
	for _, device := range []string{"desktop", "phone"} {
		t.Run(device, func(t *testing.T) {
			p := newPair(t)
			origin, peer := p.desktop, p.phone
			if device == "phone" {
				origin, peer = peer, origin
			}
			var captured mutationResult
			origin.ok(origin.call("command.capture", map[string]any{
				"project_id": atlasProject, "title": "Created on " + device,
			}), &captured)
			target := task(captured.Task.ProjectID, captured.Task.TaskID)
			other := task(atlasProject, screenshotsTask)
			settle := func() {
				t.Helper()
				if conflicts := p.settle(); len(conflicts) != 0 {
					t.Fatalf("sync conflicts: %+v", conflicts)
				}
			}
			settle()
			peer.ok(peer.call("command.edit", map[string]any{"task": other, "title": "Incoming unrelated edit"}), nil)
			peer.ok(peer.call("command.plan", map[string]any{"task": other, "action": "add_today"}), nil)
			settle()
			// The phone's receipt survives a reopen, as does the original
			// creation intent on the desktop's redo stack.
			origin.ok(origin.call("workspace.close", nil), nil)
			origin.openAs(device + "-device")
			origin.ok(origin.call("command.undo", map[string]any{"transaction_id": captured.Committed.TransactionID}), nil)
			settle()
			for _, h := range []*harness{origin, peer} {
				h.rejects(h.call("query.task", map[string]any{"task": target}), CodeNotFound)
			}
			requireTitle(t, p, other, "Incoming unrelated edit")
			peer.ok(peer.call("command.edit", map[string]any{"task": other, "title": "Later incoming edit"}), nil)
			settle()
			if _, err := origin.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); err != nil {
				t.Fatal(err)
			}
			settle()
			requireTitle(t, p, target, "Created on "+device)
			requireTitle(t, p, other, "Later incoming edit")
			for _, h := range []*harness{origin, peer} {
				if !h.detail(other.ProjectID, other.TaskID).Today {
					t.Fatal("restore lost the peer's plan for another task")
				}
			}
			origin.ok(origin.call("command.undo", map[string]any{"transaction_id": captured.Committed.TransactionID}), nil)
			settle()
			for _, h := range []*harness{origin, peer} {
				h.rejects(h.call("query.task", map[string]any{"task": target}), CodeNotFound)
			}
			requireTitle(t, p, other, "Later incoming edit")
		})
	}
}

func TestCreationUndoRefusesPeerEditOrChild(t *testing.T) {
	for _, change := range []string{"edit", "child", "today", "next"} {
		t.Run(change, func(t *testing.T) {
			p := newPair(t)
			var captured mutationResult
			p.desktop.ok(p.desktop.call("command.capture", map[string]any{"project_id": atlasProject, "title": "Created A", "destination": "backlog"}), &captured)
			target := task(captured.Task.ProjectID, captured.Task.TaskID)
			p.settle()
			switch change {
			case "edit":
				p.phone.ok(p.phone.call("command.edit", map[string]any{"task": target, "title": "Peer edited A"}), nil)
			case "child":
				p.phone.ok(p.phone.call("command.capture", map[string]any{
					"project_id": atlasProject, "text": fmt.Sprintf("Peer child >%d", captured.Task.ShortID),
				}), nil)
			default:
				p.phone.ok(p.phone.call("command.plan", map[string]any{"task": target, "action": "add_" + change}), nil)
			}
			if conflicts := p.settle(); len(conflicts) != 0 {
				t.Fatalf("sync conflicts: %+v", conflicts)
			}
			before := p.desktop.revisions()
			p.desktop.rejects(p.desktop.call("command.undo", map[string]any{"transaction_id": captured.Committed.TransactionID}), CodeStaleRevision)
			if p.desktop.revisions() != before {
				t.Fatal("refused creation undo changed workspace revisions")
			}
			if change == "edit" {
				requireTitle(t, p, target, "Peer edited A")
			} else {
				requireTitle(t, p, target, "Created A")
			}
		})
	}
}
