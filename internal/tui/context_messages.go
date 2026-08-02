package tui

// Context messages are the seam between the Context Hub and the shell. The hub
// owns every context mutation; the shell owns the active Outliner scope, the
// project cache, and status reporting, so each mutation reports what it did
// rather than reaching into the shell itself.

// ShowContextHubMsg opens the Context Hub.
type ShowContextHubMsg struct{}

// ShowProjectHubMsg opens the searchable project selection and administration
// surface.
type ShowProjectHubMsg struct{}

// ContextScopeSelectedMsg reports the active project scope the user chose, or a
// cancel. The scope is session-only and is never persisted.
type ContextScopeSelectedMsg struct {
	SelectedContext string
	Canceled        bool
}

// RegularContextSavedMsg reports a committed regular-label save: an optional
// catalog-wide rename followed by the membership the user asserted. Regular
// labels live on the catalog projects themselves, so the shell must resync its
// project cache even when the label kept its name.
type RegularContextSavedMsg struct {
	Name           string
	PreviousName   string
	IsNew          bool
	Renamed        bool
	RelabeledCount int
	Assigned       int
	Removed        int
	// Destination is where projects leaving the label went.
	Destination string
	WasActive   bool
}

// RegularContextDeletedMsg reports a committed regular-label delete. The label's
// projects were relabeled to Destination rather than losing their context, so
// WasActive means the Outliner scope must fall back to All.
type RegularContextDeletedMsg struct {
	Name         string
	Destination  string
	ProjectCount int
	WasActive    bool
}
