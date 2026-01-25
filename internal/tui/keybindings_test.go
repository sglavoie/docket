package tui

import (
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
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewDashboard)

	if len(items) == 0 {
		t.Fatal("No items returned for Dashboard view")
	}

	foundJK := false
	foundEnter := false
	for _, item := range items {
		if item.Key == "j/k" && item.Category == "Navigation" {
			foundJK = true
		}
		if item.Key == "Enter" && item.Description == "Select project" {
			foundEnter = true
		}
	}
	if !foundJK {
		t.Error("Expected j/k navigation binding for Dashboard")
	}
	if !foundEnter {
		t.Error("Expected Enter binding for Dashboard")
	}
}

func TestGetForView_ProjectTree(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewProjectTree)

	if len(items) == 0 {
		t.Fatal("No items returned for ProjectTree view")
	}

	foundFolds := false
	foundStatus := false
	for _, item := range items {
		if item.Category == "Folds" {
			foundFolds = true
		}
		if item.Key == ",s" && item.Category == "Status" {
			foundStatus = true
		}
	}
	if !foundFolds {
		t.Error("Expected Folds category bindings for ProjectTree")
	}
	if !foundStatus {
		t.Error("Expected status change bindings for ProjectTree")
	}
}

func TestGetForView_PriorityView(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewPriorityView)

	if len(items) == 0 {
		t.Fatal("No items returned for Priority view")
	}

	foundPriority := false
	for _, item := range items {
		if item.Key == "1-5" && item.Description == "Set priority (1=highest)" {
			foundPriority = true
			break
		}
	}
	if !foundPriority {
		t.Error("Expected priority setting binding for PriorityView")
	}
}

func TestGetForView_InboxProcessor(t *testing.T) {
	r := NewKeybindingRegistry()
	items := r.GetForView(ViewInboxProcessor)

	if len(items) == 0 {
		t.Fatal("No items returned for InboxProcessor view")
	}

	foundSkip := false
	for _, item := range items {
		if item.Key == "n" && item.Description == "Skip task (move to next without processing)" {
			foundSkip = true
			break
		}
	}
	if !foundSkip {
		t.Error("Expected skip task binding for InboxProcessor")
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

func TestKeybindingCategories(t *testing.T) {
	r := NewKeybindingRegistry()

	projectTreeItems := r.GetForView(ViewProjectTree)
	categories := make(map[string]bool)
	for _, item := range projectTreeItems {
		categories[item.Category] = true
	}

	expectedCategories := []string{
		"Navigation", "Folds", "Status", "Task Operations",
		"Views", "Filters", "Sort", "Multi-select", "📋 Clipboard",
	}
	for _, cat := range expectedCategories {
		if !categories[cat] {
			t.Errorf("Expected category %q in ProjectTree bindings", cat)
		}
	}
}
