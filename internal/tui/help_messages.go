package tui

// CloseHelpMsg is sent when the help view should close.
type CloseHelpMsg struct{}

// ShowHelpMsg is sent to show help for a specific view.
type ShowHelpMsg struct {
	SourceView ViewType
	ViewName   string
	Items      []HelpItem
}

// ShowKeybindingExplorerMsg opens the global keybinding explorer.
type ShowKeybindingExplorerMsg struct{}

// CloseKeybindingExplorerMsg closes the global keybinding explorer.
type CloseKeybindingExplorerMsg struct{}
