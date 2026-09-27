package valuepicker

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func taskTagRowIDs(items []listsurface.ListItem, category listsurface.ListCategoryID) []string {
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
	source := TaskTagListSource([]query.TagInfo{{Name: "urgent", TaskCount: 3}}, true)

	assertRowIDs(t, taskTagRowIDs(source.Items, TaskTagSemanticCategory), []string{
		"tag:bug", "tag:question", "tag:epic", "tag:story", "tag:feature", "tag:idea",
	})
	assertRowIDs(t, taskTagRowIDs(source.Items, TagDescriptiveCategory), []string{"tag:urgent"})

	for _, item := range source.Items {
		if item.CategoryID != TaskTagSemanticCategory {
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
	source := TaskTagListSource([]query.TagInfo{
		{Name: "urgent", TaskCount: 2},
		{Name: "idea", TaskCount: 1},
	}, false)

	assertRowIDs(t, taskTagRowIDs(source.Items, TaskTagSemanticCategory), []string{"tag:idea"})
	assertRowIDs(t, taskTagRowIDs(source.Items, TagDescriptiveCategory), []string{"tag:urgent"})
}

func TestTaskTagListSourceHasNoSemanticRowsWhenNoneAreInUse(t *testing.T) {
	source := TaskTagListSource([]query.TagInfo{
		{Name: "backend", TaskCount: 4},
		{Name: "type:bug", TaskCount: 1},
		{Name: "bugfix", TaskCount: 1},
	}, false)

	if rows := taskTagRowIDs(source.Items, TaskTagSemanticCategory); len(rows) != 0 {
		t.Fatalf("semantic rows = %v, want none", rows)
	}
	assertRowIDs(t, taskTagRowIDs(source.Items, TagDescriptiveCategory), []string{
		"tag:backend", "tag:bugfix", "tag:type:bug",
	})
}

func TestTaskTagListSourcePromotesCasedVariantsPreservingSpelling(t *testing.T) {
	source := TaskTagListSource([]query.TagInfo{
		{Name: "Bug", TaskCount: 2},
		{Name: "bug", TaskCount: 5},
		{Name: "IDEA", TaskCount: 1},
		{Name: "backend", TaskCount: 3},
	}, false)

	assertRowIDs(t, taskTagRowIDs(source.Items, TaskTagSemanticCategory), []string{
		"tag:Bug", "tag:bug", "tag:IDEA",
	})
	assertRowIDs(t, taskTagRowIDs(source.Items, TagDescriptiveCategory), []string{"tag:backend"})

	for _, item := range source.Items {
		if item.CategoryID != TaskTagSemanticCategory {
			continue
		}
		if name, ok := item.Data.(string); !ok || name != item.PrimaryText {
			t.Fatalf("row %q data = %#v, want the stored spelling", item.PrimaryText, item.Data)
		}
	}
}

func TestTaskTagListSourceMintsNoDuplicateIDsWhenSeeding(t *testing.T) {
	source := TaskTagListSource([]query.TagInfo{
		{Name: "bug", TaskCount: 2},
		{Name: "Idea", TaskCount: 1},
		{Name: "backend", TaskCount: 3},
	}, true)

	seen := make(map[listsurface.ListItemID]bool, len(source.Items))
	for _, item := range source.Items {
		if seen[item.ID] {
			t.Fatalf("duplicate item ID %q", item.ID)
		}
		seen[item.ID] = true
	}
	// Idea is in use under another spelling, so idea is still seeded.
	assertRowIDs(t, taskTagRowIDs(source.Items, TaskTagSemanticCategory), []string{
		"tag:bug", "tag:question", "tag:epic", "tag:story", "tag:feature", "tag:Idea", "tag:idea",
	})
}

func TestTaskTagListSourceKeepsDescriptiveRowsAlphabeticalWithCounts(t *testing.T) {
	source := TaskTagListSource([]query.TagInfo{
		{Name: "zeta", TaskCount: 1},
		{Name: "alpha", TaskCount: 7},
		{Name: "middle", TaskCount: 2},
	}, true)

	assertRowIDs(t, taskTagRowIDs(source.Items, TagDescriptiveCategory), []string{
		"tag:alpha", "tag:middle", "tag:zeta",
	})
	want := map[string]string{"alpha": "7 tasks", "middle": "2 tasks", "zeta": "1 task"}
	for _, item := range source.Items {
		if item.CategoryID != TagDescriptiveCategory {
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
	source := TaskTagListSource([]query.TagInfo{
		{Name: "backend", TaskCount: 2},
		{Name: "platform", TaskCount: 3, ContextCount: 2},
	}, false)

	assertRowIDs(t, taskTagRowIDs(source.Items, TaskContextCategory), []string{"tag:platform"})
	assertRowIDs(t, taskTagRowIDs(source.Items, TagDescriptiveCategory), []string{"tag:backend"})

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
		if !slices.ContainsFunc(item.Preview.Fields, func(field listsurface.ListPreviewField) bool {
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
	source := TaskTagListSource([]query.TagInfo{
		{Name: "bug", TaskCount: 1, ContextCount: 1},
		{Name: "review", TaskCount: 1, ContextCount: 1},
	}, false)

	assertRowIDs(t, taskTagRowIDs(source.Items, TaskTagSemanticCategory), []string{"tag:bug"})
	assertRowIDs(t, taskTagRowIDs(source.Items, TaskContextCategory), []string{"tag:review"})
}

func TestOperationTagPickerJumpsBetweenSections(t *testing.T) {
	picker := newFilterTagPickerForTest(t, []query.TagInfo{
		{Name: "backend", TaskCount: 1},
		{Name: "platform", TaskCount: 1, ContextCount: 1},
		{Name: "review", TaskCount: 1},
	})

	// The order is part of the assertion: the refused jump below reads the cursor
	// the last successful jump left behind.
	for _, jump := range []struct {
		key  string
		want listsurface.ListItemID
	}{
		{"t", "tag:backend"},
		{"c", "tag:platform"},
	} {
		if _, dismissed, msg := picker.HandleKey(keyPress(jump.key)); dismissed || msg != nil {
			t.Fatalf("jump %q dismissed = %v, msg = %#v", jump.key, dismissed, msg)
		}
		item, ok := picker.Surface().CurrentItem()
		if !ok || item.ID != jump.want {
			t.Fatalf("jump %q landed on %v, want %s", jump.key, item.ID, jump.want)
		}
	}

	_, dismissed, msg := picker.HandleKey(keyPress("s"))
	if dismissed {
		t.Fatal("a refused jump dismissed the picker")
	}
	if status, ok := msg.(status.StatusMsg); !ok || string(status) != "No SEMANTIC rows" {
		t.Fatalf("refused jump message = %#v, want the empty-section status", msg)
	}
	if item, ok := picker.Surface().CurrentItem(); !ok || item.ID != "tag:platform" {
		t.Fatalf("a refused jump moved the cursor to %v", item.ID)
	}
}

func TestOperationTagPickerJumpKeysStayOutOfNonTagPickers(t *testing.T) {
	picker, err := NewOperationValuePickerModel(
		operation.OperationExecutionRequest{ID: operation.OperationTaskSetPriority},
		operation.ValueProviderPriority,
		"Set Priority",
		"Task",
		listsurface.ListSourceFromPriorities([]chrome.PriorityInfo{
			{Priority: 0, Description: "Unset"},
			{Priority: 1, Description: "High"},
		}),
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("NewOperationValuePickerModel: %v", err)
	}
	before, _ := picker.Surface().CurrentItem()
	if _, dismissed, msg := picker.HandleKey(keyPress("c")); dismissed || msg != nil {
		t.Fatalf("priority picker consumed a jump key: dismissed = %v, msg = %#v", dismissed, msg)
	}
	if after, _ := picker.Surface().CurrentItem(); after.ID != before.ID {
		t.Fatalf("priority picker cursor moved to %v", after.ID)
	}
}

func TestTaskTagCategoryJumpHintNamesOnlyPopulatedSections(t *testing.T) {
	source := TaskTagListSource([]query.TagInfo{
		{Name: "backend", TaskCount: 1},
		{Name: "platform", TaskCount: 1, ContextCount: 1},
	}, false)

	styled := TaskTagCategoryJumpHint(operation.ValueProviderFilterTags, source.Items, 120)
	hint := ansi.Strip(styled)
	if got := strings.Join(strings.Fields(hint), " "); got != "jump c contexts · t tags" {
		t.Fatalf("hint = %q, want only the populated sections", got)
	}
	for _, key := range []string{"c", "t"} {
		if !strings.Contains(styled, theme.HelpKeyStyle.Render(key)) {
			t.Fatalf("hint key %q does not wear the guidance key color: %q", key, styled)
		}
	}
	seeded := TaskTagListSource([]query.TagInfo{{Name: "backend", TaskCount: 1}}, true)
	hint = ansi.Strip(TaskTagCategoryJumpHint(operation.ValueProviderTags, seeded.Items, 120))
	if got := strings.Join(strings.Fields(hint), " "); got != "jump s semantic · t tags" {
		t.Fatalf("seeded hint = %q, want the editor's two sections", got)
	}
	for _, provider := range []operation.ValueProviderID{operation.ValueProviderProjectTags, operation.ValueProviderFilterProjectTags} {
		if got := TaskTagCategoryJumpHint(provider, source.Items, 120); got != "" {
			t.Fatalf("provider %q hint = %q, want none", provider, got)
		}
	}
}

// newFilterTagPickerForTest opens the filter-tag picker over one tag set. The
// shell keeps its own copy beside the semantic-precedence tests it drives
// through AppModel; this one is what the source-level cases here need.
func newFilterTagPickerForTest(t *testing.T, tags []query.TagInfo) *OperationValuePickerModel {
	t.Helper()
	picker, err := NewOperationValuePickerModel(
		operation.OperationExecutionRequest{},
		operation.ValueProviderFilterTags,
		"Filter by Tags",
		"keep matching tasks",
		TaskTagListSource(tags, false),
		true,
		nil,
	)
	if err != nil {
		t.Fatalf("NewOperationValuePickerModel: %v", err)
	}
	return picker
}

// newTaskTagEditorForTest opens the tags-and-context editor the way the shell
// factory does: the seeded vocabulary with the context rows ahead of it, the
// membership snapshot, and then the freeze. The order of those three is the
// behavior under test, so the helper spells them rather than hiding them.
func newTaskTagEditorForTest(
	t *testing.T,
	tags []query.TagInfo,
	contexts []string,
	membership map[listsurface.ListItemID]OperationValueMembership,
) *OperationValuePickerModel {
	t.Helper()
	source := TaskTagListSource(tags, true)
	source.Items = append(TaskContextListItems(contexts), source.Items...)
	picker, err := NewOperationValuePickerModel(
		operation.OperationExecutionRequest{ID: operation.OperationTaskSetTags},
		operation.ValueProviderTags,
		"Set Tags & Context",
		"Task",
		source,
		true,
		nil,
	)
	if err != nil {
		t.Fatalf("NewOperationValuePickerModel: %v", err)
	}
	byID := make(map[listsurface.ListItemID]OperationValueMembership, len(source.Items))
	for _, item := range source.Items {
		byID[item.ID] = OperationValueNone
	}
	for id, state := range membership {
		if _, ok := byID[id]; !ok {
			t.Fatalf("membership names %s, which the source does not offer", id)
		}
		byID[id] = state
		if state == OperationValueAll && isTaskContextRowID(id) {
			picker.ContextInitial = id
		}
	}
	if err := picker.ConfigureValueMembership(byID); err != nil {
		t.Fatalf("ConfigureValueMembership: %v", err)
	}
	if err := picker.FreezeAppliedValuesFirst(); err != nil {
		t.Fatalf("FreezeAppliedValuesFirst: %v", err)
	}
	return picker
}

// drawnRowIDs is the order the list is read in, which is the matches rather than
// the items: a promotion is a grouping, so the item slice alone cannot say where
// a row is drawn.
func drawnRowIDs(picker *OperationValuePickerModel) []string {
	matches := picker.Surface().Matches()
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, string(match.Item.ID))
	}
	return ids
}

func drawnRow(t *testing.T, picker *OperationValuePickerModel, id listsurface.ListItemID) listsurface.ListItem {
	t.Helper()
	for _, match := range picker.Surface().Matches() {
		if match.Item.ID == id {
			return match.Item
		}
	}
	t.Fatalf("row %s is not drawn", id)
	return listsurface.ListItem{}
}

// TestTaskTagEditorDrawsAppliedValuesFirst states the whole of the promotion:
// which rows lead, in what order, that a promoted row is still the kind of value
// it was, and that the clear-context row every untagged task agrees on is not a
// value and therefore never leads.
func TestTaskTagEditorDrawsAppliedValuesFirst(t *testing.T) {
	picker := newTaskTagEditorForTest(t,
		[]query.TagInfo{
			{Name: "bug", TaskCount: 1},
			{Name: "audit", TaskCount: 2},
			{Name: "urgent", TaskCount: 3},
		},
		[]string{"home", "work"},
		map[listsurface.ListItemID]OperationValueMembership{
			"tag:bug":      OperationValueAll,
			"tag:audit":    OperationValueSome,
			"context:work": OperationValueAll,
		},
	)

	assertRowIDs(t, drawnRowIDs(picker), []string{
		// APPLIED, in the semantic, context, descriptive order the sections
		// underneath it keep.
		"tag:bug", "context:work", "tag:audit",
		"tag:question", "tag:epic", "tag:story", "tag:feature", "tag:idea",
		"context:0", "context:home",
		"tag:urgent",
	})

	for id, category := range map[listsurface.ListItemID]listsurface.ListCategoryID{
		"tag:bug":      TaskTagSemanticCategory,
		"context:work": TaskContextCategory,
		"tag:audit":    TagDescriptiveCategory,
	} {
		row := drawnRow(t, picker, id)
		if row.CategoryID != category {
			t.Fatalf("promoted row %s is a %s, want %s", id, row.CategoryID, category)
		}
		if row.GroupOrCategory() != TaskTagAppliedCategory {
			t.Fatalf("row %s is drawn under %s, want APPLIED", id, row.GroupOrCategory())
		}
	}

	// The marks are the meaning of the promotion: a checked value and a mixed one
	// lead together without becoming each other.
	if !picker.Surface().IsSelected("tag:bug") || !picker.Surface().IsSelected("context:work") {
		t.Fatal("a promoted applied row lost its check")
	}
	if row := drawnRow(t, picker, "tag:audit"); !row.Indeterminate || picker.Surface().IsSelected(row.ID) {
		t.Fatalf("promoted mixed row = %+v, want the third state", row)
	}
	if item, ok := picker.Surface().CurrentItem(); !ok || item.ID != "tag:bug" {
		t.Fatalf("the picker opened on %v, want the first promoted row", item.ID)
	}
}

// TestTaskTagEditorFreezesTheOrderItOpenedWith covers what the freeze is for: a
// toggle changes a mark and nothing else, in both directions, and a minted value
// enters the section its own kind owns rather than joining the promotion it was
// not part of when the picker opened.
func TestTaskTagEditorFreezesTheOrderItOpenedWith(t *testing.T) {
	picker := newTaskTagEditorForTest(t,
		[]query.TagInfo{{Name: "bug", TaskCount: 1}, {Name: "audit", TaskCount: 2}},
		[]string{"work"},
		map[listsurface.ListItemID]OperationValueMembership{
			"tag:bug":   OperationValueAll,
			"tag:audit": OperationValueSome,
		},
	)
	opened := drawnRowIDs(picker)

	for _, id := range []listsurface.ListItemID{"tag:idea", "tag:bug", "tag:audit"} {
		if !picker.Surface().SetCursorToID(id) {
			t.Fatalf("cursor did not reach %s", id)
		}
		if err := picker.ToggleCurrentMultiValue(); err != nil {
			t.Fatalf("ToggleCurrentMultiValue on %s: %v", id, err)
		}
		assertRowIDs(t, drawnRowIDs(picker), opened)
	}
	if row := drawnRow(t, picker, "tag:audit"); row.Indeterminate {
		t.Fatal("a toggled mixed row kept the third state")
	}
	if row := drawnRow(t, picker, "tag:idea"); row.GroupOrCategory() != TaskTagSemanticCategory {
		t.Fatalf("a checked row was promoted to %s", row.GroupOrCategory())
	}

	if err := picker.AddOrSelectTag("zeta"); err != nil {
		t.Fatalf("AddOrSelectTag: %v", err)
	}
	if err := picker.AddOrSelectContext("mobile"); err != nil {
		t.Fatalf("AddOrSelectContext: %v", err)
	}
	// A minted value takes its sorted place inside its own section, so the rows
	// already drawn keep their order relative to each other and only the rows
	// below the insertion shift by the row it added.
	assertRowIDs(t, drawnRowIDs(picker), []string{
		"tag:bug", "tag:audit",
		"tag:question", "tag:epic", "tag:story", "tag:feature", "tag:idea",
		"context:0", "context:mobile", "context:work",
		"tag:zeta",
	})
	if picker.Surface().IsSelected("context:work") {
		t.Fatal("a minted context left the old one selected")
	}
	if !picker.Surface().IsSelected("context:mobile") || !picker.Surface().IsSelected("tag:zeta") {
		t.Fatal("a minted value is not selected")
	}
}

// TestTaskTagEditorJumpsReachPromotedRows holds the separation the promotion
// rests on: the jump keys name a kind of value, and a row keeps its kind when it
// is drawn somewhere else, so a section emptied by the promotion answers with
// the promoted row rather than refusing.
func TestTaskTagEditorJumpsReachPromotedRows(t *testing.T) {
	picker := newTaskTagEditorForTest(t,
		[]query.TagInfo{{Name: "bug", TaskCount: 1}, {Name: "audit", TaskCount: 2}},
		[]string{"work"},
		map[listsurface.ListItemID]OperationValueMembership{
			"tag:bug":      OperationValueAll,
			"tag:audit":    OperationValueSome,
			"context:work": OperationValueAll,
		},
	)

	for _, jump := range []struct {
		key  string
		want listsurface.ListItemID
	}{
		// SEMANTIC and CONTEXTS still hold rows of their own, so the key lands
		// there; TAGS holds none, and the one descriptive value is promoted.
		{"s", "tag:question"},
		{"c", "context:0"},
		{"t", "tag:audit"},
	} {
		if _, dismissed, msg := picker.HandleKey(keyPress(jump.key)); dismissed || msg != nil {
			t.Fatalf("jump %q dismissed = %v, msg = %#v", jump.key, dismissed, msg)
		}
		if item, ok := picker.Surface().CurrentItem(); !ok || item.ID != jump.want {
			t.Fatalf("jump %q landed on %v, want %s", jump.key, item.ID, jump.want)
		}
	}
}

// TestTaskTagEditorEmitsNoEditForUntouchedValues is the other half of the mixed
// mark: a value the picker promoted and nobody touched is still partly applied,
// so applying the picker asks for nothing.
func TestTaskTagEditorEmitsNoEditForUntouchedValues(t *testing.T) {
	picker := newTaskTagEditorForTest(t,
		[]query.TagInfo{{Name: "bug", TaskCount: 1}, {Name: "audit", TaskCount: 2}},
		[]string{"work"},
		map[listsurface.ListItemID]OperationValueMembership{
			"tag:bug":      OperationValueAll,
			"tag:audit":    OperationValueSome,
			"context:work": OperationValueAll,
		},
	)

	value, err := picker.SelectedValue()
	if err != nil {
		t.Fatalf("SelectedValue: %v", err)
	}
	tags, ok := value.(operation.OperationTagsValue)
	if !ok {
		t.Fatalf("SelectedValue = %#v, want the tags patch", value)
	}
	if len(tags.Add) != 0 || len(tags.Remove) != 0 || tags.ContextSet {
		t.Fatalf("an untouched picker asked for %+v, want no edit", tags)
	}
}

// TestFreezeAppliedValuesFirstLeavesSharedPickersAlone is the scope of the
// ordering. The filter vocabulary, the project tag namespace, and the dependency
// picker are built out of the same list surface and the same membership machinery
// and are not task editing, so the call is inert for all three even when one of
// them carries a membership snapshot of its own.
func TestFreezeAppliedValuesFirstLeavesSharedPickersAlone(t *testing.T) {
	filter := newFilterTagPickerForTest(t, []query.TagInfo{
		{Name: "backend", TaskCount: 1},
		{Name: "platform", TaskCount: 1, ContextCount: 1},
	})
	if err := filter.ConfigureValueMembership(map[listsurface.ListItemID]OperationValueMembership{
		"tag:platform": OperationValueAll,
	}); err != nil {
		t.Fatalf("ConfigureValueMembership: %v", err)
	}
	before := drawnRowIDs(filter)
	if err := filter.FreezeAppliedValuesFirst(); err != nil {
		t.Fatalf("FreezeAppliedValuesFirst: %v", err)
	}
	assertRowIDs(t, drawnRowIDs(filter), before)
	for _, item := range filter.Surface().Items() {
		if item.GroupID != "" {
			t.Fatalf("filter row %s was promoted to %s", item.ID, item.GroupID)
		}
	}
}
