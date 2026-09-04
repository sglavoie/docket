package tui

import (
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
