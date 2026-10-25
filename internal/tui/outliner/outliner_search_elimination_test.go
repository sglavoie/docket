package outliner

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

func eliminationTestOutliner() *OutlinerModel {
	bank := primaryRowsTask("bank", models.StateInbox)
	bank.Title = "Call the bank"
	bank.Tags = []string{"errand"}
	release := primaryRowsTask("release", models.StateBacklog)
	release.Title = "Write release notes"
	release.Tags = []string{"docs"}
	overdue := primaryRowsTask("overdue", models.StateBacklog)
	overdue.Title = "Renew passport"
	past := time.Now().AddDate(0, 0, -3)
	overdue.DueDate = &past
	return scopeTestOutliner(primaryRowsProject("alpha", bank, release, overdue))
}

func TestEmptySearchNamesTheClauseThatEliminatedEverything(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{"bank @overdue", `No tasks match search "bank @overdue". 1 task matches "bank"; @overdue removes it. Press Esc to clear.`},
		{"releas #nope", `No tasks match search "releas #nope". No task carries #nope. Press Esc to clear.`},
		{"bank ##desk", `No tasks match search "bank ##desk". No task carries ##desk. Press Esc to clear.`},
		{"bank %zzz", `No tasks match search "bank %zzz". No project matches %zzz. Press Esc to clear.`},
		{"bank #errand @overdue", `No tasks match search "bank #errand @overdue". 1 task matches without @overdue. Press Esc to clear.`},
		// No single clause is responsible: the free text matches nothing.
		{"zzqx #docs", `No tasks match search "zzqx #docs". Press Esc to clear.`},
		// Removing the only clause of a sigil-only query explains nothing.
		{"$waiting", `No tasks match search "$waiting". Press Esc to clear.`},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			outliner := eliminationTestOutliner()
			searchRankedList(outliner, tc.query)
			if outliner.HasContentRows() {
				t.Fatalf("query %q matched %v", tc.query, searchVisibleTaskIDs(outliner))
			}
			if got := outliner.OutlinerEmptyMessage(); got != tc.want {
				t.Fatalf("message = %q\nwant      %q", got, tc.want)
			}
		})
	}
}

func TestEmptySearchPrefersTheClauseRestoringMostMatches(t *testing.T) {
	first := primaryRowsTask("first", models.StateBacklog)
	first.Title = "Plan trip"
	first.Tags = []string{"travel"}
	second := primaryRowsTask("second", models.StateBacklog)
	second.Title = "Plan trip budget"
	second.Tags = []string{"travel"}
	third := primaryRowsTask("third", models.StateBacklog)
	third.Title = "Plan trip bags"
	third.Priority = 1
	outliner := scopeTestOutliner(primaryRowsProject("alpha", first, second, third))

	searchRankedList(outliner, "plan !1 #travel")
	want := `No tasks match search "plan !1 #travel". 2 tasks match without !1. Press Esc to clear.`
	if got := outliner.OutlinerEmptyMessage(); got != want {
		t.Fatalf("message = %q\nwant      %q", got, want)
	}

	outliner.ClearSearch()
	if outliner.Search.eliminationHint != "" {
		t.Fatalf("clearing the search kept %q", outliner.Search.eliminationHint)
	}
}
