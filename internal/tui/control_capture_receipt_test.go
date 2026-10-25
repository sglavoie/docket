package tui

import (
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/control"
)

func TestControlCaptureOnceReconcilesAfterRelaunchAndUndo(t *testing.T) {
	d, ws, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })
	request := control.Request{Op: control.OpCaptureOnce, CaptureID: uuid.NewString(), Text: "Safe retry $today\nSecond safe task", Workspace: d.App().domain.runtime.Catalog().WorkspaceID}
	send := func() control.Reply {
		replies := make(chan control.Reply, 1)
		d.dispatch(controlRequestMsg{request: request, reply: replies})
		d.drain()
		select {
		case result := <-replies:
			return result
		default:
			t.Fatal("missing capture reply")
			return control.Reply{}
		}
	}
	first := send()
	if !first.OK || first.Transaction == "" {
		t.Fatalf("first: %+v", first)
	}
	app, err := ws.relaunchApp()
	if err != nil {
		t.Fatal(err)
	}
	d = NewUITestDriver(app, 100, 35)
	if again := send(); again != first {
		t.Fatalf("retry after restart: %+v want %+v", again, first)
	}
	if undo := desktopRequest(t, d, control.OpUndo, first.Transaction); !undo.OK {
		t.Fatalf("undo: %+v", undo)
	}
	if again := send(); again != first {
		t.Fatalf("retry after undo: %+v", again)
	}
	request.Text = "Different draft"
	if result := send(); result.OK {
		t.Fatal("accepted changed text with old submission identity")
	}
	request.Workspace = uuid.NewString()
	if result := send(); result.OK {
		t.Fatal("accepted another workspace's submission")
	}
}
