package bridge

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/storage"
)

func TestLifecycleUndoRedoPreservesPeerChangesAcrossDevices(t *testing.T) {
	for _, device := range []string{"desktop", "phone"} {
		for _, tc := range []struct{ from, to string }{
			{"backlog", "inbox"}, {"doing", "backlog"}, {"backlog", "doing"},
			{"doing", "waiting"}, {"doing", "paused"}, {"doing", "someday"},
			{"waiting", "done"}, {"waiting", "canceled"}, {"done", "inbox"}, {"canceled", "inbox"},
		} {
			t.Run(device+"/"+tc.from+"_to_"+tc.to, func(t *testing.T) {
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
				var captured, transitioned mutationResult
				origin.ok(origin.call("command.capture", map[string]any{
					"project_id": atlasProject, "destination": tc.from, "title": "Status A",
				}), &captured)
				target := task(captured.Task.ProjectID, captured.Task.TaskID)
				if tc.from != "done" && tc.from != "canceled" {
					origin.ok(origin.call("command.check_in", map[string]any{
						"tasks": []locator{target}, "action": "set", "date": "2026-10-06", "repeat_days": 7,
					}), nil)
					origin.ok(origin.call("command.plan", map[string]any{"task": target, "action": "add_today"}), nil)
					if origin.detail(target.ProjectID, target.TaskID).Task.State != tc.from {
						origin.ok(origin.call("command.transition", map[string]any{"task": target, "state": tc.from}), nil)
					}
				}
				before := origin.detail(target.ProjectID, target.TaskID)
				origin.clock.advance(time.Minute)
				origin.ok(origin.call("command.transition", map[string]any{"task": target, "state": tc.to}), &transitioned)
				after := origin.detail(target.ProjectID, target.TaskID)
				settle()
				other := task(atlasProject, screenshotsTask)
				for step, redo := range []bool{false, true, false, true} {
					peer.clock.advance(2 * time.Minute)
					title, description := fmt.Sprintf("Peer title %d", step), fmt.Sprintf("Peer description %d", step)
					priority := step%3 + 1
					peer.ok(peer.call("command.edit", map[string]any{
						"task": target, "title": title, "priority": priority, "description": description,
					}), nil)
					peer.ok(peer.call("command.plan", map[string]any{"task": other, "action": "add_next"}), nil)
					settle()
					want := before
					if redo {
						want = after
					}
					want.Task.Title, want.Task.Description, want.Task.Priority = title, description, priority
					if touched := origin.detail(target.ProjectID, target.TaskID).Task.UpdatedAt; touched.After(want.Task.UpdatedAt) {
						want.Task.UpdatedAt = touched
					}
					origin.ok(origin.call("workspace.close", nil), nil)
					origin.openAs(device + "-device")
					if redo {
						if _, err := origin.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); err != nil {
							t.Fatal(err)
						}
					} else {
						origin.ok(origin.call("command.undo", map[string]any{"transaction_id": transitioned.Committed.TransactionID}), nil)
					}
					settle()
					for _, h := range []*harness{origin, peer} {
						got := h.detail(target.ProjectID, target.TaskID)
						if !reflect.DeepEqual(got.Task, want.Task) || got.Today != want.Today || got.Next != want.Next {
							t.Fatalf("step %d redo=%v: got %+v, want %+v", step, redo, got, want)
						}
						if detail := h.detail(other.ProjectID, other.TaskID); !detail.Next {
							t.Fatal("restore lost unrelated Next membership")
						}
					}
				}
			})
		}
	}
}

