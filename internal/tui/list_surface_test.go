package tui

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/tui/listsurface"
)

func TestListSurfaceValidatesStableIDsAndAllowsDuplicateLabels(t *testing.T) {
	t.Parallel()

	_, err := listsurface.NewListSurfaceModel([]listsurface.ListItem{
		{ID: "same", PrimaryText: "Duplicate"},
		{ID: "same", PrimaryText: "Duplicate"},
	}, nil, listsurface.ListSurfaceConfig{})
	if !errors.Is(err, listsurface.ErrListItemDuplicateID) {
		t.Fatalf("duplicate IDs error = %v, want %v", err, listsurface.ErrListItemDuplicateID)
	}

	items := []listsurface.ListItem{
		{
			ID:          "first",
			PrimaryText: "Duplicate",
			Aliases:     []string{"one"},
			Preview: listsurface.ListPreview{
				Fields: []listsurface.ListPreviewField{{Label: "Kind", Value: "first"}},
			},
		},
		{ID: "second", PrimaryText: "Duplicate", Aliases: []string{"two"}},
	}
	model := mustListSurface(t, items, nil, listsurface.ListSurfaceConfig{})
	if !model.SetCursorToID("second") {
		t.Fatal("failed to place cursor on duplicate-label item by stable ID")
	}
	if err := model.SelectCurrent(); err != nil {
		t.Fatalf("SelectCurrent() error = %v", err)
	}

	// Constructor and accessors must not share mutable slices with adapters.
	items[0].Aliases[0] = "mutated"
	gotItems := model.Items()
	gotItems[0].Aliases[0] = "also mutated"
	gotItems[0].Preview.Fields[0].Value = "mutated"
	if got := model.Items()[0].Aliases[0]; got != "one" {
		t.Fatalf("stored alias = %q, want defensive copy %q", got, "one")
	}
	if got := model.Items()[0].Preview.Fields[0].Value; got != "first" {
		t.Fatalf("stored preview field = %q, want defensive copy %q", got, "first")
	}

	if err := model.SetItems([]listsurface.ListItem{
		{ID: "second", PrimaryText: "Duplicate"},
		{ID: "first", PrimaryText: "Duplicate"},
	}); err != nil {
		t.Fatalf("SetItems() error = %v", err)
	}
	current, ok := model.CurrentItem()
	if !ok || current.ID != "second" {
		t.Fatalf("current item after reorder = (%+v, %v), want stable ID second", current, ok)
	}
	if got := model.SelectedIDs(); len(got) != 1 || got[0] != "second" {
		t.Fatalf("selected IDs after reorder = %v, want [second]", got)
	}
}

func TestListSurfaceContextualEmptyStates(t *testing.T) {
	t.Parallel()

	config := listsurface.ListSurfaceConfig{
		EmptyState:   listsurface.ListEmptyState{Title: "Nothing here", Detail: "Create one."},
		NoMatchState: listsurface.ListEmptyState{Title: "No matches", Detail: "Clear the query."},
	}
	model := mustListSurface(t, nil, nil, config)
	model.SetSize(80, 10)

	if model.Cursor() != -1 {
		t.Fatalf("empty cursor = %d, want -1", model.Cursor())
	}
	if _, ok := model.CurrentItem(); ok {
		t.Fatal("empty model unexpectedly has a current item")
	}
	if got := model.Rows(); len(got) != 0 {
		t.Fatalf("empty rows = %v, want none", got)
	}
	if got := model.VisibleRows(); len(got) != 0 {
		t.Fatalf("empty visible rows = %v, want none", got)
	}
	empty, ok := model.EmptyContext()
	if !ok || empty.Kind != listsurface.ListEmptySource || empty.State != config.EmptyState {
		t.Fatalf("source empty context = (%+v, %v)", empty, ok)
	}

	if err := model.SetItems([]listsurface.ListItem{{ID: "one", PrimaryText: "Alpha"}}); err != nil {
		t.Fatalf("SetItems() error = %v", err)
	}
	model.SetQuery("zzzz")
	empty, ok = model.EmptyContext()
	if !ok || empty.Kind != listsurface.ListEmptyNoMatches || empty.State != config.NoMatchState {
		t.Fatalf("query empty context = (%+v, %v)", empty, ok)
	}

	model.SetQuery("")
	if _, ok := model.EmptyContext(); ok {
		t.Fatal("non-empty results unexpectedly report empty context")
	}
}

