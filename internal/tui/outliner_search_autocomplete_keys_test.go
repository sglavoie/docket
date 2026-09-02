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
	outliner := searchACOutliner(primaryRowsProject("alpha", tasks...))
	typeSearchAC(outliner, "#")

	for _, key := range []string{"down", "ctrl+n"} {
		before := outliner.searchAutocomplete.ac.Cursor
		if !outliner.handleSearchAutocompleteKey(key) {
			t.Fatalf("%s was not claimed by the open pane", key)
		}
		if got := outliner.searchAutocomplete.ac.Cursor; got != before+1 {
			t.Errorf("%s moved the cursor to %d, want %d", key, got, before+1)
		}
	}
	for _, key := range []string{"up", "ctrl+p"} {
		before := outliner.searchAutocomplete.ac.Cursor
		if !outliner.handleSearchAutocompleteKey(key) {
			t.Fatalf("%s was not claimed by the open pane", key)
		}
		if got := outliner.searchAutocomplete.ac.Cursor; got != before-1 {
			t.Errorf("%s moved the cursor to %d, want %d", key, got, before-1)
		}
	}

	// Enter belongs to the search, not to the pane: it reaches the existing
	// confirm-and-land path unchanged.
	if outliner.handleSearchAutocompleteKey("enter") {
		t.Error("the pane claimed Enter, which must stay the result-landing key")
	}
	// j and k belong to the results while the pane covers them, and to the query
	// again the moment it closes.
	for _, key := range []string{"j", "k"} {
		if !outliner.handleSearchAutocompleteKey(key) {
			t.Errorf("the open pane did not claim %q for result navigation", key)
		}
	}
	// Every other printable key still types, so completion makes no letter
	// permanently unreachable.
	for _, key := range []string{"a", "z", "1"} {
		if outliner.handleSearchAutocompleteKey(key) {
			t.Errorf("the pane claimed %q, which must stay typable in a query", key)
		}
	}

	if !outliner.handleSearchAutocompleteKey("tab") {
		t.Fatal("Tab was not claimed by the open pane")
	}
	if got := outliner.SearchInputValue(); !strings.HasPrefix(got, "#") || !strings.HasSuffix(got, " ") {
		t.Errorf("Tab produced %q, want an accepted token with its trailing space", got)
	}

	// A closed pane claims nothing at all.
	if outliner.handleSearchAutocompleteKey("tab") {
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
	outliner := searchACOutliner(primaryRowsProject("alpha", tasks...))
	for _, key := range []string{"t", "a", "s", "k", "-", " ", "#"} {
		outliner.handleKeyMsg(syntheticKeyStr(key))
	}
	if !outliner.searchAutocomplete.active() {
		t.Fatal("the tag sigil opened no pane for j/k to navigate under")
	}
	if got, want := outliner.SearchInputValue(), "task- #"; got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
	if len(outliner.rows) < 3 {
		t.Fatalf("search projection produced %d rows, want the three matching tasks", len(outliner.rows))
	}

	// The suggestion cursor is moved off its first row first, so a j that leaked
	// into the dropdown would be visible as a suggestion move rather than hidden
	// behind an already-clamped cursor.
	outliner.handleKeyMsg(syntheticKeyStr("down"))
	suggestionCursor := outliner.searchAutocomplete.ac.Cursor
	if suggestionCursor != 1 {
		t.Fatalf("suggestion cursor = %d, want 1 after one Down", suggestionCursor)
	}
	resultCursor := outliner.cursor

	outliner.handleKeyMsg(syntheticKeyStr("j"))
	if got, want := outliner.cursor, resultCursor+1; got != want {
		t.Errorf("j left the result cursor at %d, want %d", got, want)
	}
	outliner.handleKeyMsg(syntheticKeyStr("j"))
	if got, want := outliner.cursor, resultCursor+2; got != want {
		t.Errorf("a second j left the result cursor at %d, want %d", got, want)
	}
	outliner.handleKeyMsg(syntheticKeyStr("k"))
	if got, want := outliner.cursor, resultCursor+1; got != want {
		t.Errorf("k left the result cursor at %d, want %d", got, want)
	}

	// Neither letter typed, and neither moved the suggestion the pane is offering.
	if got, want := outliner.SearchInputValue(), "task- #"; got != want {
		t.Errorf("query after j/k = %q, want it untouched at %q", got, want)
	}
	if got := outliner.searchAutocomplete.ac.Cursor; got != suggestionCursor {
		t.Errorf("j/k moved the suggestion cursor to %d, want it left at %d", got, suggestionCursor)
	}
	if !outliner.searchAutocomplete.active() {
		t.Error("result navigation closed the pane it was supposed to navigate under")
	}

	// Closing the pane gives both letters back to the query, which is what keeps
	// the claim honest: no character is permanently unreachable.
	outliner.handleKeyMsg(syntheticKeyStr("esc"))
	if outliner.searchAutocomplete.active() {
		t.Fatal("Esc left the pane open")
	}
	outliner.handleKeyMsg(syntheticKeyStr("j"))
	if got, want := outliner.SearchInputValue(), "task- #j"; got != want {
		t.Errorf("query after j with the pane closed = %q, want %q", got, want)
	}
}

