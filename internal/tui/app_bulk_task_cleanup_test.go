package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// TestMultiSelectStateHasTaskIDRejectsAmbiguousIdentity stayed in the shell
// when the rest of multiselect_state_test.go followed MultiSelectState down
// into listsurface: the state it builds is listsurface's, but the function
// under test is the shell's own bulk-cleanup reducer, which is what makes an
// ID-only commit result ambiguous in the first place.
func TestMultiSelectStateHasTaskIDRejectsAmbiguousIdentity(t *testing.T) {
	sharedID := "shared-task"
	state := &listsurface.MultiSelectState{
		MultiSelectMode: true,
		Selected: map[taskref.TaskIdentity]bool{
			{ProjectID: "alpha", TaskID: sharedID}: true,
			{ProjectID: "beta", TaskID: sharedID}:  true,
		},
	}
	selectedIDs := map[string]bool{sharedID: true}

	if multiSelectStateHasTaskID(state, selectedIDs) {
		t.Fatal("ID-only lookup should reject duplicate selected task identities")
	}
	if !state.MultiSelectMode || state.SelectedCount() != 2 {
		t.Fatalf("ambiguous selection changed: active=%v count=%d", state.MultiSelectMode, state.SelectedCount())
	}
}
