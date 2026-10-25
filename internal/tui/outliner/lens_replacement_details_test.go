package outliner

import (
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
)

func TestLensReplacementValuesDescribeEveryChangedAxis(t *testing.T) {
	saved := LensState{
		Mode: chrome.ModeAll,
		Sort: query.SortState{Field: query.SortByPriority, Ascending: true},
	}
	current := LensState{
		Mode:           chrome.ModeToday,
		Query:          "login",
		ScopeProjectID: "missing-project",
		Sort:           query.SortState{Field: query.SortByDueDate},
	}
	view := &OutlinerModel{}
	got := view.LensReplacementChanges(saved, current)
	want := []string{
		"Mode: All → Today",
		"Query / filters: none → login",
		"Sort: priority ↑ → due date ↓",
		"Project: keep current scope → missing-project (unavailable)",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("changes = %q, want %q", got, want)
	}
	if got := view.LensReplacementChanges(current, current); len(got) != 0 {
		t.Fatalf("unchanged lens reports %q", got)
	}
	reverse := view.LensReplacementChanges(current, saved)
	if reverse[1] != "Query / filters: login → none" || reverse[3] != "Project: missing-project (unavailable) → keep current scope" {
		t.Fatalf("cleared axes = %q", reverse)
	}
}

func TestLensReplacementEquivalentQuerySpellingsHaveNoDiff(t *testing.T) {
	saved := LensState{Mode: chrome.ModeAll, Query: "#work"}
	current := LensState{Mode: chrome.ModeAll}
	current.Facets, _ = models.PartitionQueryString("#work")
	if got := (&OutlinerModel{}).LensReplacementChanges(saved, current); len(got) != 0 {
		t.Fatalf("equivalent typed query and facet report %q", got)
	}
}
