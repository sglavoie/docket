package outliner

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/operation"
)

// The menu bar is a projection of the operation catalog, so these are the
// parity assertions that keep it one: the bar may only name operations the
// registry holds, it may never spell a key the strategy owns, the modes it
// offers are the modes the catalog declares, and an operation the bar is the
// only route to is never an entity operation. A menu that drifts from the
// grammar fails here rather than in front of a user.
//
// This file holds what the suite shares — the fixtures, the driver helpers, and
// the resolution's own unit tests. The parity rules are in
// menu_bar_parity_test.go and the open-menu sub-state in
// menu_bar_navigation_test.go.

func registerMenuTestOperation(
	t *testing.T,
	registry *operation.OperationRegistry,
	id operation.OperationID,
	label string,
	applicable bool,
) {
	t.Helper()
	if err := registry.Register(operation.OperationDescriptor{
		ID:                id,
		Label:             label,
		Category:          operation.OperationCategorySystem,
		TargetCardinality: operation.TargetCardinalityNone,
		Risk:              operation.OperationRiskReadOnly,
		Applicability:     func(operation.CommandContext) bool { return applicable },
	}); err != nil {
		t.Fatalf("register %s: %v", id, err)
	}
}

func TestResolveMenuEntriesRefusesAnUnregisteredOperation(t *testing.T) {
	registry := operation.NewOperationRegistry()
	registerMenuTestOperation(t, registry, "test.menu.present", "Present", true)

	_, err := ResolveMenuEntries(registry, operation.InputStrategy{}, menuBarCommandContext(), []menuSpec{{
		ID:    "test",
		Title: "Test",
		Entries: []menuEntrySpec{
			{OperationID: "test.menu.present"},
			{OperationID: "test.menu.absent"},
		},
	}})
	if err == nil {
		t.Fatal("a menu naming an unregistered operation resolved without an error")
	}
}

func TestResolveMenuEntriesRemovesRatherThanGreys(t *testing.T) {
	registry := operation.NewOperationRegistry()
	registerMenuTestOperation(t, registry, "test.menu.here", "Here", true)
	registerMenuTestOperation(t, registry, "test.menu.gone", "Gone", false)

	menus, err := ResolveMenuEntries(registry, operation.InputStrategy{}, menuBarCommandContext(), []menuSpec{
		{ID: "mixed", Title: "Mixed", Entries: []menuEntrySpec{
			{OperationID: "test.menu.here"},
			menuSeparator(),
			{OperationID: "test.menu.gone"},
		}},
		{ID: "empty", Title: "Empty", Entries: []menuEntrySpec{
			menuSeparator(),
			{OperationID: "test.menu.gone"},
			menuSeparator(),
		}},
	})
	if err != nil {
		t.Fatalf("resolve menus: %v", err)
	}

	if len(menus) != 1 || menus[0].ID != "mixed" {
		t.Fatalf("resolved %d menus, want the one menu that kept an entry", len(menus))
	}
	if len(menus[0].Entries) != 1 || menus[0].Entries[0].Label != "Here" {
		t.Fatalf("mixed menu resolved to %+v, want the applicable entry alone", menus[0].Entries)
	}
}

func TestMenuGroupsContainOnlyApplicableActionsAndSkipHeadings(t *testing.T) {
	registry := operation.NewOperationRegistry()
	registerMenuTestOperation(t, registry, "test.menu.today", "Today", true)
	registerMenuTestOperation(t, registry, "test.menu.state", "Set Status", false)
	registerMenuTestOperation(t, registry, "test.menu.priority", "Priority", true)
	menus, err := ResolveMenuEntries(registry, operation.InputStrategy{}, menuBarCommandContext(), []menuSpec{{
		ID: "task", Title: "Task", Entries: []menuEntrySpec{
			{OperationID: "test.menu.today", Group: "Plan"},
			{OperationID: "test.menu.state", Group: "Status"},
			{OperationID: "test.menu.priority", Group: "Details"},
		},
	}})
	if err != nil || len(menus) != 1 {
		t.Fatalf("resolve groups: %v, %v", menus, err)
	}
	entries := menus[0].Entries
	if len(entries) != 4 || entries[0].Label != "Plan" || entries[2].Label != "Details" {
		t.Fatalf("empty group survived or first heading disappeared: %+v", entries)
	}
	if got := resolveMenuBarItem(entries, 0); got != 1 {
		t.Fatalf("first highlight = %d, want first action at 1", got)
	}
	if StepMenuBarItem(entries, 1, 1) != 3 || StepMenuBarItem(entries, 3, -1) != 1 {
		t.Fatal("menu navigation landed on a heading")
	}
}

func TestMenuKeepsUnavailableEverydayActionWithReason(t *testing.T) {
	registry := operation.NewOperationRegistry()
	registerMenuTestOperation(t, registry, operation.OperationCoreEdit, "Edit", false)
	ctx := menuBarCommandContext()
	ctx.Selection.Visual = true
	menus, err := ResolveMenuEntries(registry, operation.InputStrategy{}, ctx, []menuSpec{{ID: "task", Entries: []menuEntrySpec{{OperationID: operation.OperationCoreEdit, KeepUnavailable: true}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(menus) != 1 || len(menus[0].Entries) != 1 || menus[0].Entries[0].DisabledReason != "Leave multi-select, then choose one task to edit." {
		t.Fatalf("missing unavailable action and recovery: %+v", menus)
	}
}
