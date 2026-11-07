package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/projecthub"
)

func TestBacktickCyclesPinnedProjectsInAll(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureMultiProject, 100, 35)
	if err != nil {
		t.Fatalf("fixture %s: %v", FixtureMultiProject, err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	out := driver.App().crossOutliner
	if out == nil {
		t.Fatal("the fixture app has no Outliner")
	}

	press := func() {
		t.Helper()
		if err := driver.SendKey("`"); err != nil {
			t.Fatalf("send backtick: %v", err)
		}
		driver.drain()
	}

	press()
	if out.Scope.ProjectID != "" {
		t.Fatalf("no pinned project, yet the press scoped to %q", out.Scope.ProjectID)
	}
	if status := driver.Render(); !strings.Contains(status, "No pinned projects") {
		t.Fatalf("an empty cycle must say so; screen:\n%s", status)
	}

	runtime := driver.App().domain.runtime
	var pinnedIDs []string
	for _, project := range runtime.TaskWorkspaceProjects() {
		if project.Name == "Client Beta" || project.Name == "Reading List" {
			driver.dispatch(projecthub.ProjectPinRequestMsg{Project: project, Pinned: true})
			driver.drain()
			pinnedIDs = append(pinnedIDs, project.ID)
		}
	}
	if len(pinnedIDs) != 2 {
		t.Fatalf("pinned %d fixture projects, want 2", len(pinnedIDs))
	}

	// Leave the launch mode first, so the press is seen to force All and Esc is
	// seen to bring the origin mode back.
	out.SelectOutlinerMode(chrome.ModeBacklog)

	for _, want := range []string{pinnedIDs[0], pinnedIDs[1], pinnedIDs[0]} {
		press()
		if out.Scope.ProjectID != want {
			t.Fatalf("scope = %q, want %q", out.Scope.ProjectID, want)
		}
		if mode := out.ActiveOutlinerModeEntry().ID; mode != chrome.ModeAll {
			t.Fatalf("mode = %q, want All", mode)
		}
		for _, project := range out.ScopeProjects() {
			if project.ID != want {
				t.Fatalf("scoped view draws project %q", project.ID)
			}
		}
	}

	if err := driver.SendKey("esc"); err != nil {
		t.Fatal(err)
	}
	driver.drain()
	if out.Scope.ProjectID != "" {
		t.Fatalf("one Esc must return to the base scope, still scoped to %q", out.Scope.ProjectID)
	}
	if mode := out.ActiveOutlinerModeEntry().ID; mode != chrome.ModeBacklog {
		t.Fatalf("Esc restored mode %q, want the origin Backlog", mode)
	}
}
