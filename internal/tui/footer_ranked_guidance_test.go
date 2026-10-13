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

// TestPendingChordGuidanceUsesAtMostThreeNonwrappingHintLines holds the shell's
// chord footer to the minimum terminal width, where the three-row budget is what
// lets the workflow chord name every one of its ten entries. It stays in the shell because
// pendingChordGuidanceSpec takes a ChordHint, which belongs to the operation
// layer above chrome; chrome's own packing invariants live beside the packer in
// internal/tui/chrome/footer_helpers_test.go.
func TestPendingChordGuidanceUsesAtMostThreeNonwrappingHintLines(t *testing.T) {
	hints := workflowChordHintsForTest()

	spec := pendingChordGuidanceSpec("w", hints)
	lines := chrome.GuidanceLines(spec, chrome.MinTerminalWidth)
	if len(lines) == 0 || len(lines) > chrome.MaxGuidanceRows {
		t.Fatalf("pending chord lines = %d, want 1..%d: %q", len(lines), chrome.MaxGuidanceRows, lines)
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
			t.Fatalf("pending chord lines shed %q at the minimum width: %q", entry, lines)
		}
		if index <= last {
			t.Fatalf("declaration order changed around %q in %q", entry, joined)
		}
		last = index
	}

	rendered := chrome.RenderGuidance(spec, chrome.MinTerminalWidth)
	if got := lipgloss.Height(rendered); got > chrome.MaxGuidanceRows+1 {
		t.Fatalf("pending chord footer height = %d, want divider plus at most three rows", got)
	}
	for line := range strings.SplitSeq(rendered, "\n") {
		if got := lipgloss.Width(line); got > chrome.MinTerminalWidth {
			t.Fatalf("rendered pending chord line width = %d, want <= %d: %q", got, chrome.MinTerminalWidth, ansi.Strip(line))
		}
	}
}

// TestPendingChordGuidanceNeverStrandsADivisionHeading holds the other half of
// the same packing: a heading is not an entry of its own, so it rides the first
// binding under it and is packed, fitted, and dropped with that binding. The
// narrow footer sheds this chord's last entries, and the claim is that a
// heading is never what survives them — a lone `Status:` with nothing beneath it
// would read as a division the row could not show.
func TestPendingChordGuidanceNeverStrandsADivisionHeading(t *testing.T) {
	hints := workflowChordHintsForTest()

	// Every width the footer may be asked for, not just the floor: the packer
	// drops a different entry at each, so only a sweep reaches the width where a
	// heading is the last thing standing.
	for width := chrome.MinTerminalWidth; width <= 160; width++ {
		joined := ansi.Strip(strings.Join(chrome.GuidanceLines(pendingChordGuidanceSpec("w", hints), width), "\n"))
		for _, group := range []struct{ heading, first string }{
			{chrome.PlanHintGroup, "n next"},
			{chrome.StatusHintGroup, "i inbox"},
		} {
			if strings.Contains(joined, group.heading) && !strings.Contains(joined, group.heading+" "+group.first) {
				t.Fatalf("width %d strands the %q heading away from %q:\n%s",
					width, group.heading, group.first, joined)
			}
		}
	}
}

// TestPendingChordGuidanceShedsDoneLastOfItsRun holds the selection order the
// floor never exercises: every width down to the one that can no longer hold
// the Status heading's own entry. Done is the entry a day is closed with, so
// while that run is drawn at all Done is in it, and it never outlives the
// heading it belongs under.
func TestPendingChordGuidanceShedsDoneLastOfItsRun(t *testing.T) {
	hints := workflowChordHintsForTest()

	shedSomething := false
	for width := chrome.MinTerminalWidth; width >= 12; width-- {
		joined := ansi.Strip(strings.Join(chrome.GuidanceLines(pendingChordGuidanceSpec("w", hints), width), "\n"))
		hasStatus := strings.Contains(joined, chrome.StatusHintGroup+" i inbox")
		hasDone := strings.Contains(joined, "d done")
		if hasDone && !hasStatus {
			t.Fatalf("width %d draws Done outside its Status run:\n%s", width, joined)
		}
		if strings.Contains(joined, "b backlog") && !hasDone {
			t.Fatalf("width %d shed Done ahead of another Status entry:\n%s", width, joined)
		}
		if hasDone && !strings.Contains(joined, "c canceled") {
			shedSomething = true
		}
		if hasDone && strings.Index(joined, "d done") < strings.Index(joined, "i inbox") {
			t.Fatalf("width %d moved Done out of lifecycle order:\n%s", width, joined)
		}
	}
	if !shedSomething {
		t.Fatal("no swept width kept Done while shedding a later entry, so the priority was never exercised")
	}
}

