package tui

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/tui/operation"
)

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
		inspectID operation.OperationID = "list.inspect"
		shareID   operation.OperationID = "list.share"
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
			OperationIDs: []operation.OperationID{inspectID, shareID},
		},
		{
			ID:           "b",
			PrimaryText:  "Bravo",
			OperationIDs: []operation.OperationID{inspectID},
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

	registry := operation.NewOperationRegistry()
	for id, label := range map[operation.OperationID]string{
		inspectID: "Inspect",
		shareID:   "Share",
	} {
		if err := registry.Register(operation.OperationDescriptor{
			ID:                id,
			Label:             label,
			Category:          operation.OperationCategoryTask,
			TargetSource:      operation.OperationTargetGlobal,
			TargetCardinality: operation.TargetCardinalityNone,
			Risk:              operation.OperationRiskReadOnly,
		}); err != nil {
			t.Fatalf("register %s: %v", id, err)
		}
	}
	descriptors, err := state.Descriptors(registry, operation.CommandContext{})
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
		Context: operation.CommandContext{},
	})
	if result.Err != nil || result.Status != "inspected" {
		t.Fatalf("ExecuteAction() = %+v", result)
	}
	if invocation.Descriptor.ID != inspectID {
		t.Fatalf("executor invocation = %+v", invocation)
	}

	result = model.ExecuteAction(executor, OperationExecutionRequest{
		ID:      shareID,
		Context: operation.CommandContext{},
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
	actions := []chrome.GuidanceHint{{Key: "Space", Label: "actions", Priority: 3}}
	spec, ok := listSurfaceGuidanceSpec(state, km, actions)
	if !ok {
		t.Fatal("list guidance adapter returned no guidance")
	}
	got := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		got = append(got, chrome.GuidanceHintText(hint))
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
