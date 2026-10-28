package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// A Mail link captured through the control socket previews by name, commits as
// a URL attachment, and reads back unchanged after a relaunch.
func TestControlCaptureKeepsAMailLinkAcrossRelaunch(t *testing.T) {
	d, ws, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })
	const link = "message://%3Cabc@example.com%3E"
	const line = "Reply to Ana &" + link

	preview := desktopRequest(t, d, control.OpPreview, line)
	if !preview.OK || !strings.Contains(preview.Preview, "Attachments: "+link) {
		t.Fatalf("preview: %+v", preview)
	}
	if reply := desktopRequest(t, d, control.OpCapture, line); !reply.OK {
		t.Fatalf("capture: %+v", reply)
	}

	app, err := ws.relaunchApp()
	if err != nil {
		t.Fatal(err)
	}
	var found *models.Task
	for _, project := range app.domain.runtime.AvailableProjects() {
		if err := storage.LoadProject(project); err != nil {
			t.Fatal(err)
		}
		for _, task := range project.Tasks {
			if task.Title == "Reply to Ana" {
				found = task
			}
		}
	}
	if found == nil || len(found.Attachments) != 1 {
		t.Fatalf("reloaded task = %+v, want one attachment", found)
	}
	if got := found.Attachments[0]; !got.IsURL || got.Filename != link {
		t.Fatalf("attachment = %+v, want the Mail link as a URL", got)
	}
}
