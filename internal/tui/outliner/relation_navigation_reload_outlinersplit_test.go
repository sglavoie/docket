package outliner

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// A relationship hop is presentation, but the workspace underneath it is not
// still. These are the crossings: a read already in flight when the key was
// pressed, a reload that landed while the user was away, a project that stopped
// being available, and a lens that was suspended before any of it happened. What
// they share is a restoration point — a return frame, a lens baseline, or a
// captured reload cursor — that was written against a reading of the workspace
// the view has since replaced. A restoration point may hand back a view; it may
// never hand back data.

// reloadedRelationFixture is the same two projects read again, as a disk reload
// produces them: fresh pointers, and two titles changed so a stale reading is
// visible in an assertion rather than merely possible.
func reloadedRelationFixture(t *testing.T) (*models.Project, *models.Project) {
	t.Helper()
	alpha, beta := relationFixture(t)
	alpha.GetTaskByID("sibling").Title = "Agree the schema (edited)"
	beta.GetTaskByID("remote").Title = "Publish the contract (edited)"
	return alpha, beta
}

// loadedTaskTitle reads one title out of the live population, which is the
// reading the view renders from.
func loadedTaskTitle(t *testing.T, outlinerView *OutlinerModel, projectID, taskID string) string {
	t.Helper()
	for _, project := range outlinerView.Population.Projects {
		if project == nil || project.ID != projectID {
			continue
		}
		task := project.GetTaskByID(taskID)
		if task == nil {
			t.Fatalf("project %q holds no task %q", projectID, taskID)
		}
		return task.Title
	}
	t.Fatalf("population holds no project %q", projectID)
	return ""
}

// loadedProject returns one member of the live population, or nil when the
// population no longer holds it. Membership is the question a baseline restore
// answers, so an assertion about it must be able to see an absence.
func loadedProject(outlinerView *OutlinerModel, projectID string) *models.Project {
	for _, project := range outlinerView.Population.Projects {
		if project != nil && project.ID == projectID {
			return project
		}
	}
	return nil
}

// loadedProjectIDs names the live population in order.
func loadedProjectIDs(outlinerView *OutlinerModel) []string {
	ids := make([]string, 0, len(outlinerView.Population.Projects))
	for _, project := range outlinerView.Population.Projects {
		if project != nil {
			ids = append(ids, project.ID)
		}
	}
	return ids
}

// landCrossProjects delivers a completed read at one generation.
func landCrossProjects(
	outlinerView *OutlinerModel, generation int, projects ...*models.Project,
) {
	outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
		Projects:   projects,
		Generation: generation,
	})
}

// landReloadedFixture is the ordinary case: the whole workspace read again.
func landReloadedFixture(t *testing.T, outlinerView *OutlinerModel, generation int) {
	t.Helper()
	alpha, beta := reloadedRelationFixture(t)
	landCrossProjects(outlinerView, generation, alpha, beta)
}

// TestRelationReturnResolvesAgainstCurrentData is the whole of what a return
// frame is allowed to be: a description of a view. The tasks changed while the
// user was away, and the return shows the change rather than the frame.
func TestRelationReturnResolvesAgainstCurrentData(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	landReloadedFixture(t, outlinerView, outlinerView.Population.LoadGen)
	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}

	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
	if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema (edited)" {
		t.Fatalf("the return put a stale reading back: sibling = %q", got)
	}
	if got := loadedTaskTitle(t, outlinerView, "proj-b", "remote"); got != "Publish the contract (edited)" {
		t.Fatalf("the return put a stale reading back: remote = %q", got)
	}
}

// TestRelationReturnSeatsANearestSurvivorWhenTheSourceIsDeleted is the fallback
// the checkpoint already owns, held to over a hop: the task the picker was
// opened over was deleted while the user was visiting, so the return lands on a
// row rather than on nothing.
func TestRelationReturnSeatsANearestSurvivorWhenTheSourceIsDeleted(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	survivingAlpha, reloadedBeta := reloadedRelationFixture(t)
	survivingAlpha.Tasks = []*models.Task{survivingAlpha.GetTaskByID("sibling")}
	survivingAlpha.BuildTaskTree()
	landCrossProjects(outlinerView, outlinerView.Population.LoadGen, survivingAlpha, reloadedBeta)

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	identity := outlinerView.CurrentTaskIdentity()
	if identity.TaskID == "source" {
		t.Fatal("the return seated a task the workspace no longer holds")
	}
	if identity.TaskID == "" {
		t.Fatalf("the return seated no row at all (cursor=%d rows=%d)",
			outlinerView.Cursor, len(outlinerView.Rows))
	}
}

