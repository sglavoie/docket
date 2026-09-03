package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func newFilterTagPickerForTest(t *testing.T, tags []TagInfo) *OperationValuePickerModel {
	t.Helper()
	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{},
		ValueProviderFilterTags,
		"Filter by Tags",
		"keep matching tasks",
		taskTagListSource(tags, false),
		true,
		nil,
	)
	if err != nil {
		t.Fatalf("newOperationValuePickerModel: %v", err)
	}
	return picker
}

func TestSemanticTaskTagExplainerNamesThePrecedenceForTaskTagProvidersOnly(t *testing.T) {
	for _, provider := range []ValueProviderID{ValueProviderTags, ValueProviderFilterTags} {
		explainer := semanticTaskTagExplainer(provider, 120)
		plain := ansi.Strip(explainer)
		if !strings.Contains(plain, "semantic  bug › question › epic › story › feature › idea —") {
			t.Fatalf("provider %q explainer = %q, want the precedence chain", provider, plain)
		}
		if !strings.HasSuffix(plain, "colors its title.") {
			t.Fatalf("provider %q explainer = %q, want the full summary", provider, plain)
		}
	}
	for _, provider := range []ValueProviderID{
		ValueProviderProjectTags,
		ValueProviderFilterProjectTags,
		ValueProviderPriority,
		ValueProviderState,
	} {
		if got := semanticTaskTagExplainer(provider, 120); got != "" {
			t.Fatalf("provider %q explainer = %q, want none", provider, got)
		}
	}
}

func TestSemanticTaskTagExplainerWrapsCompleteTextAtNarrowWidths(t *testing.T) {
	want := "semantic bug › question › epic › story › feature › idea " + semanticTaskTagExplainerSummary
	for _, width := range []int{12, 30, 42, 44, 60} {
		plain := ansi.Strip(semanticTaskTagExplainer(ValueProviderTags, width))
		for _, line := range strings.Split(plain, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %q is %d columns wide", width, line, got)
			}
		}
		if got := strings.Join(strings.Fields(plain), " "); got != want {
			t.Fatalf("width %d wrapped explainer = %q, want %q", width, got, want)
		}
	}
}

func TestSemanticTagPickerReservesHeightForWrappedExplainer(t *testing.T) {
	picker := newSemanticTagPickerForTest(t, []TagInfo{{Name: "urgent", TaskCount: 1}}, true)
	rendered := picker.renderWithin(minTerminalWidth, minTerminalHeight)
	if got := lipgloss.Height(rendered); got > minTerminalHeight {
		t.Fatalf("picker height = %d, want <= %d", got, minTerminalHeight)
	}
	plain := ansi.Strip(rendered)
	// The explainer needs three lines at this width, so the border columns sit
	// between its own words: drop them before checking the text survived.
	unboxed := strings.ReplaceAll(plain, "│", " ")
	if got := strings.Join(strings.Fields(unboxed), " "); !strings.Contains(got, "semantic bug › question › epic › story › feature › idea") ||
		!strings.Contains(got, "highest one a task carries") ||
		!strings.Contains(got, "colors its title.") {
		t.Fatalf("picker dropped wrapped explainer:\n%s", plain)
	}
}

func newSemanticTagPickerForTest(t *testing.T, tags []TagInfo, seedUnused bool) *OperationValuePickerModel {
	t.Helper()
	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{ID: operationTaskSetTags},
		ValueProviderTags,
		"Set Tags",
		"Task",
		taskTagListSource(tags, seedUnused),
		true,
		nil,
	)
	if err != nil {
		t.Fatalf("newOperationValuePickerModel: %v", err)
	}
	return picker
}

func TestOperationTagPickerRoutesAMintedSemanticTagIntoTheSemanticSection(t *testing.T) {
	picker := newSemanticTagPickerForTest(t, []TagInfo{{Name: "alpha", TaskCount: 1}}, false)
	if err := picker.addOrSelectTag("Bug"); err != nil {
		t.Fatalf("addOrSelectTag: %v", err)
	}

	items := picker.surface.Items()
	assertRowIDs(t, taskTagRowIDs(items, taskTagSemanticCategory), []string{"tag:Bug"})
	assertRowIDs(t, taskTagRowIDs(items, tagDescriptiveCategory), []string{"tag:alpha"})
	entry, ok := models.SemanticTaskTagByName("Bug")
	if !ok {
		t.Fatal("bug is not in the semantic registry")
	}
	for _, item := range items {
		if item.ID != "tag:Bug" {
			continue
		}
		if item.SecondaryText != entry.RowSummary {
			t.Fatalf("minted row secondary text = %q, want %q", item.SecondaryText, entry.RowSummary)
		}
		if name, ok := item.Data.(string); !ok || name != "Bug" {
			t.Fatalf("minted row data = %#v, want the typed spelling", item.Data)
		}
	}
	if !picker.surface.IsSelected("tag:Bug") {
		t.Fatal("minted semantic tag is not selected")
	}
}

