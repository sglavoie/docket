package tui

// Boundary messages are the seam between the surfaces that collect a boundary
// intent — the Boundary Manager and the project assignment surfaces — and the
// shell that owns the transaction. Every stable ID a message carries is frozen
// by its sender, so cursor movement between the request and the commit cannot
// retarget the mutation.

// CreateBoundaryMsg asks the shell to add an empty custom boundary. The name is
// trimmed and validated by the catalog, not the sender.
type CreateBoundaryMsg struct {
	Name string
}

// RenameBoundaryMsg asks the shell to retitle one custom boundary. Membership is
// keyed on the durable ID, so a rename never moves a project.
type RenameBoundaryMsg struct {
	BoundaryID string
	Name       string
}

// DeleteBoundaryMsg asks the shell to remove one custom boundary and move every
// member — archived projects included — into DestinationID. A destination is
// always required so no project is ever left without a boundary.
type DeleteBoundaryMsg struct {
	BoundaryID    string
	DestinationID string
}

// AssignProjectBoundaryMsg asks the shell to move one project into a boundary.
// A non-archived project leaving Active loses its Today and Next memberships in
// the same transaction, because those collections describe the task workspace it
// is being taken out of.
type AssignProjectBoundaryMsg struct {
	ProjectID  string
	BoundaryID string
}
