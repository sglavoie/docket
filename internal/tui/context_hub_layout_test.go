package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// contextHubCrowdedApp gives the catalog more contexts than fit in a 60x20 list
// pane so the browse list has to scroll.
func contextHubCrowdedApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)
	projects := make([]*models.Project, 0, 14)
	for i := 0; i < 14; i++ {
		project := scopeTestProject(fmt.Sprintf("p%02d", i), fmt.Sprintf("Project %02d", i), "1")
		project.Slug = fmt.Sprintf("p%02d", i)
		project.Context = fmt.Sprintf("label-%02d", i)
		project.Path = t.TempDir()
		projects = append(projects, project)
	}
	setTestAppProjects(app, projects...)
	app.currentProject = nil
	return app
}

func TestContextHubWindowKeepsClippedRowsDiscoverable(t *testing.T) {
	lines := make([]string, 12)
	for i := range lines {
		lines[i] = fmt.Sprintf("row-%02d", i)
	}

	tests := []struct {
		name        string
		cursorLine  int
		height      int
		wantAbove   bool
		wantBelow   bool
		wantVisible string
	}{
		{name: "fits without hints", cursorLine: 0, height: 12, wantVisible: "row-11"},
		{name: "top of a clipped list", cursorLine: 0, height: 6, wantBelow: true, wantVisible: "row-00"},
		{name: "middle of a clipped list", cursorLine: 6, height: 6, wantAbove: true, wantBelow: true, wantVisible: "row-06"},
		{name: "bottom of a clipped list", cursorLine: 11, height: 6, wantAbove: true, wantVisible: "row-11"},
		{name: "too short for hints", cursorLine: 11, height: 3, wantVisible: "row-11"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			visible, scroll := contextHubWindow(lines, tc.cursorLine, 0, tc.height)
			if len(visible) > tc.height {
				t.Fatalf("window returned %d rows for height %d", len(visible), tc.height)
			}
			joined := strings.Join(visible, "\n")
			if !strings.Contains(joined, tc.wantVisible) {
				t.Errorf("cursor row %q not visible in:\n%s", tc.wantVisible, joined)
			}
			if got := strings.Contains(joined, "more above"); got != tc.wantAbove {
				t.Errorf("above hint = %v, want %v in:\n%s", got, tc.wantAbove, joined)
			}
			if got := strings.Contains(joined, "more below"); got != tc.wantBelow {
				t.Errorf("below hint = %v, want %v in:\n%s", got, tc.wantBelow, joined)
			}
			if scroll < 0 || scroll > len(lines) {
				t.Errorf("scroll = %d, out of range for %d lines", scroll, len(lines))
			}
		})
	}
}

// TestContextHubListScrollsToTheSelectedContext proves the browse list follows
// the cursor past the bottom of a minimum-height pane instead of clipping it
// away, and reports how many rows it hid.
func TestContextHubListScrollsToTheSelectedContext(t *testing.T) {
	app := contextHubCrowdedApp(t)
	hub := openContextHub(t, app)
	hub.SetSize(60, 20)

	if got := contextHubRender(hub); !strings.Contains(got, "label-00") {
		t.Fatalf("first context missing from the initial render:\n%s", got)
	}

	hub.Update(syntheticKey('G'))
	render := contextHubRender(hub)
	if !strings.Contains(render, "label-13") {
		t.Errorf("last context not scrolled into view:\n%s", render)
	}
	if !strings.Contains(render, "more above") {
		t.Errorf("scrolled list did not report the rows above:\n%s", render)
	}
	if strings.Contains(render, "more below") {
		t.Errorf("list at the bottom still claims hidden rows below:\n%s", render)
	}
}

// TestContextHubFooterHelpStaysOnOneLine guards the footer against the shell's
// status-bar swap, which replaces a full-height view's last two lines: a footer
// that wrapped onto a third line would leave an orphaned separator behind.
func TestContextHubFooterHelpStaysOnOneLine(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)

	modes := []struct {
		name  string
		setup func()
	}{
		{name: "browse", setup: func() {}},
		{name: "search active", setup: func() {
			hub.Update(syntheticKey('/'))
			typeIntoContextHub(hub, "wo")
		}},
		{name: "search committed", setup: func() {
			hub.Update(syntheticSpecialKey(tea.KeyEnter))
		}},
		{name: "editor name field", setup: func() {
			hub.Update(syntheticSpecialKey(tea.KeyEscape))
			hub.Update(syntheticKey('n'))
		}},
		{name: "editor project list", setup: func() {
			hub.Update(syntheticSpecialKey(tea.KeyTab))
		}},
	}
	for _, mode := range modes {
		mode.setup()
		for _, width := range []int{60, 70, 80, 100, 120} {
			hub.SetSize(width, 20)
			help := hub.footerHelp(width)
			footer := renderStatusBarFooter(
				HelpStyle.Render(help),
				MutedStyle.Render(contextHubFooterLabel),
				width,
				hub.AccentColor(),
			)
			// One newline: the separator row plus a single help row.
			if lines := strings.Count(footer, "\n"); lines != 1 {
				t.Errorf("%s at width %d: footer occupies %d lines, want 2\n%s",
					mode.name, width, lines+1, ansi.Strip(footer))
			}
			if got := lipgloss.Width(help) + lipgloss.Width(contextHubFooterLabel) + 1; got > width {
				t.Errorf("%s at width %d: help+label needs %d columns", mode.name, width, got)
			}
			if strings.TrimSpace(ansi.Strip(help)) == "" {
				t.Errorf("%s at width %d: help collapsed to nothing", mode.name, width)
			}
		}
	}
}

// TestContextHubPreviewCountsWrapInsteadOfClipping locks the custom-context
// preview line, which is wider than the preview pane at 100 columns and used to
// lose its task count to an ellipsis.
func TestContextHubPreviewCountsWrapInsteadOfClipping(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)
	hub.SetLoadedProjects(app.allProjects())
	hub.SetSize(100, 30)
	focusContextHubScope(t, hub, CustomContextScope("focus"))

	render := ansi.Strip(hub.View().Content)
	if strings.Contains(render, "…") {
		t.Errorf("preview clipped instead of wrapping:\n%s", render)
	}
	for _, want := range []string{"active project(s)", "0 archived", "0 missing", "task(s)"} {
		if !strings.Contains(render, want) {
			t.Errorf("preview counts missing %q:\n%s", want, render)
		}
	}
}
