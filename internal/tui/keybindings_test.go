package tui

import (
	"slices"
	"testing"
)

func TestNewKeybindingRegistry(t *testing.T) {
	r := NewKeybindingRegistry()
	if r == nil {
		t.Fatal("NewKeybindingRegistry returned nil")
	}
	if len(r.bindings) == 0 {
		t.Fatal("Registry has no bindings")
	}
}

func TestGetForView_Dashboard(t *testing.T) {
	// Dashboard is migrated to declarative keymap; verify via buildKeyMap().
	m := &DashboardModel{}
	keys := m.buildKeyMap().HandledKeys()
	if len(keys) == 0 {
		t.Fatal("No keys returned from DashboardModel.buildKeyMap()")
	}
	foundJK := false
	foundEnter := false
	for _, k := range keys {
		if k == "j" || k == "k" {
			foundJK = true
		}
		if k == "enter" {
			foundEnter = true
		}
	}
	if !foundJK {
		t.Error("Expected j/k navigation keys in Dashboard keymap")
	}
	if !foundEnter {
		t.Error("Expected enter key in Dashboard keymap")
	}
}

func TestGetForView_ProjectTree(t *testing.T) {
	// ProjectTree is migrated to declarative keymap; verify via buildKeyMap().
	m := &ProjectTreeModel{}
	keys := m.buildKeyMap().HandledKeys()
	if len(keys) == 0 {
		t.Fatal("No keys returned from ProjectTreeModel.buildKeyMap()")
	}

	foundFold := false
	foundStatus := false
	for _, k := range keys {
		if k == "fa" || k == "fo" || k == "fc" {
			foundFold = true
		}
		if k == ",s" {
			foundStatus = true
		}
	}
	if !foundFold {
		t.Error("Expected fold keys (fa/fo/fc) in ProjectTree keymap")
	}
	if !foundStatus {
		t.Error("Expected status change key ',s' in ProjectTree keymap")
	}
	for _, key := range []string{"fs", "fS"} {
		if slices.Contains(keys, key) {
			t.Errorf("ProjectTree keymap should not expose section fold key %s", key)
		}
	}

	r := NewKeybindingRegistry()
	for _, item := range r.GetForView(ViewProjectTree) {
		if item.Key == "fs" || item.Key == "fS" {
			t.Fatalf("ProjectTree registry should not advertise section fold key %s: %+v", item.Key, item)
		}
	}
}

func TestGetForView_OutlinerSectionFold(t *testing.T) {
	keys := (&OutlinerModel{}).HandledKeys()
	for _, key := range []string{"fs", "fS"} {
		if !slices.Contains(keys, key) {
			t.Fatalf("Expected OutlinerModel.HandledKeys to include section fold key %s", key)
		}
	}

	r := NewKeybindingRegistry()
	for _, view := range []ViewType{ViewOutliner, ViewCrossProjectOutliner} {
		found := map[string]bool{
			"fs": false,
			"fS": false,
		}
		for _, item := range r.GetForView(view) {
			if item.Key == "fs" && item.Description == "Toggle section fold" {
				found["fs"] = true
			}
			if item.Key == "fS" && item.Description == "Toggle all section folds" {
				found["fS"] = true
			}
		}
		for key, ok := range found {
			if !ok {
				t.Fatalf("%s registry help missing %s section fold binding", view, key)
			}
		}
	}
}

func TestGetForView_ProjectTreeAndOutlinerUseCtrlNForProjectNotes(t *testing.T) {
	r := NewKeybindingRegistry()

	for _, view := range []ViewType{ViewProjectTree, ViewOutliner} {
		items := r.GetForView(view)
		foundCtrlN := false
		for _, item := range items {
			if item.Key == "ctrl+n" && item.Description == "Open project Notes" {
				foundCtrlN = true
			}
			if item.Key == "ctrl+o" && item.Description == "Open outliner view" {
				t.Fatalf("%s should not advertise ctrl+o as Outliner", view)
			}
			if item.Key == "{" && item.Description == "Open project Notes" {
				t.Fatalf("%s should not advertise hidden Notes alias {", view)
			}
		}
		if !foundCtrlN {
			t.Fatalf("%s missing ctrl+n Open project Notes binding", view)
		}
	}
}

