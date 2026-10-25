package outliner

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// hopToRelation opens the picker over one task, puts the cursor on one listed
// row, and presses Enter, which is the whole of what a user does.
func hopToRelation(t *testing.T, outlinerView *OutlinerModel, projectID, taskID, rowID string) {
	t.Helper()
	relationPickerOver(t, outlinerView, projectID, taskID)
	if !outlinerView.RelationPicker.Surface.SetCursorToID(relationPickerRowID(rowID)) {
		t.Fatalf("row %q is not listed for %q", rowID, taskID)
	}
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
}

func relationPickerRowID(suffix string) listsurface.ListItemID {
	return listsurface.ListItemID(relationPickerItemPrefix + suffix)
}

func TestRelationJumpCrossesProjectsAndReturnsToTheViewItLeft(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	// The origin is a real working view rather than the launch one: a project
	// scope and a manual sort, both of which the hop replaces.
	outlinerView.PushProjectScope("proj-a")
	outlinerView.Sort = query.SortState{Field: query.SortByTitle, Ascending: true}
	outlinerView.TreeSort = outlinerView.Sort
	origin := outlinerView.Sort
	// A population facet decides what is in memory rather than what a row shows,
	// and the hop resolves strictly inside that loaded set, so it is one axis the
	// jump must leave exactly as it found it.
	outlinerView.ProjectFilter.ParkedMode = query.ProjectPopulationInclude
	adoptFilterCarriers(outlinerView)
	population := outlinerView.ProjectFilter.Clone()

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	if outlinerView.RelationPickerOpen() {
		t.Fatal("a successful hop left the picker open")
	}
	if outlinerView.Scope.ProjectID != "proj-b" {
		t.Fatalf("scope = %q, want the target's project", outlinerView.Scope.ProjectID)
	}
	if outlinerView.PrimaryMode != chrome.ModeAll {
		t.Fatalf("mode = %s, want All", outlinerView.PrimaryMode)
	}
	if outlinerView.Sort.Field != query.SortByDefault {
		t.Fatalf("sort = %+v, want the hop's cleared sort", outlinerView.Sort)
	}
	if query.ProjectFilterFingerprint(&outlinerView.ProjectFilter) != query.ProjectFilterFingerprint(&population) {
		t.Fatalf("the hop rewrote the population facet: %+v, want %+v",
			outlinerView.ProjectFilter, population)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"})
	if !outlinerView.HasRelationTrail() {
		t.Fatal("a successful hop published no return frame")
	}

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if outlinerView.Sort != origin {
		t.Fatalf("sort = %+v after the return, want %+v", outlinerView.Sort, origin)
	}
	if outlinerView.Scope.ProjectID != "proj-a" {
		t.Fatalf("scope = %q after the return, want the origin's scope", outlinerView.Scope.ProjectID)
	}
	// The return parks on the row the picker was opened over, not on a restored
	// index a rebuild may have moved.
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
	if outlinerView.HasRelationTrail() {
		t.Fatal("the return left its own frame behind")
	}
	if outlinerView.RelationPickerOpen() {
		t.Fatal("the return reopened the picker")
	}
}

func TestRelationJumpWithinOneProjectStillScopesAndReturns(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-a:sibling")
	if outlinerView.Scope.ProjectID != "proj-a" {
		t.Fatalf("scope = %q, want the same project", outlinerView.Scope.ProjectID)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "sibling"})

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if outlinerView.Scope.ProjectID != "" {
		t.Fatalf("scope = %q after the return", outlinerView.Scope.ProjectID)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
}

// TestRelationJumpRevealsTerminalAndDeferredTargets is the narrowing rule: All
// hides both populations by default, so a hop adds exactly the predicate its own
// target needs and nothing else.
func TestRelationJumpRevealsTerminalAndDeferredTargets(t *testing.T) {
	t.Run("terminal", func(t *testing.T) {
		alpha, beta := relationFixture(t)
		outlinerView := scopeTestOutliner(alpha, beta)
		hopToRelation(t, outlinerView, "proj-a", "source", "in:proj-b:closed")
		requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "closed"})
	})

	t.Run("deferred", func(t *testing.T) {
		alpha, beta := relationFixture(t)
		later := time.Now().Add(72 * time.Hour)
		beta.GetTaskByID("remote").DeferredUntil = &later
		outlinerView := scopeTestOutliner(alpha, beta)
		hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
		requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"})
	})

	t.Run("an open target adds no predicate", func(t *testing.T) {
		alpha, beta := relationFixture(t)
		outlinerView := scopeTestOutliner(alpha, beta)
		hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-a:sibling")
		if clauses := query.ClausesOfFields(outlinerView.FilterQuery, query.FilterQueryTaskFields()...); len(clauses) != 0 {
			t.Fatalf("an ordinary target installed %d task facets", len(clauses))
		}
	})
}

