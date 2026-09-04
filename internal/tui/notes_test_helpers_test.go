package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/route"
)

// mountNotesManager gives a standalone Notes manager the registry dispatch
// production hands it, by making it the mounted Manage module of a typed test
// app. The browse grammar resolves through that seam, so an unmounted view
// answers none of its keys.
func mountNotesManager(t *testing.T, view *NotesViewModel) *AppModel {
	t.Helper()
	model := typedTestApp(t)
	model.manageHub.params = route.ManageParams{Module: route.ManageNotes, NotesScope: view.scope}
	model.manageHub.active = view
	view.SetOperationSystem(model.GetOperationRegistry(), model.dispatchOperationKey)
	return model
}
