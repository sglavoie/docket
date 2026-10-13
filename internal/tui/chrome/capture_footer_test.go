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
		hasDraft           bool
		required           []string
	}{
		{
			name:     "normal",
			required: []string{"Enter create", "Ctrl+E create & edit", "Ctrl+V paste", "? syntax", "Esc cancel"},
		},
		{
			// An empty line leaves outright, a typed one raises the discard
			// question, and the entry names which of the two this press reaches.
			name:     "normal with a draft",
			hasDraft: true,
			required: []string{"Enter create", "Ctrl+V paste", "Esc discard"},
		},
		{
			name:               "autocomplete",
			autocompleteActive: true,
			required:           []string{"Tab/Enter accept", "Ctrl+J/K navigate", "Esc close suggestions"},
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
			spec := CaptureGuidanceSpec(tc.confirmDiscard, tc.autocompleteActive, tc.helpExpanded, false, 0, tc.hasDraft)
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
	if got := CaptureGuidanceSpec(true, true, true, false, 0, true); GuidanceHintText(got.Hints[0]) != "Y discard" {
		t.Fatalf("discard confirmation must take precedence, got %+v", got.Hints)
	}
	if got := CaptureGuidanceSpec(false, true, true, false, 0, true); GuidanceHintText(got.Hints[0]) != "Tab/Enter accept" {
		t.Fatalf("autocomplete must take precedence over help, got %+v", got.Hints)
	}
	if got := CaptureGuidanceSpec(false, false, true, false, 0, true); GuidanceHintText(got.Hints[len(got.Hints)-1]) != "Esc close syntax" {
		t.Fatalf("expanded syntax expected, got %+v", got.Hints)
	}
}

// TestCaptureFooterShedsPasteFirst holds the one reprioritisation every capture
// state shares: paste is drawn where it always was and selected last, so it is
// the entry a narrow footer loses before any other.
func TestCaptureFooterShedsPasteFirst(t *testing.T) {
	for name, spec := range map[string]GuidanceSpec{
		"normal":          CaptureGuidanceSpec(false, false, false, false, 0, false),
		"syntax":          CaptureGuidanceSpec(false, false, true, false, 0, false),
		"syntax overflow": CaptureGuidanceSpec(false, false, true, true, 50, false),
	} {
		paste, found := -1, false
		for index, hint := range spec.Hints {
			if GuidanceHintText(hint) == "Ctrl+V paste" {
				paste, found = index, true
			}
		}
		if !found {
			t.Fatalf("%s guidance declares no paste entry: %+v", name, spec.Hints)
		}
		if GuidanceHintText(spec.Hints[paste-1]) != "Ctrl+E create & edit" {
			t.Fatalf("%s guidance moved paste out of its drawn position: %+v", name, spec.Hints)
		}
		for index, hint := range spec.Hints {
			if index != paste && !hint.Required && hint.Priority >= spec.Hints[paste].Priority {
				t.Fatalf("%s guidance ranks %q behind paste", name, GuidanceHintText(hint))
			}
		}
	}
}

func TestCaptureFooterSyntaxOverflowAdvertisesScrolling(t *testing.T) {
	spec := CaptureGuidanceSpec(false, false, true, true, 73, true)
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
