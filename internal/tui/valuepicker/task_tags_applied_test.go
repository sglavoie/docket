package valuepicker

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/query"
)

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