// TestSemanticTagPickerRowsWearTheTitleColorTheyApply walks every section of
// both tag pickers, not just the semantic one the name grew out of: a semantic
// row wearing its title color is one case of the general rule that a row wears
// the color of what it does. The table is keyed by section rather than by row,
// so a registry entry added to an existing section is covered the moment it is
// declared, and a new section fails until tagPickerRowColor is taught what it
// applies.
func TestSemanticTagPickerRowsWearTheTitleColorTheyApply(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source ListSource
		rows   map[ListCategoryID]int
	}{
		{
			name: "task tags picker",
			source: taskTagListSource([]TagInfo{
				{Name: "urgent", TaskCount: 3},
				{Name: "platform", TaskCount: 1, ContextCount: 1},
				{Name: "triage", TaskCount: 1},
			}, true),
			rows: map[ListCategoryID]int{
				taskTagSemanticCategory: len(models.SemanticTaskTags),
				taskContextCategory:     1,
				tagDescriptiveCategory:  2,
			},
		},
		{
			name: "project tags picker",
			source: (&AppModel{}).projectTagListSource(&models.Project{
				Tags: []string{"infra", "ux"},
			}),
			rows: map[ListCategoryID]int{
				projectTagBehavioralCategory: len(models.BehavioralProjectTags),
				tagDescriptiveCategory:       2,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen := make(map[ListCategoryID]int, len(tc.source.Categories))
			for _, item := range tc.source.Items {
				assertRowWearsTheColorItApplies(t, item)
				seen[item.CategoryID]++
			}
			// Every declared section has to hold the rows the fixture seeds it
			// with, or the walk above would pass by silently skipping the section
			// a future registry entry lands in.
			for _, category := range tc.source.Categories {
				if got, want := seen[category.ID], tc.rows[category.ID]; got != want {
					t.Fatalf("section %q has %d rows, want %d", category.ID, got, want)
				}
			}
			if len(seen) != len(tc.source.Categories) {
				t.Fatalf(
					"rows landed in %d sections, want the %d the source declares",
					len(seen), len(tc.source.Categories),
				)
			}
		})
	}

	// A typed tag builds its row on a different path than the source does, so
	// both minting outcomes are walked too: semantic routes into SEMANTIC and
	// everything else into TAGS.
	t.Run("minted rows", func(t *testing.T) {
		picker := newSemanticTagPickerForTest(t, []TagInfo{{Name: "alpha", TaskCount: 1}}, false)
		minted := map[ListItemID]bool{"tag:bug": true, "tag:brandnew": true}
		for _, tag := range []string{"bug", "brandnew"} {
			if err := picker.addOrSelectTag(tag); err != nil {
				t.Fatalf("addOrSelectTag(%q): %v", tag, err)
			}
		}
		for _, item := range picker.surface.Items() {
			assertRowWearsTheColorItApplies(t, item)
			delete(minted, item.ID)
		}
		if len(minted) != 0 {
			t.Fatalf("minted rows never reached the picker: %v", minted)
		}
	})
}

// tagPickerRowColor is the one rule for what color a tag picker row wears: the
// color of the strongest thing the row does. A semantic row paints a task title,
// a behavioral row paints its own badge, and a row that changes no other color
// falls back to the chip its tag already wears on the task row. A section with
// no case here answers false rather than a fallback color, which is what stops a
// new one from shipping uncolored.
func tagPickerRowColor(t *testing.T, item ListItem) (lipgloss.Style, bool) {
	t.Helper()
	switch item.CategoryID {
	case taskTagSemanticCategory:
		task := &models.Task{Tags: []string{item.PrimaryText}}
		return theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{}), true
	case projectTagBehavioralCategory:
		info, ok := models.BehavioralProjectTags[item.PrimaryText]
		if !ok {
			t.Fatalf("behavioral row %q names no registry entry", item.ID)
		}
		return behavioralProjectTagBadgeStyle(info.BadgeRole), true
	case taskContextCategory, tagDescriptiveCategory:
		// Both hold tag rows: CONTEXTS merely recategorizes a tag also carried as
		// a context, and that row still toggles the tag.
		return theme.TagChipStyle(item.PrimaryText), true
	default:
		return lipgloss.Style{}, false
	}
}

// assertRowWearsTheColorItApplies ties the row to the style its effect resolves
// through rather than to a hex constant, so the assertion means "the color this
// row will appear" and a palette edit can never silently split the two.
func assertRowWearsTheColorItApplies(t *testing.T, item ListItem) {
	t.Helper()
	want, ruled := tagPickerRowColor(t, item)
	if !ruled {
		t.Fatalf("row %q sits in section %q, which declares no color", item.ID, item.CategoryID)
	}
	if item.PrimaryStyle == nil {
		t.Fatalf("row %q has no color", item.ID)
	}
	if got := item.PrimaryStyle.GetForeground(); got != want.GetForeground() {
		t.Fatalf("row %q foreground = %v, want %v", item.ID, got, want.GetForeground())
	}
}

func TestOperationTagPickerKeepsThePrecedenceOrderAfterAddingATag(t *testing.T) {
	picker := newSemanticTagPickerForTest(t, []TagInfo{{Name: "zeta", TaskCount: 1}}, true)
	if err := picker.addOrSelectTag("alpha"); err != nil {
		t.Fatalf("addOrSelectTag descriptive: %v", err)
	}
	items := picker.surface.Items()
	assertRowIDs(t, taskTagRowIDs(items, taskTagSemanticCategory), []string{
		"tag:bug", "tag:question", "tag:epic", "tag:story", "tag:feature", "tag:idea",
	})
	assertRowIDs(t, taskTagRowIDs(items, tagDescriptiveCategory), []string{"tag:alpha", "tag:zeta"})

	if err := picker.addOrSelectTag("Bug"); err != nil {
		t.Fatalf("addOrSelectTag semantic: %v", err)
	}
	items = picker.surface.Items()
	assertRowIDs(t, taskTagRowIDs(items, taskTagSemanticCategory), []string{
		"tag:bug", "tag:Bug", "tag:question", "tag:epic", "tag:story", "tag:feature", "tag:idea",
	})
	// Every semantic row must precede every descriptive one or the list surface
	// repeats both headers.
	for index, item := range items {
		if item.CategoryID == taskTagSemanticCategory && index >= len(items)-2 {
			t.Fatalf("semantic row %q sits inside the descriptive section: %+v", item.ID, items)
		}
	}
}