func TestListSurfaceSetEmptyStatesReplacesCopyOnly(t *testing.T) {
	t.Parallel()

	model := mustListSurface(t, []listsurface.ListItem{
		{ID: "one", PrimaryText: "Alpha"},
		{ID: "two", PrimaryText: "Bravo"},
	}, nil, listsurface.ListSurfaceConfig{
		SelectionMode: listsurface.ListSelectionMulti,
		EmptyState:    listsurface.ListEmptyState{Title: "Nothing here", Detail: "Create one."},
		NoMatchState:  listsurface.ListEmptyState{Title: "No matches", Detail: "Clear the query."},
	})
	if !model.SetCursorToID("two") {
		t.Fatal("failed to set the initial cursor")
	}
	if err := model.SetSelectedIDs([]listsurface.ListItemID{"one"}); err != nil {
		t.Fatalf("SetSelectedIDs() error = %v", err)
	}

	empty := listsurface.ListEmptyState{Title: "No projects hold tasks.", Detail: "Press Ctrl+T to show every project."}
	noMatch := listsurface.ListEmptyState{Title: "No projects match this search.", Detail: "Press Esc to clear the query, or Ctrl+T to show every project."}
	model.SetEmptyStates(empty, noMatch)

	if got := model.Config().EmptyState; got != empty {
		t.Fatalf("empty state = %+v, want %+v", got, empty)
	}
	if got := model.Config().NoMatchState; got != noMatch {
		t.Fatalf("no-match state = %+v, want %+v", got, noMatch)
	}
	if got := model.Items(); len(got) != 2 || got[0].ID != "one" || got[1].ID != "two" {
		t.Fatalf("items after SetEmptyStates = %+v, want them untouched", got)
	}
	assertCurrentListItem(t, model, "two")
	if got := model.SelectedIDs(); len(got) != 1 || got[0] != "one" {
		t.Fatalf("selected IDs after SetEmptyStates = %v, want [one]", got)
	}

	model.SetQuery("zzzz")
	if context, ok := model.EmptyContext(); !ok || context.Kind != listsurface.ListEmptyNoMatches || context.State != noMatch {
		t.Fatalf("no-match context = (%+v, %v), want the replaced copy", context, ok)
	}
	if err := model.SetItems(nil); err != nil {
		t.Fatalf("SetItems() error = %v", err)
	}
	if context, ok := model.EmptyContext(); !ok || context.Kind != listsurface.ListEmptySource || context.State != empty {
		t.Fatalf("source empty context = (%+v, %v), want the replaced copy", context, ok)
	}

	var absent *listsurface.ListSurfaceModel
	absent.SetEmptyStates(empty, noMatch)
}

func TestListSurfaceFuzzyRanksAliasesWithinStableCategories(t *testing.T) {
	t.Parallel()

	categories := []listsurface.ListCategory{
		{ID: "work", Label: "Work"},
		{ID: "personal", Label: "Personal"},
	}
	items := []listsurface.ListItem{
		{ID: "w1", CategoryID: "work", PrimaryText: "Quarterly report", Aliases: []string{"roadmap"}},
		{ID: "p1", CategoryID: "personal", PrimaryText: "Road trip"},
		{ID: "w2", CategoryID: "work", PrimaryText: "Roadmap review"},
		{ID: "loose", PrimaryText: "Roadside note"},
	}
	model := mustListSurface(t, items, categories, listsurface.ListSurfaceConfig{})
	model.SetQuery("road")

	matches := model.Matches()
	gotIDs := make([]listsurface.ListItemID, 0, len(matches))
	for _, match := range matches {
		gotIDs = append(gotIDs, match.Item.ID)
	}
	// Categories stay in caller order and ranking is applied inside each group;
	// uncategorized matches follow declared groups.
	wantIDs := []listsurface.ListItemID{"w2", "w1", "p1", "loose"}
	if fmt.Sprint(gotIDs) != fmt.Sprint(wantIDs) {
		t.Fatalf("ranked IDs = %v, want %v", gotIDs, wantIDs)
	}

	rows := model.Rows()
	wantKinds := []listsurface.ListRowKind{
		listsurface.ListRowCategory,
		listsurface.ListRowItem,
		listsurface.ListRowItem,
		listsurface.ListRowCategory,
		listsurface.ListRowItem,
		listsurface.ListRowItem,
	}
	gotKinds := make([]listsurface.ListRowKind, 0, len(rows))
	for _, row := range rows {
		gotKinds = append(gotKinds, row.Kind)
	}
	if !slices.Equal(gotKinds, wantKinds) {
		t.Fatalf("row kinds = %v, want %v", gotKinds, wantKinds)
	}
	if rows[0].Category.Label != "Work" || rows[3].Category.Label != "Personal" {
		t.Fatalf("category headers = %q and %q, want Work and Personal", rows[0].Category.Label, rows[3].Category.Label)
	}
}

