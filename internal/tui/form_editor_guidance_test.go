package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// The project creator and editor moved to internal/tui/projecthub with the
// forms themselves, so their guidance is asserted there; the text editor is
// still the shell's, and its budget stays here.
func TestTextEditorGuidanceRequiresCommitAndCancelByState(t *testing.T) {
	textEditor := chrome.NewTextEditorModel("description", "", "")
	assertRequiredGuidance(t, "text editor", textEditor.GuidanceSpec(), true, "Ctrl+S save", "Esc cancel")
	textEditor.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	spec, ok := textEditor.ContextualGuidance()
	assertRequiredGuidance(t, "edited text editor", spec, ok, "Ctrl+S save", "Esc discard")
	textEditor.ExternalEdit = true
	if got := textEditor.GuidanceSpec().Supplemental; got != "External editor open" {
		t.Fatalf("external-editor supplemental = %q", got)
	}
}

func TestTextEditorGuidanceFitsNarrowBudgets(t *testing.T) {
	spec := chrome.NewTextEditorModel("description", "", "").GuidanceSpec()
	if spec.Rows != 2 {
		t.Errorf("text editor row budget = %d, want 2", spec.Rows)
	}
	assertGuidanceFits(t, spec, 24)
}

func TestFormEditorMinimumLayoutsBudgetRenderedGuidance(t *testing.T) {
	// The text editor derives its floor the same way the task form does, two
	// rows below the terminal minimum, so the two budgets stay one number even
	// though the form's own coverage now lives beside it in internal/tui/taskform.
	const minHeight = chrome.MinTerminalHeight - 2

	textEditor := chrome.NewTextEditorModel("description", "Ship the release", "draft")
	textEditor.SetSize(chrome.MinTerminalWidth, minHeight)
	textView := textEditor.View().Content
	assertFitsWithin(t, textView, chrome.MinTerminalWidth, minHeight)
	// The editor is shell-guided: its keys publish to the shell footer, so its
	// own content draws none of them.
	if plain := ansi.Strip(textView); strings.Contains(plain, "Ctrl+S save") {
		t.Fatalf("text editor drew its own guidance:\n%s", plain)
	}
	spec, ok := textEditor.ContextualGuidance()
	assertRequiredGuidance(t, "text editor", spec, ok, "Ctrl+S save", "Esc cancel")
	assertGuidanceFits(t, spec, chrome.MinTerminalWidth)
}

func TestTextEditorHeaderNamesTaskAndMatchesBreadcrumbLabel(t *testing.T) {
	for _, tt := range []struct {
		field, want string
	}{
		{field: chrome.TextEditorFieldDescription, want: "Edit Description"},
		{field: chrome.TextEditorFieldFormDescription, want: "Edit Description"},
		{field: chrome.TextEditorFieldTitle, want: "Edit Title"},
	} {
		t.Run(tt.field, func(t *testing.T) {
			m := chrome.NewTextEditorModel(tt.field, "Ship the release", "body")
			m.SetSize(80, 20)
			header := strings.SplitN(ansi.Strip(m.View().Content), "\n", 2)[0]
			if !strings.Contains(header, tt.want) || !strings.Contains(header, "Ship the release") {
				t.Fatalf("header = %q, want %q naming the task", header, tt.want)
			}
			if got := m.StatusBarLabel(); got != tt.want {
				t.Fatalf("StatusBarLabel() = %q, want the header's %q", got, tt.want)
			}
		})
	}
}

func TestTextEditorReservesShellFooterExcess(t *testing.T) {
	m := chrome.NewTextEditorModel(chrome.TextEditorFieldDescription, "", "body")
	m.SetSize(60, 18)
	base := lipgloss.Height(m.View().Content)
	m.SetShellFrameMetrics(chrome.ShellFrameMetrics{ControlRows: 2})
	if got := lipgloss.Height(m.View().Content); got != base-1 {
		t.Fatalf("height under a two-row footer = %d, want %d", got, base-1)
	}
}

func TestQuickEditGuidanceOwnsGrammarAndLocalBarDoesNotDuplicateIt(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Task", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}
	out := outliner.NewOutlinerModel(project, nil, nil)
	out.QuickEdit.Enter(task, false)

	spec, ok := out.ContextualGuidance()
	assertRequiredGuidance(t, "quick edit", spec, ok, "Enter save", "Esc cancel")
	got := guidanceEntryText(spec)
	for _, want := range []string{"!1-3/!0 priority", "#tag add tag", "@date/@0 due", "$destination destination", "*pin/*unpin pin", "%project move", ">parent/>0 parent", "<child adopt"} {
		if !strings.Contains(got, want) {
			t.Errorf("quick-edit guidance = %q, want %q", got, want)
		}
	}
}

// assertFitsWithin fails if any rendered line exceeds width or the line count
// exceeds height.
func assertFitsWithin(t *testing.T, rendered string, width, height int) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	if len(lines) > height {
		t.Fatalf("render has %d lines, exceeds height %d:\n\n%s",
			len(lines), height, ansi.Strip(rendered))
	}
	for _, line := range lines {
		if w := lipgloss.Width(ansi.Strip(line)); w > width {
			t.Fatalf("line exceeds width %d: %q (width=%d)\n\n%s",
				width, ansi.Strip(line), w, ansi.Strip(rendered))
		}
	}
}
