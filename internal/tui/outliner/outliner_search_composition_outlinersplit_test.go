package outliner

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
)

// TestOneNarrowingQueryCompilesAtATime is an assertion over the compile path,
// not only the rows: both carriers reach compileActiveNarrowing, and a lens owns
// the query while one is active, so exactly one gate is ever written per
// rebuild. Session 9's candidate boundary is therefore not a place to hang lens
// logic.
func TestOneNarrowingQueryCompilesAtATime(t *testing.T) {
	needle := primaryRowsTask("needle", models.StateBacklog)
	needle.Title = "Needle groundwork"
	other := primaryRowsTask("other", models.StateBacklog)
	other.Title = "Beta rollout"
	alpha := primaryRowsProject("alpha", needle, other)

	outliner := scopeTestOutliner(alpha)
	outliner.Lens.Active = &activeLensFrame{LensID: "lens", Name: "Groundwork"}
	outliner.SetLensQuery("needle")
	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()
	if outliner.Lens.Gate == nil {
		t.Fatal("the canonical path did not compile the lens narrowing")
	}
	if outliner.Search.Gate != nil {
		t.Fatal("the lens rebuild also wrote the search gate")
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"needle"}) {
		t.Fatalf("lens-narrowed rows = %v, want the narrowing applied on the canonical path", got)
	}

	// Without a lens the same call site compiles the search query instead, and
	// the rows are still the mode's own.
	outliner.Lens.Active = nil
	outliner.Lens.Gate = nil
	outliner.SetLensQuery("")
	outliner.SetSearchQuery("beta")
	if outliner.Lens.Gate != nil {
		t.Fatal("a lens-free rebuild compiled a lens narrowing")
	}
	if outliner.Search.Gate == nil {
		t.Fatal("the search query was never compiled")
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"other"}) {
		t.Fatalf("narrowing search = %v, want the query alone deciding membership", got)
	}
}

// TestSlashUnderALensSuspendsItRatherThanNarrowingIt is the other half of that
// boundary: the key takes the lens down before it mints the projection, so the
// two narrowings still never stand at once — and the suspension is what puts the
// lens back, rather than the lens having survived underneath.
func TestSlashUnderALensSuspendsItRatherThanNarrowingIt(t *testing.T) {
	task := primaryRowsTask("task", models.StateBacklog)
	task.Title = "Needle groundwork"
	outliner := scopeTestOutliner(primaryRowsProject("alpha", task))
	outliner.Lens.Active = &activeLensFrame{LensID: "lens", Name: "Groundwork"}
	outliner.SetLensQuery("needle")

	_ = outliner.ActivateSearch()
	if outliner.HasActiveLens() {
		t.Fatal("/ under a lens left the lens owning the projection")
	}
	if !outliner.HasSuspendedLens() {
		t.Fatal("/ under a lens recorded no suspension to restore")
	}
	if !outliner.HasSearchProjection() || !outliner.ShowsRankedList() {
		t.Fatalf("/ under a lens did not open the ranked search: projection=%v ranked=%v",
			outliner.HasSearchProjection(), outliner.ShowsRankedList())
	}
	if outliner.Lens.Gate != nil || outliner.Lens.Query != "" {
		t.Fatalf("the suspended lens kept narrowing: gate=%v query=%q", outliner.Lens.Gate != nil, outliner.Lens.Query)
	}
	if got := outliner.SearchInputValue(); got != "" {
		t.Fatalf("the search input opened holding %q, want an empty query", got)
	}

	_, _ = outliner.HandleKeyMsg(syntheticSpecialKey(tea.KeyEsc))
	if outliner.HasSearchProjection() || outliner.HasSuspendedLens() {
		t.Fatal("Esc left the search projection or its suspension behind")
	}
	if !outliner.HasActiveLens() || outliner.Lens.Query != "needle" {
		t.Fatalf("Esc did not restore the lens: active=%v query=%q", outliner.HasActiveLens(), outliner.Lens.Query)
	}
}