func TestListSurfaceCursorPreservationAcrossFilteringAndReloads(t *testing.T) {
	t.Parallel()

	model := mustListSurface(t, []listsurface.ListItem{
		{ID: "a", PrimaryText: "Alpha"},
		{ID: "b", PrimaryText: "Bravo"},
		{ID: "c", PrimaryText: "Charlie"},
		{ID: "d", PrimaryText: "Delta"},
	}, nil, listsurface.ListSurfaceConfig{})
	if !model.SetCursorToID("c") {
		t.Fatal("failed to set initial cursor")
	}

	model.SetQuery("bravo")
	assertCurrentListItem(t, model, "b")
	model.SetQuery("")
	assertCurrentListItem(t, model, "c")

	// Once the user explicitly moves while filtered, that visible item becomes
	// the new preservation anchor.
	model.SetQuery("bravo")
	if !model.SetCursorToID("b") {
		t.Fatal("failed to replace hidden cursor anchor")
	}
	model.SetQuery("")
	assertCurrentListItem(t, model, "b")

	if !model.SetCursorToID("c") {
		t.Fatal("failed to restore cursor to c")
	}
	if err := model.SetItems([]listsurface.ListItem{
		{ID: "a", PrimaryText: "Alpha"},
		{ID: "b", PrimaryText: "Bravo"},
		{ID: "d", PrimaryText: "Delta"},
	}); err != nil {
		t.Fatalf("SetItems() error = %v", err)
	}
	// c disappeared, so the old ordinal is clamped and becomes the new anchor.
	assertCurrentListItem(t, model, "d")
	model.SetQuery("alpha")
	model.SetQuery("")
	assertCurrentListItem(t, model, "d")
}

func TestListSurfaceSingleAndMultiSelectionRespectDisabledItems(t *testing.T) {
	t.Parallel()

	items := []listsurface.ListItem{
		{ID: "a", PrimaryText: "Alpha"},
		{ID: "b", PrimaryText: "Bravo", DisabledReason: "read only"},
		{ID: "c", PrimaryText: "Charlie"},
	}
	multi := mustListSurface(t, items, nil, listsurface.ListSurfaceConfig{SelectionMode: listsurface.ListSelectionMulti})
	selected, err := multi.ToggleCurrentSelection()
	if err != nil || !selected {
		t.Fatalf("select a = (%v, %v), want selected", selected, err)
	}
	multi.MoveCursor(1)
	if _, err := multi.ToggleCurrentSelection(); !errors.Is(err, listsurface.ErrListItemDisabled) {
		t.Fatalf("select disabled error = %v, want %v", err, listsurface.ErrListItemDisabled)
	}
	multi.MoveCursor(1)
	if err := multi.SelectCurrent(); err != nil {
		t.Fatalf("select c error = %v", err)
	}
	if got := multi.SelectedIDs(); fmt.Sprint(got) != "[a c]" {
		t.Fatalf("multi selected IDs = %v, want [a c]", got)
	}

	multi.SetQuery("bravo")
	if got := multi.SelectedIDs(); fmt.Sprint(got) != "[a c]" {
		t.Fatalf("hidden selected IDs = %v, want stable [a c]", got)
	}
	if err := multi.SetItems([]listsurface.ListItem{
		{ID: "b", PrimaryText: "Bravo"},
		{ID: "c", PrimaryText: "Charlie", DisabledReason: "now disabled"},
	}); err != nil {
		t.Fatalf("SetItems() error = %v", err)
	}
	if got := multi.SelectedIDs(); len(got) != 0 {
		t.Fatalf("pruned selected IDs = %v, want none", got)
	}

	single := mustListSurface(t, items, nil, listsurface.ListSurfaceConfig{SelectionMode: listsurface.ListSelectionSingle})
	if err := single.SetSelectedIDs([]listsurface.ListItemID{"a", "c"}); !errors.Is(err, listsurface.ErrListSelectionCardinality) {
		t.Fatalf("single selection cardinality error = %v", err)
	}
	if got := single.SelectedIDs(); len(got) != 0 {
		t.Fatalf("failed atomic single selection changed state to %v", got)
	}
	if err := single.SetSelectedIDs([]listsurface.ListItemID{"c"}); err != nil {
		t.Fatalf("single SetSelectedIDs() error = %v", err)
	}
	if !single.SetCursorToID("a") {
		t.Fatal("failed to move single cursor")
	}
	if err := single.SelectCurrent(); err != nil {
		t.Fatalf("single SelectCurrent() error = %v", err)
	}
	if got := single.SelectedIDs(); fmt.Sprint(got) != "[a]" {
		t.Fatalf("single selected IDs = %v, want [a]", got)
	}
}

