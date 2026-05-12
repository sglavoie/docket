package keymap

import (
	"slices"
	"sort"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// nilAction is a no-op Action for tests.
var nilAction Action = func() (tea.Model, tea.Cmd) { return nil, nil }

// actFn returns a non-nil Action for use as optional-field signals in tests.
func actFn() Action { return nilAction }

func TestStatusGroup(t *testing.T) {
	deps := StatusDeps{
		ChangeStatus:   func(string) Action { return nilAction },
		HasRecurrence:  func() bool { return true },
		SkipRecurrence: nil,
	}
	entries := StatusGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("StatusGroup: want 1 top-level entry, got %d", len(entries))
	}
	pn := entries[0].PrefixNode
	if pn == nil {
		t.Fatal("StatusGroup: expected PrefixNode at root")
	}
	if pn.Key != "," {
		t.Errorf("StatusGroup: want key ',', got %q", pn.Key)
	}
	// 8 status children (no SkipRecurrence, no DeferChildren)
	if len(pn.Children) != 8 {
		t.Errorf("StatusGroup: want 8 children, got %d", len(pn.Children))
	}
	keys := childBindingKeys(pn.Children)
	for _, k := range []string{"s", "d", "b", "o", "x", "r", "S", "m"} {
		if !slices.Contains(keys, k) {
			t.Errorf("StatusGroup: missing key %q", k)
		}
	}
}

func TestStatusGroupWithDeferChildren(t *testing.T) {
	deps := StatusDeps{
		ChangeStatus:  func(string) Action { return nilAction },
		HasRecurrence: func() bool { return true },
		DeferChildren: DeferRecurrenceGroup(nilAction, nilAction, nilAction, nilAction, nilAction),
	}
	entries := StatusGroup(deps)
	pn := entries[0].PrefixNode
	// 8 status + 1 defer prefix = 9
	if len(pn.Children) != 9 {
		t.Errorf("StatusGroup+Defer: want 9 children, got %d", len(pn.Children))
	}
	// Verify ,D is a PrefixNode
	var deferEntry *Entry
	for i := range pn.Children {
		if pn.Children[i].PrefixNode != nil && pn.Children[i].PrefixNode.Key == "D" {
			e := pn.Children[i]
			deferEntry = &e
			break
		}
	}
	if deferEntry == nil {
		t.Error("StatusGroup+Defer: missing ,D prefix node")
	}
}

func TestDeferRecurrenceGroup(t *testing.T) {
	children := DeferRecurrenceGroup(nilAction, nilAction, nilAction, nilAction, nilAction)
	if len(children) != 5 {
		t.Fatalf("DeferRecurrenceGroup: want 5 entries, got %d", len(children))
	}
	keys := childBindingKeys(children)
	for _, k := range []string{"t", "3", "w", "W", "m"} {
		if !slices.Contains(keys, k) {
			t.Errorf("DeferRecurrenceGroup: missing key %q", k)
		}
	}
}

func TestPriorityGroup(t *testing.T) {
	deps := PriorityDeps{ChangePriority: func(string) Action { return nilAction }}
	entries := PriorityGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("PriorityGroup: want 1 top-level entry, got %d", len(entries))
	}
	pn := entries[0].PrefixNode
	if pn.Key != "!" {
		t.Errorf("PriorityGroup: want key '!', got %q", pn.Key)
	}
	if len(pn.Children) != 5 {
		t.Errorf("PriorityGroup: want 5 children, got %d", len(pn.Children))
	}
	keys := childBindingKeys(pn.Children)
	for _, k := range []string{"1", "2", "3", "4", "5"} {
		if !slices.Contains(keys, k) {
			t.Errorf("PriorityGroup: missing key %q", k)
		}
	}
}

func TestPriorityGroupDescriptions(t *testing.T) {
	deps := PriorityDeps{ChangePriority: func(string) Action { return nilAction }}
	entries := PriorityGroup(deps)
	pn := entries[0].PrefixNode
	wantDescs := map[string]string{
		"1": "Set priority: Critical",
		"2": "Set priority: High",
		"3": "Set priority: Normal",
		"4": "Set priority: Low",
		"5": "Set priority: Someday",
	}
	for _, child := range pn.Children {
		if child.Binding == nil {
			continue
		}
		if want, ok := wantDescs[child.Binding.Key]; ok {
			if child.Binding.Description != want {
				t.Errorf("PriorityGroup key %q: want desc %q, got %q",
					child.Binding.Key, want, child.Binding.Description)
			}
		}
	}
}

