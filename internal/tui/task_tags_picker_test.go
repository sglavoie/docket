package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func taskTagRowIDs(items []ListItem, category ListCategoryID) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.CategoryID == category {
			ids = append(ids, string(item.ID))
		}
	}
	return ids
}

func assertRowIDs(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rows = %v, want %v", got, want)
		}
	}
}

func TestTaskTagListSourceSeedsEverySemanticTagInPrecedenceOrder(t *testing.T) {
	source := taskTagListSource([]TagInfo{{Name: "urgent", TaskCount: 3}}, true)

	assertRowIDs(t, taskTagRowIDs(source.Items, taskTagSemanticCategory), []string{
		"tag:bug", "tag:epic", "tag:story", "tag:feature", "tag:idea",
	})
	assertRowIDs(t, taskTagRowIDs(source.Items, tagDescriptiveCategory), []string{"tag:urgent"})

	for _, item := range source.Items {
		if item.CategoryID != taskTagSemanticCategory {
			continue
		}
		entry, ok := models.SemanticTaskTagByName(item.PrimaryText)
		if !ok {
			t.Fatalf("semantic row %q is not in the registry", item.PrimaryText)
		}
		if item.SecondaryText != entry.RowSummary {
			t.Fatalf("row %q secondary text = %q, want %q", item.PrimaryText, item.SecondaryText, entry.RowSummary)
		}
		if len(item.Preview.Fields) == 0 || item.Preview.Fields[0].Value != entry.Summary {
			t.Fatalf("row %q preview does not retain full summary %q", item.PrimaryText, entry.Summary)
		}
		if name, ok := item.Data.(string); !ok || name != item.PrimaryText {
			t.Fatalf("row %q data = %#v, want the tag name", item.PrimaryText, item.Data)
		}
	}
}

func TestTaskTagListSourceWithoutSeedingKeepsOnlyTagsInUse(t *testing.T) {
	source := taskTagListSource([]TagInfo{
		{Name: "urgent", TaskCount: 2},
		{Name: "idea", TaskCount: 1},
	}, false)

	assertRowIDs(t, taskTagRowIDs(source.Items, taskTagSemanticCategory), []string{"tag:idea"})
	assertRowIDs(t, taskTagRowIDs(source.Items, tagDescriptiveCategory), []string{"tag:urgent"})
}

func TestTaskTagListSourceHasNoSemanticRowsWhenNoneAreInUse(t *testing.T) {
	source := taskTagListSource([]TagInfo{
		{Name: "backend", TaskCount: 4},
		{Name: "type:bug", TaskCount: 1},
		{Name: "bugfix", TaskCount: 1},
	}, false)

	if rows := taskTagRowIDs(source.Items, taskTagSemanticCategory); len(rows) != 0 {
		t.Fatalf("semantic rows = %v, want none", rows)
	}
	assertRowIDs(t, taskTagRowIDs(source.Items, tagDescriptiveCategory), []string{
		"tag:backend", "tag:bugfix", "tag:type:bug",
	})
}

func TestTaskTagListSourcePromotesCasedVariantsPreservingSpelling(t *testing.T) {
	source := taskTagListSource([]TagInfo{
		{Name: "Bug", TaskCount: 2},
		{Name: "bug", TaskCount: 5},
		{Name: "IDEA", TaskCount: 1},
		{Name: "backend", TaskCount: 3},
	}, false)

	assertRowIDs(t, taskTagRowIDs(source.Items, taskTagSemanticCategory), []string{
		"tag:Bug", "tag:bug", "tag:IDEA",
	})
	assertRowIDs(t, taskTagRowIDs(source.Items, tagDescriptiveCategory), []string{"tag:backend"})

	for _, item := range source.Items {
		if item.CategoryID != taskTagSemanticCategory {
			continue
		}
		if name, ok := item.Data.(string); !ok || name != item.PrimaryText {
			t.Fatalf("row %q data = %#v, want the stored spelling", item.PrimaryText, item.Data)
		}
	}
}

