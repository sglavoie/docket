package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestTagAdderStripsLeadingHash verifies that a tag entered with a leading
// '#' (as users would type in quick capture syntax) does not get stored
// literally with the '#' prefix.
func TestTagAdderStripsLeadingHash(t *testing.T) {
	adder := NewTagAdder()
	picker := NewBasePicker[TagInfo](nil, PickerConfig{MultiSelect: true})
	adder.EnterAddMode()
	adder.AddTagInput.SetValue("#backend")

	handled, _ := adder.HandleAddTagKey(tea.KeyPressMsg{Code: tea.KeyEnter}, picker)
	if !handled {
		t.Fatalf("expected key to be handled")
	}
	if len(picker.Items) != 1 {
		t.Fatalf("want 1 tag, got %d", len(picker.Items))
	}
	if picker.Items[0].Name != "backend" {
		t.Errorf("want tag %q, got %q", "backend", picker.Items[0].Name)
	}
}
