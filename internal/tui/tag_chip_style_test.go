package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// tagChipGoldenSlots pins the hash and the ramp length — the two inputs that
// decide where a canonical tag lands. A tag's color is a promise kept across
// machines and releases, so a change here is a deliberate repaint of every
// workspace, never a silent reshuffle: swapping the hash or resizing the ramp
// fails this table by name. The normalizer feeding it is pinned separately, by
// TestTagChipSlotCollapsesSpellingsOfOneTag.
var tagChipGoldenSlots = map[string]int{
	"bug":      1,
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
		if got := tagChipStyle(tag).GetForeground(); got != tagChipColors[first] {
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
	style := tagChipStyle("ux")
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
		resolved[variant] = tagChipStyle(probe)
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

func TestRowAndPeekTagChipsWearOneColor(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	// infra and ux hold different slots, so a row that styled the whole join
	// under one color could not pass this.
	task := &models.Task{ID: "task", Tags: []string{"infra", "ux"}}
	if tagChipSlot("infra") == tagChipSlot("ux") {
		t.Fatalf("fixture tags share slot %d; pick two that differ", tagChipSlot("infra"))
	}
	row := renderTruncatedTags(task.Tags, 2, 8)
	peek := inlinePeekTagsLine(task, 80)

	for _, tag := range task.Tags {
		chip := tagChipStyle(tag).Render("#" + tag)
		if !strings.Contains(row, chip) {
			t.Errorf("row %q is missing the %q chip %q", row, tag, chip)
		}
		if !strings.Contains(peek, chip) {
			t.Errorf("Peek %q is missing the %q chip %q", peek, tag, chip)
		}
	}
}

func TestRenderTruncatedTagsClipsTheTextButNotTheColor(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	// The full stored tag decides the color and the clip is presentation only, so
	// a chip cut to the row's eight columns wears the color its untruncated Peek
	// counterpart shows. A wider budget renders the same tag whole in that color.
	const tag = "observability"
	for _, tc := range []struct {
		maxTagLen int
		want      string
	}{
		{8, "#observa…"},
		{20, "#observability"},
	} {
		rendered := renderTruncatedTags([]string{tag}, 2, tc.maxTagLen)
		if got := ansi.Strip(rendered); got != tc.want {
			t.Errorf("chip at %d columns = %q, want %q", tc.maxTagLen, got, tc.want)
		}
		if want := tagChipStyle(tag).Render(tc.want); rendered != want {
			t.Errorf("chip at %d columns = %q, want the whole tag's color %q", tc.maxTagLen, rendered, want)
		}
	}
}

func TestRenderTruncatedTagsKeepsTheOverflowCountMuted(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	rendered := renderTruncatedTags([]string{"billing", "ux", "refactor"}, 2, 8)
	if want := MutedStyle.Render(" +1"); !strings.HasSuffix(rendered, want) {
		t.Errorf("overflow suffix of %q is not the muted %q", rendered, want)
	}
}

func TestOrderedTaskTagsSortsNonSemanticFirstThenSemantic(t *testing.T) {
	cases := []struct {
		name string
		tags []string
		want []string
	}{
		{
			name: "semantic tags fall behind every descriptive tag",
			tags: []string{"bug", "ux", "billing"},
			want: []string{"billing", "ux", "bug"},
		},
		{
			name: "each group sorts alphabetically within itself",
			tags: []string{"story", "ux", "bug", "infra", "idea", "billing"},
			want: []string{"billing", "infra", "ux", "bug", "idea", "story"},
		},
		{
			name: "a task carrying only semantic tags keeps all of them",
			tags: []string{"idea", "bug"},
			want: []string{"bug", "idea"},
		},
		{
			name: "the normalized spelling decides the position",
			tags: []string{"Zebra", "type:authentication", "BUG", "alpha"},
			want: []string{"alpha", "type:authentication", "Zebra", "BUG"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := orderedTaskTags(tc.tags); !slices.Equal(got, tc.want) {
				t.Fatalf("orderedTaskTags(%v) = %v, want %v", tc.tags, got, tc.want)
			}
		})
	}
}

func TestOrderedTaskTagsIsIndependentOfInsertionOrder(t *testing.T) {
	// AddTag appends, so these are the same tag set typed in three orders. The
	// point of the helper is that the set, not the typing history, decides.
	stored := []string{"ux", "bug", "billing", "infra"}
	want := orderedTaskTags(stored)

	for _, permutation := range [][]string{
		{"bug", "billing", "infra", "ux"},
		{"infra", "ux", "bug", "billing"},
		{"billing", "infra", "ux", "bug"},
	} {
		if got := orderedTaskTags(permutation); !slices.Equal(got, want) {
			t.Errorf("orderedTaskTags(%v) = %v, want %v", permutation, got, want)
		}
	}

	// A permutation, not a filter: the caller's overflow count depends on it.
	if got := orderedTaskTags(stored); len(got) != len(stored) {
		t.Errorf("orderedTaskTags returned %d tags for %d stored", len(got), len(stored))
	}
	if !slices.Equal(stored, []string{"ux", "bug", "billing", "infra"}) {
		t.Errorf("orderedTaskTags mutated its argument: %v", stored)
	}
}

func TestOrderedTaskTagsDeprioritizesSemanticTagsWithoutDroppingThem(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	// Deprioritize, never exclude: with nothing else competing, the semantic tag
	// still reaches the row. Under --no-color the title carries no semantic color
	// at all, and no renderer can detect that, so the chip is the only signal.
	if got := ansi.Strip(renderTruncatedTags([]string{"bug"}, 2, 8)); got != "#bug" {
		t.Errorf("a bug-only task rendered %q, want #bug", got)
	}

	// With descriptive tags competing, the two row slots go to what the title
	// color cannot say, and the dropped semantic tag is still counted.
	rendered := renderTruncatedTags([]string{"bug", "ux", "billing"}, 2, 8)
	if got, want := ansi.Strip(rendered), "#billing #ux +1"; got != want {
		t.Errorf("row tags = %q, want %q", got, want)
	}
}

func TestRenderTruncatedTagsOverflowCountSurvivesReordering(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	// Reordering moves which tags are visible, never how many exist: every
	// permutation of one set shows the same +N.
	for _, tags := range [][]string{
		{"bug", "story", "ux", "billing", "infra"},
		{"ux", "bug", "infra", "billing", "story"},
		{"infra", "billing", "story", "ux", "bug"},
	} {
		got := ansi.Strip(renderTruncatedTags(tags, 2, 8))
		if want := "#billing #infra +3"; got != want {
			t.Errorf("row tags for %v = %q, want %q", tags, got, want)
		}
	}

	// The remainder is arithmetic over the budget rather than a constant, so the
	// budget the task row happens to use is not the only one that has to add up.
	// A set the budget exactly fits is the case an off-by-one would show as a
	// stray "+0".
	tags := []string{"bug", "story", "ux", "billing", "infra"}
	for _, tc := range []struct {
		maxTags int
		want    string
	}{
		{1, "#billing +4"},
		{2, "#billing #infra +3"},
		{5, "#billing #infra #ux #bug #story"},
	} {
		if got := ansi.Strip(renderTruncatedTags(tags, tc.maxTags, 8)); got != tc.want {
			t.Errorf("row tags at a budget of %d = %q, want %q", tc.maxTags, got, tc.want)
		}
	}
}

func TestRowAndPeekAgreeOnTagOrder(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	// One helper serves both surfaces, so the row's two chips must be the Peek
	// line's first two — a row is a prefix of Peek, never a different selection.
	task := &models.Task{ID: "task", Tags: []string{"bug", "ux", "billing"}}
	row := ansi.Strip(renderTruncatedTags(task.Tags, 2, 8))
	peek := ansi.Strip(inlinePeekTagsLine(task, 120))

	if want := "Tags: #billing #ux #bug"; peek != want {
		t.Fatalf("Peek tags = %q, want %q", peek, want)
	}
	rowChips, _, _ := strings.Cut(row, " +")
	if !strings.HasPrefix(strings.TrimPrefix(peek, "Tags: "), rowChips) {
		t.Errorf("row chips %q are not the leading chips of Peek %q", rowChips, peek)
	}
}
