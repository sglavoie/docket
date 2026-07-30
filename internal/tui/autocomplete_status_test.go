package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/textgrammar"
)

func TestQuickCaptureStatusSuggestionsIncludeAllStatuses(t *testing.T) {
	q := &quickCaptureInput{}
	suggestions := q.getStatusSuggestions("")

	want := textgrammar.StateNames()
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
		{"read", []string{"ready"}},
		{"wait", []string{"waiting"}},
		{"pause", []string{"paused"}},
		{"some", []string{"someday"}},
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
