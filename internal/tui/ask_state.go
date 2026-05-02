package tui

// askState represents the current state of the AI assistant view.
type askState int

const (
	askStateInput askState = iota // User is typing a query
	askStateLoading               // Waiting for response from Claude
	askStateResult                // Displaying result, awaiting user action
)
