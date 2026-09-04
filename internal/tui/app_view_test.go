package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/tui/presentation"
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

// TestAppViewQuickCaptureUsesAppShellFooter guards the composer-section divider
// plus the single app-shell footer contract for the unified Task Composer.
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

	content := app.View().Content

	if !strings.Contains(content, "Task Composer") {
		t.Fatal("composed output should keep the Task Composer title/body")
	}

	if got := countFullWidthDividers(content); got != 2 {
		t.Fatalf("expected composer and app-shell separators, got %d\n%s", got, ansi.Strip(content))
	}
}

func TestAppViewTaskComposerBudgetsForWrappedFooter(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.handleWindowSize(tea.WindowSizeMsg{Width: 60, Height: 24})

	handled, _ := app.handleViewMessages(app.captureComposerMessage(project, app.taskWorkspaceProjects()))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}
	composer := app.taskComposer
	composer.Input.SetValue("Task $")
	composer.UpdatePreview()
	composer.RefreshAutocomplete()

	if got := lipgloss.Height(app.renderStatusBar()); got <= 2 {
		t.Fatalf("test requires a wrapped contextual footer, got %d rows", got)
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
