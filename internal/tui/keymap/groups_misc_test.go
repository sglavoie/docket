package keymap

import (
	"slices"
	"sort"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func actFn() Action { return func() (tea.Model, tea.Cmd) { return nil, nil } }

func TestListNavigationGroup(t *testing.T) {
	deps := ListNavigationDeps{}
	entries := ListNavigationGroup(deps)
	if len(entries) != 7 {
		t.Fatalf("ListNavigationGroup: want 7 entries, got %d", len(entries))
	}
	keys := directBindingKeys(entries)
	for _, k := range []string{"j", "down", "k", "up", "G", "ctrl+u", "ctrl+d"} {
		if !slices.Contains(keys, k) {
			t.Errorf("ListNavigationGroup: missing key %q", k)
		}
	}
}

func TestCommonTaskActionGroup_Base(t *testing.T) {
	deps := CommonTaskActionDeps{}
	entries := CommonTaskActionGroup(deps)
	if len(entries) != 6 {
		t.Fatalf("CommonTaskActionGroup (base): want 6 entries, got %d", len(entries))
	}
	keys := directBindingKeys(entries)
	for _, k := range []string{"o", "Z", "D", "e", "?", "S"} {
		if !slices.Contains(keys, k) {
			t.Errorf("CommonTaskActionGroup: missing key %q", k)
		}
	}
}

func TestCommonTaskActionGroup_AllOptional(t *testing.T) {
	deps := CommonTaskActionDeps{
		OpenURL:       actFn(),
		MoveToProject: actFn(),
		QuickNote:     actFn(),
	}
	entries := CommonTaskActionGroup(deps)
	if len(entries) != 9 {
		t.Fatalf("CommonTaskActionGroup (full): want 9 entries, got %d", len(entries))
	}
}

func TestMultiSelectGroup(t *testing.T) {
	deps := MultiSelectDeps{}
	entries := MultiSelectGroup(deps)
	if len(entries) != 4 {
		t.Fatalf("MultiSelectGroup: want 4 entries, got %d", len(entries))
	}
	keys := directBindingKeys(entries)
	for _, k := range []string{"v", "space", "a", "A"} {
		if !slices.Contains(keys, k) {
			t.Errorf("MultiSelectGroup: missing key %q", k)
		}
	}
}

func TestSortGroup(t *testing.T) {
	deps := SortDeps{}
	entries := SortGroup(deps)
	if len(entries) != 6 {
		t.Fatalf("SortGroup: want 6 entries, got %d", len(entries))
	}
	keys := directBindingKeys(entries)
	for _, k := range []string{"1", "2", "3", "4", "5", "0"} {
		if !slices.Contains(keys, k) {
			t.Errorf("SortGroup: missing key %q", k)
		}
	}
}

func TestTypeChangeGroup(t *testing.T) {
	deps := TypeChangeDeps{}
	entries := TypeChangeGroup(deps)
	if len(entries) != 4 {
		t.Fatalf("TypeChangeGroup: want 4 entries, got %d", len(entries))
	}
	keys := directBindingKeys(entries)
	for _, k := range []string{"<", ">", "H", "L"} {
		if !slices.Contains(keys, k) {
			t.Errorf("TypeChangeGroup: missing key %q", k)
		}
	}
}

// ---- helpers ----

// directBindingKeys returns the Key fields from a flat list of Binding entries.
func directBindingKeys(entries []Entry) []string {
	var keys []string
	for _, e := range entries {
		if e.Binding != nil {
			keys = append(keys, e.Binding.Key)
		}
	}
	sort.Strings(keys)
	return keys
}
