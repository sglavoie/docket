package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/theme"
)

type testViewWithMetadata struct{}

func (v *testViewWithMetadata) Init() tea.Cmd { return nil }

func (v *testViewWithMetadata) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v, nil
}

func (v *testViewWithMetadata) View() tea.View {
	view := tea.NewView("Subview content")
	view.ReportFocus = true
	view.MouseMode = tea.MouseModeAllMotion
	view.WindowTitle = "Subview Title"
	return view
}

func (v *testViewWithMetadata) SetSize(width, height int) {}

func TestAppViewPreservesSubviewViewMetadata(t *testing.T) {
	app := newTestAppModel()
	app.viewNav.views.Register(presentation.ViewCrossProjectOutliner, &testViewWithMetadata{})

	output := app.View()

	if !output.AltScreen {
		t.Fatal("expected app view to keep alt screen enabled")
	}
	if !output.ReportFocus {
		t.Fatal("expected report focus to be preserved from subview")
	}
	// Mouse reporting is the shell's, not a subview's: the pointer seam drops
	// bare motion, so a view asking for it is overruled rather than obeyed.
	if output.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("expected the shell's mouse mode %v, got %v", tea.MouseModeCellMotion, output.MouseMode)
	}
	if output.WindowTitle != "Subview Title" {
		t.Fatalf("expected window title to be preserved, got %q", output.WindowTitle)
	}
	if !strings.Contains(output.Content, "Subview content") {
		t.Fatal("expected composed app content to contain the subview content")
	}
}

// TestAppViewAnnouncesTheThemeBackground: the terminal's default background is
// the theme's, so its padding and a host window match the painted cells, and
// a --no-color run or the closing frame announces none.
func TestAppViewAnnouncesTheThemeBackground(t *testing.T) {
	saved := theme.ThemeBgColor
	t.Cleanup(func() { theme.ThemeBgColor = saved })
	theme.ThemeBgColor = "#1a1b26"

	app := newTestAppModel()
	if got := app.View().BackgroundColor; got != theme.ColorBackground {
		t.Fatalf("background = %v, want the theme's %v", got, theme.ColorBackground)
	}

	theme.ThemeBgColor = ""
	if got := app.View().BackgroundColor; got != nil {
		t.Fatalf("unthemed background = %v, want none", got)
	}
	theme.ThemeBgColor = "#1a1b26"

	app.lifecycle.colorDisabled = true
	if got := app.View().BackgroundColor; got != nil {
		t.Fatalf("--no-color background = %v, want none", got)
	}

	app.lifecycle.colorDisabled = false
	app.lifecycle.quitting = true
	if got := app.View().BackgroundColor; got != nil {
		t.Fatalf("closing background = %v, want none", got)
	}
}

// countFullWidthDividers returns the number of rendered lines that are a solid
// run of horizontal-rule glyphs. The composer uses one to separate input from
// derived information, and the app shell uses another above its status footer.
func countFullWidthDividers(content string) int {
	count := 0
	for _, line := range strings.Split(content, "\n") {
		plain := strings.TrimSpace(ansi.Strip(line))
		if plain == "" {
			continue
		}
		allRule := true
		runes := 0
		for _, r := range plain {
			runes++
			if r != '─' {
				allRule = false
				break
			}
		}
		if allRule && runes >= 40 {
			count++
		}
	}
	return count
}

// TestAppViewQuickCaptureUsesAppShellFooter guards the bounded capture panel's
// composition: the panel names itself on its own labeled divider instead of
// spending a row on a fullscreen title, and the app shell still contributes the
// one plain rule on screen.
func TestAppViewQuickCaptureUsesAppShellFooter(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.viewNav.width = 80
	app.viewNav.height = 24

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.taskWorkspaceProjects()))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}
	if app.currentView() != presentation.ViewTaskComposer {
		t.Fatalf("currentView should be QuickCapture, got %v", app.currentView())
	}

	content := ansi.Strip(app.View().Content)

	if !strings.Contains(content, "── Capture ─") {
		t.Fatalf("composed output should keep the panel's labeled divider:\n%s", content)
	}
	if !strings.Contains(content, "Project: Test Project · Status: Inbox") {
		t.Fatalf("composed output should keep the panel's route row:\n%s", content)
	}

	if got := countFullWidthDividers(content); got != 1 {
		t.Fatalf("expected the app-shell separator alone, got %d\n%s", got, content)
	}
}