func TestDueDateGroup(t *testing.T) {
	deps := DueDateDeps{}
	entries := DueDateGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("DueDateGroup: want 1 top-level entry")
	}
	pn := entries[0].PrefixNode
	if pn.Key != "d" {
		t.Errorf("DueDateGroup: want key 'd', got %q", pn.Key)
	}
	if len(pn.Children) != 6 {
		t.Errorf("DueDateGroup: want 6 children, got %d", len(pn.Children))
	}
	keys := childBindingKeys(pn.Children)
	for _, k := range []string{"t", "m", "w", "n", "0", "d"} {
		if !slices.Contains(keys, k) {
			t.Errorf("DueDateGroup: missing key %q", k)
		}
	}
}

func TestSnoozeGroup(t *testing.T) {
	deps := SnoozeDeps{}
	entries := SnoozeGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("SnoozeGroup: want 1 top-level entry")
	}
	pn := entries[0].PrefixNode
	if pn.Key != "s" {
		t.Errorf("SnoozeGroup: want key 's', got %q", pn.Key)
	}
	if len(pn.Children) != 6 {
		t.Errorf("SnoozeGroup: want 6 children, got %d", len(pn.Children))
	}
	keys := childBindingKeys(pn.Children)
	for _, k := range []string{"t", "w", "m", "n", "0", "d"} {
		if !slices.Contains(keys, k) {
			t.Errorf("SnoozeGroup: missing key %q", k)
		}
	}
}

func TestGotoGroup_NoOptional(t *testing.T) {
	deps := GotoDeps{JumpToTop: nilAction}
	entries := GotoGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("GotoGroup: want 1 top-level entry")
	}
	pn := entries[0].PrefixNode
	if pn.Key != "g" {
		t.Errorf("GotoGroup: want key 'g', got %q", pn.Key)
	}
	if len(pn.Children) != 1 {
		t.Errorf("GotoGroup (no gp): want 1 child, got %d", len(pn.Children))
	}
	if pn.Children[0].Binding.Key != "g" {
		t.Errorf("GotoGroup: want first child key 'g', got %q", pn.Children[0].Binding.Key)
	}
}

func TestGotoGroup_WithGoToProject(t *testing.T) {
	deps := GotoDeps{JumpToTop: nilAction, GoToProject: actFn()}
	entries := GotoGroup(deps)
	pn := entries[0].PrefixNode
	if len(pn.Children) != 2 {
		t.Errorf("GotoGroup (with gp): want 2 children, got %d", len(pn.Children))
	}
}

func TestGotoGroup_WithExtra(t *testing.T) {
	extra := []Entry{Bind("v", "Cycle view", "Views", nilAction)}
	deps := GotoDeps{JumpToTop: nilAction, Extra: extra}
	entries := GotoGroup(deps)
	pn := entries[0].PrefixNode
	// gg + 1 extra
	if len(pn.Children) != 2 {
		t.Errorf("GotoGroup (with extra): want 2 children, got %d", len(pn.Children))
	}
}

func TestMoveGroup(t *testing.T) {
	deps := MoveDeps{
		MoveToSection: func(string) Action { return nilAction },
	}
	entries := MoveGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("MoveGroup: want 1 top-level entry")
	}
	pn := entries[0].PrefixNode
	if pn.Key != "m" {
		t.Errorf("MoveGroup: want key 'm', got %q", pn.Key)
	}
	// 7 section moves + 2 position moves = 9
	if len(pn.Children) != 9 {
		t.Errorf("MoveGroup: want 9 children, got %d", len(pn.Children))
	}
	keys := childBindingKeys(pn.Children)
	for _, k := range []string{"a", "n", "i", "w", "r", "s", "S", "K", "J"} {
		if !slices.Contains(keys, k) {
			t.Errorf("MoveGroup: missing key %q", k)
		}
	}
}

