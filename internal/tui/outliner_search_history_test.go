package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// searchHistoryOutliner builds a scoped Outliner with the slash input open,
// which is the only state the walk exists in.
func searchHistoryOutliner() *OutlinerModel {
	outliner := scopeTestOutliner(scopeTestProject("work", "Work", "alpha", "beta"))
	outliner.SetSize(120, 40)
	outliner.ActivateSearch()
	return outliner
}

// submitSearchQuery types a query and submits it through the real key cascade,
// so the ring is only ever fed the way a user feeds it.
func submitSearchQuery(outliner *OutlinerModel, query string) {
	outliner.SetSearchInputValue(query)
	outliner.SetSearchInputCursor(len([]rune(query)))
	outliner.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
}

func pressSearchKey(outliner *OutlinerModel, code rune) {
	outliner.handleKeyMsg(tea.KeyPressMsg{Code: code})
}

func TestSearchHistoryRecallsSubmittedQueriesNewestFirst(t *testing.T) {
	outliner := searchHistoryOutliner()
	for _, query := range []string{"alpha", "beta", "gamma"} {
		outliner.ActivateSearch()
		submitSearchQuery(outliner, query)
	}

	outliner.ActivateSearch()
	for _, want := range []string{"gamma", "beta", "alpha"} {
		pressSearchKey(outliner, tea.KeyUp)
		if got := outliner.SearchInputValue(); got != want {
			t.Fatalf("recalled %q, want %q", got, want)
		}
		if got := outliner.SearchQuery(); got != want {
			t.Fatalf("recall left the query at %q, want %q", got, want)
		}
	}

	// The oldest entry is a floor rather than a wrap: another Up must not hand
	// back the newest query as if it were older still.
	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "alpha" {
		t.Fatalf("Up past the oldest entry moved to %q, want alpha", got)
	}

	for _, want := range []string{"beta", "gamma"} {
		pressSearchKey(outliner, tea.KeyDown)
		if got := outliner.SearchInputValue(); got != want {
			t.Fatalf("stepped forward to %q, want %q", got, want)
		}
	}
}

// The walk sets the half-typed query aside and gives it back at the newest end,
// so reaching for history never costs what the user was already writing.
func TestSearchHistoryRestoresTheDraftAtTheNewestEnd(t *testing.T) {
	outliner := searchHistoryOutliner()
	submitSearchQuery(outliner, "alpha")

	outliner.ActivateSearch()
	outliner.SetSearchInputValue("half typ")
	outliner.SetSearchInputCursor(len("half typ"))

	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "alpha" {
		t.Fatalf("recalled %q, want alpha", got)
	}
	pressSearchKey(outliner, tea.KeyDown)
	if got := outliner.SearchInputValue(); got != "half typ" {
		t.Fatalf("draft restored as %q, want %q", got, "half typ")
	}
	// Down outside a walk reports nothing, so it can never clear the input.
	pressSearchKey(outliner, tea.KeyDown)
	if got := outliner.SearchInputValue(); got != "half typ" {
		t.Fatalf("Down outside a walk changed the input to %q", got)
	}
}

func TestSearchHistoryRecordsOnlySubmittedQueries(t *testing.T) {
	outliner := searchHistoryOutliner()
	outliner.SetSearchInputValue("abandoned")
	outliner.SetSearchInputCursor(len("abandoned"))
	pressSearchKey(outliner, tea.KeyEscape)

	outliner.ActivateSearch()
	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "" {
		t.Fatalf("an abandoned query was recalled as %q", got)
	}
	if outliner.hasSearchHistory() {
		t.Fatal("an abandoned query joined the ring")
	}

	// A blank submission is not a query either.
	submitSearchQuery(outliner, "   ")
	if outliner.hasSearchHistory() {
		t.Fatal("a blank submission joined the ring")
	}
}

func TestSearchHistoryHoldsEachQueryOnceAndStaysBounded(t *testing.T) {
	var history outlinerSearchHistory
	history.record("alpha")
	history.record("beta")
	history.record("alpha")
	if got := history.entries; len(got) != 2 || got[0] != "beta" || got[1] != "alpha" {
		t.Fatalf("entries = %v, want [beta alpha]", got)
	}

	for i := range outlinerSearchHistoryLimit + 10 {
		history.record(fmt.Sprintf("query-%d", i))
	}
	if got := len(history.entries); got != outlinerSearchHistoryLimit {
		t.Fatalf("ring holds %d entries, want %d", got, outlinerSearchHistoryLimit)
	}
	if got, want := history.entries[0], fmt.Sprintf("query-%d", 10); got != want {
		t.Fatalf("oldest retained entry = %q, want %q", got, want)
	}
}