// TestRelationReturnReconcilesAScopeWhoseProjectWentAway covers the origin
// leaving the population entirely — archived, parked, or deleted while the user
// was away. Reinstating the frame as written would stand the view inside a
// project the workspace no longer holds, so the return runs the same repair the
// reload path owns and says so.
func TestRelationReturnReconcilesAScopeWhoseProjectWentAway(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.PushProjectScope("proj-a")
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	_, reloadedBeta := reloadedRelationFixture(t)
	landCrossProjects(outlinerView, outlinerView.Population.LoadGen, reloadedBeta)

	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	if outlinerView.IsProjectScoped() {
		t.Fatalf("the return reinstated a scope on the missing %q", outlinerView.Scope.ProjectID)
	}
	if len(outlinerView.Rows) == 0 {
		t.Fatal("the return landed on an empty list")
	}
	if !strings.Contains(outlinerView.StatusMessage, "returned to the base scope") {
		t.Fatalf("the repair went unreported: %q", outlinerView.StatusMessage)
	}
	if !outlinerView.StatusIsTransient() {
		t.Fatal("the repair notice is sticky, and a return raises no Cmd to clear it")
	}
}

// TestAReadInFlightCannotOverwriteARelationDestination is the stale-cursor
// crossing. The read was started before the key was pressed and is carrying the
// origin's position; it is still wanted, because its projects are the newest
// reading there is, but landing it must not undo the hop.
func TestAReadInFlightCannotOverwriteARelationDestination(t *testing.T) {
	for _, testCase := range []struct {
		name string
		row  string
		want taskref.TaskIdentity
	}{
		{
			name: "across projects",
			row:  "out:proj-b:remote",
			want: taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"},
		},
		{
			name: "within one project",
			row:  "out:proj-a:sibling",
			want: taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "sibling"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			alpha, beta := relationFixture(t)
			outlinerView := scopeTestOutliner(alpha, beta)
			outlinerView.MoveCursorToProjectTask("source", "proj-a")
			// The reload captures the origin's cursor on its way out.
			_ = outlinerView.ReloadCrossProjects()
			generation := outlinerView.Population.LoadGen

			hopToRelation(t, outlinerView, "proj-a", "source", testCase.row)
			landReloadedFixture(t, outlinerView, generation)

			requireCursorTask(t, outlinerView, testCase.want)
			// The read was re-aimed, never restarted: its projects are on screen.
			if got := loadedTaskTitle(t, outlinerView, "proj-b", "remote"); got != "Publish the contract (edited)" {
				t.Fatalf("the re-aimed read published nothing: remote = %q", got)
			}
		})
	}
}

// TestAReadInFlightCannotOverwriteARestoredReturn is the same crossing on the
// way back: the read was started at the destination and would drag the cursor
// out of the origin the return frame just restored.
func TestAReadInFlightCannotOverwriteARestoredReturn(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-a:sibling")

	_ = outlinerView.ReloadCrossProjects()
	generation := outlinerView.Population.LoadGen
	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}
	landReloadedFixture(t, outlinerView, generation)

	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
}

// TestAReadInFlightCannotOverwriteAReset is the third act that replaces the
// whole projection rather than moving within it. `0` promised a launch view, and
// a read landing behind it must not put the discarded position back.
func TestAReadInFlightCannotOverwriteAReset(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-a:sibling")

	_ = outlinerView.ReloadCrossProjects()
	generation := outlinerView.Population.LoadGen
	outlinerView.ResetToLaunchView()
	reset := outlinerView.CurrentTaskIdentity()

	landReloadedFixture(t, outlinerView, generation)
	if got := outlinerView.CurrentTaskIdentity(); got != reset {
		t.Fatalf("a read landing after the reset moved the cursor to %+v, want %+v", got, reset)
	}
	if outlinerView.HasRelationTrail() {
		t.Fatal("a read landing after the reset resurrected the trail")
	}
}

