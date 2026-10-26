package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

type queryNotePort struct {
	app.WorkspaceNotePort
	byLocation map[app.NoteLocation][]*models.Note
	errs       map[app.NoteLocation]error
	calls      []app.NoteLocation
}

func (p *queryNotePort) Notes(location app.NoteLocation, _ *models.Project) ([]*models.Note, error) {
	p.calls = append(p.calls, location)
	return append([]*models.Note(nil), p.byLocation[location]...), p.errs[location]
}

func TestNotesAtLocationsUsesOneSnapshotAndRetainsPerLocationErrors(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha"}
	beta := &models.Project{ID: "beta", Name: "Beta"}
	repo := newFakeRepo(alpha, beta)
	betaErr := errors.New("beta notes unavailable")
	port := &queryNotePort{
		byLocation: map[app.NoteLocation][]*models.Note{
			app.GlobalNoteLocation():         {{Filename: "global.md"}},
			app.ProjectNoteLocation("alpha"): {{Filename: "alpha.md"}},
		},
		errs: map[app.NoteLocation]error{
			app.ProjectNoteLocation("beta"): betaErr,
		},
	}
	service := app.NewServiceWithArtifacts(repo, nil, nil, port)
	missing := app.ProjectNoteLocation("missing")
	locations := []app.NoteLocation{
		app.GlobalNoteLocation(),
		app.ProjectNoteLocation("alpha"),
		app.ProjectNoteLocation("beta"),
		missing,
	}

	results, err := service.NotesAtLocations(locations)
	if err != nil {
		t.Fatalf("NotesAtLocations: %v", err)
	}
	if repo.snapshots != 1 {
		t.Fatalf("snapshots = %d, want one", repo.snapshots)
	}
	if len(results) != len(locations) {
		t.Fatalf("results = %d, want %d", len(results), len(locations))
	}
	if len(results[0].Notes) != 1 || results[0].Notes[0].Filename != "global.md" ||
		len(results[1].Notes) != 1 || results[1].Notes[0].Filename != "alpha.md" {
		t.Fatalf("successful results = %#v", results[:2])
	}
	if !errors.Is(results[2].Err, betaErr) {
		t.Fatalf("beta error = %v, want %v", results[2].Err, betaErr)
	}
	if !errors.Is(results[3].Err, app.ErrProjectNotFound) {
		t.Fatalf("missing-project error = %v, want ErrProjectNotFound", results[3].Err)
	}
	if len(port.calls) != 3 {
		t.Fatalf("note-port calls = %v, want resolved locations only", port.calls)
	}
}

func TestNotesAtLocationsSnapshotFailureIsTotal(t *testing.T) {
	snapshotErr := errors.New("workspace snapshot unavailable")
	repo := newFakeRepo()
	repo.snapErr = snapshotErr
	port := &queryNotePort{}
	service := app.NewServiceWithArtifacts(repo, nil, nil, port)

	results, err := service.NotesAtLocations([]app.NoteLocation{app.GlobalNoteLocation()})
	if !errors.Is(err, snapshotErr) || results != nil {
		t.Fatalf("results=%#v err=%v, want nil and snapshot error", results, err)
	}
	if len(port.calls) != 0 {
		t.Fatalf("note port called after snapshot failure: %v", port.calls)
	}
}

// catalogNoteRepo is a repository that can enumerate its catalog without the
// tasks, the capability a note query prefers over a full snapshot.
type catalogNoteRepo struct {
	*fakeRepo
	catalog      []*models.Project
	catalogReads int
	catalogErr   error
}

func (r *catalogNoteRepo) NoteLocationProjects() ([]*models.Project, error) {
	r.catalogReads++
	return r.catalog, r.catalogErr
}

// TestNotesAtLocationsResolvesAgainstCatalogWithoutSnapshot pins the cheap
// path: a repository offering the task-free catalog answers every location
// from it, takes no snapshot, and still refuses a project it does not hold.
func TestNotesAtLocationsResolvesAgainstCatalogWithoutSnapshot(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha"}
	repo := &catalogNoteRepo{fakeRepo: newFakeRepo(alpha), catalog: []*models.Project{alpha}}
	port := &queryNotePort{byLocation: map[app.NoteLocation][]*models.Note{
		app.ProjectNoteLocation("alpha"): {{Filename: "alpha.md"}},
	}}
	service := app.NewServiceWithArtifacts(repo, nil, nil, port)

	results, err := service.NotesAtLocations([]app.NoteLocation{
		app.ProjectNoteLocation("alpha"), app.ProjectNoteLocation("missing"),
	})
	if err != nil {
		t.Fatalf("NotesAtLocations: %v", err)
	}
	if repo.snapshots != 0 || repo.catalogReads != 1 {
		t.Fatalf("snapshots = %d, catalog reads = %d; want none and one", repo.snapshots, repo.catalogReads)
	}
	if len(results[0].Notes) != 1 || results[0].Notes[0].Filename != "alpha.md" {
		t.Fatalf("alpha result = %#v", results[0])
	}
	if !errors.Is(results[1].Err, app.ErrProjectNotFound) {
		t.Fatalf("missing-project error = %v, want ErrProjectNotFound", results[1].Err)
	}

	repo.catalogErr = errors.New("catalog unreadable")
	if _, err := service.NotesAt(app.GlobalNoteLocation()); !errors.Is(err, repo.catalogErr) {
		t.Fatalf("NotesAt error = %v, want the catalog failure", err)
	}
}
