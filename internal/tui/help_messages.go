package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
)

// CloseHelpMsg is sent when the help view should close.
type CloseHelpMsg struct{ route.PopViewMsg }

// ShowHelpMsg is sent to show help for a specific view. It carries no source
// view: CloseHelpMsg is a PopViewMsg, so the view stack is what returns the user
// where they were, and the help viewer never needs to name its opener.
type ShowHelpMsg struct {
	ViewName string
	Items    []operation.HelpItem
}

// helpCmd raises the help viewer for one named surface. It sits beside the
// message it constructs rather than with the navigation helpers, because a
// surface asking for `?` is asking the shell to push a view, not to move a
// cursor.
func helpCmd(name string, items []operation.HelpItem) tea.Cmd {
	return func() tea.Msg {
		return ShowHelpMsg{ViewName: name, Items: items}
	}
}
