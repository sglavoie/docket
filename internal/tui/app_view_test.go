package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
	app.views.Register(ViewDashboard, &testViewWithMetadata{})

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
