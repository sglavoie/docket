package route

// =============================================================================
// Close-View Interface
// =============================================================================

// PopStyle indicates how a close-view message should pop the view stack.
type PopStyle int

const (
	PopOne      PopStyle = iota // pop a single view (popView)
	PopToAnchor                 // pop to the nearest anchor (popToAnchorView)
)

// CloseViewMsg is implemented by close messages whose only effect is popping
// the view stack. The handler in Update checks this interface first, avoiding
// individual case arms for each Close*Msg type.
type CloseViewMsg interface {
	ClosePopStyle() PopStyle
}

// PopViewMsg is embedded in Close*Msg types that need popView().
type PopViewMsg struct{}

func (PopViewMsg) ClosePopStyle() PopStyle { return PopOne }
