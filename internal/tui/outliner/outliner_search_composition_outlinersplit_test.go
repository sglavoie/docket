package outliner

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/models"
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

// TestSlashUnderALensEditsTheLensQueryRatherThanStartingASearch is the other
// half of that boundary: the key never mints a scope-wide projection beneath a
// lens, so no rebuild can reach both narrowings at once.
func TestSlashUnderALensEditsTheLensQueryRatherThanStartingASearch(t *testing.T) {
	task := primaryRowsTask("task", models.StateBacklog)
	task.Title = "Needle groundwork"
	outliner := scopeTestOutliner(primaryRowsProject("alpha", task))
	outliner.Lens.Active = &activeLensFrame{LensID: "lens", Name: "Groundwork"}
	outliner.SetLensQuery("needle")

	_ = outliner.ActivateSearch()
	if !outliner.EditingLensQuery() {
		t.Fatal("/ under a lens did not open the lens query editor")
	}
	if outliner.HasSearchProjection() {
		t.Fatal("/ under a lens started a scope-wide search projection")
	}
	if got := outliner.SearchInputValue(); got != "needle" {
		t.Fatalf("lens query editor seeded with %q, want the lens's own query", got)
	}
	if got := outliner.SearchQuery(); got != "" {
		t.Fatalf("ordinary search query = %q, want it left empty under a lens", got)
	}

	_, _ = outliner.HandleKeyMsg(syntheticSpecialKey(tea.KeyEsc))
	if outliner.EditingLensQuery() {
		t.Fatal("Esc did not close the lens query editor")
	}
	if outliner.HasSearchProjection() {
		t.Fatal("Esc left a search projection behind")
	}
}
