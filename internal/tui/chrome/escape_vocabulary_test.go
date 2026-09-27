package chrome

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// One key, two acts: a surface holding unsaved text raises the discard question
// and an untouched one leaves outright. Both spellings come from here, so these
// assertions are the whole of what a draft-bearing editor may say about Esc.

func TestDraftEscapeHintNamesTheActEscReaches(t *testing.T) {
	if got := DraftEscapeHint(true); got != "Esc discard" {
		t.Fatalf("DraftEscapeHint(true) = %q, want the discard question", got)
	}
	if got := DraftEscapeHint(false); got != "Esc cancel" {
		t.Fatalf("DraftEscapeHint(false) = %q, want the outright exit", got)
	}
}

// The unsaved-work question is not the destructive-confirmation chrome.
// Declining it keeps the draft and stays in the editor, which "cancel" over an
// open editor reads as the opposite of.
func TestDiscardGuidanceIsNotConfirmationGuidance(t *testing.T) {
	discard := guidanceEntryText(DiscardGuidanceSpec())
	if !strings.Contains(discard, "Y discard") || !strings.Contains(discard, "N/Esc keep editing") {
		t.Fatalf("discard guidance = %q, want the keep-editing outcome named", discard)
	}
	if confirm := guidanceEntryText(ConfirmationGuidanceSpec()); confirm == discard {
		t.Fatalf("discard guidance borrowed the destructive confirmation chrome: %q", confirm)
	}
}

// RequiredExitGuidanceSpec is the contract an editor uses when Esc does
// something other than cancel outright, so the exit entry is the caller's word
// and is reserved beside the commit before any optional control.
func TestRequiredExitGuidanceReservesTheCallersExit(t *testing.T) {
	spec := RequiredExitGuidanceSpec("Ctrl+S save", "Esc discard", "Tab next")
	if len(spec.Hints) != 3 {
		t.Fatalf("spec carried %d hints, want the commit, the exit, and one optional", len(spec.Hints))
	}
	if !spec.Hints[0].Required || !spec.Hints[1].Required || spec.Hints[2].Required {
		t.Fatalf("required flags = %+v, want the first two reserved", spec.Hints)
	}
	if got := guidanceEntryText(spec); !strings.Contains(got, "Esc discard") {
		t.Fatalf("exit guidance = %q, want the caller's own exit", got)
	}
	// The default exit stays "cancel", which is what a surface holding nothing
	// to lose owes.
	if got := guidanceEntryText(RequiredActionGuidanceSpec("Enter apply")); !strings.Contains(got, "Esc cancel") {
		t.Fatalf("default exit guidance = %q, want the cancel wording", got)
	}
}

// Every shared editor profile pays for its required entries out of the app's own
// floor. A required entry that does not fit there is truncated rather than
// dropped, so the narrowest supported terminal would show half a sentence.
func TestSharedEditorGuidanceFitsMinimumTerminal(t *testing.T) {
	profiles := map[string]GuidanceSpec{
		"discard question":  DiscardGuidanceSpec(),
		"confirmation":      ConfirmationGuidanceSpec(),
		"capture":           CaptureGuidanceSpec(false, false, false, false, 0, false),
		"capture draft":     CaptureGuidanceSpec(false, false, false, false, 0, true),
		"capture suggest":   CaptureGuidanceSpec(false, true, false, false, 0, true),
		"capture syntax":    CaptureGuidanceSpec(false, false, true, true, 40, true),
		"capture discard":   CaptureGuidanceSpec(true, false, false, false, 0, true),
		"editor save":       RequiredExitGuidanceSpec("Ctrl+S save", "Esc discard", "Ctrl+E external editor", "Shift+Enter new line"),
		"editor cancel":     RequiredActionGuidanceSpec("Enter apply", "Tab next", "Shift+Tab previous"),
		"prompt back":       RequiredExitGuidanceSpec("Enter apply", "Esc back"),
		"form save":         RequiredExitGuidanceSpec("Ctrl+S save", "Esc discard", "Shift+←/→ tabs", "Tab next", "Shift+Tab previous"),
		"project form save": RequiredExitGuidanceSpec("Ctrl+S save", "Esc cancel", "Tab next", "Shift+Tab previous", "Enter next/save"),
	}

	for name, spec := range profiles {
		// Styling is applied after packing, so the plain projection is what the
		// packer measured and the only place a truncation shows.
		rendered := ansi.Strip(RenderGuidanceText(spec, MinTerminalWidth))
		if lines := strings.Split(rendered, "\n"); len(lines) > max(spec.Rows, 1) {
			t.Errorf("%s guidance took %d rows at width %d, want at most %d",
				name, len(lines), MinTerminalWidth, max(spec.Rows, 1))
		}
		for _, hint := range spec.Hints {
			if !hint.Required {
				continue
			}
			if entry := GuidanceHintText(hint); !strings.Contains(rendered, entry) {
				t.Errorf("%s guidance truncated the required entry %q at width %d: %q",
					name, entry, MinTerminalWidth, rendered)
			}
		}
	}
}

func guidanceEntryText(spec GuidanceSpec) string {
	entries := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		entries = append(entries, GuidanceHintText(hint))
	}
	return strings.Join(entries, FooterHelpSeparator)
}