// workflowChordHintsForTest is the workflow chord as its strategy declares it:
// the two planning collections under Plan, then the eight lifecycle states
// under Status, each entry carrying its own group so the shell decides where a
// heading is drawn.
func workflowChordHintsForTest() []operation.ChordHint {
	return []operation.ChordHint{
		{Key: "n", Label: "next", Group: chrome.PlanHintGroup},
		{Key: "t", Label: "today", Group: chrome.PlanHintGroup},
		{Key: "i", Label: "inbox", Group: chrome.StatusHintGroup},
		{Key: "b", Label: "backlog", Group: chrome.StatusHintGroup},
		{Key: "g", Label: "doing", Group: chrome.StatusHintGroup},
		{Key: "w", Label: "waiting", Group: chrome.StatusHintGroup},
		{Key: "p", Label: "paused", Group: chrome.StatusHintGroup},
		{Key: "m", Label: "someday", Group: chrome.StatusHintGroup},
		{Key: "d", Label: "done", Group: chrome.StatusHintGroup, Keep: true},
		{Key: "c", Label: "canceled", Group: chrome.StatusHintGroup},
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
	// The first six rungs are the acts a task row is read for, and the second tier
	// behind them is the axes that row is changed on, then the sheets and
	// selections that reach the rest. `M menu` and `v select` sit after those
	// because revealing the map of this grammar is worth less on a narrow row than
	// `? help`, which already names discovery and is ranked ahead of both.
	wantRanked := []string{
		"a capture", "w workflow", "e edit", "/ search", "b peek", "? help",
		"@ date", "! priority", "# tags/context", "Space actions", "M menu", "v select",
	}
	if len(pairs) < len(wantRanked) || !slices.Equal(pairs[:len(wantRanked)], wantRanked) {
		t.Fatalf("ranked footer prefix = %v, want daily-act task guidance %v", pairs, wantRanked)
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

	// The tier boundary is where the narrow row ends: the head fits at the floor
	// and the first entry of the second tier is what the packer refuses, which is
	// the claim TestOutlinerFooterKeepsCaptureThroughHelpAtTheFloor holds the
	// rendered row to.
	if strings.Contains(lines[0], "@ date") {
		t.Fatalf("floor row reaches past the six leading acts: %q", lines[0])
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

// TestOutlinerFooterKeepsCaptureThroughHelpAtTheFloor is the contract the
// ladder's leading six are sized against. `a capture`, `w workflow`, `e edit`,
// `/ search`, `b peek`, and `? help` plus their separators come to exactly
// chrome.MinTerminalWidth, so the smallest supported terminal shows every one of
// them and the packer's first refusal falls on rank 7. Adding a rank ahead of
// them, or lengthening one of their labels, costs the narrow footer whichever of
// the six sits last — which is why this reads the rendered row rather than the
// ranked list.
func TestOutlinerFooterKeepsCaptureThroughHelpAtTheFloor(t *testing.T) {
	app := newFilterChordTestApp(t)
	outlinerView := app.crossOutliner
	for index, row := range outlinerView.Rows {
		if row.IsTaskRow() {
			outlinerView.Cursor = index
			break
		}
	}
	outlinerView.SetSize(chrome.MinTerminalWidth, chrome.MinTerminalHeight-2)

	spec, ok := outlinerView.ContextualGuidance()
	if !ok {
		t.Fatal("Outliner did not provide its operation footer")
	}
	lines := chrome.GuidanceLines(spec, chrome.MinTerminalWidth)
	if len(lines) != 1 {
		t.Fatalf("Outliner browse guidance rows = %d, want one: %q", len(lines), lines)
	}
	row := ansi.Strip(lines[0])

	leading := []string{"a capture", "w workflow", "e edit", "/ search", "b peek", "? help"}
	previous := -1
	for _, entry := range leading {
		index := strings.Index(row, entry)
		if index < 0 {
			t.Fatalf("floor row sheds leading act %q: %q", entry, row)
		}
		if index <= previous {
			t.Fatalf("floor row reorders %q against the ladder: %q", entry, row)
		}
		previous = index
	}
	if strings.Contains(row, "@ date") {
		t.Fatalf("floor row reaches rank 7, so the head is no longer what it is sized to: %q", row)
	}
	if got := lipgloss.Width(lines[0]); got != chrome.MinTerminalWidth {
		t.Fatalf("floor row width = %d, want exactly %d so the head spends the whole row: %q",
			got, chrome.MinTerminalWidth, row)
	}
}
