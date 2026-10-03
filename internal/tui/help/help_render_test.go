package help

import (
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestHelpRenderKeepsSetStateChordOnOneLine(t *testing.T) {
	strategy := operation.SimplifiedInputStrategy()
	key := operation.DisplayKeysForOperation(strategy, operation.OperationTaskSetState)
	if !strings.HasPrefix(key, "w") {
		t.Fatalf("Set State key = %q, want the w chord prefix", key)
	}
	for _, binding := range strategy.Bindings {
		if binding.OperationID != operation.OperationTaskSetState || len(binding.Sequence) != 2 {
			continue
		}
		if letter := binding.Sequence[1]; !strings.Contains(key, letter) {
			t.Fatalf("Set State key %q omits the w%s terminal", key, letter)
		}
	}

	items := []operation.HelpItem{
		{Key: key, Description: "Set State", Category: "Status"},
		{Key: "!0-3", Description: "Set Priority", Category: "Priority"},
	}

	for _, width := range []int{46, 60, 80, 160} {
		m := NewHelpViewModel(items, "Outliner")
		m.SetSize(width, 40)
		lines := strings.Split(ansi.Strip(m.renderContent()), "\n")

		keyLines := make([]string, 0, 1)
		// A wrapped chord splits at one of the slashes between its terminals, so
		// counting the lines carrying a slash is what proves it stayed whole: the
		// only other item here spells itself !0-3, which has none. The retired
		// spelling could count its own unique `$`; `w` is an ordinary letter that
		// also appears in the prose beside the column.
		fragmentLines := 0
		for _, line := range lines {
			if strings.Contains(line, key) {
				keyLines = append(keyLines, line)
			}
			if strings.Contains(line, "/") {
				fragmentLines++
			}
		}
		if len(keyLines) != 1 {
			t.Fatalf("width %d: %d lines carry the whole %q chord, want 1:\n%s",
				width, len(keyLines), key, strings.Join(lines, "\n"))
		}
		if fragmentLines != 1 {
			t.Errorf("width %d: %d lines carry a chord fragment, want the chord unsplit on 1:\n%s",
				width, fragmentLines, strings.Join(lines, "\n"))
		}
		if !strings.Contains(keyLines[0], key+"  S") {
			t.Errorf("width %d: chord line %q does not open the description", width, keyLines[0])
		}
	}
}

func TestHelpRenderKeyColumnFitsEveryComposedSpelling(t *testing.T) {
	strategy := operation.SimplifiedInputStrategy()
	seen := make(map[operation.OperationID]bool, len(strategy.Bindings))
	for _, binding := range strategy.Bindings {
		if seen[binding.OperationID] {
			continue
		}
		seen[binding.OperationID] = true
		key := operation.DisplayKeysForOperation(strategy, binding.OperationID)
		if width := lipgloss.Width(key); width > helpKeyColumnWidth {
			t.Errorf("%s displays as %q (%d cells), which wraps the %d-cell help key column",
				binding.OperationID, key, width, helpKeyColumnWidth)
		}
	}
}

func TestHelpColumnsFollowAvailableDescriptionWidth(t *testing.T) {
	m := NewHelpViewModel([]operation.HelpItem{
		{Key: "e", Description: "Edit the task title", Category: "Tasks"},
		{Key: "a", Description: "Capture a new task", Category: "Tasks"},
	}, "Outliner")
	for _, width := range []int{60, 80, 91, 92, 100, 160, 60} {
		m.SetSize(width, 20)
		plain := ansi.Strip(m.renderContent())
		together := false
		for _, line := range strings.Split(plain, "\n") {
			if lipgloss.Width(line) > m.viewport.Width() {
				t.Fatalf("width %d overflows: %q", width, line)
			}
			if strings.Contains(line, "Edit the task title") && strings.Contains(line, "Capture a new task") {
				together = true
			}
		}
		if together != (width >= 92) {
			t.Fatalf("width %d: unexpected column layout:\n%s", width, plain)
		}
		for _, text := range []string{"Edit the task title", "Capture a new task"} {
			if !strings.Contains(plain, text) {
				t.Fatalf("width %d lost description %q", width, text)
			}
		}
	}
}

func TestHelpHighlightsMatchesAfterWrappingUnicodeDescription(t *testing.T) {
	// A visible marker makes the test independent of terminal color support.
	previous := theme.HighlightStyle
	theme.HighlightStyle = lipgloss.NewStyle().Underline(true)
	t.Cleanup(func() { theme.HighlightStyle = previous })
	m := NewHelpViewModel([]operation.HelpItem{{
		Key: "e", Category: "Tasks",
		Description: "Read 界界界 café details before choosing the zebra action",
	}}, "Outliner")
	m.searchQuery = "zebra"
	m.filterItems()
	for _, width := range []int{60, 100} {
		m.SetSize(width, 20)
		rendered := m.renderContent()
		if !utf8.ValidString(rendered) {
			t.Fatalf("width %d split a Unicode character", width)
		}
		found := false
		for i, line := range strings.Split(rendered, "\n") {
			if lipgloss.Width(line) > m.viewport.Width() {
				t.Fatalf("width %d overflows: %q", width, line)
			}
			if strings.Contains(ansi.Strip(line), "zebra") {
				if i <= 1 || !strings.Contains(line, theme.HighlightStyle.Render("z")) {
					t.Fatalf("wrapped match lost highlighting: %q", line)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("missing wrapped match:\n%s", rendered)
		}
	}
}
