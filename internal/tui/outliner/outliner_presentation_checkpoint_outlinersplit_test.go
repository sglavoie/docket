package outliner

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// checkpointQuery parses one canonical narrowing for a checkpoint fixture. The
// tests spell their predicates rather than authoring carriers, because the
// clauses they care about — a negation, both dependency directions at once — are
// exactly the ones no carrier can hold.
func checkpointQuery(t *testing.T, spelling string) models.Query {
	t.Helper()
	parsed, err := models.ParseQuery(spelling)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", spelling, err)
	}
	return parsed
}

func queryParts(narrowing models.Query) []string {
	return query.FilterClauseParts(narrowing.Canonical().Clauses)
}

func checkpointOutliner() *OutlinerModel {
	first := primaryRowsTask("first", models.StateBacklog)
	second := primaryRowsTask("second", models.StateBacklog)
	third := primaryRowsTask("third", models.StateInbox)
	return scopeTestOutliner(primaryRowsProject("alpha", first, second, third))
}

// TestPresentationCheckpointRoundTripsNegativeAndDependencyClauses is the whole
// reason the checkpoint carries the canonical query rather than the carrier
// caches derived from it. Both of these clauses survive the round trip only
// because the predicate itself is what was captured: the row carrier drops a
// negation outright and collapses the dependency axis to one enum, so a frame
// lifted back through it would return narrowed by something the user never
// asked for.
func TestPresentationCheckpointRoundTripsNegativeAndDependencyClauses(t *testing.T) {
	outlinerView := checkpointOutliner()
	captured := checkpointQuery(t, "#keep -#legacy ?blocked ?blocking !2")
	outlinerView.SetFilterQuery(captured)
	want := queryParts(outlinerView.FilterQuery)

	checkpoint := outlinerView.CaptureOutlinerPresentationCheckpoint()
	outlinerView.SetFilterQuery(checkpointQuery(t, "$done"))
	outlinerView.RestoreOutlinerPresentationCheckpoint(checkpoint)

	if got := queryParts(outlinerView.FilterQuery); !slices.Equal(got, want) {
		t.Fatalf("restored query = %v, want %v", got, want)
	}
	// The carriers are derived, so a lossless predicate restore is also what puts
	// the chips and pickers back in agreement with it.
	if !slices.Equal(
		queryParts(models.Query{Clauses: query.CommonFilterClauses(&outlinerView.Filter.CommonFilterState)}),
		queryParts(models.Query{Clauses: query.CommonFilterClauses(
			ptr(query.CommonFilterStateFromQuery(outlinerView.FilterQuery)))}),
	) {
		t.Fatalf("the row carrier disagrees with the restored predicate: %+v", outlinerView.Filter)
	}
}

func ptr[T any](value T) *T { return &value }

// TestPresentationCheckpointKeepsTheLivePopulationHalf pins the split the
// checkpoint is named for: the projection clauses are the frame's and come back
// exactly, while the population clauses name projects that were loaded and stay
// whatever the live view is standing in. Restoring the captured population half
// here would leave the query claiming a population nothing went and read.
func TestPresentationCheckpointKeepsTheLivePopulationHalf(t *testing.T) {
	outlinerView := checkpointOutliner()
	outlinerView.SetFilterQuery(checkpointQuery(t, "#keep +parked:only"))

	checkpoint := outlinerView.CaptureOutlinerPresentationCheckpoint()
	outlinerView.SetFilterQuery(checkpointQuery(t, "$done +archived"))
	outlinerView.RestoreOutlinerPresentationCheckpoint(checkpoint)

	if got := queryParts(outlinerView.FilterQuery); !slices.Equal(got, []string{"#keep", "+archived"}) {
		t.Fatalf("restored query = %v, want the captured row half over the live population half", got)
	}
	if outlinerView.ProjectFilter.ParkedMode != query.ProjectPopulationExclude {
		t.Fatalf("restoration reinstated a population facet: %+v", outlinerView.ProjectFilter)
	}
}

