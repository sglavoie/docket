package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/route"
)

// combinedNotesService is the in-memory note collection the shell's Notes tests
// load from. internal/tui/notes keeps its own copy for the unit tests homed
// there: a test fixture exported to cross a package boundary is
// production-unused API, which scripts/dead_code.sh rejects. What stays here is
// what drives more than the manager — the migration, guidance, hub and move
// tests that reach through the shell.
type combinedNotesService struct {
	byLocation  map[app.NoteLocation][]*models.Note
	errors      map[app.NoteLocation]error
	snapshotErr error
	queryCount  int
	requests    []app.NoteBatchRequest
}

func (s *combinedNotesService) NotesAtLocations(locations []app.NoteLocation) ([]app.NoteCollectionResult, error) {
	s.queryCount++
	if s.snapshotErr != nil {
		return nil, s.snapshotErr
	}
	results := make([]app.NoteCollectionResult, len(locations))
	for i, location := range locations {
		results[i] = app.NoteCollectionResult{
			Location: location,
			Notes:    append([]*models.Note(nil), s.byLocation[location]...),
			Err:      s.errors[location],
		}
	}
	return results, nil
}

func (s *combinedNotesService) ApplyNoteBatch(request app.NoteBatchRequest) (app.NoteBatchResult, error) {
	s.requests = append(s.requests, request)
	return app.NoteBatchResult{}, nil
}

// mountNotesManager gives a standalone Notes manager the registry dispatch
// production hands it, by making it the mounted Manage module of a typed test
// app. The browse grammar resolves through that seam, so an unmounted view
// answers none of its keys.
//
// The scope is passed rather than read back off the view: the hub's params are
// what pick the presentation the executor binds against, and every caller
// already names the scope it constructed the view with. Asking the view for it
// would need an accessor no production caller wants.
func mountNotesManager(t *testing.T, view *notes.NotesViewModel, scope route.NotesScope) *AppModel {
	t.Helper()
	model := typedTestApp(t)
	model.manageHub.MountModule(route.ManageParams{Module: route.ManageNotes, NotesScope: scope}, view)
	view.SetOperationSystem(model.GetOperationRegistry(), model.dispatchOperationKey)
	return model
}
