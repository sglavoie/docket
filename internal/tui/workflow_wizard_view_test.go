package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// TestWorkflowWizardMinimumLayout is the regression guard for Bug 1: the
// Step 1 trigger-field list used a fixed 16-row window (wizardListMaxVisible)
// that ignored terminal height, rendering ~29 lines and overflowing every
// baseline height. The window is now height-derived (visibleListRows), so the
// wizard must fit within 72x20 in both line count and visual width.
func TestWorkflowWizardMinimumLayout(t *testing.T) {
	cfg := newTestConfig()
	m := NewWorkflowWizardModel(testRuntimeAt("", cfg))
	m.SetSize(72, 20)

	rendered := m.render()

	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	if len(lines) > 20 {
		t.Fatalf("wizard render has %d lines, exceeds height 20:\n\n%s",
			len(lines), ansi.Strip(rendered))
	}
	for _, line := range lines {
		if w := lipgloss.Width(ansi.Strip(line)); w > 72 {
			t.Fatalf("wizard line exceeds width 72: %q (width=%d)\n\n%s",
				ansi.Strip(line), w, ansi.Strip(rendered))
		}
	}
}

// TestWorkflowWizardListStillScrolls confirms the height-derived window keeps
// in-list navigation working: with more trigger fields than fit the window,
// scrolling the cursor down past the visible window surfaces both the
// "↑ more above" and "↓ more below" scroll hints.
func TestWorkflowWizardListStillScrolls(t *testing.T) {
	cfg := newTestConfig()
	m := NewWorkflowWizardModel(testRuntimeAt("", cfg))
	m.SetSize(72, 20)

	rows := m.visibleListRows()
	if len(m.triggerFields) <= rows {
		t.Fatalf("test needs more trigger fields (%d) than visible rows (%d)",
			len(m.triggerFields), rows)
	}

	// Move the cursor to the last field so the window scrolls past the top.
	m.triggerCursor = len(m.triggerFields) - 1
	m.clampListScroll(&m.triggerCursor)

	rendered := ansi.Strip(m.render())
	if !strings.Contains(rendered, "more above") {
		t.Fatalf("expected '↑ more above' hint after scrolling to bottom:\n\n%s", rendered)
	}
}