func TestLinkedTaskGroup_Base(t *testing.T) {
	deps := LinkedTaskDeps{}
	entries := LinkedTaskGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("LinkedTaskGroup: want 1 top-level entry")
	}
	pn := entries[0].PrefixNode
	if pn.Key != "c" {
		t.Errorf("LinkedTaskGroup: want key 'c', got %q", pn.Key)
	}
	// 4 base (b, d, s, f); navigate/cross-project/unlink are nil
	if len(pn.Children) != 4 {
		t.Errorf("LinkedTaskGroup (base): want 4 children, got %d", len(pn.Children))
	}
}

func TestLinkedTaskGroup_AllOptional(t *testing.T) {
	deps := LinkedTaskDeps{
		NavigateLinked:   actFn(),
		CrossProjectLink: actFn(),
		UnlinkTask:       actFn(),
	}
	entries := LinkedTaskGroup(deps)
	pn := entries[0].PrefixNode
	if len(pn.Children) != 7 {
		t.Errorf("LinkedTaskGroup (all optional): want 7 children, got %d", len(pn.Children))
	}
}

func TestTaskPrefixGroup(t *testing.T) {
	deps := TaskPrefixDeps{}
	entries := TaskPrefixGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("TaskPrefixGroup: want 1 top-level entry")
	}
	pn := entries[0].PrefixNode
	if pn.Key != "t" {
		t.Errorf("TaskPrefixGroup: want key 't', got %q", pn.Key)
	}
	if len(pn.Children) != 15 {
		t.Errorf("TaskPrefixGroup: want 15 children, got %d", len(pn.Children))
	}
	keys := childBindingKeys(pn.Children)
	for _, k := range []string{"n", "N", "r", "s", "o", "p", "h", "t", "c", "a", "l", "e", "m", "d", "D"} {
		if !slices.Contains(keys, k) {
			t.Errorf("TaskPrefixGroup: missing key %q", k)
		}
	}
}

func TestFoldGroup(t *testing.T) {
	deps := FoldDeps{}
	entries := FoldGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("FoldGroup: want 1 top-level entry")
	}
	pn := entries[0].PrefixNode
	if pn.Key != "z" {
		t.Errorf("FoldGroup: want key 'z', got %q", pn.Key)
	}
	if len(pn.Children) != 10 {
		t.Errorf("FoldGroup: want 10 children, got %d", len(pn.Children))
	}
	keys := childBindingKeys(pn.Children)
	for _, k := range []string{"a", "o", "c", "R", "M", "v", "p", "1", "2", "3"} {
		if !slices.Contains(keys, k) {
			t.Errorf("FoldGroup: missing key %q", k)
		}
	}
}

func TestStageGroup_Empty(t *testing.T) {
	deps := StageDeps{ChangeStage: func(string) Action { return nilAction }}
	entries := StageGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("StageGroup: want 1 top-level entry")
	}
	pn := entries[0].PrefixNode
	if pn.Key != "alt+." {
		t.Errorf("StageGroup: want key 'alt+.', got %q", pn.Key)
	}
	// Only "0" (clear) when no stages
	if len(pn.Children) != 1 {
		t.Errorf("StageGroup (empty): want 1 child, got %d", len(pn.Children))
	}
	if pn.Children[0].Binding.Key != "0" {
		t.Errorf("StageGroup (empty): want child key '0', got %q", pn.Children[0].Binding.Key)
	}
}

func TestStageGroup_WithStages(t *testing.T) {
	stages := []string{"Backlog", "In Review", "Staging"}
	deps := StageDeps{
		ChangeStage: func(string) Action { return nilAction },
		Stages:      stages,
	}
	entries := StageGroup(deps)
	pn := entries[0].PrefixNode
	// 1 clear + 3 stage keys
	if len(pn.Children) != 4 {
		t.Errorf("StageGroup (3 stages): want 4 children, got %d", len(pn.Children))
	}
	keys := childBindingKeys(pn.Children)
	for _, k := range []string{"0", "1", "2", "3"} {
		if !slices.Contains(keys, k) {
			t.Errorf("StageGroup: missing key %q", k)
		}
	}
}

