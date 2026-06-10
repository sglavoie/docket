package keymap

import (
	"slices"
	"sort"
	"testing"
)

func TestListNavigationGroup(t *testing.T) {
	deps := ListNavigationDeps{}
	entries := ListNavigationGroup(deps)
	if len(entries) != 5 {
		t.Fatalf("ListNavigationGroup: want 5 entries, got %d", len(entries))
	}
	keys := directBindingKeys(entries)
	for _, k := range []string{"j", "k", "G", "ctrl+u", "ctrl+d"} {
		if !slices.Contains(keys, k) {
			t.Errorf("ListNavigationGroup: missing key %q", k)
		}
	}
}

func TestCommonTaskActionGroup_Base(t *testing.T) {
	deps := CommonTaskActionDeps{}
	entries := CommonTaskActionGroup(deps)
	if len(entries) != 7 {
		t.Fatalf("CommonTaskActionGroup (base): want 7 entries, got %d", len(entries))
	}
	keys := directBindingKeys(entries)
	for _, k := range []string{"o", "Z", "D", "e", "?", "*", "S"} {
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
	if len(entries) != 10 {
		t.Fatalf("CommonTaskActionGroup (full): want 10 entries, got %d", len(entries))
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

func TestCommonPrimaryViewGroup_Base(t *testing.T) {
	deps := CommonPrimaryViewDeps{}
	entries := CommonPrimaryViewGroup(deps)
	// 17 base entries (EditLastModified and MoveReparent nil)
	if len(entries) != 17 {
		t.Errorf("CommonPrimaryViewGroup (base): want 17 entries, got %d", len(entries))
	}
}

func TestCommonPrimaryViewGroup_AllOptional(t *testing.T) {
	deps := CommonPrimaryViewDeps{
		EditLastModified: actFn(),
		MoveReparent:     actFn(),
	}
	entries := CommonPrimaryViewGroup(deps)
	if len(entries) != 19 {
		t.Errorf("CommonPrimaryViewGroup (full): want 19 entries, got %d", len(entries))
	}
}

func TestCommonPrimaryViewGroup_ProjectNotesShortcut(t *testing.T) {
	entries := CommonPrimaryViewGroup(CommonPrimaryViewDeps{})
	keys := directBindingKeys(entries)

	if slices.Contains(keys, "ctrl+o") {
		t.Fatal("CommonPrimaryViewGroup should not bind ctrl+o to Outliner")
	}
	for _, k := range []string{"ctrl+n", "{", "}"} {
		if !slices.Contains(keys, k) {
			t.Fatalf("CommonPrimaryViewGroup missing key %q", k)
		}
	}

	for _, e := range entries {
		if e.Binding == nil {
			continue
		}
		switch e.Binding.Key {
		case "ctrl+n":
			if e.Binding.Description != "Open project Notes" {
				t.Fatalf("ctrl+n description = %q", e.Binding.Description)
			}
			if e.Binding.Hidden {
				t.Fatal("ctrl+n should be visible in help")
			}
		case "{":
			if !e.Binding.Hidden {
				t.Fatal("{ should remain handled as a hidden compatibility alias")
			}
		}
	}
}

// TestIntegration_MergedHandledKeys builds a keymap from multiple groups and
// verifies that HandledKeys() contains the expected compound sequences.
func TestIntegration_MergedHandledKeys(t *testing.T) {
	statusDeps := StatusDeps{
		ChangeStatus:  func(string) Action { return nilAction },
		HasRecurrence: func() bool { return false },
	}
	priorityDeps := PriorityDeps{ChangePriority: func(string) Action { return nilAction }}
	navDeps := ListNavigationDeps{}

	km := &KeyMap{
		ViewName: "test",
		Entries: MergeEntries(
			StatusGroup(statusDeps),
			PriorityGroup(priorityDeps),
			ListNavigationGroup(navDeps),
		),
	}

	handled := km.HandledKeys()
	handledSet := make(map[string]bool, len(handled))
	for _, k := range handled {
		handledSet[k] = true
	}

	// Prefix activators should be present
	for _, k := range []string{",", "!"} {
		if !handledSet[k] {
			t.Errorf("HandledKeys missing prefix activator %q", k)
		}
	}
	// Compound leaf keys
	for _, k := range []string{",s", ",d", ",b", ",o", ",x", ",r", ",S", "!1", "!2", "!3", "!4", "!5"} {
		if !handledSet[k] {
			t.Errorf("HandledKeys missing compound key %q", k)
		}
	}
	// Top-level nav keys
	for _, k := range []string{"j", "k", "G", "ctrl+u", "ctrl+d"} {
		if !handledSet[k] {
			t.Errorf("HandledKeys missing nav key %q", k)
		}
	}
}

// TestStageKeyForIndex verifies the key-assignment helper.
func TestStageKeyForIndex(t *testing.T) {
	tests := []struct {
		i    int
		want string
	}{
		{0, "1"}, {8, "9"}, {9, "a"}, {34, "z"}, {35, ""},
	}
	for _, tc := range tests {
		if got := stageKeyForIndex(tc.i); got != tc.want {
			t.Errorf("stageKeyForIndex(%d) = %q, want %q", tc.i, got, tc.want)
		}
	}
}

// ---- helpers ----

// childBindingKeys returns the Key fields of all direct Binding children.
// PrefixNode children are not included.
func childBindingKeys(entries []Entry) []string {
	var keys []string
	for _, e := range entries {
		if e.Binding != nil {
			keys = append(keys, e.Binding.Key)
		}
	}
	sort.Strings(keys)
	return keys
}

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
