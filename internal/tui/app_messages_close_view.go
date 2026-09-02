package tui

// =============================================================================
// Close-View Interface
// =============================================================================

// popStyle indicates how a close-view message should pop the view stack.
type popStyle int

const (
	popOne      popStyle = iota // pop a single view (popView)
	popToAnchor                 // pop to the nearest anchor (popToAnchorView)
)

// closeViewMsg is implemented by close messages whose only effect is popping
// the view stack. The handler in Update checks this interface first, avoiding
// individual case arms for each Close*Msg type.
type closeViewMsg interface {
	closePopStyle() popStyle
}

// PopViewMsg is embedded in Close*Msg types that need popView().
type PopViewMsg struct{}

func (PopViewMsg) closePopStyle() popStyle { return popOne }
