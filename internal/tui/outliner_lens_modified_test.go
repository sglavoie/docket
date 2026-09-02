package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestSwitchingLensesRetainsTheOriginalBaseline(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.selectOutlinerMode(ModeInbox)

	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))
	first := outliner.activeLens.origin
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(2), "Held", ""))

	if outliner.activeLens.name != "Held" {
		t.Fatalf("active lens = %q, want Held", outliner.activeLens.name)
	}
	if outliner.activeLens.origin.presentation.mode != first.presentation.mode {
		t.Fatal("a switch replaced the original restoration point")
	}
	if outliner.lensQuery != "" {
		t.Fatalf("switch kept the previous narrowing query %q", outliner.lensQuery)
	}

	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if outliner.hasActiveLens() {
		t.Fatal("one Esc must leave the lens entirely")
	}
	if outliner.primaryMode != ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-lens %v", outliner.primaryMode, ModeInbox)
	}
}

func TestApplyNamedLensFromAcceptedSearchRestoresThatSearch(t *testing.T) {
	outliner := lensLifecycleOutliner()
	outliner.SetSearchQuery("needle")
	if !outliner.hasSearchProjection() {
		t.Fatal("expected an ordinary search projection")
	}
	searchRows := searchVisibleTaskIDs(outliner)

	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", ""))
	if outliner.hasSearchProjection() {
		t.Fatal("the lens must replace the ordinary search projection")
	}

	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if !outliner.hasSearchProjection() {
		t.Fatal("restoration did not hand the search projection back")
	}
	if outliner.SearchQuery() != "needle" {
		t.Fatalf("restored search query = %q, want needle", outliner.SearchQuery())
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, searchRows) {
		t.Fatalf("restored search rows = %v, want %v", got, searchRows)
	}
	// The search frame is whole again, so its own Esc still restores the ordinary
	// projection underneath it.
	if !RunEscCascade(outliner.escSteps()) {
		t.Fatal("expected Esc to consume the restored search projection")
	}
	if outliner.hasSearchProjection() {
		t.Fatal("the restored search projection did not dismiss")
	}
}

func TestLensModifiedTracksOwnedAxesOnly(t *testing.T) {
	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))

	if outliner.lensModified() {
		t.Fatal("a freshly applied lens is not modified")
	}
	if got := outliner.activeLensLabel(); got != "Lens: Waiting" {
		t.Fatalf("label = %q, want %q", got, "Lens: Waiting")
	}

	// Navigation, folds, selection, and Peek are the view, not the lens.
	outliner.cursor = 0
	outliner.collapsedSections["some-section"] = true
	outliner.peekMode = true
	outliner.selectedTaskIdentities[TaskIdentity{ProjectID: "alpha", TaskID: "waiting"}] = true
	if outliner.lensModified() {
		t.Fatal("cursor, folds, selection, and Peek must not mark a lens modified")
	}

	outliner.filter.Priority = int(models.PriorityHigh)
	outliner.adoptFilterCarriers()
	if !outliner.lensModified() {
		t.Fatal("a facet change must mark the lens modified")
	}
	if got := outliner.activeLensLabel(); got != "Lens: Waiting*" {
		t.Fatalf("label = %q, want %q", got, "Lens: Waiting*")
	}

	outliner.filter.Priority = 0
	outliner.adoptFilterCarriers()
	if outliner.lensModified() {
		t.Fatal("reverting the facet must clear the marker")
	}
}

func TestLensModifiedTracksModeQuerySortAndProjectFacets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*OutlinerModel)
	}{
		{"mode", func(o *OutlinerModel) { o.selectOutlinerMode(ModeInbox) }},
		{"query", func(o *OutlinerModel) { o.setLensQuery("other") }},
		{"sort", func(o *OutlinerModel) { o.treeSort = SortState{Field: SortByTitle, Ascending: true} }},
		{"project facets", func(o *OutlinerModel) {
			o.projectFilter.Untagged = true
			o.adoptFilterCarriers()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outliner := lensLifecycleOutliner()
			applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))
			tc.modify(outliner)
			if !outliner.lensModified() {
				t.Fatalf("changing %s must mark the lens modified", tc.name)
			}
		})
	}
}

func TestOrdinaryFiltersAndSortRemainEditableUnderALensQuery(t *testing.T) {
	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))

	outliner.filter.Priority = int(models.PriorityHigh)
	outliner.adoptFilterCarriers()
	outliner.treeSort = SortState{Field: SortByTitle, Ascending: true}
	outliner.sort = outliner.treeSort
	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()

	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"waiting"}) {
		t.Fatalf("rows under lens query plus priority facet = %v, want [waiting]", got)
	}
	if outliner.lensQuery != "needle" {
		t.Fatalf("editing filters disturbed the narrowing query: %q", outliner.lensQuery)
	}
	if !outliner.hasActiveLens() {
		t.Fatal("editing filters must not dismiss the lens")
	}
}

func TestApplyNamedLensRefusesInvalidDefinitionsWithoutTouchingTheView(t *testing.T) {
	for _, tc := range []struct {
		name string
		lens func() config.NamedLens
	}{
		{"unknown mode", func() config.NamedLens {
			lens := waitingLens(lensTestID(1), "Waiting", "")
			lens.Mode = "nonsense"
			return lens
		}},
		{"query past the durable bound", func() config.NamedLens {
			lens := waitingLens(lensTestID(1), "Waiting", "")
			lens.Query = strings.Repeat("a", config.MaxNamedLensQueryRunes+1)
			return lens
		}},
		{"unhonorable query", func() config.NamedLens {
			return waitingLens(lensTestID(1), "Waiting", "!9")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outliner := lensLifecycleOutliner()
			outliner.selectOutlinerMode(ModeInbox)
			before := searchVisibleTaskIDs(outliner)

			cmd, err := outliner.ApplyNamedLens(tc.lens())
			if err == nil {
				t.Fatal("expected the definition to refuse")
			}
			if cmd != nil {
				t.Fatal("a refusal must return no command")
			}
			if outliner.hasActiveLens() || outliner.pendingLens != nil {
				t.Fatal("a refusal must leave no lens state behind")
			}
			if outliner.primaryMode != ModeInbox || outliner.lensQuery != "" {
				t.Fatal("a refusal must leave the current projection untouched")
			}
			if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, before) {
				t.Fatalf("rows = %v, want the untouched %v", got, before)
			}
		})
	}
}
