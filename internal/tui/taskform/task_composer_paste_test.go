package taskform

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
)

func TestCaptureRejectsOversizedPasteWithoutChangingDraft(t *testing.T) {
	project := &models.Project{ID: "web", Slug: "web", Name: "Web"}
	m := NewTaskComposerModel(TaskComposerPreset{Project: project}, nil)
	m.SetSize(chrome.MinTerminalWidth, chrome.MinTerminalHeight)
	m.Update(tea.PasteMsg{Content: "First task\nSecond task"})
	m.Input.MoveToBegin()
	m.Input.SetCursorColumn(3)
	before, row, col := m.Input.Value(), m.Input.Line(), m.Input.Column()
	m.Update(tea.PasteMsg{Content: strings.Repeat("Another task\n", 400)})
	if m.Input.Value() != before || m.Input.Line() != row || m.Input.Column() != col {
		t.Fatal("refused paste changed the draft or caret")
	}
	for _, size := range [][2]int{{60, 18}, {100, 35}, {60, 18}} {
		m.SetSize(size[0], size[1])
		view := m.View().Content
		if !strings.Contains(ansi.Strip(view), "nothing pasted") || lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
			t.Fatalf("paste refusal missing or outside %v:\n%s", size, view)
		}
	}
	m.Update(tea.PasteMsg{Content: " new"})
	if m.pasteRefusal != "" || m.Input.Value() != "Fir newst task\nSecond task" {
		t.Fatalf("smaller retry lost caret or kept refusal: %q / %q", m.Input.Value(), m.pasteRefusal)
	}
}

func TestCapturePasteCapacityUsesTextareaNormalization(t *testing.T) {
	for _, tc := range []struct {
		name, existing, paste, want string
		refused                     bool
	}{
		{"exact limit", strings.Repeat("a", 3999), "é", strings.Repeat("a", 3999) + "é", false},
		{"over limit", strings.Repeat("a", 3999), "éx", strings.Repeat("a", 3999), true},
		{"wide runes", "", strings.Repeat("界", 1500), strings.Repeat("界", 1500), false},
		{"expanded tab fits", strings.Repeat("a", 3996), "\t", strings.Repeat("a", 3996) + "    ", false},
		{"expanded tab refuses", strings.Repeat("a", 3997), "\t", strings.Repeat("a", 3997), true},
		{"controls removed", strings.Repeat("a", 3999), "\x00é\x07", strings.Repeat("a", 3999) + "é", false},
		{"full", strings.Repeat("a", 4000), "x", strings.Repeat("a", 4000), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewTaskComposerModel(TaskComposerPreset{Project: &models.Project{ID: "web", Slug: "web"}}, nil)
			m.Input.SetValue(tc.existing)
			m.Input.MoveToEnd()
			m.Update(tea.PasteMsg{Content: tc.paste})
			if m.Input.Value() != tc.want || (m.pasteRefusal != "") != tc.refused {
				t.Fatalf("paste result length=%d, refusal=%q", len(m.Input.Value()), m.pasteRefusal)
			}
		})
	}
}

func TestCaptureCapacityAppearsOnlyNearLimit(t *testing.T) {
	m := NewTaskComposerModel(TaskComposerPreset{Project: &models.Project{ID: "web", Slug: "web"}}, nil)
	if strings.Contains(m.composerPanelErrorBlock(60), "remaining") {
		t.Fatal("blank capture shows capacity")
	}
	m.Update(tea.PasteMsg{Content: strings.Repeat("a", 3999)})
	if !strings.Contains(ansi.Strip(m.composerPanelErrorBlock(60)), "1 character remaining") {
		t.Fatal("near-full capture does not show remaining capacity")
	}
}
