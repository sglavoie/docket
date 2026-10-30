package bridge

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/sglavoie/docket/internal/storage"
)

func TestTaskFieldUndoRedoPreservesPeerChangesAcrossDevices(t *testing.T) {
	for _, device := range []string{"desktop", "phone"} {
		for _, fields := range []map[string]any{
			{"title": "Local title"},
			{"priority": 1},
			{"context": "office"},
			{"resume_note": "Local resume"},
			{"title": "Local title", "priority": 1, "context": "", "resume_note": ""},
		} {
			t.Run(fmt.Sprintf("%s/%v", device, fields), func(t *testing.T) {
				p := newPair(t)
				origin, peer := p.desktop, p.phone
				if device == "phone" {
					origin, peer = peer, origin
				}
				settle := func() {
					t.Helper()
					if conflicts := p.settle(); len(conflicts) != 0 {
						t.Fatalf("sync conflicts: %+v", conflicts)
					}
				}
				var captured mutationResult
				origin.ok(origin.call("command.capture", map[string]any{
					"project_id": atlasProject, "destination": "backlog", "title": "Original title",
					"priority": 2, "context": "home", "resume_note": "Original resume",
				}), &captured)
				target := task(captured.Task.ProjectID, captured.Task.TaskID)
				before := origin.detail(target.ProjectID, target.TaskID).Task
				payload := map[string]any{"task": target}
				for key, value := range fields {
					payload[key] = value
				}
				var edited mutationResult
				origin.ok(origin.call("command.edit", payload), &edited)
				after := origin.detail(target.ProjectID, target.TaskID).Task
				settle()
				peer.ok(peer.call("command.transition", map[string]any{"task": target, "state": "doing"}), nil)
				peer.ok(peer.call("command.plan", map[string]any{"task": target, "action": "add_today"}), nil)
				for step, redo := range []bool{false, true, false, true} {
					peerEdit := map[string]any{"task": target, "description": fmt.Sprintf("Peer description %d", step)}
					if len(fields) == 1 {
						if _, titleEdit := fields["title"]; titleEdit {
							peerEdit["priority"] = step%3 + 1
						} else {
							peerEdit["title"] = fmt.Sprintf("Peer title %d", step)
						}
					}
					peer.ok(peer.call("command.edit", peerEdit), nil)
					settle()
					want := origin.detail(target.ProjectID, target.TaskID).Task
					values := before
					if redo {
						values = after
					}
					for field := range fields {
						switch field {
						case "title":
							want.Title = values.Title
						case "priority":
							want.Priority = values.Priority
						case "context":
							want.Context = values.Context
						case "resume_note":
							want.ResumeNote = values.ResumeNote
						}
					}
					origin.ok(origin.call("workspace.close", nil), nil)
					origin.openAs(device + "-device")
					if redo {
						if _, err := origin.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); err != nil {
							t.Fatal(err)
						}
					} else {
						origin.ok(origin.call("command.undo", map[string]any{"transaction_id": edited.Committed.TransactionID}), nil)
					}
					settle()
					for _, h := range []*harness{origin, peer} {
						got := h.detail(target.ProjectID, target.TaskID)
						if !reflect.DeepEqual(got.Task, want) || !got.Today || got.Next {
							t.Fatalf("redo=%v: restored task = %+v, want %+v with Today preserved", redo, got, want)
						}
					}
				}
			})
		}
	}
}

func TestTaskFieldUndoRedoRefusesOverlappingPeerEdit(t *testing.T) {
	for _, redo := range []bool{false, true} {
		t.Run(fmt.Sprintf("redo=%v", redo), func(t *testing.T) {
			p := newPair(t)
			target := task(atlasProject, releaseNotesTask)
			var edited mutationResult
			p.desktop.ok(p.desktop.call("command.edit", map[string]any{"task": target, "title": "Local title"}), &edited)
			if conflicts := p.settle(); len(conflicts) != 0 {
				t.Fatalf("edit conflicts: %+v", conflicts)
			}
			if redo {
				p.desktop.ok(p.desktop.call("command.undo", map[string]any{"transaction_id": edited.Committed.TransactionID}), nil)
				if conflicts := p.settle(); len(conflicts) != 0 {
					t.Fatalf("undo conflicts: %+v", conflicts)
				}
			}
			p.phone.ok(p.phone.call("command.edit", map[string]any{"task": target, "title": "Peer title", "priority": 3}), nil)
			if conflicts := p.settle(); len(conflicts) != 0 {
				t.Fatalf("peer edit conflicts: %+v", conflicts)
			}
			before := p.desktop.revisions()
			if redo {
				if _, err := p.desktop.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); err == nil {
					t.Fatal("redo overwrote an overlapping peer edit")
				}
			} else {
				p.desktop.rejects(p.desktop.call("command.undo", map[string]any{"transaction_id": edited.Committed.TransactionID}), CodeStaleRevision)
			}
			if p.desktop.revisions() != before {
				t.Fatal("refused restore changed workspace revisions")
			}
			requireTitle(t, p, target, "Peer title")
		})
	}
}
