package tui

import (
	"slices"
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

func TestTaskTagListSourceKeepsAPromotedContextInItsPromotedSection(t *testing.T) {
	source := taskTagListSource([]TagInfo{
		{Name: "bug", TaskCount: 1, ContextCount: 1},
		{Name: "review", TaskCount: 1, ContextCount: 1},
	}, false)

	assertRowIDs(t, taskTagRowIDs(source.Items, taskTagWorkflowCategory), []string{"tag:review"})
	assertRowIDs(t, taskTagRowIDs(source.Items, taskTagSemanticCategory), []string{"tag:bug"})
	if rows := taskTagRowIDs(source.Items, taskContextCategory); len(rows) != 0 {
		t.Fatalf("context rows = %v, want none", rows)
	}
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
		{"w", "tag:review"},
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
	if status, ok := msg.(StatusMsg); !ok || string(status) != "No SEMANTIC rows" {
		t.Fatalf("refused jump message = %#v, want the empty-section status", msg)
	}
	if item, ok := picker.surface.CurrentItem(); !ok || item.ID != "tag:review" {
		t.Fatalf("a refused jump moved the cursor to %v", item.ID)
	}
}

func TestOperationTagPickerJumpKeysStayOutOfNonTagPickers(t *testing.T) {
	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{ID: operationTaskSetPriority},
		ValueProviderPriority,
		"Set Priority",
		"Task",
		ListSourceFromPriorities([]PriorityInfo{
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
		if !strings.Contains(styled, HelpKeyStyle.Render(key)) {
			t.Fatalf("hint key %q does not wear the guidance key color: %q", key, styled)
		}
	}
	seeded := taskTagListSource([]TagInfo{{Name: "backend", TaskCount: 1}}, true)
	hint = ansi.Strip(taskTagCategoryJumpHint(ValueProviderTags, seeded.Items, 120))
	if got := strings.Join(strings.Fields(hint), " "); got != "jump w workflow · s semantic · t tags" {
		t.Fatalf("seeded hint = %q, want the editor's three sections", got)
	}
	for _, provider := range []ValueProviderID{ValueProviderProjectTags, ValueProviderFilterProjectTags} {
		if got := taskTagCategoryJumpHint(provider, source.Items, 120); got != "" {
			t.Fatalf("provider %q hint = %q, want none", provider, got)
		}
	}
}

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
				{Name: reviewTaskTag, TaskCount: 1},
			}, true),
			rows: map[ListCategoryID]int{
				taskTagWorkflowCategory: len(promotedTaskTags),
				taskTagSemanticCategory: len(models.SemanticTaskTags),
				taskContextCategory:     1,
				tagDescriptiveCategory:  1,
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
// color of the strongest thing the row does. A workflow row paints an All lane,
// a semantic row paints a task title, a behavioral row paints its own badge, and
// a row that changes no other color falls back to the chip its tag already wears
// on the task row. A section with no case here answers false rather than a
// fallback color, which is what stops a new one from shipping uncolored.
func tagPickerRowColor(t *testing.T, item ListItem) (lipgloss.Style, bool) {
	t.Helper()
	switch item.CategoryID {
	case taskTagWorkflowCategory:
		return PurpleStyle, true
	case taskTagSemanticCategory:
		task := &models.Task{Tags: []string{item.PrimaryText}}
		return TaskTitleStyleFor(task, TaskTitleStyleOptions{}), true
	case projectTagBehavioralCategory:
		info, ok := models.BehavioralProjectTags[item.PrimaryText]
		if !ok {
			t.Fatalf("behavioral row %q names no registry entry", item.ID)
		}
		return behavioralProjectTagBadgeStyle(info.BadgeRole), true
	case taskContextCategory, tagDescriptiveCategory:
		// Both hold tag rows: CONTEXTS merely recategorizes a tag also carried as
		// a context, and that row still toggles the tag.
		return tagChipStyle(item.PrimaryText), true
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
