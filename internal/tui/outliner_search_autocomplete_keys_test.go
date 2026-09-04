package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestSearchAutocompleteKeysClaimOnlyTheDropdownControls is the key contract. Tab
// and the four navigation keys belong to the pane; every other key, printable
// letters among them, still reaches the search input, so completion never makes a
// character untypeable.
func TestSearchAutocompleteKeysClaimOnlyTheDropdownControls(t *testing.T) {
	tasks := []*models.Task{
		searchACTask("t1", []string{"alpha"}, ""),
		searchACTask("t2", []string{"beta"}, ""),
		searchACTask("t3", []string{"gamma"}, ""),
	}
	outlinerView := searchACOutliner(primaryRowsProject("alpha", tasks...))
	typeSearchAC(outlinerView, "#")

	for _, key := range []string{"down", "ctrl+n"} {
		before := outlinerView.Search.Autocomplete.Ac.Cursor
		if !outlinerView.HandleSearchAutocompleteKey(key) {
			t.Fatalf("%s was not claimed by the open pane", key)
		}
		if got := outlinerView.Search.Autocomplete.Ac.Cursor; got != before+1 {
			t.Errorf("%s moved the cursor to %d, want %d", key, got, before+1)
		}
	}
	for _, key := range []string{"up", "ctrl+p"} {
		before := outlinerView.Search.Autocomplete.Ac.Cursor
		if !outlinerView.HandleSearchAutocompleteKey(key) {
			t.Fatalf("%s was not claimed by the open pane", key)
		}
		if got := outlinerView.Search.Autocomplete.Ac.Cursor; got != before-1 {
			t.Errorf("%s moved the cursor to %d, want %d", key, got, before-1)
		}
	}

	// Enter belongs to the search, not to the pane: it reaches the existing
	// confirm-and-land path unchanged.
	if outlinerView.HandleSearchAutocompleteKey("enter") {
		t.Error("the pane claimed Enter, which must stay the result-landing key")
	}
	// j and k belong to the results while the pane covers them, and to the query
	// again the moment it closes.
	for _, key := range []string{"j", "k"} {
		if !outlinerView.HandleSearchAutocompleteKey(key) {
			t.Errorf("the open pane did not claim %q for result navigation", key)
		}
	}
	// Every other printable key still types, so completion makes no letter
	// permanently unreachable.
	for _, key := range []string{"a", "z", "1"} {
		if outlinerView.HandleSearchAutocompleteKey(key) {
			t.Errorf("the pane claimed %q, which must stay typable in a query", key)
		}
	}

	if !outlinerView.HandleSearchAutocompleteKey("tab") {
		t.Fatal("Tab was not claimed by the open pane")
	}
	if got := outlinerView.SearchInputValue(); !strings.HasPrefix(got, "#") || !strings.HasSuffix(got, " ") {
		t.Errorf("Tab produced %q, want an accepted token with its trailing space", got)
	}

	// A closed pane claims nothing at all.
	if outlinerView.HandleSearchAutocompleteKey("tab") {
		t.Error("a closed pane claimed Tab")
	}
}

// TestSearchAutocompleteJKNavigateResultsWhilePaneOpen is the two-axis contract:
// while the pane covers the results, the arrow family walks the suggestions and
// j/k walk the results underneath them, and neither axis disturbs the other or
// the query. It drives real key presses so the whole path — the pane's claim, the
// shared list navigation, and the search input it must not reach — is exercised.
func TestSearchAutocompleteJKNavigateResultsWhilePaneOpen(t *testing.T) {
	tasks := []*models.Task{
		searchACTask("task-one", []string{"alpha"}, ""),
		searchACTask("task-two", []string{"beta"}, ""),
		searchACTask("task-three", []string{"gamma"}, ""),
	}
	outlinerView := searchACOutliner(primaryRowsProject("alpha", tasks...))
	for _, key := range []string{"t", "a", "s", "k", "-", " ", "#"} {
		outlinerView.HandleKeyMsg(syntheticKeyStr(key))
	}
	if !outlinerView.Search.Autocomplete.Active() {
		t.Fatal("the tag sigil opened no pane for j/k to navigate under")
	}
	if got, want := outlinerView.SearchInputValue(), "task- #"; got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
	if len(outlinerView.Rows) < 3 {
		t.Fatalf("search projection produced %d rows, want the three matching tasks", len(outlinerView.Rows))
	}

	// The suggestion cursor is moved off its first row first, so a j that leaked
	// into the dropdown would be visible as a suggestion move rather than hidden
	// behind an already-clamped cursor.
	outlinerView.HandleKeyMsg(syntheticKeyStr("down"))
	suggestionCursor := outlinerView.Search.Autocomplete.Ac.Cursor
	if suggestionCursor != 1 {
		t.Fatalf("suggestion cursor = %d, want 1 after one Down", suggestionCursor)
	}
	resultCursor := outlinerView.Cursor

	outlinerView.HandleKeyMsg(syntheticKeyStr("j"))
	if got, want := outlinerView.Cursor, resultCursor+1; got != want {
		t.Errorf("j left the result cursor at %d, want %d", got, want)
	}
	outlinerView.HandleKeyMsg(syntheticKeyStr("j"))
	if got, want := outlinerView.Cursor, resultCursor+2; got != want {
		t.Errorf("a second j left the result cursor at %d, want %d", got, want)
	}
	outlinerView.HandleKeyMsg(syntheticKeyStr("k"))
	if got, want := outlinerView.Cursor, resultCursor+1; got != want {
		t.Errorf("k left the result cursor at %d, want %d", got, want)
	}

	// Neither letter typed, and neither moved the suggestion the pane is offering.
	if got, want := outlinerView.SearchInputValue(), "task- #"; got != want {
		t.Errorf("query after j/k = %q, want it untouched at %q", got, want)
	}
	if got := outlinerView.Search.Autocomplete.Ac.Cursor; got != suggestionCursor {
		t.Errorf("j/k moved the suggestion cursor to %d, want it left at %d", got, suggestionCursor)
	}
	if !outlinerView.Search.Autocomplete.Active() {
		t.Error("result navigation closed the pane it was supposed to navigate under")
	}

	// Closing the pane gives both letters back to the query, which is what keeps
	// the claim honest: no character is permanently unreachable.
	outlinerView.HandleKeyMsg(syntheticKeyStr("esc"))
	if outlinerView.Search.Autocomplete.Active() {
		t.Fatal("Esc left the pane open")
	}
	outlinerView.HandleKeyMsg(syntheticKeyStr("j"))
	if got, want := outlinerView.SearchInputValue(), "task- #j"; got != want {
		t.Errorf("query after j with the pane closed = %q, want %q", got, want)
	}
}

