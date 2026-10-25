package outliner

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// TestARestoredBaselineKeepsTheParkedOriginItNamed is that rule's exception, and
// the reason the eligibility asked of a live member is the frame's own rather
// than the base workspace's. A view standing in an explicitly focused parked
// project reads it whatever the parked axis says, so the frame naming it is
// entitled to hand it back — the axis withdrew nothing here, the user did the
// opposite and named it.
func TestARestoredBaselineKeepsTheParkedOriginItNamed(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := parkedOriginOutliner(t, alpha, beta)
	outlinerView.SetPersistActiveLens(func(string) error { return nil })

	state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
	outlinerView.PublishLens(state.NamedLens("lens-1", "Trail"), state, nil,
		outlinerView.CaptureOutlinerViewSnapshot())

	outlinerView.RestoreLensBaseline()
	if loadedProject(outlinerView, "proj-a") == nil {
		t.Fatalf("the baseline dropped the parked project its own scope names: %v",
			loadedProjectIDs(outlinerView))
	}
}

// parkedOriginOutliner is the settled parked exception: one parked project the
// base workspace does not admit, focused by name the way P focuses it, with the
// load that admits it already landed and its scope already pushed.
func parkedOriginOutliner(t *testing.T, alpha, beta *models.Project) *OutlinerModel {
	t.Helper()
	alpha.Tags = []string{models.ParkedProjectTag}
	outlinerView := scopeTestOutliner(alpha, beta)
	_ = outlinerView.BeginParkedProjectFocus("proj-a")
	outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
		Projects:       []*models.Project{alpha, beta},
		Generation:     outlinerView.Population.LoadGen,
		FocusProjectID: "proj-a",
	})
	if outlinerView.Scope.ProjectID != "proj-a" || outlinerView.Scope.ParkedProjectID != "proj-a" {
		t.Fatalf("the focus did not settle: scope = %q exception = %q",
			outlinerView.Scope.ProjectID, outlinerView.Scope.ParkedProjectID)
	}
	return outlinerView
}

// TestARelationReturnKeepsAnExplicitlyFocusedParkedOrigin is the parked
// exception's other crossing. The origin is in the snapshot only because the
// user named it, and a hop is a visit with a promised way back, so the exception
// is retained across it rather than shed the way leaving a focus sheds it. The
// return is owed the view it left, and a parked project the workspace still
// holds is not a project that went away.
func TestARelationReturnKeepsAnExplicitlyFocusedParkedOrigin(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := parkedOriginOutliner(t, alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"})

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if outlinerView.Scope.ProjectID != "proj-a" {
		t.Fatalf("the return landed in scope %q, want the parked origin", outlinerView.Scope.ProjectID)
	}
	if outlinerView.ScopedProject() == nil {
		t.Fatal("the return restored the origin's scope without the project behind it")
	}
	if outlinerView.Scope.ParkedProjectID != "proj-a" {
		t.Fatalf("the exception admitting the origin is gone: %q", outlinerView.Scope.ParkedProjectID)
	}
	if strings.Contains(outlinerView.StatusMessage, "no longer exists") {
		t.Fatalf("the return reported a repair it did not need: %q", outlinerView.StatusMessage)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
	// The visited project is off screen again: the restored scope is the origin's.
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.ProjectID == "proj-b" {
			t.Fatal("the return kept the visited project's tasks on screen")
		}
	}
}

// TestARetainedParkedOriginKeepsReadingCurrentData is what retaining the
// exception buys the return: the origin is still the project every later read
// re-requests, so a reload landing at the destination refreshes it and the
// return shows that reading rather than the one the hop departed from.
func TestARetainedParkedOriginKeepsReadingCurrentData(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := parkedOriginOutliner(t, alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	// A steady-state reload lands at the destination carrying the adopted focus,
	// which is the way an ordinary refresh reaches a parked project at all.
	reloadedAlpha, reloadedBeta := reloadedRelationFixture(t)
	reloadedAlpha.Tags = []string{models.ParkedProjectTag}
	outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
		Projects:               []*models.Project{reloadedAlpha, reloadedBeta},
		Generation:             outlinerView.Population.LoadGen,
		retainedFocusProjectID: "proj-a",
	})
	if outlinerView.Scope.ParkedProjectID != "proj-a" {
		t.Fatalf("a reload at the destination retired the exception: %q",
			outlinerView.Scope.ParkedProjectID)
	}

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema (edited)" {
		t.Fatalf("the return put a stale reading of the parked origin back: sibling = %q", got)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
}

// TestARelationReturnFallsBackWhenTheParkedOriginWentAway is the other half of
// that rule. Retaining the exception is not a promise the project survived, so a
// parked origin that was deleted, archived, or made unreadable while the user was
// away still unwinds to the base scope and says so.
func TestARelationReturnFallsBackWhenTheParkedOriginWentAway(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := parkedOriginOutliner(t, alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	// The origin is gone: the read that lands carries the visited project alone.
	_, reloadedBeta := reloadedRelationFixture(t)
	setTestProjects(outlinerView.Runtime, []*models.Project{reloadedBeta})
	landCrossProjects(outlinerView, outlinerView.Population.LoadGen, reloadedBeta)

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if outlinerView.IsProjectScoped() {
		t.Fatalf("the return reinstated a scope on the missing %q", outlinerView.Scope.ProjectID)
	}
	if outlinerView.Scope.ParkedProjectID != "" {
		t.Fatalf("the repair left an exception re-requesting a project that is gone: %q",
			outlinerView.Scope.ParkedProjectID)
	}
	if !strings.Contains(outlinerView.StatusMessage, "returned to the base scope") {
		t.Fatalf("the repair went unreported: %q", outlinerView.StatusMessage)
	}
}
