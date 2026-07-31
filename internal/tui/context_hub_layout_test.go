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
	projects := make([]*models.Project, 0, 24)
	for i := 0; i < 24; i++ {
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
	if !strings.Contains(render, "label-23") {
		t.Errorf("last context not scrolled into view:\n%s", render)
	}
	if !strings.Contains(render, "↑") || !strings.Contains(render, "more") {
		t.Errorf("scrolled list did not report the rows above:\n%s", render)
	}
	if strings.Contains(render, "↓") {
		t.Errorf("list at the bottom still claims hidden rows below:\n%s", render)
	}
}

// TestContextHubListFollowsEachJToTheLastProject guards the renderer/model
// height boundary: continuation rows must be included in the list's viewport
// budget or the shell clips the selected last row.
func TestContextHubListFollowsEachJToTheLastProject(t *testing.T) {
	app := contextHubCrowdedApp(t)
	hub := openContextHub(t, app)
	hub.SetSize(60, 20)

	for index := 0; index < len(hub.surface.Matches()); index++ {
		if index > 0 {
			hub.Update(syntheticKey('j'))
		}
		rendered := ansi.Strip(hub.View().Content)
		if got := lipgloss.Height(rendered); got > 20 {
			t.Fatalf("cursor %d rendered %d rows, want <= 20:\n%s", index, got, rendered)
		}
		current, ok := hub.surface.CurrentItem()
		if !ok {
			t.Fatalf("cursor %d has no current item", index)
		}
		if want := "› " + current.PrimaryText; !strings.Contains(rendered, want) {
			t.Fatalf("cursor row %q is outside the Hub viewport:\n%s", want, rendered)
		}
	}

	current, ok := hub.surface.CurrentItem()
	if !ok || current.PrimaryText != "label-23" {
		t.Fatalf("last item = (%q, %v), want label-23", current.PrimaryText, ok)
	}
}

func TestContextHubCtrlDUHalfPageNavigationFollowsTheCursor(t *testing.T) {
	for _, mode := range []struct {
		name         string
		activate     func(*ContextHubModel)
		searchActive bool
	}{
		{name: "browse"},
		{
			name:         "active search",
			activate:     func(hub *ContextHubModel) { hub.Update(syntheticKey('/')) },
			searchActive: true,
		},
	} {
		t.Run(mode.name, func(t *testing.T) {
			app := contextHubCrowdedApp(t)
			hub := openContextHub(t, app)
			hub.SetSize(60, 20)
			_ = hub.View()
			if mode.activate != nil {
				mode.activate(hub)
			}

			start := hub.surface.Cursor()
			downStep := HalfPage(hub.surface.Viewport().Height)
			hub.Update(syntheticCtrlKey('d'))
			if got, want := hub.surface.Cursor(), start+downStep; got != want {
				t.Fatalf("Ctrl+D cursor = %d, want %d", got, want)
			}
			rendered := ansi.Strip(hub.View().Content)
			current, ok := hub.surface.CurrentItem()
			if !ok || !strings.Contains(rendered, "› "+current.PrimaryText) {
				t.Fatalf("Ctrl+D cursor row is outside the Hub viewport:\n%s", rendered)
			}

			beforeUp := hub.surface.Cursor()
			upStep := HalfPage(hub.surface.Viewport().Height)
			hub.Update(syntheticCtrlKey('u'))
			if got, want := hub.surface.Cursor(), max(beforeUp-upStep, 0); got != want {
				t.Fatalf("Ctrl+U cursor = %d, want %d", got, want)
			}
			rendered = ansi.Strip(hub.View().Content)
			current, ok = hub.surface.CurrentItem()
			if !ok || !strings.Contains(rendered, "› "+current.PrimaryText) {
				t.Fatalf("Ctrl+U cursor row is outside the Hub viewport:\n%s", rendered)
			}
			if hub.HasActiveSearch() != mode.searchActive {
				t.Fatalf("search active = %v, want %v", hub.HasActiveSearch(), mode.searchActive)
			}
		})
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
	hub.SetLoadedProjects(app.projectsInRuntimeOrder(app.runtime.AllProjects()))
	hub.SetSize(100, 30)
	focusContextHubScope(t, hub, CustomContextScope("focus"))

	render := contextHubRender(hub)
	if strings.Contains(render, "…") {
		t.Errorf("preview clipped instead of wrapping:\n%s", render)
	}
	preview, ok := hub.surface.CurrentPreview()
	if !ok {
		t.Fatal("selected custom context has no preview")
	}
	normalizedPreview := strings.Join(strings.Fields(preview.Body), " ")
	for _, want := range []string{"active project(s)", "0 archived", "0 missing", "task(s)"} {
		if !strings.Contains(normalizedPreview, want) {
			t.Errorf("preview counts missing %q:\n%s", want, normalizedPreview)
		}
	}
}
