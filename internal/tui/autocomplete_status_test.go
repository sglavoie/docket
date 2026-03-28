package tui

import (
	"testing"
)

func TestGlobalSearchStatusSuggestionsIncludeAllStatuses(t *testing.T) {
	m := &GlobalSearchModel{}
	suggestions := m.getStatusSuggestions("")

	want := []string{"todo", "in_progress", "review", "standby", "done", "blocked", "canceled"}
	if len(suggestions) != len(want) {
		t.Fatalf("got %d suggestions, want %d", len(suggestions), len(want))
	}
	for i, w := range want {
		if suggestions[i].Value != w {
			t.Errorf("suggestions[%d].Value = %q, want %q", i, suggestions[i].Value, w)
		}
	}
}

func TestGlobalSearchStatusSuggestionsFilterByPartial(t *testing.T) {
	m := &GlobalSearchModel{}

	tests := []struct {
		partial string
		want    []string
	}{
		{"rev", []string{"review"}},
		{"stand", []string{"standby"}},
		{"do", []string{"todo", "done"}},
		{"block", []string{"blocked"}},
	}

	for _, tt := range tests {
		suggestions := m.getStatusSuggestions(tt.partial)
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

func TestQuickCaptureStatusSuggestionsIncludeAllStatuses(t *testing.T) {
	q := &quickCaptureInput{}
	suggestions := q.getStatusSuggestions("")

	want := []string{"start", "review", "standby", "blocked", "done", "canceled"}
	if len(suggestions) != len(want) {
		t.Fatalf("got %d suggestions, want %d", len(suggestions), len(want))
	}
	for i, w := range want {
		if suggestions[i].Value != w {
			t.Errorf("suggestions[%d].Value = %q, want %q", i, suggestions[i].Value, w)
		}
	}
}

func TestQuickCaptureStatusSuggestionsFilterByPartial(t *testing.T) {
	q := &quickCaptureInput{}

	tests := []struct {
		partial string
		want    []string
	}{
		{"rev", []string{"review"}},
		{"stand", []string{"standby"}},
		{"start", []string{"start"}},
		{"block", []string{"blocked"}},
	}

	for _, tt := range tests {
		suggestions := q.getStatusSuggestions(tt.partial)
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
