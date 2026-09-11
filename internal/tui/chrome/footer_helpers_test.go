package chrome

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestDimmedGuidancePreservesTextPackingAndClippedStyle(t *testing.T) {
	spec := GuidanceSpec{Rows: 2, Hints: []GuidanceHint{
		{Key: "c", Label: "conflicts", Required: true, Dimmed: true},
		{Key: "Esc", Label: "cancel", Required: true},
	}}
	for _, width := range []int{1, 5, 12, 40} {
		plain := strings.Join(GuidanceLines(spec, width), "\n")
		rendered := RenderGuidanceText(spec, width)
		if ansi.Strip(rendered) != plain || lipgloss.Width(rendered) > width {
			t.Fatalf("width %d: rendered %q does not preserve %q within budget", width, rendered, plain)
		}
		segments := guidanceSegmentLines(spec, width)
		first := segments[0][0]
		if !first.Dimmed || !strings.Contains(rendered, theme.DimStyle.Render(first.text())) {
			t.Fatalf("width %d: dimmed styling lost: %q", width, rendered)
		}
		for i := range spec.Hints {
			spec.Hints[i].Dimmed = false
		}
		if got := strings.Join(GuidanceLines(spec, width), "\n"); got != plain {
			t.Fatalf("dimming changed packing at width %d: %q != %q", width, got, plain)
		}
		spec.Hints[0].Dimmed = true
	}
}

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
	line := strings.Join(GuidanceLines(spec, 20), "\n")
	if line != "a alpha · Esc cancel" {
		t.Fatalf("packed guidance = %q, want highest-priority optional plus required control", line)
	}
	if strings.Contains(line, "b beta") || strings.Contains(line, "50%") {
		t.Fatalf("lower-priority content displaced a required control: %q", line)
	}
	wide := strings.Join(GuidanceLines(spec, 40), "\n")
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
	exact := lipgloss.Width(first + FooterHelpSeparator + second)
	if got := GuidanceLines(spec, exact); !slices.Equal(got, []string{first + FooterHelpSeparator + second}) {
		t.Fatalf("Unicode exact-fit guidance = %q", got)
	}

	spec.Rows = 1
	oneRow := GuidanceLines(spec, lipgloss.Width(first))
	if !slices.Equal(oneRow, []string{first}) {
		t.Fatalf("one-row guidance = %q, want only first complete hint", oneRow)
	}
	spec.Rows = 2
	twoRows := GuidanceLines(spec, lipgloss.Width(second))
	if !slices.Equal(twoRows, []string{first, second}) {
		t.Fatalf("two-row guidance = %q, want both complete hints", twoRows)
	}
}

// TestGuidanceSegmentsKeepKeysApartFromLabelsAndSupplemental builds the shape a
// pending chord footer has — a bare prefix key, one labeled hint, the required
// Esc control — without naming the shell helper that assembles it, because the
// invariant under test is the packer's, not the chord's.
func TestGuidanceSegmentsKeepKeysApartFromLabelsAndSupplemental(t *testing.T) {
	spec := GuidanceSpec{
		Rows: 2,
		Hints: []GuidanceHint{
			{Key: "w…"},
			{Key: "i", Label: "inbox", Priority: 1},
			{Key: "Esc", Label: "cancel", Required: true},
		},
	}
	spec.Supplemental = "search: alpha · location: Global"

	lines := guidanceSegmentLines(spec, 80)
	if len(lines) != 1 {
		t.Fatalf("segment lines = %d, want 1: %v", len(lines), lines)
	}
	want := []guidanceSegment{
		{Key: "w…"},
		{Key: "i", Label: "inbox"},
		{Key: "Esc", Label: "cancel"},
		{Verbatim: "search: alpha · location: Global"},
	}
	if !slices.Equal(lines[0], want) {
		t.Fatalf("segments = %v, want %v", lines[0], want)
	}
	plain := "w… · i inbox · Esc cancel · search: alpha · location: Global"
	if got := strings.Join(GuidanceLines(spec, 80), "\n"); got != plain {
		t.Fatalf("plain guidance = %q, want %q", got, plain)
	}
}

func TestRenderedGuidanceColorsKeysAndPreservesPlainText(t *testing.T) {
	spec := GuidanceSpec{
		Hints:        RequiredGuidanceHints("Enter create", "Esc cancel"),
		Supplemental: "50%",
		Rows:         1,
	}
	rendered := RenderGuidanceText(spec, 60)
	for _, want := range []string{
		theme.HelpKeyStyle.Render("Enter"),
		theme.HelpStyle.Render(" create"),
		theme.HelpKeyStyle.Render("Esc"),
		theme.HelpStyle.Render(" cancel"),
		theme.HelpStyle.Render("50%"),
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered guidance %q omits styled fragment %q", rendered, want)
		}
	}
	plain := strings.Join(GuidanceLines(spec, 60), "\n")
	if got := ansi.Strip(rendered); got != plain {
		t.Fatalf("styled guidance text = %q, want plain %q", got, plain)
	}
	if got := lipgloss.Width(rendered); got != lipgloss.Width(plain) {
		t.Fatalf("styled guidance width = %d, want %d", got, lipgloss.Width(plain))
	}
}

func TestGuidanceSegmentFitToWidthKeepsKeyAndClipsLabel(t *testing.T) {
	segment := guidanceSegment{Key: "Ctrl+E", Label: "create & edit"}
	fitted := segment.fitToWidth(10)
	if fitted.Key != "Ctrl+E" {
		t.Fatalf("fitted key = %q, want the keystroke intact", fitted.Key)
	}
	if got := lipgloss.Width(fitted.text()); got > 10 {
		t.Fatalf("fitted width = %d, want <= 10: %q", got, fitted.text())
	}
	degraded := segment.fitToWidth(4)
	if degraded.Key != "" || degraded.Verbatim == "" {
		t.Fatalf("segment narrower than its key = %+v, want verbatim clip", degraded)
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

	spec, ok := GuidanceSpecFromKeyMap(km, 1)
	if !ok || len(spec.Hints) != 2 {
		t.Fatalf("live keymap guidance = (%+v, %v), want two visible active hints", spec, ok)
	}
	if got := []string{GuidanceHintText(spec.Hints[0]), GuidanceHintText(spec.Hints[1])}; !slices.Equal(got, []string{"/ search", "enter choose"}) {
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
		if strings.Contains(strings.Join(GuidanceLines(spec, 80), "\n"), unavailable) {
			t.Errorf("guidance exposed hidden or gated binding %q", unavailable)
		}
	}

	live = false
	spec, ok = GuidanceSpecFromKeyMap(km, 1)
	if !ok || len(spec.Hints) != 1 || spec.Hints[0].Key != "enter" {
		t.Fatalf("closed gate guidance = (%+v, %v), want required Enter only", spec, ok)
	}
}