// TestALensApplicationInFlightCannotPublishOverAHop closes the delayed-result
// crossing on the lens side. The hop suspends the frame, and the suspension has
// to invalidate the read the application armed as well, or a lens the user left
// behind installs itself over the destination.
func TestALensApplicationInFlightCannotPublishOverAHop(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.SetPersistActiveLens(func(string) error { return nil })

	// A project facet the live population was not read under is what makes the
	// application asynchronous rather than immediate.
	state := LensState{Mode: chrome.ModeAll, Sort: query.NewSortState()}
	lens := state.NamedLens("6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6d01", "Parked included")
	lens.Query = "+parked"
	if _, err := outlinerView.ApplyNamedLens(lens); err != nil {
		t.Fatalf("ApplyNamedLens: %v", err)
	}
	if !outlinerView.Lens.hasPending() {
		t.Fatal("the application published without reading a population")
	}
	generation := outlinerView.Lens.Pending.Generation

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")

	reloadedAlpha, reloadedBeta := reloadedRelationFixture(t)
	if _, published := outlinerView.HandleLensProjectsLoaded(LensProjectsLoadedMsg{
		Projects:   []*models.Project{reloadedAlpha, reloadedBeta},
		Generation: generation,
	}); published {
		t.Fatal("a lens read landing after the hop installed itself over the destination")
	}
	if outlinerView.Lens.hasActive() {
		t.Fatal("a lens read landing after the hop went live")
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"})
}

// TestAnAbandonedParkedFocusLoadPublishesNothing is the parked exception's own
// crossing. The focus request was abandoned when the hop pushed its scope, so
// its read carries a project the live view has not asked for: publishing it
// would open a parked project nobody is looking at and take the cursor with it.
func TestAnAbandonedParkedFocusLoadPublishesNothing(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	parked := renderFixtureProject(t, "proj-p", "parked", "Parked", []*models.Task{
		renderFixtureTask(t, "parked-task", 1, "Parked work", models.StateBacklog, 1, models.StateBacklog),
	})
	parked.Tags = []string{"parked"}
	setTestProjects(outlinerView.Runtime, []*models.Project{alpha, beta, parked})

	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	_ = outlinerView.BeginParkedProjectFocus("proj-p")
	generation := outlinerView.Population.LoadGen

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
	if outlinerView.Scope.PendingParkedID != "" {
		t.Fatalf("the hop left the focus request outstanding: %q", outlinerView.Scope.PendingParkedID)
	}

	outlinerView.HandleCrossProjectsLoaded(CrossProjectsLoadedMsg{
		Projects:       []*models.Project{alpha, beta, parked},
		Generation:     generation,
		FocusProjectID: "proj-p",
	})

	if outlinerView.Scope.ProjectID != "proj-b" {
		t.Fatalf("the abandoned focus pushed a scope: %q", outlinerView.Scope.ProjectID)
	}
	if outlinerView.Scope.ParkedProjectID != "" {
		t.Fatalf("the abandoned focus installed an exception: %q", outlinerView.Scope.ParkedProjectID)
	}
	if outlinerView.Population.Loading {
		t.Fatal("the abandoned read was never put down")
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-b", TaskID: "remote"})
	if !outlinerView.HasRelationTrail() {
		t.Fatal("the abandoned focus discarded the return trail")
	}
}

// originStateFacet is one row facet armed the way a filter chord arms it, so the
// axis under test is the one the checkpoint carries rather than a field poked
// into place.
func originStateFacet(outlinerView *OutlinerModel, state models.WorkflowState) {
	clauses := query.AppendFilterClause(nil, models.QueryFieldState,
		models.QueryOperatorIs, state.String())
	outlinerView.ReplaceFilterClauses(query.FilterQueryTaskFields(), clauses)
}

// stateFacetValues reads the armed state facet back as its values, so an
// assertion names what the user narrowed to.
func stateFacetValues(outlinerView *OutlinerModel) []string {
	values := make([]string, 0, 2)
	for _, clause := range query.ClausesOfFields(outlinerView.FilterQuery, models.QueryFieldState) {
		values = append(values, clause.Value)
	}
	return values
}

// TestAHopReplacesTheOriginRowFacetsAndTheReturnPutsThemBack is the changed-
// population-filter crossing. The hop owns the row facets while it is standing
// on its target — it installs the at most two predicates that target needs and
// nothing else — so the origin's narrowing has to come back from the frame, and
// come back over the rows as they now read rather than as they read when it was
// captured.
func TestAHopReplacesTheOriginRowFacetsAndTheReturnPutsThemBack(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.PushProjectScope("proj-a")
	// Doing selects the source and drops its sibling, so the facet is observable
	// in the rows and the return still has the row it departed from to land on.
	originStateFacet(outlinerView, models.StateDoing)
	outlinerView.MoveCursorToProjectTask("source", "proj-a")

	hopToRelation(t, outlinerView, "proj-a", "source", "out:proj-b:remote")
	// Waiting is neither terminal nor deferred, so the target needs no predicate
	// at all and the origin's narrowing is simply gone.
	if got := stateFacetValues(outlinerView); len(got) != 0 {
		t.Fatalf("the hop carried the origin's state facet into the destination: %v", got)
	}

	landReloadedFixture(t, outlinerView, outlinerView.Population.LoadGen)
	if !outlinerView.PopRelationReturn() {
		t.Fatal("the return refused")
	}

	got := stateFacetValues(outlinerView)
	if len(got) != 1 || got[0] != models.StateDoing.String() {
		t.Fatalf("the return put back %v, want the origin's doing facet", got)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == "sibling" {
			t.Fatal("the restored facet is not selecting rows")
		}
	}
	// The facet is the frame's; the reading it selects over is not. The sibling was
	// retitled while the user was away, and the population the restored narrowing
	// runs against has to be that one.
	if got := loadedTaskTitle(t, outlinerView, "proj-a", "sibling"); got != "Agree the schema (edited)" {
		t.Fatalf("the restored facet is selecting over a stale reading: sibling = %q", got)
	}
}

// TestAHopRefusesATargetThatLeftThePopulationWhileThePickerWasOpen is the
// archived-target crossing. Archiving, parking, or deleting the target's project
// all reach the view the same way: the next read simply does not carry it. The
// picker was drawn while it was still there, so the row is on screen and the key
// press is legitimate; the resolution behind it is not, and a hop that cannot
// land must leave the view and the trail exactly as it found them.
func TestAHopRefusesATargetThatLeftThePopulationWhileThePickerWasOpen(t *testing.T) {
	alpha, beta := relationFixture(t)
	outlinerView := scopeTestOutliner(alpha, beta)
	outlinerView.PushProjectScope("proj-a")
	outlinerView.MoveCursorToProjectTask("source", "proj-a")
	relationPickerOver(t, outlinerView, "proj-a", "source")
	if !outlinerView.RelationPicker.Surface.SetCursorToID(relationPickerRowID("out:proj-b:remote")) {
		t.Fatal("the cross-project target is not listed")
	}

	// The target's project is archived: the read that lands carries the origin
	// alone, and the row the cursor is sitting on now names nothing.
	reloadedAlpha, _ := reloadedRelationFixture(t)
	landCrossProjects(outlinerView, outlinerView.Population.LoadGen, reloadedAlpha)

	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	if outlinerView.RelationPicker.Notice != relationStaleTargetRefusal {
		t.Fatalf("notice = %q, want %q", outlinerView.RelationPicker.Notice, relationStaleTargetRefusal)
	}
	if outlinerView.HasRelationTrail() {
		t.Fatal("a refused hop published a return frame")
	}
	if outlinerView.Scope.ProjectID != "proj-a" {
		t.Fatalf("a refused hop moved the scope to %q", outlinerView.Scope.ProjectID)
	}
	requireCursorTask(t, outlinerView, taskref.TaskIdentity{ProjectID: "proj-a", TaskID: "source"})
}
