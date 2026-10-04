package presentation

import "testing"

func TestStringNamesEveryRoutedPresentation(t *testing.T) {
	t.Parallel()

	want := map[PresentationID]string{
		ViewLastMessage:          "LastMessage",
		ViewProjectCreator:       "ProjectCreator",
		ViewProjectEditor:        "ProjectEditor",
		ViewHelp:                 "Help",
		ViewPriorityPicker:       "PriorityPicker",
		ViewParentSelector:       "ParentSelector",
		ViewTagSelector:          "TagSelector",
		ViewTaskComments:         "TaskComments",
		ViewTextEditor:           "TextEditor",
		ViewTaskAttachments:      "TaskAttachments",
		ViewNotes:                "Notes",
		ViewGlobalNotes:          "GlobalNotes",
		ViewCrossProjectOutliner: "CrossProjectOutliner",
		ViewTaskComposer:         "TaskComposer",
		ViewProjectHub:           "ProjectHub",
		ViewOutlineErrors:        "OutlineErrors",
	}

	// The routed inventory is contiguous from the first named presentation to
	// the last, so a constant added without a name fails here rather than
	// rendering as "Unknown" in a scenario assertion.
	for id := ViewProjectCreator; id <= ViewLastMessage; id++ {
		name, ok := want[id]
		if !ok {
			t.Fatalf("presentation %d has no expected name", int(id))
		}
		if got := id.String(); got != name {
			t.Errorf("PresentationID(%d).String() = %q, want %q", int(id), got, name)
		}
	}
	if len(want) != int(ViewLastMessage) {
		t.Errorf("named presentations = %d, want %d", len(want), int(ViewLastMessage))
	}
}

func TestStringReportsUnknownOutsideTheInventory(t *testing.T) {
	t.Parallel()

	for _, id := range []PresentationID{ViewUnknown, ViewLastMessage + 1, -1} {
		if got := id.String(); got != "Unknown" {
			t.Errorf("PresentationID(%d).String() = %q, want %q", int(id), got, "Unknown")
		}
	}
}