// TestAppViewHostedCapturePanelKeepsItsHostVisible is the composition the panel
// exists for: capture is raised over the outline rather than instead of it, the
// panel is pinned to the bottom of the content area, and closing it hands the
// host back every row it lent.
func TestAppViewHostedCapturePanelKeepsItsHostVisible(t *testing.T) {
	model := typedTestApp(t)
	model.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 30})
	_, hostHeight := model.viewNav.viewSize()

	handled, _ := model.handleViewMessages(model.captureComposerMessage(model.currentProject, model.taskWorkspaceProjects()))
	if !handled || model.currentView() != presentation.ViewTaskComposer {
		t.Fatalf("composer did not open: handled=%v view=%v", handled, model.currentView())
	}
	model.syncHostedPanelHost()

	frame, ok := model.hostedPanelFrame()
	if !ok || frame.host != presentation.ViewCrossProjectOutliner {
		t.Fatalf("capture panel host = %v (ok=%v), want the outline it was raised from", frame.host, ok)
	}
	if got := model.crossOutliner.Height(); got != frame.layout.HostRows {
		t.Fatalf("host height = %d, want the %d rows the panel left it", got, frame.layout.HostRows)
	}

	lines := strings.Split(ansi.Strip(model.View().Content), "\n")
	divider := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "── Capture ─") {
			divider = i
			break
		}
	}
	if divider <= 0 {
		t.Fatalf("panel divider at line %d, want the outline drawn above it:\n%s", divider, strings.Join(lines, "\n"))
	}
	// The host's own title row, byte for byte: a fixture whose pulse is all zeros
	// reports no pulse at all, so the presence of `workspace:` answers a different
	// question than whose row this is.
	title := ansi.Strip(model.crossOutliner.BuildOutlinerTitleLine(model.crossOutliner.Width()))
	if strings.TrimRight(lines[0], " ") != strings.TrimRight(title, " ") {
		t.Fatalf("host title is not the first row under a hosted panel:\n%s", strings.Join(lines, "\n"))
	}

	// Closing the panel is the release: one view is owed its budget back.
	model.taskComposer = nil
	model.viewNav.navigation.Overlays.CloseTop()
	model.syncHostedPanelHost()
	if got := model.crossOutliner.Height(); got != hostHeight {
		t.Fatalf("host height after close = %d, want its whole budget %d", got, hostHeight)
	}
}

func TestAppViewTaskComposerBudgetsForATallerFooter(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.handleWindowSize(tea.WindowSizeMsg{Width: 60, Height: 24})

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.taskWorkspaceProjects()))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}
	composer := app.taskComposer
	// Three draft rows under ten destinations is more content than the panel's
	// ceiling allows, which is the state the budget is about: the pane is windowed
	// rather than the frame overrun.
	composer.Input.SetValue("First task\nSecond task\nThird task $")
	composer.UpdatePreview()
	composer.RefreshAutocomplete()
	// The feedback row is the excess: the capture controls alone fit the two rows
	// the shell already subtracts, so a footer taller than its baseline needs a
	// message beside them. It is the same row a saved capture publishes, which is
	// exactly when the next one is being typed under an open suggestion list.
	app.feedback.message = "task saved"

	if got := lipgloss.Height(app.renderStatusBar()); got <= 2 {
		t.Fatalf("test requires a footer taller than its two-row baseline, got %d rows", got)
	}
	content := ansi.Strip(app.View().Content)
	if strings.Contains(content, "resize terminal to view all") {
		t.Fatalf("shell clipped the height-budgeted autocomplete pane:\n%s", content)
	}
	if !strings.Contains(content, "more") {
		t.Fatalf("constrained autocomplete should expose its own overflow hint:\n%s", content)
	}
	if got := lipgloss.Height(content); got != 24 {
		t.Fatalf("composed frame height = %d, want 24", got)
	}
}

// TestAppViewTitlesTheWindowWithThePulse: with no view naming a title, the
// window title is the workspace pulse Docket.app reads its Dock badge from, and the
// closing frame clears it.
func TestAppViewTitlesTheWindowWithThePulse(t *testing.T) {
	app := newTestAppModel()
	app.workspacePulse = pulse.WorkspacePulse{Today: 2, Overdue: 3}
	if got := app.View().WindowTitle; got != "Docket · today 2 · overdue 3" {
		t.Fatalf("window title = %q, want the pulse", got)
	}
	if !app.View().ReportFocus {
		t.Fatal("the app view does not ask for focus reports")
	}
	app.lifecycle.quitting = true
	if got := app.View().WindowTitle; got != "" {
		t.Fatalf("closing frame title = %q, want none", got)
	}
}