// Typing ends a walk, so the next Up starts from the newest query rather than
// from wherever the abandoned walk had reached.
func TestSearchHistoryTypingEndsTheWalk(t *testing.T) {
	outliner := searchHistoryOutliner()
	for _, query := range []string{"alpha", "beta"} {
		outliner.ActivateSearch()
		submitSearchQuery(outliner, query)
	}

	outliner.ActivateSearch()
	pressSearchKey(outliner, tea.KeyUp)
	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "alpha" {
		t.Fatalf("walked to %q, want alpha", got)
	}
	outliner.handleKeyMsg(tea.KeyPressMsg{Code: 'x', Text: "x"})
	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "beta" {
		t.Fatalf("after typing, Up recalled %q, want the newest query beta", got)
	}
}

// Reopening the input starts a fresh walk over the surviving ring.
func TestSearchHistoryReopeningResetsThePositionButKeepsTheRing(t *testing.T) {
	outliner := searchHistoryOutliner()
	for _, query := range []string{"alpha", "beta"} {
		outliner.ActivateSearch()
		submitSearchQuery(outliner, query)
	}

	outliner.ActivateSearch()
	pressSearchKey(outliner, tea.KeyUp)
	pressSearchKey(outliner, tea.KeyUp)
	pressSearchKey(outliner, tea.KeyEscape)

	outliner.ActivateSearch()
	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "beta" {
		t.Fatalf("reopened walk started at %q, want the newest query beta", got)
	}
}

// The ring is session-only state on the model: a freshly constructed Outliner
// recalls nothing, which is what "discarded on exit" means for a process that
// reads no file to rebuild it.
func TestSearchHistoryIsSessionOnly(t *testing.T) {
	outliner := searchHistoryOutliner()
	submitSearchQuery(outliner, "alpha")

	relaunched := searchHistoryOutliner()
	if relaunched.hasSearchHistory() {
		t.Fatal("a fresh Outliner carried query history")
	}
	pressSearchKey(relaunched, tea.KeyUp)
	if got := relaunched.SearchInputValue(); got != "" {
		t.Fatalf("a fresh Outliner recalled %q", got)
	}
}

// The suggestion pane keeps Up and Down while it is open, and one Esc closes it
// and gives them back to the walk. That ordering is what lets both cursors exist
// without either borrowing the other's keys.
func TestSearchHistoryYieldsUpAndDownToAnOpenSuggestionPane(t *testing.T) {
	outliner := searchHistoryOutliner()
	submitSearchQuery(outliner, "alpha")

	outliner.ActivateSearch()
	outliner.SetSearchInputValue("$")
	outliner.SetSearchInputCursor(1)
	outliner.refreshSearchAutocomplete()
	if !outliner.searchAutocomplete.active() {
		t.Fatal("fixture produced no suggestion pane")
	}

	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "$" {
		t.Fatalf("the pane lost Up to the walk: input = %q", got)
	}

	pressSearchKey(outliner, tea.KeyEscape)
	if outliner.searchAutocomplete.active() {
		t.Fatal("Esc left the suggestion pane open")
	}
	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "alpha" {
		t.Fatalf("after the pane closed, Up recalled %q, want alpha", got)
	}
}

// Recall binds no root key: it is claimed inside the input, which the shell
// hands every key while search mode is open, and the strategy's own table still
// knows Up and Down only as the cursor keys.
func TestSearchHistoryBindsNoRootKey(t *testing.T) {
	outliner := searchHistoryOutliner()
	if !outliner.simplifiedInlineControlOwnsInput() {
		t.Fatal("search mode does not own the input, so recall would reach root dispatch")
	}

	for _, binding := range SimplifiedInputStrategy().Bindings {
		for _, key := range binding.Keys {
			if key != "up" && key != "down" {
				continue
			}
			if binding.OperationID != operationCoreCursorUp && binding.OperationID != operationCoreCursorDown {
				t.Fatalf("root %q is bound to %q, want the cursor operations alone", key, binding.OperationID)
			}
		}
	}
}

// The footer advertises the walk only once something is there to recall.
func TestSearchHistoryGuidanceAppearsOnlyWithEntries(t *testing.T) {
	outliner := searchHistoryOutliner()
	spec, ok := outliner.ContextualGuidance()
	if !ok {
		t.Fatal("search mode produced no guidance")
	}
	if labels := guidanceHintLabels(spec); strings.Contains(labels, "recall query") {
		t.Fatalf("an empty ring advertised recall: %s", labels)
	}

	submitSearchQuery(outliner, "alpha")
	outliner.ActivateSearch()
	spec, ok = outliner.ContextualGuidance()
	if !ok {
		t.Fatal("search mode produced no guidance after a submission")
	}
	if labels := guidanceHintLabels(spec); !strings.Contains(labels, "↑/↓ recall query") {
		t.Fatalf("recall is not advertised: %s", labels)
	}
}

func guidanceHintLabels(spec GuidanceSpec) string {
	labels := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		labels = append(labels, strings.TrimSpace(hint.Key+" "+hint.Label))
	}
	return strings.Join(labels, " | ")
}