// TestSearchAutocompleteEnterStillLandsOnTheResult holds the one key the pane
// must never intercept: Enter belongs to the search's own landing path, and an
// open pane may not turn it into an acceptance.
func TestSearchAutocompleteEnterStillLandsOnTheResult(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outliner := searchACOutliner(primaryRowsProject("alpha", task))
	typeSearchAC(outliner, "#inf")
	if !outliner.searchAutocomplete.active() {
		t.Fatal("no pane open for Enter to bypass")
	}

	outliner.handleKeyMsg(syntheticKeyStr("enter"))
	if got := outliner.SearchInputValue(); got != "#inf" {
		t.Errorf("query after Enter = %q, want the typed text rather than an accepted suggestion", got)
	}
	if outliner.IsSearchMode() {
		t.Error("Enter left the input focused instead of landing on the results")
	}
	if got, want := outliner.SearchQuery(), "#inf"; got != want {
		t.Errorf("applied query = %q, want %q", got, want)
	}
	if outliner.searchAutocomplete.active() {
		t.Error("landing on the results left the pane open")
	}
}

// TestSearchAutocompleteEscCascadesSuggestionsThenSearch is the required two-step
// unwind: the first Esc gives back the rows the pane covered, the second gives
// back the projection.
func TestSearchAutocompleteEscCascadesSuggestionsThenSearch(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outliner := searchACOutliner(primaryRowsProject("alpha", task))
	typeSearchAC(outliner, "#")
	if !outliner.searchAutocomplete.active() {
		t.Fatal("no pane to unwind")
	}

	outliner.handleKeyMsg(syntheticKeyStr("esc"))
	if outliner.searchAutocomplete.active() {
		t.Fatal("the first Esc left the pane open")
	}
	if !outliner.IsSearchMode() {
		t.Fatal("the first Esc closed the search instead of the pane")
	}
	if got := outliner.SearchInputValue(); got != "#" {
		t.Errorf("query after the first Esc = %q, want it untouched", got)
	}

	outliner.handleKeyMsg(syntheticKeyStr("esc"))
	if outliner.IsSearchMode() {
		t.Error("the second Esc left the search active")
	}
}

// TestSearchAutocompleteClosesWhenTheActiveTokenStopsOffering covers the third
// close reason beside Esc and search ending: the token under the caret simply
// runs out of candidates as more is typed.
func TestSearchAutocompleteClosesWhenTheActiveTokenStopsOffering(t *testing.T) {
	task := searchACTask("t1", []string{"infra"}, "")
	outliner := searchACOutliner(primaryRowsProject("alpha", task))

	typeSearchAC(outliner, "#in")
	if !outliner.searchAutocomplete.active() {
		t.Fatal("#in offered nothing")
	}
	typeSearchAC(outliner, "#zzz")
	if outliner.searchAutocomplete.active() {
		t.Fatalf("#zzz kept the pane open: %v", searchACValues(outliner))
	}
	// Free text is not a token at all.
	typeSearchAC(outliner, "plain text")
	if outliner.searchAutocomplete.active() {
		t.Fatalf("free text opened a pane: %v", searchACValues(outliner))
	}
	// Leaving search closes it regardless of what was on screen.
	typeSearchAC(outliner, "#")
	outliner.ClearSearch()
	if outliner.searchAutocomplete.active() {
		t.Error("clearing the search left the pane open")
	}
}
