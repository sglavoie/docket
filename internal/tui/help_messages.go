package tui

// CloseHelpMsg is sent when the help view should close.
type CloseHelpMsg struct{ PopViewMsg }

// ShowHelpMsg is sent to show help for a specific view.
type ShowHelpMsg struct {
	SourceView PresentationID
	ViewName   string
	Items      []HelpItem
}
