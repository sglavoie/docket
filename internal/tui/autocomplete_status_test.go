package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestQuickCaptureDestinationSuggestionsIncludeAllDestinations(t *testing.T) {
	q := &quickCaptureInput{}
	suggestions := q.getDestinationSuggestions("")

	want := captureDestinationNamesForTest()
	if len(suggestions) != len(want) {
		t.Fatalf("got %d suggestions, want %d", len(suggestions), len(want))
	}
	for i, w := range want {
		if suggestions[i].Value != w {
			t.Errorf("suggestions[%d].Value = %q, want %q", i, suggestions[i].Value, w)
		}
	}
}

func TestQuickCaptureDestinationSuggestionsFilterByPartial(t *testing.T) {
	q := &quickCaptureInput{}

	tests := []struct {
		partial string
		want    []string
	}{
		{"back", []string{"backlog"}},
		{"wait", []string{"waiting"}},
		{"pause", []string{"paused"}},
		{"some", []string{"someday"}},
	}

	for _, tt := range tests {
		suggestions := q.getDestinationSuggestions(tt.partial)
		if len(suggestions) != len(tt.want) {
			t.Errorf("partial=%q: got %d suggestions, want %d", tt.partial, len(suggestions), len(tt.want))
			continue
		}
		for i, w := range tt.want {
			if suggestions[i].Value != w {
				t.Errorf("partial=%q: suggestions[%d].Value = %q, want %q", tt.partial, i, suggestions[i].Value, w)
			}
		}
	}
}

func TestDetectTriggerPrefersLongestMatchingText(t *testing.T) {
	tests := []struct {
		name     string
		triggers []ACTrigger
	}{
		{
			name: "short trigger first",
			triggers: []ACTrigger{
				{Text: "#", Mode: acModeTag},
				{Text: "##", Mode: acModeProject},
			},
		},
		{
			name: "long trigger first",
			triggers: []ACTrigger{
				{Text: "##", Mode: acModeProject},
				{Text: "#", Mode: acModeTag},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, pos, partial := DetectTrigger("title ##frontend", tt.triggers)
			if mode != acModeProject || pos != len("title ") || partial != "frontend" {
				t.Fatalf("DetectTrigger() = (%d, %d, %q), want (%d, %d, %q)", mode, pos, partial, acModeProject, len("title "), "frontend")
			}
		})
	}
}

func TestAutocompleteAcceptsMultiByteTriggerText(t *testing.T) {
	ac := AutocompleteState{
		Mode:       acModeProject,
		TriggerPos: len("title "),
		Items:      []ACSuggestion{{Value: "frontend"}},
	}

	if got, want := ac.BuildAcceptedInput("title ##fro", "##"), "title ##frontend "; got != want {
		t.Fatalf("BuildAcceptedInput() = %q, want %q", got, want)
	}
}

func TestBoundedAutocompleteUsesEveryAvailableRow(t *testing.T) {
	items := make([]ACSuggestion, 10)
	for i := range items {
		items[i] = ACSuggestion{Display: fmt.Sprintf("item %d", i), Value: fmt.Sprintf("%d", i)}
	}

	t.Run("all items fit", func(t *testing.T) {
		ac := AutocompleteState{Mode: acModeDestination, Items: items}
		rendered := ansi.Strip(ac.RenderDropdownWithLimit("", 10))
		if lipgloss.Height(rendered) != 10 || strings.Contains(rendered, "more") {
			t.Fatalf("fitting dropdown should show all ten rows without hints:\n%s", rendered)
		}
		if ac.VisibleCount != 10 || ac.ViewportTop != 0 {
			t.Fatalf("window = top %d count %d, want top 0 count 10", ac.ViewportTop, ac.VisibleCount)
		}
	})

	tests := []struct {
		name   string
		cursor int
		want   []string
	}{
		{name: "top", cursor: 0, want: []string{"item 0", "item 3", "↓ 6 more"}},
		{name: "middle", cursor: 5, want: []string{"↑", "item 5", "↓"}},
		{name: "bottom", cursor: 9, want: []string{"↑ 6 more", "item 6", "item 9"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := AutocompleteState{Mode: acModeDestination, Items: items, Cursor: tt.cursor}
			rendered := ansi.Strip(ac.RenderDropdownWithLimit("", 5))
			if got := lipgloss.Height(rendered); got != 5 {
				t.Fatalf("dropdown height = %d, want 5:\n%s", got, rendered)
			}
			for _, want := range tt.want {
				if !strings.Contains(rendered, want) {
					t.Fatalf("dropdown missing %q:\n%s", want, rendered)
				}
			}
		})
	}
}
