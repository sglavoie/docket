package theme

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/config"
)

// tagChipGoldenSlots pins the hash and the ramp length — the two inputs that
// decide where a canonical tag lands. A tag's color is a promise kept across
// machines and releases, so a change here is a deliberate repaint of every
// workspace, never a silent reshuffle: swapping the hash or resizing the ramp
// fails this table by name. The normalizer feeding it is pinned separately, by
// TestTagChipSlotCollapsesSpellingsOfOneTag.
//
// This table lives beside tagChipStyle rather than beside the row that renders
// its output: the chip ramp is unreadable from outside this package by design,
// so the golden test is the only legitimate reader of a chip color.
var tagChipGoldenSlots = map[string]int{
	"bug":      1,
	"question": 3,
	"epic":     2,
	"story":    0,
	"feature":  5,
	"idea":     2,
	"review":   1,
	"ux":       4,
	"billing":  4,
	"refactor": 5,
	"infra":    1,
	"security": 3,
}

func TestTagChipSlotsMatchTheGoldenMap(t *testing.T) {
	if got := len(tagChipColors); got != len(config.DefaultTagChipRamp("dark")) {
		t.Fatalf("chip ramp = %d slots, want %d", got, len(config.DefaultTagChipRamp("dark")))
	}
	if got := len(tagChipColors); got != 6 {
		t.Fatalf("chip ramp = %d slots; the golden slots below are taken mod 6", got)
	}
	for tag, want := range tagChipGoldenSlots {
		if got := tagChipSlot(tag); got != want {
			t.Errorf("tag %q now lands on slot %d, want %d: every existing workspace repaints this tag", tag, got, want)
		}
	}
}

func TestTagChipSlotIsDeterministicAcrossCalls(t *testing.T) {
	for _, tag := range []string{"ux", "billing", "type:authentication", "über-tag"} {
		first := tagChipSlot(tag)
		for range 8 {
			if got := tagChipSlot(tag); got != first {
				t.Fatalf("tag %q resolved to slot %d then %d", tag, first, got)
			}
		}
		if got := TagChipStyle(tag).GetForeground(); got != tagChipColors[first] {
			t.Errorf("tag %q chip foreground = %v, want slot %d color %v", tag, got, first, tagChipColors[first])
		}
	}
}

func TestTagChipSlotCollapsesSpellingsOfOneTag(t *testing.T) {
	want := tagChipSlot("bug")
	for _, spelling := range []string{"Bug", "BUG", "#bug", "#Bug", "  bug  "} {
		if got := tagChipSlot(spelling); got != want {
			t.Errorf("%q resolved to slot %d, want %d — normalization decides the slot", spelling, got, want)
		}
	}
}

func TestTagChipStyleCarriesColorOnly(t *testing.T) {
	style := TagChipStyle("ux")
	if style.GetBold() {
		t.Error("a tag chip is bold; bold stays the title's selection channel")
	}
	if unset := lipgloss.NewStyle().GetBackground(); style.GetBackground() != unset {
		t.Errorf("a tag chip carries background %v; chips are foreground only", style.GetBackground())
	}
}

func TestTagChipColorsReResolveAfterApplyTheme(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	const probe = "ux"
	resolved := map[string]lipgloss.Style{}
	slots := map[string]int{}

	for _, variant := range []string{"dark", "light"} {
		ApplyTheme(ThemeConfig{Name: variant})

		want := config.DefaultTagChipRamp(variant)
		if len(tagChipColors) != len(want) {
			t.Fatalf("%s ramp = %d slots, want %d", variant, len(tagChipColors), len(want))
		}
		for i, hex := range want {
			if tagChipColors[i] != lipgloss.Color(hex) {
				t.Errorf("%s slot %d = %v, want %s", variant, i, tagChipColors[i], hex)
			}
		}
		slots[variant] = tagChipSlot(probe)
		resolved[variant] = TagChipStyle(probe)
		if got, want := resolved[variant].GetForeground(), tagChipColors[slots[variant]]; got != want {
			t.Errorf("%s: %q chip = %v, want its slot color %v", variant, probe, got, want)
		}
	}

	// Slot identity survives T; only the hex behind the slot changes.
	if slots["dark"] != slots["light"] {
		t.Errorf("%q moved from slot %d to slot %d across themes", probe, slots["dark"], slots["light"])
	}
	if resolved["dark"].GetForeground() == resolved["light"].GetForeground() {
		t.Errorf("%q kept the dark color %v under the light theme", probe, resolved["dark"].GetForeground())
	}
}
