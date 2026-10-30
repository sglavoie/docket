package bridge

import (
	"fmt"
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/storage"
)

func TestProjectCommentUndoRedoPreservesPeerWorkAcrossDevices(t *testing.T) {
	for _, device := range []string{"desktop", "phone"} {
		for _, action := range []string{"add", "edit", "delete"} {
			t.Run(device+"/"+action, func(t *testing.T) {
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
				var before, after projectDetail
				origin.ok(origin.call("query.project", map[string]any{"project_id": atlasProject}), &before)
				payload := map[string]any{"project_id": atlasProject, "action": action, "text": "Local comment"}
				if action != "add" {
					payload["comment_id"], payload["original"] = before.Comments[0].ID, before.Comments[0].Text
				}
				var result commentResult
				origin.ok(origin.call("command.comment", payload), &result)
				origin.ok(origin.call("query.project", map[string]any{"project_id": atlasProject}), &after)
				settle()
				other := task(atlasProject, screenshotsTask)
				peer.ok(peer.call("command.transition", map[string]any{"task": other, "state": "done"}), nil)
				for step, redo := range []bool{false, true, false, true} {
					peer.ok(peer.call("command.edit", map[string]any{"task": other, "title": fmt.Sprintf("Peer task %d", step)}), nil)
					// Both an addition and a change to an untouched original comment
					// must survive every restore, including after restart.
					peer.ok(peer.call("command.comment", map[string]any{
						"project_id": atlasProject, "action": "add", "text": fmt.Sprintf("Peer addition %d", step),
					}), nil)
					var current projectDetail
					peer.ok(peer.call("query.project", map[string]any{"project_id": atlasProject}), &current)
					for _, comment := range current.Comments {
						if comment.ID == before.Comments[1].ID {
							peer.ok(peer.call("command.comment", map[string]any{
								"project_id": atlasProject, "action": "edit", "comment_id": comment.ID,
								"original": comment.Text, "text": fmt.Sprintf("Peer edit %d", step),
							}), nil)
						}
					}
					settle()
					origin.ok(origin.call("query.project", map[string]any{"project_id": atlasProject}), &current)
					wanted := before.Comments
					if redo {
						wanted = after.Comments
					}
					// The original transaction touches only its one comment.
					targetID := result.CommentID
					if action != "add" {
						targetID = before.Comments[0].ID
					}
					wantComments := slices.DeleteFunc(slices.Clone(current.Comments), func(c commentView) bool { return c.ID == targetID })
					for _, comment := range wanted {
						if comment.ID == targetID {
							wantComments = append(wantComments, comment)
						}
					}
					origin.ok(origin.call("workspace.close", nil), nil)
					origin.openAs(device + "-device")
					if redo {
						if _, err := origin.bridge.workspace.runtime.Ledger().Redo(storage.GlobalScope()); err != nil {
							t.Fatal(err)
						}
					} else {
						origin.ok(origin.call("command.undo", map[string]any{"transaction_id": result.Committed.TransactionID}), nil)
					}
					settle()
					for _, h := range []*harness{origin, peer} {
						var got projectDetail
						h.ok(h.call("query.project", map[string]any{"project_id": atlasProject}), &got)
						if len(got.Comments) != len(wantComments) {
							t.Fatalf("comments = %+v, want %+v", got.Comments, wantComments)
						}
						for _, want := range wantComments {
							if !slices.ContainsFunc(got.Comments, func(c commentView) bool {
								return c.ID == want.ID && c.Text == want.Text && c.CreatedAt.Equal(want.CreatedAt)
							}) {
								t.Fatalf("lost comment %+v in %+v", want, got.Comments)
							}
						}
						if got := h.detail(other.ProjectID, other.TaskID).Task; got.State != "done" || got.Title != fmt.Sprintf("Peer task %d", step) {
							t.Fatalf("restore lost peer task change: %+v", got)
						}
					}
				}
			})
		}
	}
}
