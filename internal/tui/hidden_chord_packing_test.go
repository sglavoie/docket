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

func liveChordHintsForTest(t *testing.T, prefix string) []operation.ChordHint {
	t.Helper()
	app := newFilterChordTestApp(t)
	app.operations.Chord = []string{prefix}
	ctx := app.operationCommandContext()
	ctx.View = app.currentView()
	return app.operations.Registry.DecoratedChordHintsForPrefix(
		operation.InputStrategySimplified,
		app.operations.Chord,
		ctx,
		app.crossOutliner.PendingChordHintDecorator(ctx),
	)
}

func floorChordLinesForTest(t *testing.T, prefix string, hints []operation.ChordHint) []string {
	t.Helper()
	lines := chrome.GuidanceLines(pendingChordGuidanceSpec(prefix, hints), chrome.MinTerminalWidth)
	if len(lines) == 0 || len(lines) > chrome.MaxGuidanceRows {
		t.Fatalf("%s chord lines = %d, want 1..%d: %q", prefix, len(lines), chrome.MaxGuidanceRows, lines)
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > chrome.MinTerminalWidth {
			t.Fatalf("%s chord line width = %d, want <= %d: %q", prefix, got, chrome.MinTerminalWidth, line)
		}
	}
	return lines
}

// TestWorkflowChordDrawsHideAtTheFloor holds the live w chord after it gained
// wh: every entry, eleven of them, still fits chrome.MaxGuidanceRows at
// chrome.MinTerminalWidth, and wh closes the Status run.
func TestWorkflowChordDrawsHideAtTheFloor(t *testing.T) {
	hints := liveChordHintsForTest(t, "w")
	joined := ansi.Strip(strings.Join(floorChordLinesForTest(t, "w", hints), "\n"))
	for _, hint := range hints {
		if entry := hint.Key + " " + hint.Label; !strings.Contains(joined, entry) {
			t.Fatalf("w chord shed %q at the floor:\n%s", entry, joined)
		}
	}
	last := hints[len(hints)-1]
	if last.Key != "h" || last.Label != "hide/unhide" || last.Group != chrome.StatusHintGroup {
		t.Fatalf("wh should close the Status run, last hint = %+v", last)
	}
}

// TestFilterChordHiddenCostsNothingAtTheFloor holds the live f chord after it
// gained fH. The chord already sheds entries at the floor, so the claim is that
// fH is declared last and ungrouped, which makes it the first entry shed: the
// floor draws exactly what it drew without it, and a wide row draws it after f%.
func TestFilterChordHiddenCostsNothingAtTheFloor(t *testing.T) {
	hints := liveChordHintsForTest(t, "f")
	last := hints[len(hints)-1]
	if last.Key != "H" || last.Label != "hidden" || last.Group != "" {
		t.Fatalf("fH should be the last, ungrouped f entry, got %+v", last)
	}
	with := floorChordLinesForTest(t, "f", hints)
	without := floorChordLinesForTest(t, "f", hints[:len(hints)-1])
	if !slices.Equal(with, without) {
		t.Fatalf("fH changed the floor rows:\nwith:\n%s\nwithout:\n%s",
			ansi.Strip(strings.Join(with, "\n")), ansi.Strip(strings.Join(without, "\n")))
	}
	wide := ansi.Strip(strings.Join(chrome.GuidanceLines(pendingChordGuidanceSpec("f", hints), 160), "\n"))
	at, after := strings.Index(wide, "H hidden"), strings.Index(wide, "% projects")
	if at < 0 || after < 0 || at < after {
		t.Fatalf("a wide f chord should draw fH after f%%:\n%s", wide)
	}
}
