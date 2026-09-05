package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// TestPendingChordGuidanceUsesAtMostTwoNonwrappingHintLines holds the shell's
// chord footer to the minimum terminal width. It stays in the shell because
// pendingChordGuidanceSpec takes a ChordHint, which belongs to the operation
// layer above chrome; chrome's own packing invariants live beside the packer in
// internal/tui/chrome/footer_helpers_test.go.
func TestPendingChordGuidanceUsesAtMostTwoNonwrappingHintLines(t *testing.T) {
	hints := []operation.ChordHint{
		{Key: "i", Label: "inbox"},
		{Key: "b", Label: "backlog"},
		{Key: "n", Label: "next"},
		{Key: "t", Label: "today"},
		{Key: "g", Label: "doing"},
		{Key: "w", Label: "waiting"},
		{Key: "p", Label: "paused"},
		{Key: "m", Label: "someday"},
		{Key: "d", Label: "done"},
		{Key: "c", Label: "canceled"},
	}

	spec := pendingChordGuidanceSpec("w", hints)
	lines := chrome.GuidanceLines(spec, chrome.MinTerminalWidth)
	if len(lines) == 0 || len(lines) > 2 {
		t.Fatalf("pending chord lines = %d, want 1..2: %q", len(lines), lines)
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > chrome.MinTerminalWidth {
			t.Fatalf("pending chord line width = %d, want <= %d: %q", got, chrome.MinTerminalWidth, line)
		}
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Esc cancel") {
		t.Fatalf("pending chord lines omit Esc cancellation: %q", lines)
	}
	last := -1
	for _, hint := range hints {
		entry := hint.Key + " " + hint.Label
		index := strings.Index(joined, entry)
		if index < 0 {
			continue
		}
		if index <= last {
			t.Fatalf("declaration order changed around %q in %q", entry, joined)
		}
		last = index
	}

	rendered := chrome.RenderGuidance(spec, chrome.MinTerminalWidth)
	if got := lipgloss.Height(rendered); got > 3 {
		t.Fatalf("pending chord footer height = %d, want divider plus at most two rows", got)
	}
	for line := range strings.SplitSeq(rendered, "\n") {
		if got := lipgloss.Width(line); got > chrome.MinTerminalWidth {
			t.Fatalf("rendered pending chord line width = %d, want <= %d: %q", got, chrome.MinTerminalWidth, ansi.Strip(line))
		}
	}
}

func TestOutlinerFooterUsesApplicableRankedMetadataAtNarrowWidth(t *testing.T) {
	app := newFilterChordTestApp(t)
	outlinerView := app.crossOutliner
	for index, row := range outlinerView.Rows {
		if row.IsTaskRow() {
			outlinerView.Cursor = index
			break
		}
	}
	outlinerView.SetSize(chrome.MinTerminalWidth, chrome.MinTerminalHeight-2)

	hints := app.operations.Registry.GuidanceHintsForInputStrategy(
		operation.InputStrategySimplified, outlinerView.OperationCommandContext(),
	)
	pairs := make([]string, 0, len(hints))
	for _, hint := range hints {
		pairs = append(pairs, chrome.GuidanceHintText(hint))
	}
	// `M menu` sits sixth because revealing the map of this grammar outranks every
	// act the map lists; the four hints the rendered row is held to below are
	// ahead of it, so the bar's rank buys its place without costing a required one.
	wantWorkflow := []string{
		"$ workflow", "! priority", "@ date", "# tags/context", "Space actions",
		"M menu", "v select", "a capture", "Enter focus", "Ctrl+Enter peek", "/ search", "? help",
	}
	if len(pairs) < len(wantWorkflow) || !slices.Equal(pairs[:len(wantWorkflow)], wantWorkflow) {
		t.Fatalf("ranked footer prefix = %v, want workflow-first task guidance %v", pairs, wantWorkflow)
	}
	spec, ok := outlinerView.ContextualGuidance()
	if !ok {
		t.Fatal("Outliner did not provide its operation footer")
	}
	lines := chrome.GuidanceLines(spec, chrome.MinTerminalWidth)
	if len(lines) != 1 {
		t.Fatalf("Outliner browse guidance rows = %d, want one: %q", len(lines), lines)
	}
	for _, want := range pairs[:4] {
		if !strings.Contains(lines[0], want) {
			t.Fatalf("Outliner guidance omits ranked hint %q: %q", want, lines[0])
		}
	}
	if lipgloss.Width(lines[0]) > chrome.MinTerminalWidth {
		t.Fatalf("normal guidance exceeds one row: %q", lines[0])
	}

	// A visual selection makes cursor-only edit operations inapplicable. At a
	// wide size they would otherwise be visible, so their absence proves the
	// footer uses live descriptor applicability rather than a static key list.
	outlinerView.MultiSelectMode = true
	outlinerView.SetSize(240, chrome.MinTerminalHeight-2)
	spec, ok = outlinerView.ContextualGuidance()
	if !ok {
		t.Fatal("selected Outliner did not provide its operation footer")
	}
	allHints := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		allHints = append(allHints, chrome.GuidanceHintText(hint))
	}
	joined := strings.Join(allHints, "\n")
	for _, unavailable := range []string{"e edit", "E description", "Ctrl+E editor"} {
		if strings.Contains(joined, unavailable) {
			t.Errorf("selected guidance exposes unavailable operation %q: %q", unavailable, joined)
		}
	}
}
