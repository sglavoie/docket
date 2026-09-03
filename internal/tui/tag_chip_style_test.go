package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestRowAndPeekTagChipsWearOneColor(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	// infra and ux hold different slots, so a row that styled the whole join
	// under one color could not pass this. Which slot each holds is theme's to
	// know — the fixture proves they differ by rendering the same text through
	// both chips, which is all a caller outside theme may ask.
	task := &models.Task{ID: "task", Tags: []string{"infra", "ux"}}
	if theme.TagChipStyle("infra").Render("x") == theme.TagChipStyle("ux").Render("x") {
		t.Fatalf("fixture tags share one chip color; pick two that differ")
	}
	row := renderTruncatedTags(task.Tags, 2, 8)
	peek := inlinePeekTagsLine(task, 80)

	for _, tag := range task.Tags {
		chip := theme.TagChipStyle(tag).Render("#" + tag)
		if !strings.Contains(row, chip) {
			t.Errorf("row %q is missing the %q chip %q", row, tag, chip)
		}
		if !strings.Contains(peek, chip) {
			t.Errorf("Peek %q is missing the %q chip %q", peek, tag, chip)
		}
	}
}

func TestRenderTruncatedTagsClipsTheTextButNotTheColor(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

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
		if want := theme.TagChipStyle(tag).Render(tc.want); rendered != want {
			t.Errorf("chip at %d columns = %q, want the whole tag's color %q", tc.maxTagLen, rendered, want)
		}
	}
}

func TestRenderTruncatedTagsKeepsTheOverflowCountMuted(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	rendered := renderTruncatedTags([]string{"billing", "ux", "refactor"}, 2, 8)
	if want := theme.MutedStyle.Render(" +1"); !strings.HasSuffix(rendered, want) {
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
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

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
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

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
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

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
