package operation

// HelpItem represents a single help entry. Aliases carry the vocabulary a user
// is likely to type for an operation whose label does not contain it — "defer"
// for Set Due or Defer — so `?` search can reach a row its label hides. They are
// matched, never rendered as a column of their own.
type HelpItem struct {
	Key         string // e.g., "j/k", "ctrl+O"
	Description string // e.g., "Move cursor"
	Category    string // e.g., "Navigation", "Actions"
	Aliases     []string
}