func TestTaskTagListSourceMintsNoDuplicateIDsWhenSeeding(t *testing.T) {
	source := taskTagListSource([]TagInfo{
		{Name: "bug", TaskCount: 2},
		{Name: "Idea", TaskCount: 1},
		{Name: "backend", TaskCount: 3},
	}, true)

	seen := make(map[ListItemID]bool, len(source.Items))
	for _, item := range source.Items {
		if seen[item.ID] {
			t.Fatalf("duplicate item ID %q", item.ID)
		}
		seen[item.ID] = true
	}
	// Idea is in use under another spelling, so idea is still seeded.
	assertRowIDs(t, taskTagRowIDs(source.Items, taskTagSemanticCategory), []string{
		"tag:bug", "tag:epic", "tag:story", "tag:feature", "tag:Idea", "tag:idea",
	})
}

func TestTaskTagListSourceKeepsDescriptiveRowsAlphabeticalWithCounts(t *testing.T) {
	source := taskTagListSource([]TagInfo{
		{Name: "zeta", TaskCount: 1},
		{Name: "alpha", TaskCount: 7},
		{Name: "middle", TaskCount: 2},
	}, true)

	assertRowIDs(t, taskTagRowIDs(source.Items, tagDescriptiveCategory), []string{
		"tag:alpha", "tag:middle", "tag:zeta",
	})
	want := map[string]string{"alpha": "7 tasks", "middle": "2 tasks", "zeta": "1 tasks"}
	for _, item := range source.Items {
		if item.CategoryID != tagDescriptiveCategory {
			continue
		}
		if item.SecondaryText != want[item.PrimaryText] {
			t.Fatalf("row %q secondary text = %q, want %q", item.PrimaryText, item.SecondaryText, want[item.PrimaryText])
		}
		if name, ok := item.Data.(string); !ok || name != item.PrimaryText {
			t.Fatalf("row %q data = %#v, want the tag name", item.PrimaryText, item.Data)
		}
	}
}

func TestSemanticTaskTagExplainerNamesThePrecedenceForTaskTagProvidersOnly(t *testing.T) {
	for _, provider := range []ValueProviderID{ValueProviderTags, ValueProviderFilterTags} {
		explainer := semanticTaskTagExplainer(provider, 120)
		plain := ansi.Strip(explainer)
		if !strings.Contains(plain, "semantic  bug › epic › story › feature › idea —") {
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
	want := "semantic bug › epic › story › feature › idea " + semanticTaskTagExplainerSummary
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
	rendered := picker.renderWithin(minTerminalWidth, minTerminalHeight-2)
	if got := lipgloss.Height(rendered); got > minTerminalHeight-2 {
		t.Fatalf("picker height = %d, want <= %d", got, minTerminalHeight-2)
	}
	plain := ansi.Strip(rendered)
	want := "semantic bug › epic › story › feature › idea " + semanticTaskTagExplainerSummary
	if got := strings.Join(strings.Fields(plain), " "); !strings.Contains(got, want) {
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

func TestOperationTagPickerKeepsThePrecedenceOrderAfterAddingATag(t *testing.T) {
	picker := newSemanticTagPickerForTest(t, []TagInfo{{Name: "zeta", TaskCount: 1}}, true)
	if err := picker.addOrSelectTag("alpha"); err != nil {
		t.Fatalf("addOrSelectTag descriptive: %v", err)
	}
	items := picker.surface.Items()
	assertRowIDs(t, taskTagRowIDs(items, taskTagSemanticCategory), []string{
		"tag:bug", "tag:epic", "tag:story", "tag:feature", "tag:idea",
	})
	assertRowIDs(t, taskTagRowIDs(items, tagDescriptiveCategory), []string{"tag:alpha", "tag:zeta"})

	if err := picker.addOrSelectTag("Bug"); err != nil {
		t.Fatalf("addOrSelectTag semantic: %v", err)
	}
	items = picker.surface.Items()
	assertRowIDs(t, taskTagRowIDs(items, taskTagSemanticCategory), []string{
		"tag:bug", "tag:Bug", "tag:epic", "tag:story", "tag:feature", "tag:idea",
	})
	// Every semantic row must precede every descriptive one or the list surface
	// repeats both headers.
	for index, item := range items {
		if item.CategoryID == taskTagSemanticCategory && index >= len(items)-2 {
			t.Fatalf("semantic row %q sits inside the descriptive section: %+v", item.ID, items)
		}
	}
}
