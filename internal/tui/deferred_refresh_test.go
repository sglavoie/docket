package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/storage"
)

func TestDeferredRefreshAdoptsOnInteractionExit(t *testing.T) {
	for _, key := range []string{"a", "@", "v", "/", "?", "e"} {
		t.Run(key, func(t *testing.T) {
			d, workspace, err := NewFixtureDriver(FixtureRelease, 100, 35)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = workspace.Cleanup() })
			m := d.App()
			out := m.crossOutliner
			uxClarityPress(t, d, uxClarityKeys("g", "b", "G")...)
			for i := len(out.Rows) - 1; i >= 0; i-- {
				row := out.Rows[i]
				if row.Task != nil && !row.IsHeaderRow() {
					out.Cursor = i
					break
				}
			}
			row := out.Rows[out.Cursor]
			id, projectID := row.Task.ID, row.ProjectID
			m.setInfoStatus("Saved: keep this feedback")
			m.syncShellFrameMetrics()
			_ = m.View()
			offset := out.ViewScrollOffset
			if offset == 0 {
				t.Fatal("fixture did not exercise a scrolled viewport")
			}
			if err := d.runAction(UITestAction{Type: UITestActionKey, Key: key}); err != nil {
				t.Fatal(err)
			}
			if m.safeToAdoptExternalChange() {
				t.Fatal("interaction did not protect the projection")
			}
			d.dispatch(tea.BlurMsg{})
			d.drain()
			project := *m.domain.projects[projectID]
			if _, err := storage.LoadProjectWithCachedDocument(&project, nil); err != nil {
				t.Fatal(err)
			}
			for _, task := range project.Tasks {
				if task.ID == id {
					task.Title = "Changed on another device"
				}
			}
			if err := storage.SaveProject(&project); err != nil {
				t.Fatal(err)
			}
			d.dispatch(tea.FocusMsg{})
			d.drain()
			if !m.externalReload.owed {
				t.Fatal("refresh did not wait for the interaction")
			}
			m.setInfoStatus("Saved: keep this feedback")
			if err := d.runAction(UITestAction{Type: UITestActionKey, Key: "esc"}); err != nil {
				t.Fatal(err)
			}
			if m.externalReload.owed {
				t.Fatal("refresh still waits for a timer after closing the interaction")
			}
			row = out.Rows[out.Cursor]
			if row.Task == nil || row.Task.ID != id || row.Task.Title != "Changed on another device" {
				t.Fatalf("cursor did not retain the refreshed task: %+v", row)
			}
			_ = m.View()
			if out.ViewScrollOffset != offset {
				t.Fatalf("scroll moved from %d to %d", offset, out.ViewScrollOffset)
			}
			if m.feedback.message != "Saved: keep this feedback" {
				t.Fatalf("refresh replaced feedback: %q", m.feedback.message)
			}
		})
	}
}