// TestOutlinerViewSnapshotRestoresThePopulationHalfBesideItsProjects is the
// other side of that split: a lens is the one caller holding projects to put
// back, and the facets that selected them travel with them.
func TestOutlinerViewSnapshotRestoresThePopulationHalfBesideItsProjects(t *testing.T) {
	parked := primaryRowsProject("parked", primaryRowsTask("parked-task", models.StateBacklog))
	outlinerView := checkpointOutliner()
	outlinerView.SetFilterQuery(checkpointQuery(t, "#keep +parked:only"))
	baselineProjects := loadedProjectIDs(outlinerView)

	snapshot := outlinerView.CaptureOutlinerViewSnapshot()
	outlinerView.SetFilterQuery(checkpointQuery(t, "$done +archived"))
	outlinerView.setLoadedProjects([]*models.Project{parked})
	outlinerView.restoreOutlinerViewSnapshot(snapshot)

	if got := queryParts(outlinerView.FilterQuery); !slices.Equal(got, []string{"#keep", "+parked:only"}) {
		t.Fatalf("restored query = %v, want the captured predicate whole", got)
	}
	// Membership is what a baseline speaks for, so this is an identity comparison
	// by project and not by pointer: a member the live population no longer holds
	// comes back as the frame retained it under the catalog's current description
	// of the project, which is a different object from the one captured.
	if got := loadedProjectIDs(outlinerView); !slices.Equal(got, baselineProjects) {
		t.Fatalf("restored population = %v, want the captured projects %v", got, baselineProjects)
	}
}

// TestPresentationCheckpointRestoresRankedAndOrdinarySearch covers both search
// presentations. The checkpoint carries the search cluster, so a frame taken
// while a query was held comes back holding it — and comes back holding it in
// the same presentation, because the ranked flag is search's own state and not a
// second axis the checkpoint copies.
func TestPresentationCheckpointRestoresRankedAndOrdinarySearch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ranked bool
	}{{name: "ordinary", ranked: false}, {name: "ranked", ranked: true}} {
		t.Run(tc.name, func(t *testing.T) {
			outlinerView := checkpointOutliner()
			if tc.ranked {
				searchRankedList(outlinerView, "first")
			} else {
				outlinerView.SetSearchQuery("first")
			}
			wantRows := searchVisibleTaskIDs(outlinerView)

			checkpoint := outlinerView.CaptureOutlinerPresentationCheckpoint()
			outlinerView.ClearSearch()
			if outlinerView.HasSearchProjection() {
				t.Fatal("ClearSearch left the projection open")
			}
			outlinerView.RestoreOutlinerPresentationCheckpoint(checkpoint)

			if !outlinerView.HasSearchProjection() {
				t.Fatal("restoration dropped the held query")
			}
			if outlinerView.ShowsRankedList() != tc.ranked {
				t.Fatalf("restored ranked presentation = %v, want %v",
					outlinerView.ShowsRankedList(), tc.ranked)
			}
			if got := searchVisibleTaskIDs(outlinerView); !slices.Equal(got, wantRows) {
				t.Fatalf("restored rows = %v, want %v", got, wantRows)
			}
		})
	}
}

