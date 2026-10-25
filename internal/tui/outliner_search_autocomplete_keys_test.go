package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

// TestSearchAutocompleteKeysClaimOnlyTheDropdownControls is the key contract. Tab
// and the navigation keys belong to the pane; every other key, printable
// letters among them, still reaches the search input, so completion never makes a
// character untypeable.
func TestSearchAutocompleteKeysClaimOnlyTheDropdownControls(t *testing.T) {
	tasks := []*models.Task{
		searchACTask("t1", []string{"alpha"}),
		searchACTask("t2", []string{"beta"}),
		searchACTask("t3", []string{"gamma"}),
		searchACTask("t4", []string{"delta"}),
	}
	outlinerView := searchACOutliner(primaryRowsProject("alpha", tasks...))
	typeSearchAC(outlinerView, "#")

	for _, key := range []string{"down", "ctrl+n", "ctrl+j"} {
		before := outlinerView.Search.Autocomplete.Ac.Cursor
		if !outlinerView.HandleSearchAutocompleteKey(key) {
			t.Fatalf("%s was not claimed by the open pane", key)
		}
		if got := outlinerView.Search.Autocomplete.Ac.Cursor; got != before+1 {
			t.Errorf("%s moved the cursor to %d, want %d", key, got, before+1)
		}
	}
	for _, key := range []string{"up", "ctrl+p", "ctrl+k"} {
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
	// Every printable key still types, j and k included, so completion makes no
	// letter unreachable while a sigil value is being typed.
	for _, key := range []string{"a", "z", "1", "j", "k"} {
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

// TestSearchAutocompleteJKTypeIntoTheQueryWhilePaneOpen drives real key presses:
// the pane opens exactly while a sigil value is typed, so a tag spelled with j or
// k must reach the query whole rather than lose those letters to navigation.
func TestSearchAutocompleteJKTypeIntoTheQueryWhilePaneOpen(t *testing.T) {
	tasks := []*models.Task{
		searchACTask("task-one", []string{"junk"}),
		searchACTask("task-two", []string{"kiwi"}),
	}
	outlinerView := searchACOutliner(primaryRowsProject("alpha", tasks...))
	outlinerView.HandleKeyMsg(syntheticKeyStr("#"))
	if !outlinerView.Search.Autocomplete.Active() {
		t.Fatal("the tag sigil opened no pane")
	}
	resultCursor := outlinerView.Cursor
	for _, key := range []string{"j", "u", "n", "k"} {
		outlinerView.HandleKeyMsg(syntheticKeyStr(key))
	}
	if got, want := outlinerView.SearchInputValue(), "#junk"; got != want {
		t.Errorf("query = %q, want %q", got, want)
	}
	if got := outlinerView.Cursor; got != resultCursor {
		t.Errorf("typing moved the result cursor to %d, want it left at %d", got, resultCursor)
	}
}

// TestSearchAutocompleteEnterStillLandsOnTheResult holds the one key the pane
// must never intercept: Enter belongs to the search's own landing path, and an
// open pane may not turn it into an acceptance.
func TestSearchAutocompleteEnterStillLandsOnTheResult(t *testing.T) {
	task := searchACTask("t1", []string{"infra"})
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
	task := searchACTask("t1", []string{"infra"})
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
	task := searchACTask("t1", []string{"infra"})
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