// TestRelationJumpExpandsTheFoldsHidingItsTarget covers the reveal path that
// runs when the first rebuild does not list the row: the enclosing section and
// project-group folds open and every unrelated fold is put back.
func TestRelationJumpExpandsTheFoldsHidingItsTarget(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)

	// Fold every section of the destination and one that has nothing to do with
	// the hop, so the assertion can tell an opened fold from a cleared map.
	outlinerView.PushProjectScope("proj-b")
	outlinerView.SelectOutlinerMode(chrome.ModeAll)
	outlinerView.RebuildRows()
	for _, row := range outlinerView.Rows {
		if row.IsSectionHeader {
			outlinerView.Folds.setSection(row.EffectiveCollapseKey(), true)
		}
	}
	outlinerView.Folds.setSection("_state:untouched-by-the-hop", true)
	outlinerView.PopProjectScope()
	outlinerView.RebuildRows()

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"})
	if !outlinerView.Folds.CollapsedSections["_state:untouched-by-the-hop"] {
		t.Fatal("the hop cleared a fold that was not hiding its target")
	}
}

// TestRelationTrailReturnsOneHopAtATime is A → B → C: each Esc undoes exactly
// one hop, in order, and the trail is empty at the origin.
func TestRelationTrailReturnsOneHopAtATime(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
	hopToRelation(t, outlinerView, "proj-b", "remote", "out:proj-a:source")
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
	if len(outlinerView.relationTrail) != 2 {
		t.Fatalf("trail holds %d frames, want two", len(outlinerView.relationTrail))
	}

	// Esc is the route the user has: the trail step sits after Peek and selection
	// in the cascade and before the search projection, and it takes one hop even
	// though the destination is scoped and would otherwise answer the key.
	listsurface.RunEscCascade(outlinerView.EscSteps())
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"})
	if outlinerView.Scope.ProjectID != "proj-b" {
		t.Fatalf("scope = %q after one return, want the middle hop", outlinerView.Scope.ProjectID)
	}
	listsurface.RunEscCascade(outlinerView.EscSteps())
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
	if outlinerView.Scope.ProjectID != "" || outlinerView.HasRelationTrail() {
		t.Fatalf("the origin was not restored: scope=%q trail=%v",
			outlinerView.Scope.ProjectID, outlinerView.HasRelationTrail())
	}
}

// TestRelationJumpSuspendsTheLiveLensAndPutsItBack is the lens rule. A hop
// replaces the projection, so the lens stops being applied; it is suspended
// rather than dismissed, and nothing machine-local is written either way.
func TestRelationJumpSuspendsTheLiveLensAndPutsItBack(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	persisted := make([]string, 0, 2)
	outlinerView.SetPersistActiveLens(func(id string) error {
		persisted = append(persisted, id)
		return nil
	})

	lens := config.NamedLens{ID: "lens-1", Name: "Waiting"}
	state := LensState{Mode: chrome.ModeAll, Query: "contract"}
	outlinerView.PublishLens(lens, state, nil, outlinerView.CaptureOutlinerViewSnapshot())
	if !outlinerView.Lens.hasActive() {
		t.Fatal("the lens did not become live")
	}
	if len(persisted) != 1 || persisted[0] != "lens-1" {
		t.Fatalf("publishing recorded %v, want the one pointer write", persisted)
	}

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
	if outlinerView.Lens.hasActive() {
		t.Fatal("the hop left the lens applied over the destination")
	}
	if outlinerView.Lens.Query != "" {
		t.Fatalf("the hop kept the lens narrowing: %q", outlinerView.Lens.Query)
	}
	if len(persisted) != 1 {
		t.Fatalf("the hop wrote the machine-local pointer: %v", persisted)
	}

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if !outlinerView.Lens.hasActive() || outlinerView.Lens.Active.Name != "Waiting" {
		t.Fatalf("the return did not put the lens back: %+v", outlinerView.Lens.Active)
	}
	if outlinerView.Lens.Query != "contract" {
		t.Fatalf("the return did not restore the lens narrowing: %q", outlinerView.Lens.Query)
	}
	if len(persisted) != 1 {
		t.Fatalf("the return wrote the machine-local pointer: %v", persisted)
	}
}