// TestPresentationCheckpointRestoresFoldsPeekAndScope walks the axes that are
// neither the mode nor the query: they are carried by their own clusters, and
// this is what proves the checkpoint asks each one rather than remembering a
// list of fields.
func TestPresentationCheckpointRestoresFoldsPeekAndScope(t *testing.T) {
	outlinerView := checkpointOutliner()
	outlinerView.SelectOutlinerMode(chrome.ModeInbox)
	outlinerView.PushProjectScope("alpha")
	outlinerView.Folds.CollapsedSections["frozen-section"] = true
	outlinerView.Folds.ExpandedNodes[taskref.TaskIdentity{ProjectID: "alpha", TaskID: "first"}] = true
	outlinerView.Folds.ExpandedChecklists[taskref.TaskIdentity{ProjectID: "alpha", TaskID: "second"}] = true
	outlinerView.PeekMode = true
	outlinerView.Selected[taskref.TaskIdentity{ProjectID: "alpha", TaskID: "third"}] = true
	wantSections := maps.Clone(outlinerView.Folds.CollapsedSections)
	wantNodes := maps.Clone(outlinerView.Folds.ExpandedNodes)
	wantChecklists := maps.Clone(outlinerView.Folds.ExpandedChecklists)

	checkpoint := outlinerView.CaptureOutlinerPresentationCheckpoint()
	outlinerView.SelectOutlinerMode(chrome.ModeAll)
	outlinerView.ClearAllFolds()
	outlinerView.PeekMode = false
	outlinerView.ClearSelection()
	outlinerView.Scope.clearProject()
	outlinerView.RestoreOutlinerPresentationCheckpoint(checkpoint)

	if outlinerView.PrimaryMode != chrome.ModeInbox {
		t.Fatalf("restored mode = %v, want %v", outlinerView.PrimaryMode, chrome.ModeInbox)
	}
	if !maps.Equal(outlinerView.Folds.CollapsedSections, wantSections) ||
		!maps.Equal(outlinerView.Folds.ExpandedNodes, wantNodes) ||
		!maps.Equal(outlinerView.Folds.ExpandedChecklists, wantChecklists) {
		t.Fatalf("restored folds differ: sections=%v nodes=%v checklists=%v",
			outlinerView.Folds.CollapsedSections, outlinerView.Folds.ExpandedNodes,
			outlinerView.Folds.ExpandedChecklists)
	}
	if !outlinerView.PeekMode {
		t.Fatal("restoration dropped Peek")
	}
	if !outlinerView.Selected[taskref.TaskIdentity{ProjectID: "alpha", TaskID: "third"}] {
		t.Fatal("restoration dropped the selection")
	}
	if outlinerView.Scope.ProjectID != "alpha" {
		t.Fatalf("restored scope = %q, want alpha", outlinerView.Scope.ProjectID)
	}
}

// TestPresentationCheckpointRestoresRecentFrozenOrdering proves the freeze
// travels as a freeze. Recent reports what happened, so a checkpoint that came
// back with the timestamps re-read would reshuffle the list the user was
// triaging out from under them.
func TestPresentationCheckpointRestoresRecentFrozenOrdering(t *testing.T) {
	stale := recentTask("stale", models.StateBacklog, 3)
	fresh := recentTask("fresh", models.StateBacklog, 1)
	outlinerView := scopeTestOutliner(primaryRowsProject("alpha", stale, fresh))
	outlinerView.SelectOutlinerMode(chrome.ModeRecent)
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	identity := taskref.TaskIdentity{ProjectID: "alpha", TaskID: "stale"}
	frozen, ok := outlinerView.Recency.frozenAt(identity)
	if !ok {
		t.Fatal("Recent did not freeze the seeded activity")
	}

	checkpoint := outlinerView.CaptureOutlinerPresentationCheckpoint()
	outlinerView.Recency.thaw()
	stale.UpdatedAt = time.Now()
	outlinerView.RestoreOutlinerPresentationCheckpoint(checkpoint)

	restored, ok := outlinerView.Recency.frozenAt(identity)
	if !ok || !restored.Equal(frozen) {
		t.Fatalf("restored freeze = %v/%v, want the captured %v", restored, ok, frozen)
	}
	if outlinerView.Recency.Projection != checkpoint.recency.Projection {
		t.Fatalf("restored freeze projection = %q, want %q",
			outlinerView.Recency.Projection, checkpoint.recency.Projection)
	}
}

