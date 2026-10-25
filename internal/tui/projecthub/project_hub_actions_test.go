package projecthub

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/operation"
)

func containsOperationID(ids []operation.OperationID, want operation.OperationID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// TestProjectHubActiveProjectOffersOnlyArchive holds the Space menu to what the
// direct grammar cannot reach. Opening, editing, pinning, tagging and notes are
// all one press away, so a single active row offers archiving alone; the four
// bulk tag actions need a selection and drop out of a one-row sheet.
func TestProjectHubActiveProjectOffersOnlyArchive(t *testing.T) {
	hub := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	).open()
	focusProjectHubItem(t, hub, "project-hub:project:beta")
	hub.Update(syntheticKey(' '))

	got := projectHubActionIDs(hub)
	if len(got) != 1 || !containsOperationID(got, operation.OperationScopeArchiveProject) {
		t.Fatalf("active project actions = %v, want archive alone", got)
	}
}

// TestProjectHubArchivedProjectSheetOmitsNotes is the sheet half of the archived
// project's notes refusal: the operation's lifecycle requirement is what keeps it
// off the row, so the entry never appears for the user to run. The handler half,
// which refuses the same operation dispatched from any host, is asserted beside
// the shell that owns it.
func TestProjectHubArchivedProjectSheetOmitsNotes(t *testing.T) {
	fixture := newProjectHubFixture(t, scopeTestProject("alpha", "Alpha", "1", "2"))
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	fixture.add(archived)

	hub := fixture.open()
	focusProjectHubItem(t, hub, "project-hub:archived:archived")
	hub.Update(syntheticKey(' '))
	if got := projectHubActionIDs(hub); containsOperationID(got, operation.OperationScopeOpenProjectNote) {
		t.Fatalf("archived project offered Open project notes: %v", got)
	}
}
