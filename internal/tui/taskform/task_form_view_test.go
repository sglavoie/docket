package taskform

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// TestTaskFormMinimumLayout guards the expanded form at the common 80x24
// terminal, where the form is allotted 80x22 (terminalHeight-2). The guard now
// derives from the app floor (formMinHeight = chrome.MinTerminalHeight - 2 = 18) and
// the body is clamped to the available rows, so the form must actually render
// (no "terminal too small" notice) and fit both width and height at:
//   - the 80x24 terminal's allotted size, SetSize(80, 22)
//   - the form's genuine floor, SetSize(formMinWidth, chrome.MinTerminalHeight-2)
//
// The floor uses formMinWidth (68), not the app's chrome.MinTerminalWidth (60): the
// form body is fixed-width, so width genuinely cannot go below it and the
// min-size notice still (correctly) appears below formMinWidth. Only the height
// floor changed.
func TestTaskFormMinimumLayout(t *testing.T) {
	sizes := []struct {
		w, h int
	}{
		{80, 22},
		{formMinWidth, chrome.MinTerminalHeight - 2},
	}
	for _, sz := range sizes {
		m := newTestTaskForm()
		m.SetSize(sz.w, sz.h)

		rendered := m.View().Content
		if strings.Contains(ansi.Strip(rendered), "Terminal too small") {
			t.Fatalf("form should render (not show the min-size notice) at %dx%d:\n\n%s",
				sz.w, sz.h, ansi.Strip(rendered))
		}
		assertFitsWithin(t, rendered, sz.w, sz.h)
	}
}

// TestTaskFormFitsWidthWhenRendered verifies the primary width fix: when the
// terminal is tall enough for the form to render (height >= formMinHeight) but
// too narrow for the two-column help pane, the form falls back to a single
// column instead of overflowing to width 95. Before the fix this rendered at
// width 95 at any width >= 60.
func TestTaskFormFitsWidthWhenRendered(t *testing.T) {
	m := newTestTaskForm()
	m.SetSize(72, 30)

	rendered := m.View().Content
	if strings.Contains(ansi.Strip(rendered), "Terminal too small") {
		t.Fatalf("form should render (not show the min-size notice) at 72x30:\n\n%s",
			ansi.Strip(rendered))
	}
	assertFitsWithin(t, rendered, 72, 30)
}

func TestTaskFormInlineOverlayKeepsCompleteFrameAtMinimumSize(t *testing.T) {
	m := newTestTaskForm()
	m.SetSize(formMinWidth, formMinHeight)
	m.activeOverlay = formOverlayPriority
	m.priorityPicker = newFormPriorityPicker(m.draft.priority)

	rendered := m.View().Content
	assertFitsWithin(t, rendered, formMinWidth, formMinHeight)

	lines := strings.Split(ansi.Strip(rendered), "\n")
	topBorder, bottomBorder := -1, -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "╭") && strings.HasSuffix(trimmed, "╮") {
			topBorder = i
		}
		if strings.HasPrefix(trimmed, "╰") && strings.HasSuffix(trimmed, "╯") {
			bottomBorder = i
		}
	}
	if topBorder < 0 || bottomBorder <= topBorder {
		t.Fatalf("inline overlay frame was clipped at minimum size:\n%s", ansi.Strip(rendered))
	}
	if bottomBorder+1 >= len(lines) || strings.TrimSpace(lines[bottomBorder+1]) != "" {
		t.Fatalf("inline overlay bottom halo was clipped at minimum size:\n%s", ansi.Strip(rendered))
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
