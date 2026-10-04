package outliner

import (
	"errors"
	"slices"
	"testing"
)

func TestSearchHistoryPersistenceFailureKeepsSearchAndClearIsAtomic(t *testing.T) {
	out := searchHistoryOutliner()
	persisted := []string{"older"}
	fail := true
	out.ConfigureSearchHistory(persisted, func(entries []string) error {
		if fail {
			return errors.New("disk unavailable")
		}
		persisted = slices.Clone(entries)
		return nil
	})
	// Submission still accepts the search and retains in-session recall even
	// when the machine file cannot be written.
	submitSearchQuery(out, "new query")
	if out.IsSearchMode() || out.SearchQuery() != "new query" {
		t.Fatal("history failure interrupted search")
	}
	if !slices.Equal(out.Search.history.entries, []string{"older", "new query"}) {
		t.Fatal(out.Search.history.entries)
	}
	if err := out.ClearSearchHistory(); err == nil {
		t.Fatal("failed clear reported success")
	}
	if len(out.Search.history.entries) != 2 {
		t.Fatal("failed clear forgot history")
	}
	// Rewiring must not restore the old snapshot over newer session entries.
	out.ConfigureSearchHistory([]string{"stale"}, nil)
	if len(out.Search.history.entries) != 2 {
		t.Fatal("rewiring replaced history")
	}
	fail = false
	if err := out.recordSearchHistoryEntry("new query"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(persisted, []string{"older", "new query"}) {
		t.Fatal("resubmission did not retry persistence")
	}
	if err := out.ClearSearchHistory(); err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 0 || out.hasSearchHistory() {
		t.Fatal("successful clear kept history")
	}
	if out.SearchQuery() != "new query" {
		t.Fatal("clear changed the active search")
	}
}
