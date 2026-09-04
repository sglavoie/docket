package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestSwitchingLensesRetainsTheOriginalBaseline(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	outlinerView.SelectOutlinerMode(chrome.ModeInbox)

	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))
	first := outlinerView.Lens.Active.Origin
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(2), "Held", ""))

	if outlinerView.Lens.Active.Name != "Held" {
		t.Fatalf("active lens = %q, want Held", outlinerView.Lens.Active.Name)
	}
	if outlinerView.Lens.Active.Origin.Presentation.Mode != first.Presentation.Mode {
		t.Fatal("a switch replaced the original restoration point")
	}
	if outlinerView.Lens.Query != "" {
		t.Fatalf("switch kept the previous narrowing query %q", outlinerView.Lens.Query)
	}

	if !listsurface.RunEscCascade(outlinerView.EscSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if outlinerView.HasActiveLens() {
		t.Fatal("one Esc must leave the lens entirely")
	}
	if outlinerView.PrimaryMode != chrome.ModeInbox {
		t.Fatalf("restored mode = %v, want the pre-lens %v", outlinerView.PrimaryMode, chrome.ModeInbox)
	}
}

func TestApplyNamedLensFromAcceptedSearchRestoresThatSearch(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	outlinerView.SetSearchQuery("needle")
	if !outlinerView.HasSearchProjection() {
		t.Fatal("expected an ordinary search projection")
	}
	searchRows := searchVisibleTaskIDs(outlinerView)

	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", ""))
	if outlinerView.HasSearchProjection() {
		t.Fatal("the lens must replace the ordinary search projection")
	}

	if !listsurface.RunEscCascade(outlinerView.EscSteps()) {
		t.Fatal("expected Esc to consume the lens")
	}
	if !outlinerView.HasSearchProjection() {
		t.Fatal("restoration did not hand the search projection back")
	}
	if outlinerView.SearchQuery() != "needle" {
		t.Fatalf("restored search query = %q, want needle", outlinerView.SearchQuery())
	}
	if got := searchVisibleTaskIDs(outlinerView); !slices.Equal(got, searchRows) {
		t.Fatalf("restored search rows = %v, want %v", got, searchRows)
	}
	// The search frame is whole again, so its own Esc still restores the ordinary
	// projection underneath it.
	if !listsurface.RunEscCascade(outlinerView.EscSteps()) {
		t.Fatal("expected Esc to consume the restored search projection")
	}
	if outlinerView.HasSearchProjection() {
		t.Fatal("the restored search projection did not dismiss")
	}
}

func TestLensModifiedTracksOwnedAxesOnly(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))

	if outlinerView.LensModified() {
		t.Fatal("a freshly applied lens is not modified")
	}
	if got := outlinerView.ActiveLensLabel(); got != "Lens: Waiting" {
		t.Fatalf("label = %q, want %q", got, "Lens: Waiting")
	}

	// Navigation, folds, selection, and Peek are the view, not the lens.
	outlinerView.Cursor = 0
	outlinerView.Folds.CollapsedSections["some-section"] = true
	outlinerView.PeekMode = true
	outlinerView.Selected[taskref.TaskIdentity{ProjectID: "alpha", TaskID: "waiting"}] = true
	if outlinerView.LensModified() {
		t.Fatal("cursor, folds, selection, and Peek must not mark a lens modified")
	}

	outlinerView.Filter.Priority = int(models.PriorityHigh)
	outlinerView.AdoptFilterCarriers()
	if !outlinerView.LensModified() {
		t.Fatal("a facet change must mark the lens modified")
	}
	if got := outlinerView.ActiveLensLabel(); got != "Lens: Waiting*" {
		t.Fatalf("label = %q, want %q", got, "Lens: Waiting*")
	}

	outlinerView.Filter.Priority = 0
	outlinerView.AdoptFilterCarriers()
	if outlinerView.LensModified() {
		t.Fatal("reverting the facet must clear the marker")
	}
}

func TestLensModifiedTracksModeQuerySortAndProjectFacets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*outliner.OutlinerModel)
	}{
		{"mode", func(o *outliner.OutlinerModel) { o.SelectOutlinerMode(chrome.ModeInbox) }},
		{"query", func(o *outliner.OutlinerModel) { o.SetLensQuery("other") }},
		{"sort", func(o *outliner.OutlinerModel) {
			o.TreeSort = query.SortState{Field: query.SortByTitle, Ascending: true}
		}},
		{"project facets", func(o *outliner.OutlinerModel) {
			o.ProjectFilter.Untagged = true
			o.AdoptFilterCarriers()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outlinerView := lensLifecycleOutliner()
			applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))
			tc.modify(outlinerView)
			if !outlinerView.LensModified() {
				t.Fatalf("changing %s must mark the lens modified", tc.name)
			}
		})
	}
}

func TestOrdinaryFiltersAndSortRemainEditableUnderALensQuery(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))

	outlinerView.Filter.Priority = int(models.PriorityHigh)
	outlinerView.AdoptFilterCarriers()
	outlinerView.TreeSort = query.SortState{Field: query.SortByTitle, Ascending: true}
	outlinerView.Sort = outlinerView.TreeSort
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()

	if got := searchVisibleTaskIDs(outlinerView); !slices.Equal(got, []string{"waiting"}) {
		t.Fatalf("rows under lens query plus priority facet = %v, want [waiting]", got)
	}
	if outlinerView.Lens.Query != "needle" {
		t.Fatalf("editing filters disturbed the narrowing query: %q", outlinerView.Lens.Query)
	}
	if !outlinerView.HasActiveLens() {
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
			outlinerView := lensLifecycleOutliner()
			outlinerView.SelectOutlinerMode(chrome.ModeInbox)
			before := searchVisibleTaskIDs(outlinerView)

			cmd, err := outlinerView.ApplyNamedLens(tc.lens())
			if err == nil {
				t.Fatal("expected the definition to refuse")
			}
			if cmd != nil {
				t.Fatal("a refusal must return no command")
			}
			if outlinerView.HasActiveLens() || outlinerView.Lens.Pending != nil {
				t.Fatal("a refusal must leave no lens state behind")
			}
			if outlinerView.PrimaryMode != chrome.ModeInbox || outlinerView.Lens.Query != "" {
				t.Fatal("a refusal must leave the current projection untouched")
			}
			if got := searchVisibleTaskIDs(outlinerView); !slices.Equal(got, before) {
				t.Fatalf("rows = %v, want the untouched %v", got, before)
			}
		})
	}
}
