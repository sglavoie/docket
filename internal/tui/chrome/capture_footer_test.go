package chrome

import (
	"strings"
	"testing"
)

// TestCaptureFooterSpecContainsRequiredActions pins the four capture footer
// states to their required action vocabulary. The state selection priority is
// discard confirmation → autocomplete → expanded syntax → normal capture, and
// each state must advertise its full action set so no control is hidden.
func TestCaptureFooterSpecContainsRequiredActions(t *testing.T) {
	cases := []struct {
		name               string
		confirmDiscard     bool
		autocompleteActive bool
		helpExpanded       bool
		required           []string
	}{
		{
			name:     "normal",
			required: []string{"Enter create", "Ctrl+E create & edit", "Ctrl+V paste", "? syntax", "Esc cancel"},
		},
		{
			name:               "autocomplete",
			autocompleteActive: true,
			required:           []string{"Tab/Enter accept", "Ctrl+J/K navigate", "Esc dismiss"},
		},
		{
			name:         "help",
			helpExpanded: true,
			required:     []string{"Enter create", "Ctrl+E create & edit", "Ctrl+V paste", "? close syntax", "Esc close syntax"},
		},
		{
			name:           "confirmation",
			confirmDiscard: true,
			required:       []string{"Y discard", "N/Esc keep editing"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := CaptureGuidanceSpec(tc.confirmDiscard, tc.autocompleteActive, tc.helpExpanded, false, 0)
			var entries []string
			for _, hint := range spec.Hints {
				entries = append(entries, GuidanceHintText(hint))
			}
			guidance := strings.Join(entries, FooterHelpSeparator)
			for _, want := range tc.required {
				if !strings.Contains(guidance, want) {
					t.Fatalf("%s guidance missing %q, got %q", tc.name, want, guidance)
				}
			}
		})
	}
}

// TestCaptureFooterSpecStatePriority verifies the selection precedence when
// multiple state flags are set simultaneously: discard confirmation wins over
// autocomplete, which wins over expanded syntax.
func TestCaptureFooterSpecStatePriority(t *testing.T) {
	if got := CaptureGuidanceSpec(true, true, true, false, 0); GuidanceHintText(got.Hints[0]) != "Y discard" {
		t.Fatalf("discard confirmation must take precedence, got %+v", got.Hints)
	}
	if got := CaptureGuidanceSpec(false, true, true, false, 0); GuidanceHintText(got.Hints[0]) != "Tab/Enter accept" {
		t.Fatalf("autocomplete must take precedence over help, got %+v", got.Hints)
	}
	if got := CaptureGuidanceSpec(false, false, true, false, 0); GuidanceHintText(got.Hints[len(got.Hints)-1]) != "Esc close syntax" {
		t.Fatalf("expanded syntax expected, got %+v", got.Hints)
	}
}

func TestCaptureFooterSyntaxOverflowAdvertisesScrolling(t *testing.T) {
	spec := CaptureGuidanceSpec(false, false, true, true, 73)
	entries := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		entries = append(entries, GuidanceHintText(hint))
	}
	guidance := strings.Join(entries, FooterHelpSeparator)
	for _, want := range []string{"Ctrl+J/K scroll", "PgUp/PgDn page", "? close syntax", "Esc close syntax"} {
		if !strings.Contains(guidance, want) {
			t.Errorf("overflow syntax guidance = %q, want %q", guidance, want)
		}
	}
	if spec.Supplemental != "73%" {
		t.Fatalf("overflow syntax supplemental = %q, want 73%%", spec.Supplemental)
	}
}
