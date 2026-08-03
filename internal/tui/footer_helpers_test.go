package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestFitFooterHintLineKeepsRankedPairsWhole(t *testing.T) {
	pairs := []string{"a capture", "Space actions", "/ search", "c scope"}
	firstTwo := strings.Join(pairs[:2], footerHelpSeparator)

	if got := fitFooterHintLine(pairs, lipgloss.Width(firstTwo)); got != firstTwo {
		t.Fatalf("exact-fit footer = %q, want %q", got, firstTwo)
	}
	if got := fitFooterHintLine(pairs, lipgloss.Width(firstTwo)-1); got != pairs[0] {
		t.Fatalf("narrow footer = %q, want only the first complete pair %q", got, pairs[0])
	}
	if got := fitFooterHintLine(pairs, lipgloss.Width(pairs[0])-1); got != "" {
		t.Fatalf("footer with no complete pair = %q, want empty", got)
	}
}

func TestPendingChordFooterUsesAtMostTwoNonwrappingHintLines(t *testing.T) {
	hints := []ChordHint{
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

	lines := fitPendingChordLines("s", hints, minTerminalWidth)
	if len(lines) == 0 || len(lines) > 2 {
		t.Fatalf("pending chord lines = %d, want 1..2: %q", len(lines), lines)
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > minTerminalWidth {
			t.Fatalf("pending chord line width = %d, want <= %d: %q", got, minTerminalWidth, line)
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

	rendered := renderPendingChordFooter(lines, minTerminalWidth)
	if got := lipgloss.Height(rendered); got > 3 {
		t.Fatalf("pending chord footer height = %d, want divider plus at most two rows", got)
	}
	for line := range strings.SplitSeq(rendered, "\n") {
		if got := lipgloss.Width(line); got > minTerminalWidth {
			t.Fatalf("rendered pending chord line width = %d, want <= %d: %q", got, minTerminalWidth, ansi.Strip(line))
		}
	}
}

func TestOutlinerFooterUsesApplicableRankedMetadataAtNarrowWidth(t *testing.T) {
	app := newUITestApp()
	outliner := app.crossOutliner
	outliner.SetSize(minTerminalWidth, minTerminalHeight-2)

	pairs := app.operationRegistry.FooterHintPairsForInputStrategy(
		InputStrategySimplified,
		outliner.operationCommandContext(),
	)
	if len(pairs) < 3 || !slices.Equal(pairs[:3], []string{"a capture", "Space actions", "/ search"}) {
		t.Fatalf("ranked footer prefix = %v, want capture actions search", pairs)
	}
	spec, ok := outliner.ContextualFooter()
	if !ok {
		t.Fatal("Outliner did not provide its operation footer")
	}
	if got, want := spec.Left, fitFooterHintLine(pairs, minTerminalWidth); got != want {
		t.Fatalf("Outliner footer = %q, want fitted metadata %q", got, want)
	}
	if strings.Contains(spec.Left, "\n") || lipgloss.Width(spec.Left) > minTerminalWidth {
		t.Fatalf("normal footer must be one fitted line: %q", spec.Left)
	}

	// A visual selection makes cursor-only edit operations inapplicable. At a
	// wide size they would otherwise be visible, so their absence proves the
	// footer uses live descriptor applicability rather than a static key list.
	outliner.multiSelectMode = true
	outliner.SetSize(240, minTerminalHeight-2)
	spec, ok = outliner.ContextualFooter()
	if !ok {
		t.Fatal("selected Outliner did not provide its operation footer")
	}
	for _, unavailable := range []string{"e title", "E description", "Ctrl+E editor"} {
		if strings.Contains(spec.Left, unavailable) {
			t.Errorf("selected footer exposes unavailable operation %q: %q", unavailable, spec.Left)
		}
	}
}