// TestSearchAutocompleteEnterStillLandsOnTheResult holds the one key the pane
// must never intercept: Enter belongs to the search's own landing path, and an
// open pane may not turn it into an acceptance.
func TestSearchAutocompleteEnterStillLandsOnTheResult(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outlinerView := searchACOutliner(primaryRowsProject("alpha", task))
	typeSearchAC(outlinerView, "#inf")
	if !outlinerView.Search.Autocomplete.Active() {
		t.Fatal("no pane open for Enter to bypass")
	}

	outlinerView.HandleKeyMsg(syntheticKeyStr("enter"))
	if got := outlinerView.SearchInputValue(); got != "#inf" {
		t.Errorf("query after Enter = %q, want the typed text rather than an accepted suggestion", got)
	}
	if outlinerView.IsSearchMode() {
		t.Error("Enter left the input focused instead of landing on the results")
	}
	if got, want := outlinerView.SearchQuery(), "#inf"; got != want {
		t.Errorf("applied query = %q, want %q", got, want)
	}
	if outlinerView.Search.Autocomplete.Active() {
		t.Error("landing on the results left the pane open")
	}
}

// TestSearchAutocompleteEscCascadesSuggestionsThenSearch is the required two-step
// unwind: the first Esc gives back the rows the pane covered, the second gives
// back the projection.
func TestSearchAutocompleteEscCascadesSuggestionsThenSearch(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outlinerView := searchACOutliner(primaryRowsProject("alpha", task))
	typeSearchAC(outlinerView, "#")
	if !outlinerView.Search.Autocomplete.Active() {
		t.Fatal("no pane to unwind")
	}

	outlinerView.HandleKeyMsg(syntheticKeyStr("esc"))
	if outlinerView.Search.Autocomplete.Active() {
		t.Fatal("the first Esc left the pane open")
	}
	if !outlinerView.IsSearchMode() {
		t.Fatal("the first Esc closed the search instead of the pane")
	}
	if got := outlinerView.SearchInputValue(); got != "#" {
		t.Errorf("query after the first Esc = %q, want it untouched", got)
	}

	outlinerView.HandleKeyMsg(syntheticKeyStr("esc"))
	if outlinerView.IsSearchMode() {
		t.Error("the second Esc left the search active")
	}
}

// TestSearchAutocompleteClosesWhenTheActiveTokenStopsOffering covers the third
// close reason beside Esc and search ending: the token under the caret simply
// runs out of candidates as more is typed.
func TestSearchAutocompleteClosesWhenTheActiveTokenStopsOffering(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outlinerView := searchACOutliner(primaryRowsProject("alpha", task))

	typeSearchAC(outlinerView, "#in")
	if !outlinerView.Search.Autocomplete.Active() {
		t.Fatal("#in offered nothing")
	}
	typeSearchAC(outlinerView, "#zzz")
	if outlinerView.Search.Autocomplete.Active() {
		t.Fatalf("#zzz kept the pane open: %v", searchACValues(outlinerView))
	}
	// Free text is not a token at all.
	typeSearchAC(outlinerView, "plain text")
	if outlinerView.Search.Autocomplete.Active() {
		t.Fatalf("free text opened a pane: %v", searchACValues(outlinerView))
	}
	// Leaving search closes it regardless of what was on screen.
	typeSearchAC(outlinerView, "#")
	outlinerView.ClearSearch()
	if outlinerView.Search.Autocomplete.Active() {
		t.Error("clearing the search left the pane open")
	}
}
