package outliner

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TestPeekTagChipsWearOneColorPerTag is the chip color rule with one surface
// left to hold it to. The row carries no tags now, so Peek is where
// theme.TagChipStyle has to keep answering per tag rather than per line.
func TestPeekTagChipsWearOneColorPerTag(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	// infra and ux hold different slots, so a line styled whole under one color
	// could not pass this. Which slot each holds is theme's to know — the fixture
	// proves they differ by rendering the same text through both chips, which is
	// all a caller outside theme may ask.
	task := &models.Task{ID: "task", Tags: []string{"infra", "ux"}}
	if theme.TagChipStyle("infra").Render("x") == theme.TagChipStyle("ux").Render("x") {
		t.Fatalf("fixture tags share one chip color; pick two that differ")
	}
	peek := strings.Join(inlinePeekTagsLines(task, "", 80), "\n")

	for _, tag := range task.Tags {
		chip := theme.TagChipStyle(tag).Render("#" + tag)
		if !strings.Contains(peek, chip) {
			t.Errorf("Peek %q is missing the %q chip %q", peek, tag, chip)
		}
	}
}

// TestPeekRendersTheWholeStoredTag is the counterpart to the row's old clip. A
// tag was cut to eight columns on a row that had no room for it; Peek has a
// wrapped line, so the stored spelling arrives whole in its own color.
func TestPeekRendersTheWholeStoredTag(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	const tag = "observability"
	task := &models.Task{ID: "task", Tags: []string{tag}}
	rendered := strings.Join(inlinePeekTagsLines(task, "", 80), "\n")
	if got := ansi.Strip(rendered); got != "Tags: #"+tag {
		t.Errorf("Peek tags = %q, want the whole stored spelling", got)
	}
	if !strings.Contains(rendered, theme.TagChipStyle(tag).Render("#"+tag)) {
		t.Errorf("Peek %q dropped the tag's own color", rendered)
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
			if got := listsurface.OrderedTaskTags(tc.tags); !slices.Equal(got, tc.want) {
				t.Fatalf("OrderedTaskTags(%v) = %v, want %v", tc.tags, got, tc.want)
			}
		})
	}
}

func TestOrderedTaskTagsIsIndependentOfInsertionOrder(t *testing.T) {
	// AddTag appends, so these are the same tag set typed in three orders. The
	// point of the helper is that the set, not the typing history, decides.
	stored := []string{"ux", "bug", "billing", "infra"}
	want := listsurface.OrderedTaskTags(stored)

	for _, permutation := range [][]string{
		{"bug", "billing", "infra", "ux"},
		{"infra", "ux", "bug", "billing"},
		{"billing", "infra", "ux", "bug"},
	} {
		if got := listsurface.OrderedTaskTags(permutation); !slices.Equal(got, want) {
			t.Errorf("OrderedTaskTags(%v) = %v, want %v", permutation, got, want)
		}
	}

	// A permutation, not a filter: the caller's overflow count depends on it.
	if got := listsurface.OrderedTaskTags(stored); len(got) != len(stored) {
		t.Errorf("OrderedTaskTags returned %d tags for %d stored", len(got), len(stored))
	}
	if !slices.Equal(stored, []string{"ux", "bug", "billing", "infra"}) {
		t.Errorf("OrderedTaskTags mutated its argument: %v", stored)
	}
}

func TestOrderedTaskTagsDeprioritizesSemanticTagsWithoutDroppingThem(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	// Deprioritize, never exclude. Peek is the only place a semantic tag is said
	// at all — the title is neutral text and the row carries no chip — so a tag
	// pushed to the back of the order must still arrive.
	task := &models.Task{ID: "task", Tags: []string{"bug", "ux", "billing"}}
	got := ansi.Strip(strings.Join(inlinePeekTagsLines(task, "", 120), "\n"))
	if want := "Tags: #billing #ux #bug"; got != want {
		t.Errorf("Peek tags = %q, want %q", got, want)
	}
}

// TestPeekTagOrderIsIndependentOfInsertionOrder is the stored-set rule at the
// one surface that renders it: reordering the stored tags never changes what a
// reader sees, because OrderedTaskTags decides and nothing is dropped.
func TestPeekTagOrderIsIndependentOfInsertionOrder(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: ""})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	for _, tags := range [][]string{
		{"bug", "story", "ux", "billing", "infra"},
		{"ux", "bug", "infra", "billing", "story"},
		{"infra", "billing", "story", "ux", "bug"},
	} {
		task := &models.Task{ID: "task", Tags: tags}
		got := ansi.Strip(strings.Join(inlinePeekTagsLines(task, "", 120), "\n"))
		if want := "Tags: #billing #infra #ux #bug #story"; got != want {
			t.Errorf("Peek tags for %v = %q, want %q", tags, got, want)
		}
	}
}
