package valuepicker

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
)

func TestQuitDistinguishesBrowsingFromUnappliedPickerEdits(t *testing.T) {
	source := TaskTagListSource(nil, true)
	m, err := NewOperationValuePickerModel(operation.OperationExecutionRequest{}, operation.ValueProviderTags, "Tags", "", source, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ConfigureValueMembership(map[listsurface.ListItemID]OperationValueMembership{"tag:bug": OperationValueSome}); err != nil {
		t.Fatal(err)
	}
	m.searching = true
	m.surface.SetQuery("bug")
	if reason := m.QuitBlockReason(); reason != "" {
		t.Fatalf("unchanged search: %s", reason)
	}
	if !m.surface.SetCursorToID("tag:bug") {
		t.Fatal("missing tag")
	}
	if err = m.ToggleCurrentMultiValue(); err != nil {
		t.Fatal(err)
	}
	if m.QuitBlockReason() == "" {
		t.Fatal("unapplied selection was discarded")
	}
	m.membershipCurrent["tag:bug"] = OperationValueSome
	if reason := m.QuitBlockReason(); reason != "" {
		t.Fatalf("restored selection: %s", reason)
	}
	m.tagAdder.AddTagMode = true
	m.tagAdder.AddTagInput.SetValue("new-name")
	if m.QuitBlockReason() == "" || m.DesktopTextBuffer() == nil {
		t.Fatal("typed tag did not own its draft")
	}
}