func TestGetForView_ProjectNotesScopeDoesNotAdvertiseStaleCtrlN(t *testing.T) {
	r := NewKeybindingRegistry()

	for _, view := range projectNotesViews() {
		items := r.GetForView(view)
		for _, item := range items {
			if item.Key == "ctrl+n" && item.Description != "Open project Notes" {
				t.Fatalf("%s advertises ctrl+n as %q", view, item.Description)
			}
		}
	}
}

func TestGetForView_WeeklyReviewDoesNotAdvertiseSupersededCtrlN(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewWeeklyReview)

	for _, item := range items {
		if item.Key == "ctrl+n" {
			t.Fatalf("Weekly Review should not advertise local ctrl+n binding %q", item.Description)
		}
	}

	keys := (&WeeklyReviewModel{}).buildKeyMap().HandledKeys()
	if !slices.Contains(keys, "ctrl+n") {
		t.Fatal("Weekly Review should keep local ctrl+n handled as a hidden fallback")
	}
}

func TestListNavigationGroupConsumersExposeArrowKeys(t *testing.T) {
	tests := []struct {
		name string
		keys []string
	}{
		{name: "All Tasks", keys: (&AllTasksViewModel{}).HandledKeys()},
		{name: "Focus", keys: (&FocusViewModel{}).HandledKeys()},
		{name: "Priority", keys: (&PriorityViewModel{}).HandledKeys()},
		{name: "Recent Activity", keys: (&RecentActivityModel{}).HandledKeys()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{"j", "down", "k", "up"} {
				if !slices.Contains(tc.keys, key) {
					t.Errorf("HandledKeys missing %q", key)
				}
			}
		})
	}
}

func TestGetForView_InboxProcessorPriority(t *testing.T) {
	// InboxProcessor is migrated to declarative keymap; verify via buildKeyMap().
	m := &InboxProcessorModel{}
	keys := m.buildKeyMap().HandledKeys()
	if len(keys) == 0 {
		t.Fatal("No keys returned from InboxProcessorModel.buildKeyMap()")
	}
	// Priority keys 1-5 should be handled.
	found1 := slices.Contains(keys, "1")
	if !found1 {
		t.Error("Expected priority key '1' in InboxProcessor keymap")
	}
}

func TestGetForView_InboxProcessor(t *testing.T) {
	// InboxProcessor is migrated to declarative keymap; verify via buildKeyMap().
	m := &InboxProcessorModel{}
	keys := m.buildKeyMap().HandledKeys()
	if len(keys) == 0 {
		t.Fatal("No keys returned from InboxProcessorModel.buildKeyMap()")
	}
	// 'n' is the skip/next key in the inbox processor.
	foundN := slices.Contains(keys, "n")
	if !foundN {
		t.Error("Expected 'n' key in InboxProcessor keymap")
	}
}

func TestGetForView_IncludesGlobalBindings(t *testing.T) {
	r := NewKeybindingRegistry()

	for _, view := range []ViewType{ViewDashboard, ViewProjectTree, ViewPriorityView, ViewInboxProcessor} {
		items := r.GetForView(view)
		foundQuit := false
		foundHelp := false
		for _, item := range items {
			if item.Key == "ctrl+c" && item.Description == "Quit" {
				foundQuit = true
			}
			if item.Key == "?" && item.Description == "Toggle help" {
				foundHelp = true
			}
		}
		if !foundQuit {
			t.Errorf("Global quit binding missing for view %v", view)
		}
		if !foundHelp {
			t.Errorf("Global help binding missing for view %v", view)
		}
	}
}

func TestGetGlobalBindings(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetGlobalBindings()

	if len(items) == 0 {
		t.Fatal("No global bindings returned")
	}

	foundCommandPalette := false
	for _, item := range items {
		if item.Key == ":" && item.Description == "Command palette" {
			foundCommandPalette = true
			break
		}
	}
	if !foundCommandPalette {
		t.Error("Expected command palette in global bindings")
	}

	for _, item := range items {
		if item.Category == "Folds" || item.Category == "Task Operations" {
			t.Errorf("Global bindings should not include view-specific category %s", item.Category)
		}
	}
}

