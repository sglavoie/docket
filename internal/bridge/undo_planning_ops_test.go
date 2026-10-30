package bridge

import (
	"errors"
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
)

func TestPlanningUndoRedoPreservesOtherPlansAcrossDevices(t *testing.T) {
	for _, device := range []string{"desktop", "phone"} {
		for _, tc := range []struct {
			name, start, action string
			transition          bool
			beforeToday         bool
			beforeNext          bool
			afterToday          bool
			afterNext           bool
		}{
			{name: "add_today", start: "backlog", action: "add_today", afterToday: true},
			{name: "add_next", start: "backlog", action: "add_next", afterNext: true},
			{name: "next_to_today", start: "next", action: "add_today", beforeNext: true, afterToday: true},
			{name: "remove_today", start: "today", action: "remove_today", beforeToday: true},
			{name: "complete_today", start: "today", action: "done", transition: true, beforeToday: true},
		} {
			t.Run(device+"/"+tc.name, func(t *testing.T) {
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
				var captured, planned mutationResult
				origin.ok(origin.call("command.capture", map[string]any{
					"project_id": atlasProject, "title": "Plan A", "destination": tc.start,
				}), &captured)
				target := task(captured.Task.ProjectID, captured.Task.TaskID)
				method, key := "command.plan", "action"
				if tc.transition {
					method, key = "command.transition", "state"
				}
				origin.ok(origin.call(method, map[string]any{"task": target, key: tc.action}), &planned)
				settle()
				other := task(atlasProject, screenshotsTask)
				wantTitle := peer.detail(other.ProjectID, other.TaskID).Task.Title
				if !tc.transition {
					wantTitle = "Peer's B"
					peer.ok(peer.call("command.edit", map[string]any{"task": other, "title": wantTitle}), nil)
				}
				for step, redo := range []bool{false, true, false, true} {
					// New planning reaches the origin between every restore. The
					// next restore must use it even after reopening the workspace.
					action := "add_next"
					if step%2 == 1 {
						action = "add_today"
					}
					peer.ok(peer.call("command.plan", map[string]any{"task": other, "action": action}), nil)
					settle()
					before := planningWithoutTarget(t, origin, target)
					origin.ok(origin.call("workspace.close", nil), nil)
					origin.openAs(device + "-device")

					if redo {
						if _, err := origin.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); err != nil {
							t.Fatal(err)
						}
					} else {
						origin.ok(origin.call("command.undo", map[string]any{"transaction_id": planned.Committed.TransactionID}), nil)
					}
					settle()
					for _, h := range []*harness{origin, peer} {
						detail := h.detail(target.ProjectID, target.TaskID)
						today, next := tc.beforeToday, tc.beforeNext
						if redo {
							today, next = tc.afterToday, tc.afterNext
						}
						if detail.Today != today || detail.Next != next {
							t.Fatalf("redo=%v: Today=%v Next=%v, want %v/%v", redo, detail.Today, detail.Next, today, next)
						}
						if tc.transition {
							wantState := "backlog"
							if redo {
								wantState = "done"
							}
							if detail.Task.State != wantState {
								t.Fatalf("redo=%v: state=%s, want %s", redo, detail.Task.State, wantState)
							}
						}
						after := planningWithoutTarget(t, h, target)
						if !slices.Equal(before.Today, after.Today) || !slices.Equal(before.Next, after.Next) {
							t.Fatalf("restore changed unrelated memberships or order:\nbefore %+v\nafter %+v", before, after)
						}
					}
					requireTitle(t, p, other, wantTitle)
				}
			})
		}
	}
}