func TestStageGroup_DescriptionContainsStageName(t *testing.T) {
	stages := []string{"Discovery"}
	deps := StageDeps{
		ChangeStage: func(string) Action { return nilAction },
		Stages:      stages,
	}
	entries := StageGroup(deps)
	pn := entries[0].PrefixNode
	var stageEntry *Entry
	for i := range pn.Children {
		if pn.Children[i].Binding != nil && pn.Children[i].Binding.Key == "1" {
			e := pn.Children[i]
			stageEntry = &e
			break
		}
	}
	if stageEntry == nil {
		t.Fatal("StageGroup: missing key '1' for stage 'Discovery'")
	}
	if stageEntry.Binding.Description != "Set stage: Discovery" {
		t.Errorf("StageGroup: want desc 'Set stage: Discovery', got %q",
			stageEntry.Binding.Description)
	}
}

func TestBlockerGroup(t *testing.T) {
	deps := BlockerDeps{}
	entries := BlockerGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("BlockerGroup: want 1 top-level entry")
	}
	pn := entries[0].PrefixNode
	if pn.Key != "b" {
		t.Errorf("BlockerGroup: want key 'b', got %q", pn.Key)
	}
	if len(pn.Children) != 2 {
		t.Errorf("BlockerGroup: want 2 children, got %d", len(pn.Children))
	}
	keys := childBindingKeys(pn.Children)
	for _, k := range []string{"b", "m"} {
		if !slices.Contains(keys, k) {
			t.Errorf("BlockerGroup: missing key %q", k)
		}
	}
}

func TestFilterGroup_Base(t *testing.T) {
	deps := FilterDeps{
		TypeKey:        func(string) Action { return nilAction },
		StatusKey:      func(string) Action { return nilAction },
		AttrDirectKey:  func(string) Action { return nilAction },
		AttrSubKey:     func(string, string) Action { return nilAction },
		OpenAttrPicker: nilAction,
		ScoreKey:       func(string) Action { return nilAction },
	}
	entries := FilterGroup(deps)
	if len(entries) != 1 {
		t.Fatalf("FilterGroup: want 1 top-level entry")
	}
	pn := entries[0].PrefixNode
	if pn.Key != "F" {
		t.Errorf("FilterGroup: want key 'F', got %q", pn.Key)
	}
	// Ft, Fs, Fa, FS + direct: #, p, h, x = 8 children (no Fd, no F.)
	if len(pn.Children) != 8 {
		t.Errorf("FilterGroup (base): want 8 children, got %d", len(pn.Children))
	}
}

func TestFilterGroup_WithDueDateAndStage(t *testing.T) {
	deps := FilterDeps{
		TypeKey:          func(string) Action { return nilAction },
		StatusKey:        func(string) Action { return nilAction },
		DueDateKey:       func(string) Action { return nilAction },
		AttrDirectKey:    func(string) Action { return nilAction },
		AttrSubKey:       func(string, string) Action { return nilAction },
		OpenAttrPicker:   nilAction,
		ScoreKey:         func(string) Action { return nilAction },
		StageFilterKey:   func(string) Action { return nilAction },
		StageFilterNames: []string{"Backlog"},
	}
	entries := FilterGroup(deps)
	pn := entries[0].PrefixNode
	// Ft, Fs, Fd, Fa, FS, F. + #, p, h, x = 10
	if len(pn.Children) != 10 {
		t.Errorf("FilterGroup (full): want 10 children, got %d", len(pn.Children))
	}
}

func TestFilterGroup_AttrPickerBind(t *testing.T) {
	deps := FilterDeps{
		TypeKey:        func(string) Action { return nilAction },
		StatusKey:      func(string) Action { return nilAction },
		AttrDirectKey:  func(string) Action { return nilAction },
		AttrSubKey:     func(string, string) Action { return nilAction },
		OpenAttrPicker: nilAction,
		ScoreKey:       func(string) Action { return nilAction },
	}
	entries := FilterGroup(deps)
	pn := entries[0].PrefixNode

	// Fa is now a direct bind (picker), not a sub-prefix node.
	var faEntry *Entry
	for _, e := range pn.Children {
		if e.Binding != nil && e.Binding.Key == "a" {
			eCopy := e
			faEntry = &eCopy
			break
		}
	}
	if faEntry == nil {
		t.Fatal("FilterGroup: missing 'a' (Fa) entry under F")
	}
	if faEntry.PrefixNode != nil {
		t.Error("Fa should be a direct bind (picker), not a sub-prefix node")
	}
}

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
