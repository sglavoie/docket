package bridge

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/storage"
)

func TestDeletionUndoRedoPreservesOtherTasksAcrossDevices(t *testing.T) {
	for _, device := range []string{"desktop", "phone"} {
		t.Run(device, func(t *testing.T) {
			p := newPair(t)
			origin, peer := p.desktop, p.phone
			if device == "phone" {
				origin, peer = peer, origin
			}
			var captured mutationResult
			origin.ok(origin.call("command.capture", map[string]any{
				"project_id": atlasProject, "title": "Deleted on " + device, "destination": "backlog",
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
			// Task deletion is an application use case; the bridge has no
			// separate task-delete command. Its receipt undo uses this same ledger.
			deleted, err := origin.service().Delete(app.DeleteRequest{Locator: target.app(), Confirm: true})
			if err != nil {
				t.Fatal(err)
			}
			settle()
			peer.ok(peer.call("command.edit", map[string]any{"task": other, "title": "Incoming unrelated edit"}), nil)
			peer.ok(peer.call("command.plan", map[string]any{"task": other, "action": "add_today"}), nil)
			settle()
			for i, redo := range []bool{false, true, false, true} {
				origin.ok(origin.call("workspace.close", nil), nil)
				origin.openAs(device + "-device")
				if redo {
					if _, err := origin.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); err != nil {
						t.Fatal(err)
					}
				} else {
					origin.ok(origin.call("command.undo", map[string]any{"transaction_id": deleted.TransactionID}), nil)
				}
				settle()
				for _, h := range []*harness{origin, peer} {
					if redo {
						h.rejects(h.call("query.task", map[string]any{"task": target}), CodeNotFound)
					} else if got := h.detail(target.ProjectID, target.TaskID).Task; got.Title != captured.Task.Title || got.ShortID != captured.Task.ShortID {
						t.Fatalf("undo did not restore original task: %+v", got)
					}
					if !h.detail(other.ProjectID, other.TaskID).Today {
						t.Fatal("restore lost another task's planning membership")
					}
				}
				want := "Incoming unrelated edit"
				if i > 0 {
					want = fmt.Sprintf("Later incoming edit %d", i-1)
				}
				requireTitle(t, p, other, want)
				peer.ok(peer.call("command.edit", map[string]any{"task": other, "title": fmt.Sprintf("Later incoming edit %d", i)}), nil)
				settle()
			}
		})
	}
}

func TestDeletionRedoRefusesPeerChanges(t *testing.T) {
	for _, change := range []string{"edit", "child", "today", "next"} {
		t.Run(change, func(t *testing.T) {
			p := newPair(t)
			var captured mutationResult
			p.desktop.ok(p.desktop.call("command.capture", map[string]any{
				"project_id": atlasProject, "title": "Deleted A", "destination": "backlog",
			}), &captured)
			target := task(captured.Task.ProjectID, captured.Task.TaskID)
			deleted, err := p.desktop.service().Delete(app.DeleteRequest{Locator: target.app(), Confirm: true})
			if err != nil {
				t.Fatal(err)
			}
			p.desktop.ok(p.desktop.call("command.undo", map[string]any{"transaction_id": deleted.TransactionID}), nil)
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
			if _, err := p.desktop.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); !errors.Is(err, storage.ErrStaleArtifact) {
				t.Fatalf("redo error = %v, want ErrStaleArtifact", err)
			}
			if p.desktop.revisions() != before {
				t.Fatal("refused deletion redo changed workspace revisions")
			}
			want := "Deleted A"
			if change == "edit" {
				want = "Peer edited A"
			}
			requireTitle(t, p, target, want)
		})
	}
}

func TestPlannedDeletionUndoKeepsExactChecks(t *testing.T) {
	p := newPair(t)
	var captured mutationResult
	p.desktop.ok(p.desktop.call("command.capture", map[string]any{
		"project_id": atlasProject, "title": "Planned A", "destination": "today",
	}), &captured)
	target := task(captured.Task.ProjectID, captured.Task.TaskID)
	deleted, err := p.desktop.service().Delete(app.DeleteRequest{Locator: target.app(), Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	p.settle()
	other := task(atlasProject, screenshotsTask)
	p.phone.ok(p.phone.call("command.edit", map[string]any{"task": other, "title": "Incoming B"}), nil)
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("sync conflicts: %+v", conflicts)
	}
	before := p.desktop.revisions()
	p.desktop.rejects(p.desktop.call("command.undo", map[string]any{"transaction_id": deleted.TransactionID}), CodeStaleRevision)
	if p.desktop.revisions() != before {
		t.Fatal("refused coupled deletion undo changed workspace revisions")
	}
	p.desktop.rejects(p.desktop.call("query.task", map[string]any{"task": target}), CodeNotFound)
	requireTitle(t, p, other, "Incoming B")
}