func TestListSurfaceViewportIsBoundedAcrossLargeListsCategoriesAndResize(t *testing.T) {
	t.Parallel()

	categories := []listsurface.ListCategory{
		{ID: "first", Label: "First"},
		{ID: "second", Label: "Second"},
	}
	items := make([]listsurface.ListItem, 0, 100)
	for index := range 100 {
		category := listsurface.ListCategoryID("first")
		if index >= 50 {
			category = "second"
		}
		items = append(items, listsurface.ListItem{
			ID:          listsurface.ListItemID(fmt.Sprintf("item-%03d", index)),
			CategoryID:  category,
			PrimaryText: fmt.Sprintf("Item %03d", index),
		})
	}
	model := mustListSurface(t, items, categories, listsurface.ListSurfaceConfig{})
	model.SetSize(80, 5)
	if !model.MoveCursorToEnd() {
		t.Fatal("failed to move cursor to end")
	}

	viewport := model.Viewport()
	if viewport.TotalRows != 102 {
		t.Fatalf("total rows = %d, want 100 items + 2 headers", viewport.TotalRows)
	}
	if viewport.Offset < 0 || viewport.Offset > viewport.TotalRows-viewport.Height {
		t.Fatalf("viewport offset %d is out of bounds: %+v", viewport.Offset, viewport)
	}
	if !viewport.CanScrollUp || viewport.CanScrollDown {
		t.Fatalf("end viewport boundaries = %+v", viewport)
	}
	visible := model.VisibleRows()
	if len(visible) != 5 {
		t.Fatalf("visible rows = %d, want 5", len(visible))
	}
	if !visible[len(visible)-1].Cursor || visible[len(visible)-1].Match.Item.ID != "item-099" {
		t.Fatalf("last visible row = %+v, want cursor item-099", visible[len(visible)-1])
	}

	model.SetSize(40, 2)
	viewport = model.Viewport()
	if viewport.Offset != 100 || len(model.VisibleRows()) != 2 {
		t.Fatalf("small viewport = %+v with %d rows, want offset 100 and 2 rows", viewport, len(model.VisibleRows()))
	}

	model.SetSize(120, 200)
	viewport = model.Viewport()
	if viewport.Offset != 0 || viewport.CanScrollUp || viewport.CanScrollDown {
		t.Fatalf("expanded viewport = %+v, want fully unscrolled", viewport)
	}

	model.SetSize(80, 5)
	if !model.MoveCursorToStart() {
		t.Fatal("failed to move cursor to start")
	}
	if !model.PageCursor(1) {
		t.Fatal("failed to page cursor")
	}
	if model.Cursor() != 4 {
		t.Fatalf("paged cursor = %d, want selectable index 4", model.Cursor())
	}
	if row := model.Rows()[0]; row.Kind != listsurface.ListRowCategory || row.Cursor {
		t.Fatalf("category row became selectable: %+v", row)
	}
}
