package outliner

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/notes"
)

func matchCountOutliner() *OutlinerModel {
	first := primaryRowsTask("first", models.StateBacklog)
	first.Title = "Needle groundwork"
	second := primaryRowsTask("second", models.StateBacklog)
	second.Title = "Needle rollout"
	other := primaryRowsTask("other", models.StateBacklog)
	other.Title = "Beta review"
	return noteTierOutliner([]notes.LocatedNoteRow{
		noteIndexRow(app.GlobalNoteLocation(), "Global", "Needle notes", "needle.md"),
	}, primaryRowsProject("alpha", first, second, other))
}

func searchIndicatorText(outliner *OutlinerModel, width int) string {
	return ansi.Strip(outliner.buildOutlinerSearchIndicator(width))
}

// The count states the rows the projection draws, notes tier included, in the
// singular and plural spellings and at zero.
func TestSearchIndicatorCountsDrawnTaskAndNoteRows(t *testing.T) {
	for _, testCase := range []struct {
		query string
		want  string
	}{
		{"needle", "3 matches"},
		{"beta", "1 match"},
		{"zzzz", "0 matches"},
	} {
		outliner := matchCountOutliner()
		searchRankedList(outliner, testCase.query)
		got := searchIndicatorText(outliner, 120)
		if !strings.HasSuffix(got, "· "+testCase.want) {
			t.Fatalf("ranked %q indicator = %q, want it to end with %q", testCase.query, got, testCase.want)
		}
	}
}

// The narrowing presentation counts the mode's own drawn rows, and the notes
// tier, which only the ranked list has, is absent from that count.
func TestSearchIndicatorCountsTheNarrowingForm(t *testing.T) {
	outliner := matchCountOutliner()
	outliner.SetSearchQuery("needle")
	if outliner.ShowsRankedList() {
		t.Fatal("SetSearchQuery opened the ranked list")
	}
	if got := searchIndicatorText(outliner, 120); !strings.Contains(got, "· 2 matches") {
		t.Fatalf("narrowing indicator = %q, want 2 matches", got)
	}
	outliner.ToggleRankedList()
	if got := searchIndicatorText(outliner, 120); !strings.Contains(got, "· 3 matches") {
		t.Fatalf("ranked indicator = %q, want 3 matches", got)
	}
}

// The typing form carries the same count, and an empty input draws none.
func TestSearchIndicatorCountsWhileTyping(t *testing.T) {
	outliner := matchCountOutliner()
	_ = outliner.ActivateSearch()
	if got := searchIndicatorText(outliner, 120); strings.Contains(got, "match") {
		t.Fatalf("empty search input indicator = %q, want no count", got)
	}
	outliner.SetSearchInputValue("beta")
	outliner.SetSearchQuery("beta")
	if !outliner.IsSearchMode() {
		t.Fatal("search input closed")
	}
	if got := searchIndicatorText(outliner, 120); !strings.Contains(got, "· 1 match") {
		t.Fatalf("typing indicator = %q, want 1 match", got)
	}
}

// The count is a badge: it sheds whole before the query text gives way.
func TestSearchMatchCountShedsBeforeTheQuery(t *testing.T) {
	outliner := matchCountOutliner()
	searchRankedList(outliner, "needle")
	wide := searchIndicatorText(outliner, 120)
	query := strings.TrimSuffix(wide, " · 3 matches")
	if query == wide {
		t.Fatalf("wide indicator = %q, want the count", wide)
	}
	if got := searchIndicatorText(outliner, len(query)+3); got != query {
		t.Fatalf("narrow indicator = %q, want the query alone %q", got, query)
	}
}

// Q reads the count on the query row.
func TestQueryReadoutStatesTheMatchCount(t *testing.T) {
	outliner := matchCountOutliner()
	searchRankedList(outliner, "beta")
	for _, row := range outliner.QueryReadoutRows() {
		if row.Label == "query" {
			if row.note != "1 match" {
				t.Fatalf("query readout note = %q, want 1 match", row.note)
			}
			return
		}
	}
	t.Fatal("the readout has no query row")
}

// On the context row the count is the search entry's full form alone: when
// every entry no longer fits, the compact pass keeps the query and drops the
// count rather than dropping the whole search entry.
func TestContextRowShedsTheCountBeforeTheSearchEntry(t *testing.T) {
	outliner := matchCountOutliner()
	searchRankedList(outliner, "needle")
	wide := ansi.Strip(outliner.BuildOutlinerContextRow(120))
	if !strings.Contains(wide, "3 matches") {
		t.Fatalf("wide context row = %q, want the count", wide)
	}
	for width := 119; width >= 10; width-- {
		row := ansi.Strip(outliner.BuildOutlinerContextRow(width))
		if !strings.Contains(row, "matches") && !strings.Contains(row, `Search: "needle"`) {
			t.Fatalf("context row at %d = %q dropped the query with the count", width, row)
		}
		if strings.Contains(row, `Search: "needle"`) && !strings.Contains(row, "matches") {
			return
		}
	}
	t.Fatal("the context row never shed the count while keeping the query")
}