func TestContainsView(t *testing.T) {
	tests := []struct {
		views    []ViewType
		target   ViewType
		expected bool
	}{
		{[]ViewType{ViewDashboard, ViewProjectTree}, ViewDashboard, true},
		{[]ViewType{ViewDashboard, ViewProjectTree}, ViewPriorityView, false},
		{[]ViewType{}, ViewDashboard, false},
		{[]ViewType{ViewProjectTree}, ViewProjectTree, true},
	}

	for _, tc := range tests {
		result := containsView(tc.views, tc.target)
		if result != tc.expected {
			t.Errorf("containsView(%v, %v) = %v, want %v", tc.views, tc.target, result, tc.expected)
		}
	}
}

func TestGetAllDeduplicatedPreservesScope(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetAllDeduplicated()

	// Global bindings must have nil Views.
	for _, item := range items {
		if item.Key == "ctrl+c" && item.Description == "Quit" {
			if item.Views != nil {
				t.Error("Global binding 'ctrl+c' should have nil Views")
			}
			break
		}
	}

	// View-specific binding with a single view.
	for _, item := range items {
		if item.Key == "Enter" && item.Description == "Select project" {
			if len(item.Views) == 0 {
				t.Error("Dashboard-only 'Enter' binding should have non-nil Views")
			}
			if !containsView(item.Views, ViewDashboard) {
				t.Error("Dashboard-only 'Enter' binding should include ViewDashboard")
			}
			break
		}
	}

	// Status bindings are shared across registry views; verify non-global scope.
	// Note: views migrated to declarative keymaps no longer appear in the registry,
	// so the count reflects only the remaining registry-based views (Outliner, ProjectTree).
	for _, item := range items {
		if item.Key == ",s" && item.Description == "Start (in progress)" {
			if len(item.Views) < 2 {
				t.Errorf(",s binding should span at least 2 views, got %d", len(item.Views))
			}
			if item.Views == nil {
				t.Error(",s binding should not be global")
			}
			break
		}
	}

	// Every deduplicated item must have Views set (non-nil) for view-specific,
	// or nil for global. Verify no empty-but-non-nil slices exist.
	for _, item := range items {
		if item.Views != nil && len(item.Views) == 0 {
			t.Errorf("binding %q (%q) has empty non-nil Views slice", item.Key, item.Description)
		}
	}
}

func TestScopeText(t *testing.T) {
	tests := []struct {
		views []ViewType
		want  string
	}{
		{nil, "Global"},
		{[]ViewType{}, "Global"},
		{[]ViewType{ViewDashboard}, "Dashboard"},
		{[]ViewType{ViewDashboard, ViewProjectTree}, "Dashboard, ProjectTree"},
		{[]ViewType{ViewDashboard, ViewProjectTree, ViewPriorityView}, "Dashboard, ProjectTree, PriorityView"},
		{[]ViewType{ViewDashboard, ViewProjectTree, ViewPriorityView, ViewFocusView}, "4 views"},
		{[]ViewType{ViewDashboard, ViewProjectTree, ViewPriorityView, ViewFocusView, ViewOutliner}, "5 views"},
	}
	for _, tc := range tests {
		got := scopeText(tc.views)
		if got != tc.want {
			t.Errorf("scopeText(%v) = %q, want %q", tc.views, got, tc.want)
		}
	}
}

func TestKeybindingCategories(t *testing.T) {
	// ProjectTree is migrated to declarative keymap; verify via buildKeyMap().
	m := &ProjectTreeModel{}
	keys := m.buildKeyMap().HandledKeys()
	if len(keys) == 0 {
		t.Fatal("No keys from ProjectTreeModel.buildKeyMap()")
	}

	// Spot-check that the keymap includes keys from the expected categories.
	hasNav := false
	hasFold := false
	hasStatus := false
	hasFilter := false
	for _, k := range keys {
		switch k {
		case "j", "k", "G":
			hasNav = true
		case "fa", "fo":
			hasFold = true
		case ",s", ",d":
			hasStatus = true
		case "F", "Fp", "Fx":
			hasFilter = true
		}
	}
	if !hasNav {
		t.Error("Expected navigation keys in ProjectTree keymap")
	}
	if !hasFold {
		t.Error("Expected fold keys in ProjectTree keymap")
	}
	if !hasStatus {
		t.Error("Expected status keys in ProjectTree keymap")
	}
	if !hasFilter {
		t.Error("Expected filter keys in ProjectTree keymap")
	}
	for _, key := range []string{"fs", "fS"} {
		if slices.Contains(keys, key) {
			t.Errorf("ProjectTree keymap should not categorize unsupported section fold key %s", key)
		}
	}
}
