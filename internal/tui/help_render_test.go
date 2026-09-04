package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/tui/operation"
)

func TestHelpRenderKeepsSetStateChordOnOneLine(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	key := operation.DisplayKeysForOperation(strategy, operation.OperationTaskSetState)
	if !strings.HasPrefix(key, "$") {
		t.Fatalf("Set State key = %q, want the $ chord prefix", key)
	}
	for _, binding := range strategy.Bindings {
		if binding.OperationID != operation.OperationTaskSetState || len(binding.Sequence) != 2 {
			continue
		}
		if letter := binding.Sequence[1]; !strings.Contains(key, letter) {
			t.Fatalf("Set State key %q omits the $%s terminal", key, letter)
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
		sigilLines := 0
		for _, line := range lines {
			if strings.Contains(line, key) {
				keyLines = append(keyLines, line)
			}
			if strings.Contains(line, "$") {
				sigilLines++
			}
		}
		if len(keyLines) != 1 {
			t.Fatalf("width %d: %d lines carry the whole %q chord, want 1:\n%s",
				width, len(keyLines), key, strings.Join(lines, "\n"))
		}
		if sigilLines != 1 {
			t.Errorf("width %d: %d lines carry a $ token, want the chord unsplit on 1:\n%s",
				width, sigilLines, strings.Join(lines, "\n"))
		}
		if !strings.Contains(keyLines[0], key+"  S") {
			t.Errorf("width %d: chord line %q does not open the description", width, keyLines[0])
		}
	}
}

func TestHelpRenderKeyColumnFitsEveryComposedSpelling(t *testing.T) {
	strategy := SimplifiedInputStrategy()
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
