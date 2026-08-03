package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// The Context Hub is not the only surface that can empty a regular label: the
// project editor, archive, and delete all rewrite the same catalog from outside
// the hub. These tests drive those mutations through the real shell handlers and
// let the cross-project reload settle, so they cover the reconciliation boundary
// every catalog mutation reaches rather than the hub's own save path.

// externalContextMutationFixture builds a file-backed shell whose global
// Outliner is loaded from disk and scoped to the "work" label, which alpha is
// the only active member of.
func externalContextMutationFixture(t *testing.T) (*session4LifecycleFixture, *models.Project, *models.Project) {
	t.Helper()
	alpha := newSession4Project(t, "Alpha")
	alpha.Context = "work"
	beta := newSession4Project(t, "Beta")
	beta.Context = "home"

	fixture := newSession4LifecycleFixture(t, alpha, beta)
	fixture.cross.applyContextScope(RegularContextScope("work"))
	if ids := taskRowProjectIDs(fixture.cross); !containsProject(ids, alpha.ID) || containsProject(ids, beta.ID) {
		t.Fatalf("the work context did not narrow the Outliner first: %v", ids)
	}
	return fixture, alpha, beta
}

// assertContextScopeFellBackToAll checks the fallback end to end: the scope, the
// admitted-ID map, and the rows the user actually sees.
func assertContextScopeFellBackToAll(t *testing.T, cross *OutlinerModel, survivorID string) {
	t.Helper()
	if got := cross.activeContext; got != AllContextScope() {
		t.Fatalf("Outliner scope = %#v, want All", got)
	}
	if got := cross.contextScopeProjectIDs; got != nil {
		t.Fatalf("All must admit every project, got %#v", got)
	}
	if ids := taskRowProjectIDs(cross); !containsProject(ids, survivorID) {
		t.Fatalf("Outliner rows = %v, want the surviving project %q", ids, survivorID)
	}
}

// TestProjectRelabelOutsideHubFallsBackToAllContextScope covers the project
// editor: relabeling the active label's last member leaves that label carried by
// nothing, so the hub stops deriving it and the Outliner must not stay on it.
func TestProjectRelabelOutsideHubFallsBackToAllContextScope(t *testing.T) {
	fixture, alpha, beta := externalContextMutationFixture(t)
	live := fixture.app.projectByID(alpha.ID)
	live.Context = "home"
	session4CommitCatalogProject(t, fixture.app, live)
	fixture.app.showView(ViewProjectEditor, NewProjectEditorModel(
		live,
		fixture.app.configManager,
		fixture.app.runtime,
		fixture.app.appService,
		false,
	))

	_, cmd := fixture.app.handleProjectMessages(ProjectUpdatedMsg{})
	runSession2AppCommand(t, fixture.app, cmd)

	assertContextScopeFellBackToAll(t, fixture.cross, beta.ID)
}

// TestProjectArchiveOutsideHubFallsBackToAllContextScope covers archival, which
// empties a label without touching any project's Context field: the label is
// derived from active projects only.
func TestProjectArchiveOutsideHubFallsBackToAllContextScope(t *testing.T) {
	fixture, alpha, beta := externalContextMutationFixture(t)

	_, cmd := fixture.app.handleProjectMessages(ProjectArchivedMsg{Project: fixture.app.projectByID(alpha.ID)})
	runSession2AppCommand(t, fixture.app, cmd)

	assertContextScopeFellBackToAll(t, fixture.cross, beta.ID)
}
