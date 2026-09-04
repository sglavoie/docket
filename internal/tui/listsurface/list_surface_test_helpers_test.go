package listsurface

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// This file holds the fixtures the listsurface tests share. Each one is a
// deliberate copy of a shell helper rather than an export: a test helper
// exported to cross a package boundary is production-unused API, which
// scripts/dead_code.sh rejects, and the alternative — an exported constructor
// wrapper on a package that already exports its constructor — buys nothing.

// mustListSurface builds a list surface or fails the test. The shell keeps its
// own copy for list_surface_registry_test.go, which stays a root integration
// test because it drives the surface through the operation registry.
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

// syntheticSpecialKey and syntheticCtrlKey mirror internal/tui/key_helpers_test.go.
// Only the two spellings the always-search picker tests press are copied; a
// wholesale copy of that file would leave three unused helpers behind.
func syntheticSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func syntheticCtrlKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}
