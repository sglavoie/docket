package tui

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

func TestListSurfaceValidatesStableIDsAndAllowsDuplicateLabels(t *testing.T) {
	t.Parallel()

	_, err := NewListSurfaceModel([]ListItem{
		{ID: "same", PrimaryText: "Duplicate"},
		{ID: "same", PrimaryText: "Duplicate"},
	}, nil, ListSurfaceConfig{})
	if !errors.Is(err, ErrListItemDuplicateID) {
		t.Fatalf("duplicate IDs error = %v, want %v", err, ErrListItemDuplicateID)
	}

	items := []ListItem{
		{
			ID:          "first",
			PrimaryText: "Duplicate",
			Aliases:     []string{"one"},
			Preview: ListPreview{
				Fields: []ListPreviewField{{Label: "Kind", Value: "first"}},
			},
		},
		{ID: "second", PrimaryText: "Duplicate", Aliases: []string{"two"}},
	}
	model := mustListSurface(t, items, nil, ListSurfaceConfig{})
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

	if err := model.SetItems([]ListItem{
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

	config := ListSurfaceConfig{
		EmptyState:   ListEmptyState{Title: "Nothing here", Detail: "Create one."},
		NoMatchState: ListEmptyState{Title: "No matches", Detail: "Clear the query."},
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
	if !ok || empty.Kind != ListEmptySource || empty.State != config.EmptyState {
		t.Fatalf("source empty context = (%+v, %v)", empty, ok)
	}

	if err := model.SetItems([]ListItem{{ID: "one", PrimaryText: "Alpha"}}); err != nil {
		t.Fatalf("SetItems() error = %v", err)
	}
	model.SetQuery("zzzz")
	empty, ok = model.EmptyContext()
	if !ok || empty.Kind != ListEmptyNoMatches || empty.State != config.NoMatchState {
		t.Fatalf("query empty context = (%+v, %v)", empty, ok)
	}

	model.SetQuery("")
	if _, ok := model.EmptyContext(); ok {
		t.Fatal("non-empty results unexpectedly report empty context")
	}
}

func TestListSurfaceSetEmptyStatesReplacesCopyOnly(t *testing.T) {
	t.Parallel()

	model := mustListSurface(t, []ListItem{
		{ID: "one", PrimaryText: "Alpha"},
		{ID: "two", PrimaryText: "Bravo"},
	}, nil, ListSurfaceConfig{
		SelectionMode: ListSelectionMulti,
		EmptyState:    ListEmptyState{Title: "Nothing here", Detail: "Create one."},
		NoMatchState:  ListEmptyState{Title: "No matches", Detail: "Clear the query."},
	})
	if !model.SetCursorToID("two") {
		t.Fatal("failed to set the initial cursor")
	}
	if err := model.SetSelectedIDs([]ListItemID{"one"}); err != nil {
		t.Fatalf("SetSelectedIDs() error = %v", err)
	}

	empty := ListEmptyState{Title: "No projects hold tasks.", Detail: "Press Ctrl+T to show every project."}
	noMatch := ListEmptyState{Title: "No projects match this search.", Detail: "Press Esc to clear the query, or Ctrl+T to show every project."}
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
	if context, ok := model.EmptyContext(); !ok || context.Kind != ListEmptyNoMatches || context.State != noMatch {
		t.Fatalf("no-match context = (%+v, %v), want the replaced copy", context, ok)
	}
	if err := model.SetItems(nil); err != nil {
		t.Fatalf("SetItems() error = %v", err)
	}
	if context, ok := model.EmptyContext(); !ok || context.Kind != ListEmptySource || context.State != empty {
		t.Fatalf("source empty context = (%+v, %v), want the replaced copy", context, ok)
	}

	var absent *ListSurfaceModel
	absent.SetEmptyStates(empty, noMatch)
}

func TestListSurfaceFuzzyRanksAliasesWithinStableCategories(t *testing.T) {
	t.Parallel()

	categories := []ListCategory{
		{ID: "work", Label: "Work"},
		{ID: "personal", Label: "Personal"},
	}
	items := []ListItem{
		{ID: "w1", CategoryID: "work", PrimaryText: "Quarterly report", Aliases: []string{"roadmap"}},
		{ID: "p1", CategoryID: "personal", PrimaryText: "Road trip"},
		{ID: "w2", CategoryID: "work", PrimaryText: "Roadmap review"},
		{ID: "loose", PrimaryText: "Roadside note"},
	}
	model := mustListSurface(t, items, categories, ListSurfaceConfig{})
	model.SetQuery("road")

	matches := model.Matches()
	gotIDs := make([]ListItemID, 0, len(matches))
	for _, match := range matches {
		gotIDs = append(gotIDs, match.Item.ID)
	}
	// Categories stay in caller order and ranking is applied inside each group;
	// uncategorized matches follow declared groups.
	wantIDs := []ListItemID{"w2", "w1", "p1", "loose"}
	if fmt.Sprint(gotIDs) != fmt.Sprint(wantIDs) {
		t.Fatalf("ranked IDs = %v, want %v", gotIDs, wantIDs)
	}

	rows := model.Rows()
	wantKinds := []ListRowKind{
		ListRowCategory,
		ListRowItem,
		ListRowItem,
		ListRowCategory,
		ListRowItem,
		ListRowItem,
	}
	gotKinds := make([]ListRowKind, 0, len(rows))
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

	model := mustListSurface(t, []ListItem{
		{ID: "a", PrimaryText: "Alpha"},
		{ID: "b", PrimaryText: "Bravo"},
		{ID: "c", PrimaryText: "Charlie"},
		{ID: "d", PrimaryText: "Delta"},
	}, nil, ListSurfaceConfig{})
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
	if err := model.SetItems([]ListItem{
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

	items := []ListItem{
		{ID: "a", PrimaryText: "Alpha"},
		{ID: "b", PrimaryText: "Bravo", DisabledReason: "read only"},
		{ID: "c", PrimaryText: "Charlie"},
	}
	multi := mustListSurface(t, items, nil, ListSurfaceConfig{SelectionMode: ListSelectionMulti})
	selected, err := multi.ToggleCurrentSelection()
	if err != nil || !selected {
		t.Fatalf("select a = (%v, %v), want selected", selected, err)
	}
	multi.MoveCursor(1)
	if _, err := multi.ToggleCurrentSelection(); !errors.Is(err, ErrListItemDisabled) {
		t.Fatalf("select disabled error = %v, want %v", err, ErrListItemDisabled)
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
	if err := multi.SetItems([]ListItem{
		{ID: "b", PrimaryText: "Bravo"},
		{ID: "c", PrimaryText: "Charlie", DisabledReason: "now disabled"},
	}); err != nil {
		t.Fatalf("SetItems() error = %v", err)
	}
	if got := multi.SelectedIDs(); len(got) != 0 {
		t.Fatalf("pruned selected IDs = %v, want none", got)
	}

	single := mustListSurface(t, items, nil, ListSurfaceConfig{SelectionMode: ListSelectionSingle})
	if err := single.SetSelectedIDs([]ListItemID{"a", "c"}); !errors.Is(err, ErrListSelectionCardinality) {
		t.Fatalf("single selection cardinality error = %v", err)
	}
	if got := single.SelectedIDs(); len(got) != 0 {
		t.Fatalf("failed atomic single selection changed state to %v", got)
	}
	if err := single.SetSelectedIDs([]ListItemID{"c"}); err != nil {
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

	categories := []ListCategory{
		{ID: "first", Label: "First"},
		{ID: "second", Label: "Second"},
	}
	items := make([]ListItem, 0, 100)
	for index := range 100 {
		category := ListCategoryID("first")
		if index >= 50 {
			category = "second"
		}
		items = append(items, ListItem{
			ID:          ListItemID(fmt.Sprintf("item-%03d", index)),
			CategoryID:  category,
			PrimaryText: fmt.Sprintf("Item %03d", index),
		})
	}
	model := mustListSurface(t, items, categories, ListSurfaceConfig{})
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
	if row := model.Rows()[0]; row.Kind != ListRowCategory || row.Cursor {
		t.Fatalf("category row became selectable: %+v", row)
	}
}

func TestLayoutListTextUsesUnicodeDisplayWidth(t *testing.T) {
	t.Parallel()

	layout := LayoutListText("修复界面布局问题", "🚀 very long secondary detail", 18)
	if got := lipgloss.Width(layout.Text()); got > 18 {
		t.Fatalf("layout width = %d, want <= 18: %q", got, layout.Text())
	}

	tiny := LayoutListText("界面", "detail", 1)
	if got := lipgloss.Width(tiny.Text()); got > 1 {
		t.Fatalf("tiny layout width = %d, want <= 1: %q", got, tiny.Text())
	}

	full := LayoutListText("Alpha", "Beta", 20)
	if full.Text() != "Alpha  Beta" {
		t.Fatalf("unconstrained layout = %+v", full)
	}
}

func TestListSurfacePreviewAndActionsUseOperationRegistryAndExecutor(t *testing.T) {
	t.Parallel()

	const (
		inspectID OperationID = "list.inspect"
		shareID   OperationID = "list.share"
	)
	items := []ListItem{
		{
			ID:          "a",
			PrimaryText: "Alpha",
			Preview: ListPreview{
				Title:   "Alpha preview",
				Summary: "Summary",
				Body:    "Body",
				Fields:  []ListPreviewField{{Label: "Kind", Value: "task"}},
			},
			OperationIDs: []OperationID{inspectID, shareID},
		},
		{
			ID:           "b",
			PrimaryText:  "Bravo",
			OperationIDs: []OperationID{inspectID},
		},
	}
	model := mustListSurface(t, items, nil, ListSurfaceConfig{SelectionMode: ListSelectionMulti})
	preview, ok := model.CurrentPreview()
	if !ok || preview.Title != "Alpha preview" || preview.Fields[0].Value != "task" {
		t.Fatalf("current preview = (%+v, %v)", preview, ok)
	}
	preview.Fields[0].Value = "mutated"
	again, _ := model.CurrentPreview()
	if again.Fields[0].Value != "task" {
		t.Fatalf("preview accessor shared fields: %+v", again)
	}

	if err := model.SelectCurrent(); err != nil {
		t.Fatalf("select a error = %v", err)
	}
	model.MoveCursor(1)
	if err := model.SelectCurrent(); err != nil {
		t.Fatalf("select b error = %v", err)
	}
	state := model.ActionState()
	if fmt.Sprint(state.ItemIDs) != "[a b]" || fmt.Sprint(state.OperationIDs) != "[list.inspect]" {
		t.Fatalf("multi action state = %+v", state)
	}

	registry := NewOperationRegistry()
	for id, label := range map[OperationID]string{
		inspectID: "Inspect",
		shareID:   "Share",
	} {
		if err := registry.Register(OperationDescriptor{
			ID:                id,
			Label:             label,
			Category:          OperationCategoryTask,
			TargetSource:      OperationTargetGlobal,
			TargetCardinality: TargetCardinalityNone,
			Risk:              OperationRiskReadOnly,
		}); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
	}
	descriptors, err := state.Descriptors(registry, CommandContext{})
	if err != nil {
		t.Fatalf("Descriptors() error = %v", err)
	}
	if len(descriptors) != 1 || descriptors[0].ID != inspectID {
		t.Fatalf("action descriptors = %+v, want inspect", descriptors)
	}

	executor := NewOperationExecutor(registry)
	var invocation OperationInvocation
	if err := executor.Register(OperationBinding{
		OperationID: inspectID,
		Handler: func(got OperationInvocation) OperationHandlerResult {
			invocation = got
			return OperationHandlerResult{Status: "inspected"}
		},
	}); err != nil {
		t.Fatalf("register executor binding: %v", err)
	}
	result := model.ExecuteAction(executor, OperationExecutionRequest{
		ID:      inspectID,
		Context: CommandContext{},
	})
	if result.Err != nil || result.Status != "inspected" {
		t.Fatalf("ExecuteAction() = %+v", result)
	}
	if invocation.Descriptor.ID != inspectID {
		t.Fatalf("executor invocation = %+v", invocation)
	}

	result = model.ExecuteAction(executor, OperationExecutionRequest{
		ID:      shareID,
		Context: CommandContext{},
	})
	if !errors.Is(result.Err, ErrListActionUnavailable) {
		t.Fatalf("non-common action error = %v, want %v", result.Err, ErrListActionUnavailable)
	}
}

func TestListSurfaceGuidanceProfilesCoverLiveInteractionStates(t *testing.T) {
	model := mustListSurface(t, []ListItem{{ID: "a", PrimaryText: "Alpha"}}, nil, ListSurfaceConfig{
		EmptyState:   ListEmptyState{Title: "Empty source"},
		NoMatchState: ListEmptyState{Title: "No matches"},
	})
	if got := model.GuidanceState(false).Profile; got != ListGuidanceBrowse {
		t.Fatalf("browse profile = %v", got)
	}

	model.SetQuery("alpha")
	if got := model.GuidanceState(false); got.Profile != ListGuidanceSearchApplied || got.Query != "alpha" {
		t.Fatalf("applied-search profile = %+v", got)
	}
	if got := model.GuidanceState(true).Profile; got != ListGuidanceSearchEditing {
		t.Fatalf("editing-search profile = %v", got)
	}

	model.SetQuery("")
	if err := model.SelectCurrent(); err != nil {
		t.Fatalf("select current: %v", err)
	}
	if got := model.GuidanceState(false); got.Profile != ListGuidanceSelection || got.SelectionCount != 1 {
		t.Fatalf("selection profile = %+v", got)
	}

	disabled := mustListSurface(t, []ListItem{{
		ID:             "disabled",
		PrimaryText:    "Disabled",
		DisabledReason: "requires another project",
	}}, nil, ListSurfaceConfig{})
	if got := disabled.GuidanceState(false); got.Profile != ListGuidanceDisabled || got.DisabledReason != "requires another project" {
		t.Fatalf("disabled profile = %+v", got)
	}

	empty := mustListSurface(t, nil, nil, ListSurfaceConfig{
		EmptyState:   ListEmptyState{Title: "Empty source"},
		NoMatchState: ListEmptyState{Title: "No matches"},
	})
	if got := empty.GuidanceState(false); got.Profile != ListGuidanceEmpty || got.Empty.Kind != ListEmptySource {
		t.Fatalf("empty-source profile = %+v", got)
	}
	model.ClearSelection()
	model.SetQuery("missing")
	if got := model.GuidanceState(false); got.Profile != ListGuidanceEmpty || got.Empty.Kind != ListEmptyNoMatches {
		t.Fatalf("empty-results profile = %+v", got)
	}
}

func TestListSurfaceGuidanceUsesLiveKeymapAndApplicableActionHints(t *testing.T) {
	liveSearch := true
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("/", "Search items", "Filters", keymap.WithFooter(2, "search", false), keymap.WithGate(func() bool {
			return liveSearch
		})),
		keymap.Bind("Esc", "Clear search", "Navigation", keymap.WithFooter(1, "clear", true)),
		keymap.Bind("x", "Hidden alias", "Actions", keymap.AsHidden(), keymap.WithFooter(3, "delete", false)),
	}}
	state := ListGuidanceState{
		Profile:        ListGuidanceSelection,
		SelectionCount: 2,
	}
	actions := []GuidanceHint{{Key: "Space", Label: "actions", Priority: 3}}
	spec, ok := listSurfaceGuidanceSpec(state, km, actions)
	if !ok {
		t.Fatal("list guidance adapter returned no guidance")
	}
	got := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		got = append(got, guidanceHintText(hint))
	}
	if !slices.Equal(got, []string{"Esc clear", "/ search", "Space actions"}) {
		t.Fatalf("list guidance hints = %v", got)
	}
	if spec.Supplemental != "2 selected" || spec.Rows != 1 {
		t.Fatalf("selection guidance metadata = %+v", spec)
	}

	liveSearch = false
	spec, ok = listSurfaceGuidanceSpec(ListGuidanceState{Profile: ListGuidanceSearchEditing}, km, nil)
	if !ok || len(spec.Hints) != 1 || spec.Hints[0].Key != "Esc" || spec.Rows != 2 {
		t.Fatalf("editing guidance with closed gate = (%+v, %v)", spec, ok)
	}
}

func TestListSurfaceRejectsUnknownCategoriesWithoutMutation(t *testing.T) {
	t.Parallel()

	categories := []ListCategory{{ID: "known", Label: "Known"}}
	model := mustListSurface(t, []ListItem{
		{ID: "one", CategoryID: "known", PrimaryText: "One"},
	}, categories, ListSurfaceConfig{})
	err := model.SetItems([]ListItem{
		{ID: "two", CategoryID: "missing", PrimaryText: "Two"},
	})
	if !errors.Is(err, ErrListCategoryUnknown) {
		t.Fatalf("unknown category error = %v, want %v", err, ErrListCategoryUnknown)
	}
	if got := model.Items(); len(got) != 1 || got[0].ID != "one" {
		t.Fatalf("failed SetItems mutated source: %+v", got)
	}
}

func mustListSurface(
	t *testing.T,
	items []ListItem,
	categories []ListCategory,
	config ListSurfaceConfig,
) *ListSurfaceModel {
	t.Helper()
	model, err := NewListSurfaceModel(items, categories, config)
	if err != nil {
		t.Fatalf("NewListSurfaceModel() error = %v", err)
	}
	return model
}

func assertCurrentListItem(t *testing.T, model *ListSurfaceModel, want ListItemID) {
	t.Helper()
	item, ok := model.CurrentItem()
	if !ok || item.ID != want {
		t.Fatalf("current item = (%+v, %v), want %s", item, ok, want)
	}
}
