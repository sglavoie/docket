package tui

var viewTypeNames = map[PresentationID]string{
	ViewProjectCreator:       "ProjectCreator",
	ViewProjectEditor:        "ProjectEditor",
	ViewHelp:                 "Help",
	ViewDateInput:            "DateInput",
	ViewDependencySelector:   "DependencySelector",
	ViewDependencyViewer:     "DependencyViewer",
	ViewTagFilterSelector:    "TagFilterSelector",
	ViewPriorityPicker:       "PriorityPicker",
	ViewParentSelector:       "ParentSelector",
	ViewTagSelector:          "TagSelector",
	ViewBulkTagSelector:      "BulkTagSelector",
	ViewTaskComments:         "TaskComments",
	ViewProjectSelector:      "ProjectSelector",
	ViewTextEditor:           "TextEditor",
	ViewTaskAttachments:      "TaskAttachments",
	ViewCrossProjectLinker:   "CrossProjectLinker",
	ViewNotes:                "Notes",
	ViewGlobalNotes:          "GlobalNotes",
	ViewSnoozeInput:          "SnoozeInput",
	ViewTaskChecklist:        "TaskChecklist",
	ViewTaskDetail:           "TaskDetail",
	ViewCrossProjectOutliner: "CrossProjectOutliner",
	ViewTaskForm:             "TaskForm",
	ViewTaskComposer:         "TaskComposer",
	ViewTaskDecisions:        "TaskDecisions",
	ViewConflictResolution:   "ConflictResolution",
	ViewContextHub:           "ContextHub",
	ViewProjectHub:           "ProjectHub",
}

// String returns the name of the PresentationID for readable test output.
func (v PresentationID) String() string {
	if name, ok := viewTypeNames[v]; ok {
		return name
	}
	return "Unknown"
}
