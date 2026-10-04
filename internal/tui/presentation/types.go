// Package presentation owns PresentationID and the View constants that name
// every routed TUI view. It sits at the bottom of the internal/tui layering and
// imports nothing from it: the operation layer names PresentationID through
// OperationBinding.Views and CommandContext.View, while the route layer reads
// the same constants, so the bare enum lives below both rather than inside
// either.
package presentation

// PresentationID represents the different views in the TUI.
type PresentationID int

const (
	ViewUnknown PresentationID = iota
	ViewProjectCreator
	ViewProjectEditor
	ViewHelp
	ViewPriorityPicker
	ViewParentSelector
	ViewTagSelector
	ViewTaskComments
	ViewTextEditor
	ViewTaskAttachments
	ViewNotes
	ViewGlobalNotes
	ViewCrossProjectOutliner
	ViewTaskComposer
	ViewProjectHub
	ViewOutlineErrors
	ViewLastMessage
)
