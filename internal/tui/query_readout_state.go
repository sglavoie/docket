package tui

// queryReadoutState is the expanded readout's whole sub-state. One flag is all
// there is: the rows themselves are read off the live axes each frame by the
// pure queryReadoutBody that statusStripHeight measures, so there is no second
// copy of the readout here that could go stale against the view it describes.
//
// It is a named field on OutlinerModel rather than a promoted bool for the same
// reason the menu bar beside it is: the flag is written in exactly two places
// and read by the gate, the guidance, and the body, and naming the cluster is
// what lets those readers say which sub-state they are asking about.
type queryReadoutState struct {
	expanded bool
}

// toggle opens or closes the stack. It is the whole of the operation's handler:
// the strip reads the flag when it next renders.
func (s *queryReadoutState) toggle() {
	if s == nil {
		return
	}
	s.expanded = !s.expanded
}

// close puts the stack down without asking what it was showing. The resets that
// replace the whole projection call it, so a readout cannot survive into a view
// whose axes it was never built from.
func (s *queryReadoutState) close() {
	if s == nil {
		return
	}
	s.expanded = false
}
