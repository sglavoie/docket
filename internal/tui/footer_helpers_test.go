package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

func TestGuidanceRequiredHintsEvictOptionalAndSupplemental(t *testing.T) {
	spec := GuidanceSpec{
		Hints: []GuidanceHint{
			{Key: "a", Label: "alpha", Priority: 1},
			{Key: "b", Label: "beta", Priority: 2},
			{Key: "Esc", Label: "cancel", Required: true},
		},
		Supplemental: "50%",
		Rows:         1,
	}
	line := strings.Join(guidanceLines(spec, 20), "\n")
	if line != "a alpha · Esc cancel" {
		t.Fatalf("packed guidance = %q, want highest-priority optional plus required control", line)
	}
	if strings.Contains(line, "b beta") || strings.Contains(line, "50%") {
		t.Fatalf("lower-priority content displaced a required control: %q", line)
	}
	wide := strings.Join(guidanceLines(spec, 40), "\n")
	for _, want := range []string{"a alpha", "b beta", "Esc cancel", "50%"} {
		if !strings.Contains(wide, want) {
			t.Fatalf("wide guidance omitted %q: %q", want, wide)
		}
	}
}

func TestGuidanceUsesUnicodeCellWidthAndRowBudget(t *testing.T) {
	first := "界 open"
	second := "Enter choose"
	spec := GuidanceSpec{Hints: []GuidanceHint{
		{Key: "界", Label: "open", Priority: 1},
		{Key: "Enter", Label: "choose", Priority: 2},
	}}
	exact := lipgloss.Width(first + footerHelpSeparator + second)
	if got := guidanceLines(spec, exact); !slices.Equal(got, []string{first + footerHelpSeparator + second}) {
		t.Fatalf("Unicode exact-fit guidance = %q", got)
	}

	spec.Rows = 1
	oneRow := guidanceLines(spec, lipgloss.Width(first))
	if !slices.Equal(oneRow, []string{first}) {
		t.Fatalf("one-row guidance = %q, want only first complete hint", oneRow)
	}
	spec.Rows = 2
	twoRows := guidanceLines(spec, lipgloss.Width(second))
	if !slices.Equal(twoRows, []string{first, second}) {
		t.Fatalf("two-row guidance = %q, want both complete hints", twoRows)
	}
}

func TestPendingChordGuidanceUsesAtMostTwoNonwrappingHintLines(t *testing.T) {
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

	spec := pendingChordGuidanceSpec("s", hints)
	lines := guidanceLines(spec, minTerminalWidth)
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

	rendered := renderGuidance(spec, minTerminalWidth)
	if got := lipgloss.Height(rendered); got > 3 {
		t.Fatalf("pending chord footer height = %d, want divider plus at most two rows", got)
	}
	for line := range strings.SplitSeq(rendered, "\n") {
		if got := lipgloss.Width(line); got > minTerminalWidth {
			t.Fatalf("rendered pending chord line width = %d, want <= %d: %q", got, minTerminalWidth, ansi.Strip(line))
		}
	}
}