// TestPresentationCheckpointsOwnTheirMapsAndSlices is the independence rule. A
// checkpoint is taken so it can be restored later, possibly more than once, so
// nothing it carries may alias state the live model goes on to edit.
func TestPresentationCheckpointsOwnTheirMapsAndSlices(t *testing.T) {
	outlinerView := checkpointOutliner()
	outlinerView.SetFilterQuery(checkpointQuery(t, "#keep -#legacy"))
	outlinerView.Folds.CollapsedSections["frozen-section"] = true
	outlinerView.Focus.CrossStack = []CrossProjectFocusEntry{{ProjectID: "alpha", TaskID: "first"}}

	checkpoint := outlinerView.CaptureOutlinerPresentationCheckpoint()
	// Every write below is on state the checkpoint read, through the live model.
	outlinerView.Folds.CollapsedSections["later-section"] = true
	outlinerView.Focus.CrossStack = append(outlinerView.Focus.CrossStack,
		CrossProjectFocusEntry{ProjectID: "alpha", TaskID: "second"})
	outlinerView.SetFilterQuery(checkpointQuery(t, "$done"))

	for pass := range 2 {
		outlinerView.RestoreOutlinerPresentationCheckpoint(checkpoint)
		if got := queryParts(outlinerView.FilterQuery); !slices.Equal(got, []string{"#keep", "-#legacy"}) {
			t.Fatalf("pass %d restored query = %v", pass, got)
		}
		if !maps.Equal(outlinerView.Folds.CollapsedSections, map[string]bool{"frozen-section": true}) {
			t.Fatalf("pass %d restored folds = %v", pass, outlinerView.Folds.CollapsedSections)
		}
		if len(outlinerView.Focus.CrossStack) != 1 {
			t.Fatalf("pass %d restored focus depth = %d, want 1", pass, len(outlinerView.Focus.CrossStack))
		}
		// Editing the restored copies must not reach back into the checkpoint.
		outlinerView.Folds.CollapsedSections["scratch"] = true
		outlinerView.Focus.CrossStack = append(outlinerView.Focus.CrossStack,
			CrossProjectFocusEntry{ProjectID: "alpha", TaskID: "third"})
	}
}

// TestPresentationCheckpointRestoresOverChangedTaskData is the restore-over-
// current-data path: the checkpoint holds no task pointer, so a task that
// vanished between capture and restore costs the cursor its exact seat and
// nothing else. The fallback is the one ordinary search already uses.
func TestPresentationCheckpointRestoresOverChangedTaskData(t *testing.T) {
	first := primaryRowsTask("first", models.StateBacklog)
	second := primaryRowsTask("second", models.StateBacklog)
	third := primaryRowsTask("third", models.StateBacklog)
	project := primaryRowsProject("alpha", first, second, third)
	outlinerView := scopeTestOutliner(project)
	outlinerView.SetFilterQuery(checkpointQuery(t, "-#legacy"))
	if !outlinerView.MoveCursorToProjectTask("second", "alpha") {
		t.Fatal("cursor could not be seated on the second task")
	}

	checkpoint := outlinerView.CaptureOutlinerPresentationCheckpoint()
	project.Tasks = []*models.Task{first, third}
	project.BuildTaskTree()
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()
	outlinerView.RestoreOutlinerPresentationCheckpoint(checkpoint)

	if got := queryParts(outlinerView.FilterQuery); !slices.Equal(got, []string{"-#legacy"}) {
		t.Fatalf("restored query = %v, want the captured negation", got)
	}
	if got := searchVisibleTaskIDs(outlinerView); !slices.Equal(got, []string{"first", "third"}) {
		t.Fatalf("restored rows = %v, want the surviving tasks", got)
	}
	if identity := outlinerView.CurrentTaskIdentity(); identity.TaskID == "second" {
		t.Fatal("restoration seated the cursor on a task that no longer exists")
	}
	if !outlinerView.CurrentTaskIdentity().Valid() {
		t.Fatal("restoration left the cursor off every surviving task row")
	}
}