func TestPlanningRestoreRefusesMissingOrTerminalTargets(t *testing.T) {
	for _, redo := range []bool{false, true} {
		for _, change := range []string{"complete", "delete"} {
			name := "undo/"
			if redo {
				name = "redo/"
			}
			t.Run(name+change, func(t *testing.T) {
				p := newPair(t)
				var captured, planned mutationResult
				p.desktop.ok(p.desktop.call("command.capture", map[string]any{
					"project_id": atlasProject, "title": "Plan A", "destination": "today",
				}), &captured)
				target := task(captured.Task.ProjectID, captured.Task.TaskID)
				action := "remove_today"
				if redo {
					p.desktop.ok(p.desktop.call("command.plan", map[string]any{"task": target, "action": action}), nil)
					action = "add_today"
				}
				p.desktop.ok(p.desktop.call("command.plan", map[string]any{"task": target, "action": action}), &planned)
				if redo {
					p.desktop.ok(p.desktop.call("command.undo", map[string]any{"transaction_id": planned.Committed.TransactionID}), nil)
				}
				p.settle()
				if change == "complete" {
					p.phone.ok(p.phone.call("command.transition", map[string]any{"task": target, "state": "done"}), nil)
				} else if _, err := p.phone.service().Delete(app.DeleteRequest{Locator: target.app(), Confirm: true}); err != nil {
					t.Fatal(err)
				}
				if conflicts := p.settle(); len(conflicts) != 0 {
					t.Fatalf("sync conflicts: %+v", conflicts)
				}
				// Planning is byte-identical to the expected image here. The
				// eligibility check must also guard that fast restore path.
				before := p.desktop.revisions()
				if redo {
					if _, err := p.desktop.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); !errors.Is(err, storage.ErrStaleArtifact) {
						t.Fatalf("redo error = %v, want stale artifact", err)
					}
				} else {
					p.desktop.rejects(p.desktop.call("command.undo", map[string]any{"transaction_id": planned.Committed.TransactionID}), CodeStaleRevision)
				}
				if p.desktop.revisions() != before {
					t.Fatal("refused restore changed workspace revisions")
				}
			})
		}
	}
}

func planningWithoutTarget(t *testing.T, h *harness, target locator) *config.PlanningState {
	t.Helper()
	planning, err := config.NewDomainFileStore(h.dataDir).LoadPlanningReadOnly()
	if err != nil {
		t.Fatal(err)
	}
	planning.RemoveDurableTaskLocators([]config.ProjectTaskLocator{{ProjectID: target.ProjectID, TaskID: target.TaskID}})
	return planning
}

func TestPlanningRestoreRefusesPeerChangesToTarget(t *testing.T) {
	for _, redo := range []bool{false, true} {
		for _, change := range []string{"add_next", "remove_today", "add_today"} {
			// These would be no-ops and cannot establish a competing intent.
			if (!redo && change == "add_today") || (redo && change == "remove_today") {
				continue
			}
			name := "undo/"
			if redo {
				name = "redo/"
			}
			t.Run(name+change, func(t *testing.T) {
				p := newPair(t)
				var captured, planned mutationResult
				p.desktop.ok(p.desktop.call("command.capture", map[string]any{
					"project_id": atlasProject, "title": "Plan A", "destination": "backlog",
				}), &captured)
				target := task(captured.Task.ProjectID, captured.Task.TaskID)
				p.desktop.ok(p.desktop.call("command.plan", map[string]any{"task": target, "action": "add_today"}), &planned)
				if redo {
					p.desktop.ok(p.desktop.call("command.undo", map[string]any{"transaction_id": planned.Committed.TransactionID}), nil)
				}
				p.settle()
				p.phone.ok(p.phone.call("command.plan", map[string]any{"task": target, "action": change}), nil)
				if conflicts := p.settle(); len(conflicts) != 0 {
					t.Fatalf("sync conflicts: %+v", conflicts)
				}
				before := p.desktop.revisions()
				if redo {
					if _, err := p.desktop.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); !errors.Is(err, storage.ErrStaleArtifact) {
						t.Fatalf("redo error = %v, want stale artifact", err)
					}
				} else {
					p.desktop.rejects(p.desktop.call("command.undo", map[string]any{"transaction_id": planned.Committed.TransactionID}), CodeStaleRevision)
				}
				if p.desktop.revisions() != before {
					t.Fatal("refused restore changed workspace revisions")
				}
			})
		}
	}
}
