package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPaletteSuggestionsAreDeterministicAcrossRegistrationOrder(t *testing.T) {
	commands := []Command{
		{Name: "zeta action", Category: CategoryActions, Views: []ViewType{ViewDashboard}, Handler: func() tea.Msg { return nil }},
		{Name: "alpha filter", Category: CategoryFilters, Views: []ViewType{ViewDashboard}, Handler: func() tea.Msg { return nil }},
		{Name: "beta action", Category: CategoryActions, Views: []ViewType{ViewDashboard}, Handler: func() tea.Msg { return nil }},
		{Name: "other-view filter", Category: CategoryFilters, Views: []ViewType{ViewOutliner}, Handler: func() tea.Msg { return nil }},
	}

	build := func(order []Command) []string {
		registry := NewCommandRegistry()
		if err := registry.RegisterBatch(order); err != nil {
			t.Fatalf("RegisterBatch: %v", err)
		}
		runtime := newTestConfig()
		palette := NewCommandPaletteModel(testRuntimeAt(runtime.DataDir(), runtime), registry, ViewDashboard, nil)
		return suggestedCommandNames(palette)
	}

	forward := build(commands)
	reverseInput := slices.Clone(commands)
	slices.Reverse(reverseInput)
	reverse := build(reverseInput)

	want := []string{"alpha filter", "beta action", "zeta action"}
	if !slices.Equal(forward, want) {
		t.Fatalf("forward suggestions = %v, want %v", forward, want)
	}
	if !slices.Equal(reverse, want) {
		t.Fatalf("reverse suggestions = %v, want %v", reverse, want)
	}
	if slices.Contains(forward, "other-view filter") {
		t.Fatalf("suggestions included a command unavailable in Dashboard: %v", forward)
	}
}