// TestResetToLaunchViewDiscardsTheRelationTrail is the `0` rule: the trail is an
// axis of its own, so a reset clears it and reports it even when the destination
// already looks like a launch view.
func TestResetToLaunchViewDiscardsTheRelationTrail(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-a:sibling")
	// Leave the destination looking exactly like a launch view, so the trail is
	// the only thing left to clear.
	outlinerView.PopProjectScope()
	outlinerView.SelectOutlinerMode(chrome.ModeAll)

	cleared, _ := outlinerView.ResetToLaunchView()
	if len(cleared) == 0 {
		t.Fatal("the reset found nothing to clear with a trail outstanding")
	}
	if outlinerView.HasRelationTrail() {
		t.Fatal("the reset left the trail standing")
	}
	named := false
	for _, axis := range cleared {
		if axis == "relationship trail" {
			named = true
		}
	}
	if !named {
		t.Fatalf("the reset did not name the trail: %v", cleared)
	}
}

// TestRelationJumpUnderASingleProjectViewRefusesInPlace keeps the one-project
// Outliner honest: it holds one project and cannot open another, so the hop
// says so and leaves the view and the trail exactly as they were.
func TestRelationJumpUnderASingleProjectViewRefusesInPlace(t *testing.T) {
	alpha, _ := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha)
	outlinerView.Project = alpha
	relationPickerOver(t, outlinerView, "proj-a", "source")

	// Beta was never loaded, so its row is disabled; the same-project row is the
	// one that proves a single-project view still navigates.
	if !outlinerView.RelationPicker.Surface.SetCursorToID(relationPickerRowID("out:proj-a:sibling")) {
		t.Fatal("the same-project target is not listed")
	}
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if outlinerView.RelationPickerOpen() {
		t.Fatalf("a same-project hop was refused: %q", outlinerView.RelationPicker.Notice)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "sibling"})
}

// TestRelationJumpLeavesAnArchivedTargetReadOnly keeps a loaded archived project
// reachable and untouched: the hop is presentation, and it writes nothing.
func TestRelationJumpLeavesAnArchivedTargetReadOnly(t *testing.T) {
	alpha, beta := relationFixture(t)
	beta.Archived = true
	outlinerView := scopeTestOutliner(alpha, beta)

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
	if !beta.Archived {
		t.Fatal("the hop unarchived its destination")
	}
	if got := beta.GetTaskByID("remote"); got.State() != models.StateWaiting {
		t.Fatalf("the hop moved the target task to %s", got.State())
	}
}

// TestRelationJumpRestoresThePresentationWhenRevelationFails is the other half
// of "publish only after landing": a target the destination cannot show leaves
// the view, the lens, and the trail exactly as the hop found them. The subject
// is a task with no identity, which is the one revelation failure a test can
// stage without inventing a population the index could not have produced.
func TestRelationJumpRestoresThePresentationWhenRevelationFails(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.PushProjectScope("proj-a")
	outlinerView.Sort = query.SortState{Field: query.SortByTitle, Ascending: true}
	outlinerView.TreeSort = outlinerView.Sort
	origin := outlinerView.Sort
	relationPickerOver(t, outlinerView, "proj-a", "source")

	if refusal := outlinerView.jumpToRelation(&models.Task{Title: "no identity"}, beta); refusal == "" {
		t.Fatal("a target with no identity was navigated to")
	}
	if outlinerView.HasRelationTrail() {
		t.Fatal("a failed hop published a return frame")
	}
	if outlinerView.Scope.ProjectID != "proj-a" {
		t.Fatalf("scope = %q after a failed hop, want the origin's", outlinerView.Scope.ProjectID)
	}
	if outlinerView.Sort != origin {
		t.Fatalf("sort = %+v after a failed hop, want %+v", outlinerView.Sort, origin)
	}
}
