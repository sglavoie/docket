package tui

// askState represents the current state of the AI assistant view.
type askState int

const (
	askStateInput              askState = iota // User is typing a query
	askStateConsent                            // First-use context disclosure
	askStateLoading                            // Waiting for response from Claude
	askStateResult                             // Displaying structured plan
	askStateConfirmSafe                        // Safe batch confirmation
	askStateConfirmDestructive                 // Separate destructive escalation
	askStateApplying                           // Committing the accepted plan
)
