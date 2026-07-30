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
	IsCustomContext bool // True if a custom context was selected
	Canceled        bool
}

// CustomContextSavedMsg reports a committed custom-context create, membership
// edit, or rename. PreviousName is empty on create. WasActive reports whether
// the saved context is the one the Outliner is currently scoped to, so the
// shell knows to re-derive the scope.
type CustomContextSavedMsg struct {
	Name         string
	PreviousName string
	IsNew        bool
	Renamed      bool
	ProjectCount int
	WasActive    bool
}

// CustomContextDeletedMsg reports a committed custom-context delete. WasActive
// means the Outliner scope pointed at it and must fall back to All.
type CustomContextDeletedMsg struct {
	Name      string
	WasActive bool
}

// RegularContextSavedMsg reports a committed regular-label save: an optional
// catalog-wide rename followed by the membership the user asserted. Regular
// labels live on the catalog projects themselves, so the shell must resync its
// project cache even when the label kept its name.
type RegularContextSavedMsg struct {
	Name           string
	PreviousName   string
	Renamed        bool
	RelabeledCount int
	Assigned       int
	Removed        int
	// Destination is where projects leaving the label went.
	Destination string
	WasActive   bool
	// DefaultContextMoved reports that the rename carried the workspace
	// default-context setting with it, in the same checked transaction.
	DefaultContextMoved bool
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