func TestLifecycleUndoRedoRefusesPeerStatusScheduleAndPlan(t *testing.T) {
	for _, redo := range []bool{false, true} {
		for _, change := range []string{"status", "check_in", "review_history", "plan", "new_child"} {
			// Reopening clears no schedule, so review history alone is unrelated
			// to Undo of that action. Completion's changed schedule is checked
			// on Redo here and in both directions by storage tests.
			if !redo && change == "review_history" {
				continue
			}
			t.Run(fmt.Sprintf("%s/redo=%v", change, redo), func(t *testing.T) {
				p := newPair(t)
				var captured, transitioned mutationResult
				p.desktop.ok(p.desktop.call("command.capture", map[string]any{
					"project_id": atlasProject, "destination": "waiting", "title": "Status A",
				}), &captured)
				target := task(captured.Task.ProjectID, captured.Task.TaskID)
				p.desktop.ok(p.desktop.call("command.check_in", map[string]any{
					"tasks": []locator{target}, "action": "set", "date": "2026-10-06", "repeat_days": 7,
				}), nil)
				// Restore a terminal state on Undo, a nonterminal state on Redo,
				// so each peer mutation below is legal before the attempted restore.
				p.desktop.ok(p.desktop.call("command.transition", map[string]any{"task": target, "state": "done"}), nil)
				p.desktop.ok(p.desktop.call("command.transition", map[string]any{"task": target, "state": "inbox"}), &transitioned)
				if redo {
					// Instead test Redo of completion, including its check-in cleanup.
					p.desktop.ok(p.desktop.call("command.check_in", map[string]any{
						"tasks": []locator{target}, "action": "set", "date": "2026-10-06", "repeat_days": 7,
					}), nil)
					p.desktop.ok(p.desktop.call("command.plan", map[string]any{"task": target, "action": "add_today"}), nil)
					p.desktop.ok(p.desktop.call("command.transition", map[string]any{"task": target, "state": "done"}), &transitioned)
					p.desktop.ok(p.desktop.call("command.undo", map[string]any{"transaction_id": transitioned.Committed.TransactionID}), nil)
				}
				if conflicts := p.settle(); len(conflicts) != 0 {
					t.Fatalf("setup conflicts: %+v", conflicts)
				}
				p.phone.ok(p.phone.call("command.edit", map[string]any{"task": target, "title": "Peer title"}), nil)
				switch change {
				case "status":
					p.phone.ok(p.phone.call("command.transition", map[string]any{"task": target, "state": "doing"}), nil)
				case "check_in":
					p.phone.ok(p.phone.call("command.check_in", map[string]any{"tasks": []locator{target}, "action": "set", "date": "2026-10-09", "repeat_days": 14}), nil)
				case "review_history":
					p.phone.ok(p.phone.call("command.check_in", map[string]any{"tasks": []locator{target}, "action": "acknowledge"}), nil)
				case "plan":
					p.phone.ok(p.phone.call("command.plan", map[string]any{"task": target, "action": "add_next"}), nil)
				case "new_child":
					p.phone.ok(p.phone.call("command.capture", map[string]any{"project_id": atlasProject, "text": fmt.Sprintf("New open child >%d", p.phone.detail(target.ProjectID, target.TaskID).Task.ShortID)}), nil)
				}
				if conflicts := p.settle(); len(conflicts) != 0 {
					t.Fatalf("peer conflicts: %+v", conflicts)
				}
				before := p.desktop.revisions()
				if redo {
					if _, err := p.desktop.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); !errors.Is(err, storage.ErrStaleArtifact) {
						t.Fatalf("redo error = %v, want stale artifact", err)
					}
				} else {
					p.desktop.rejects(p.desktop.call("command.undo", map[string]any{"transaction_id": transitioned.Committed.TransactionID}), CodeStaleRevision)
				}
				if p.desktop.revisions() != before {
					t.Fatal("refused restore changed workspace revisions")
				}
				requireTitle(t, p, target, "Peer title")
			})
		}
	}
}

func TestLifecycleBulkRestorePreservesPeerFieldsAndRefusesWholeOnOverlap(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		t.Run(fmt.Sprintf("overlap=%v", refuse), func(t *testing.T) {
			p := newPair(t)
			var targets []locator
			for _, projectID := range []string{atlasProject, homeProject} {
				var captured mutationResult
				p.desktop.ok(p.desktop.call("command.capture", map[string]any{"project_id": projectID, "title": "Bulk target", "destination": "today"}), &captured)
				targets = append(targets, task(captured.Task.ProjectID, captured.Task.TaskID))
			}
			p.desktop.ok(p.desktop.call("command.check_in", map[string]any{"tasks": targets, "action": "set", "date": "2026-10-06", "repeat_days": 7}), nil)
			var completed tasksResultView
			p.desktop.ok(p.desktop.call("command.bulk", map[string]any{"tasks": targets, "action": "state", "state": "done"}), &completed)
			settle := func() {
				t.Helper()
				if conflicts := p.settle(); len(conflicts) != 0 {
					t.Fatalf("sync conflicts: %+v", conflicts)
				}
			}
			settle()
			for _, target := range targets {
				p.phone.ok(p.phone.call("command.edit", map[string]any{"task": target, "title": "Peer bulk title", "priority": 1}), nil)
			}
			if refuse {
				p.phone.ok(p.phone.call("command.transition", map[string]any{"task": targets[1], "state": "inbox"}), nil)
			}
			settle()
			before := p.desktop.revisions()
			if refuse {
				p.desktop.rejects(p.desktop.call("command.undo", map[string]any{"transaction_id": completed.Committed.TransactionID}), CodeStaleRevision)
				if p.desktop.revisions() != before {
					t.Fatal("refused bulk restore partially wrote a project or planning")
				}
				return
			}
			p.desktop.ok(p.desktop.call("command.undo", map[string]any{"transaction_id": completed.Committed.TransactionID}), nil)
			settle()
			for _, redo := range []bool{false, true} {
				if redo {
					for _, target := range targets {
						p.phone.ok(p.phone.call("command.edit", map[string]any{"task": target, "description": "Newer peer work"}), nil)
					}
					settle()
					if _, err := p.desktop.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); err != nil {
						t.Fatal(err)
					}
					settle()
				}
				for _, h := range []*harness{p.desktop, p.phone} {
					for _, target := range targets {
						got := h.detail(target.ProjectID, target.TaskID)
						if got.Task.Title != "Peer bulk title" || got.Task.Priority != 1 || got.Today == redo || (got.Task.State == "done") != redo || (got.Task.NextCheckIn == nil) != redo {
							t.Fatalf("redo=%v: task or coupled effects lost: %+v", redo, got)
						}
						if redo && got.Task.Description != "Newer peer work" {
							t.Fatal("Redo lost newer peer edit")
						}
					}
				}
			}
		})
	}
}
