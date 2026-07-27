package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
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
	app.views.Register(ViewCrossProjectOutliner, &testViewWithMetadata{})

	output := app.View()

	if !output.AltScreen {
		t.Fatal("expected app view to keep alt screen enabled")
	}
	if !output.ReportFocus {
		t.Fatal("expected report focus to be preserved from subview")
	}
	if output.MouseMode != tea.MouseModeAllMotion {
		t.Fatalf("expected mouse mode %v, got %v", tea.MouseModeAllMotion, output.MouseMode)
	}
	if output.WindowTitle != "Subview Title" {
		t.Fatalf("expected window title to be preserved, got %q", output.WindowTitle)
	}
	if !strings.Contains(output.Content, "Subview content") {
		t.Fatal("expected composed app content to contain the subview content")
	}
}

// countFullWidthDividers returns the number of rendered lines that are a solid
// run of horizontal-rule glyphs (the app-shell status-bar / footer separator).
// A footer separator is the only place a line is composed entirely of "─"; the
// textarea box border interleaves corner glyphs, so it never matches.
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

// TestAppViewQuickCaptureUsesAppShellFooter guards the single-footer contract
// for the unified Task Composer overlay.
func TestAppViewQuickCaptureUsesAppShellFooter(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.width = 80
	app.height = 24

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.activeProjects()))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}
	if app.currentView() != ViewTaskComposer {
		t.Fatalf("currentView should be QuickCapture, got %v", app.currentView())
	}

	content := app.View().Content

	if !strings.Contains(content, "Task Composer") {
		t.Fatal("composed output should keep the Task Composer title/body")
	}

	if got := countFullWidthDividers(content); got != 1 {
		t.Fatalf("expected exactly one app-shell footer separator, got %d\n%s", got, ansi.Strip(content))
	}
}
