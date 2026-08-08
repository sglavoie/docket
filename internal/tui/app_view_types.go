package tui

var viewTypeNames = map[PresentationID]string{
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
	ViewTaskChecklist:        "TaskChecklist",
	ViewCrossProjectOutliner: "CrossProjectOutliner",
	ViewTaskForm:             "TaskForm",
	ViewTaskComposer:         "TaskComposer",
	ViewProjectHub:           "ProjectHub",
}

// String returns the name of the PresentationID for readable test output.
func (v PresentationID) String() string {
	if name, ok := viewTypeNames[v]; ok {
		return name
	}
	return "Unknown"
}