func TestGuidanceFromGatedKeyMapMatchesHelpAndHandledKeys(t *testing.T) {
	live := true
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("enter", "Choose current item", "Actions", keymap.WithFooter(2, "choose", true)),
		keymap.Bind("/", "Search items", "Filters", keymap.WithFooter(1, "search", false), keymap.WithGate(func() bool {
			return live
		})),
		keymap.Bind("x", "Hidden alias", "Actions", keymap.AsHidden(), keymap.WithFooter(3, "delete", false)),
		keymap.Bind("z", "Unavailable action", "Actions", keymap.WithFooter(4, "unavailable", false), keymap.WithGate(func() bool {
			return false
		})),
	}}

	spec, ok := guidanceSpecFromKeyMap(km, 1)
	if !ok || len(spec.Hints) != 2 {
		t.Fatalf("live keymap guidance = (%+v, %v), want two visible active hints", spec, ok)
	}
	if got := []string{guidanceHintText(spec.Hints[0]), guidanceHintText(spec.Hints[1])}; !slices.Equal(got, []string{"/ search", "enter choose"}) {
		t.Fatalf("ranked keymap guidance = %v", got)
	}
	if !spec.Hints[1].Required {
		t.Fatal("required binding metadata was not preserved")
	}

	helpKeys := make(map[string]bool)
	for _, item := range km.HelpItems() {
		helpKeys[item.Key] = true
	}
	handled := km.HandledKeys()
	for _, hint := range spec.Hints {
		if !helpKeys[hint.Key] {
			t.Errorf("guidance key %q is absent from live Help", hint.Key)
		}
		if !slices.Contains(handled, hint.Key) {
			t.Errorf("guidance key %q is absent from HandledKeys", hint.Key)
		}
	}
	for _, unavailable := range []string{"x delete", "z unavailable"} {
		if strings.Contains(strings.Join(guidanceLines(spec, 80), "\n"), unavailable) {
			t.Errorf("guidance exposed hidden or gated binding %q", unavailable)
		}
	}

	live = false
	spec, ok = guidanceSpecFromKeyMap(km, 1)
	if !ok || len(spec.Hints) != 1 || spec.Hints[0].Key != "enter" {
		t.Fatalf("closed gate guidance = (%+v, %v), want required Enter only", spec, ok)
	}
}

func TestOutlinerFooterUsesApplicableRankedMetadataAtNarrowWidth(t *testing.T) {
	app := newFilterChordTestApp(t)
	outliner := app.crossOutliner
	for index, row := range outliner.rows {
		if row.isTaskRow() {
			outliner.cursor = index
			break
		}
	}
	outliner.SetSize(minTerminalWidth, minTerminalHeight-2)

	hints := app.operationRegistry.GuidanceHintsForInputStrategy(
		InputStrategySimplified, outliner.operationCommandContext(),
	)
	pairs := make([]string, 0, len(hints))
	for _, hint := range hints {
		pairs = append(pairs, guidanceHintText(hint))
	}
	wantWorkflow := []string{
		"s workflow", "! priority", "@ date", "# tags", "Space actions",
		"v select", "a capture", "Enter peek", "/ search", "? help",
	}
	if len(pairs) < len(wantWorkflow) || !slices.Equal(pairs[:len(wantWorkflow)], wantWorkflow) {
		t.Fatalf("ranked footer prefix = %v, want workflow-first task guidance %v", pairs, wantWorkflow)
	}
	spec, ok := outliner.ContextualGuidance()
	if !ok {
		t.Fatal("Outliner did not provide its operation footer")
	}
	lines := guidanceLines(spec, minTerminalWidth)
	if len(lines) != 1 {
		t.Fatalf("Outliner browse guidance rows = %d, want one: %q", len(lines), lines)
	}
	for _, want := range pairs[:4] {
		if !strings.Contains(lines[0], want) {
			t.Fatalf("Outliner guidance omits ranked hint %q: %q", want, lines[0])
		}
	}
	if lipgloss.Width(lines[0]) > minTerminalWidth {
		t.Fatalf("normal guidance exceeds one row: %q", lines[0])
	}

	// A visual selection makes cursor-only edit operations inapplicable. At a
	// wide size they would otherwise be visible, so their absence proves the
	// footer uses live descriptor applicability rather than a static key list.
	outliner.multiSelectMode = true
	outliner.SetSize(240, minTerminalHeight-2)
	spec, ok = outliner.ContextualGuidance()
	if !ok {
		t.Fatal("selected Outliner did not provide its operation footer")
	}
	allHints := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		allHints = append(allHints, guidanceHintText(hint))
	}
	joined := strings.Join(allHints, "\n")
	for _, unavailable := range []string{"e title", "E description", "Ctrl+E editor"} {
		if strings.Contains(joined, unavailable) {
			t.Errorf("selected guidance exposes unavailable operation %q: %q", unavailable, joined)
		}
	}
}
