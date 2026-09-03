package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"
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
		"tag:bug", "tag:question", "tag:epic", "tag:story", "tag:feature", "tag:idea",
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
		"tag:bug", "tag:question", "tag:epic", "tag:story", "tag:feature", "tag:Idea", "tag:idea",
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

func TestTaskTagListSourceGivesContextValuesTheirOwnSection(t *testing.T) {
	source := taskTagListSource([]TagInfo{
		{Name: "backend", TaskCount: 2},
		{Name: "platform", TaskCount: 3, ContextCount: 2},
	}, false)

	assertRowIDs(t, taskTagRowIDs(source.Items, taskContextCategory), []string{"tag:platform"})
	assertRowIDs(t, taskTagRowIDs(source.Items, tagDescriptiveCategory), []string{"tag:backend"})

	for _, item := range source.Items {
		if item.ID != "tag:platform" {
			continue
		}
		if item.SecondaryText != "3 tasks" {
			t.Fatalf("context row secondary text = %q, want the merged count", item.SecondaryText)
		}
		if !slices.Contains(item.Aliases, "##platform") {
			t.Fatalf("context row aliases = %v, want the ## sigil spelling", item.Aliases)
		}
		if !slices.ContainsFunc(item.Preview.Fields, func(field ListPreviewField) bool {
			return field.Label == "Context" && field.Value == "##platform"
		}) {
			t.Fatalf("context row preview = %v, want a Context field", item.Preview.Fields)
		}
	}
}

// A semantic tag also carried as a context stays in SEMANTIC rather than
// migrating to CONTEXTS: the row toggles the tag either way, and the semantic
// section is the one that colors it. "review" is an ordinary descriptive tag
// now, so it lands in TAGS beside every other name the vocabulary does not own.
func TestTaskTagListSourceKeepsAPromotedContextInItsPromotedSection(t *testing.T) {
	source := taskTagListSource([]TagInfo{
		{Name: "bug", TaskCount: 1, ContextCount: 1},
		{Name: "review", TaskCount: 1, ContextCount: 1},
	}, false)

	assertRowIDs(t, taskTagRowIDs(source.Items, taskTagSemanticCategory), []string{"tag:bug"})
	assertRowIDs(t, taskTagRowIDs(source.Items, taskContextCategory), []string{"tag:review"})
}

func TestOperationTagPickerJumpsBetweenSections(t *testing.T) {
	picker := newFilterTagPickerForTest(t, []TagInfo{
		{Name: "backend", TaskCount: 1},
		{Name: "platform", TaskCount: 1, ContextCount: 1},
		{Name: "review", TaskCount: 1},
	})

	// The order is part of the assertion: the refused jump below reads the cursor
	// the last successful jump left behind.
	for _, jump := range []struct {
		key  string
		want ListItemID
	}{
		{"t", "tag:backend"},
		{"c", "tag:platform"},
	} {
		if _, dismissed, msg := picker.handleKey(keyPress(jump.key)); dismissed || msg != nil {
			t.Fatalf("jump %q dismissed = %v, msg = %#v", jump.key, dismissed, msg)
		}
		item, ok := picker.surface.CurrentItem()
		if !ok || item.ID != jump.want {
			t.Fatalf("jump %q landed on %v, want %s", jump.key, item.ID, jump.want)
		}
	}

	_, dismissed, msg := picker.handleKey(keyPress("s"))
	if dismissed {
		t.Fatal("a refused jump dismissed the picker")
	}
	if status, ok := msg.(status.StatusMsg); !ok || string(status) != "No SEMANTIC rows" {
		t.Fatalf("refused jump message = %#v, want the empty-section status", msg)
	}
	if item, ok := picker.surface.CurrentItem(); !ok || item.ID != "tag:platform" {
		t.Fatalf("a refused jump moved the cursor to %v", item.ID)
	}
}

func TestOperationTagPickerJumpKeysStayOutOfNonTagPickers(t *testing.T) {
	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{ID: operationTaskSetPriority},
		ValueProviderPriority,
		"Set Priority",
		"Task",
		ListSourceFromPriorities([]chrome.PriorityInfo{
			{Priority: 0, Description: "Unset"},
			{Priority: 1, Description: "High"},
		}),
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("newOperationValuePickerModel: %v", err)
	}
	before, _ := picker.surface.CurrentItem()
	if _, dismissed, msg := picker.handleKey(keyPress("c")); dismissed || msg != nil {
		t.Fatalf("priority picker consumed a jump key: dismissed = %v, msg = %#v", dismissed, msg)
	}
	if after, _ := picker.surface.CurrentItem(); after.ID != before.ID {
		t.Fatalf("priority picker cursor moved to %v", after.ID)
	}
}

func TestTaskTagCategoryJumpHintNamesOnlyPopulatedSections(t *testing.T) {
	source := taskTagListSource([]TagInfo{
		{Name: "backend", TaskCount: 1},
		{Name: "platform", TaskCount: 1, ContextCount: 1},
	}, false)

	styled := taskTagCategoryJumpHint(ValueProviderFilterTags, source.Items, 120)
	hint := ansi.Strip(styled)
	if got := strings.Join(strings.Fields(hint), " "); got != "jump c contexts · t tags" {
		t.Fatalf("hint = %q, want only the populated sections", got)
	}
	for _, key := range []string{"c", "t"} {
		if !strings.Contains(styled, theme.HelpKeyStyle.Render(key)) {
			t.Fatalf("hint key %q does not wear the guidance key color: %q", key, styled)
		}
	}
	seeded := taskTagListSource([]TagInfo{{Name: "backend", TaskCount: 1}}, true)
	hint = ansi.Strip(taskTagCategoryJumpHint(ValueProviderTags, seeded.Items, 120))
	if got := strings.Join(strings.Fields(hint), " "); got != "jump s semantic · t tags" {
		t.Fatalf("seeded hint = %q, want the editor's two sections", got)
	}
	for _, provider := range []ValueProviderID{ValueProviderProjectTags, ValueProviderFilterProjectTags} {
		if got := taskTagCategoryJumpHint(provider, source.Items, 120); got != "" {
			t.Fatalf("provider %q hint = %q, want none", provider, got)
		}
	}
}
