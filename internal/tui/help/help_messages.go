package help

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/route"
)

// CloseHelpMsg is sent when the help view should close.
type CloseHelpMsg struct{ route.PopViewMsg }

// ShowHelpMsg shows a surface's reference. The view stack owns returning to its
// opener; Outliner action search additionally freezes the invocation context.
type ShowHelpMsg struct {
	ViewName string
	Items    []operation.HelpItem
	// Only the Outliner supplies runnable search results. Other surfaces retain
	// their reference viewer, including transient modal controls.
	ActionContext *operation.CommandContext
	SearchItems   []operation.HelpItem
}

// RunActionMsg asks the shell to close this help instance and execute the
// selected registry action against the context frozen when help opened.
type RunActionMsg struct {
	Source  *HelpViewModel
	Request operation.OperationExecutionRequest
}

// HelpCmd raises the help viewer for one named surface. It sits beside the
// message it constructs rather than with the navigation helpers, because a
// surface asking for `?` is asking the shell to push a view, not to move a
// cursor.
func HelpCmd(name string, items []operation.HelpItem) tea.Cmd {
	return func() tea.Msg {
		return ShowHelpMsg{ViewName: name, Items: items}
	}
}
