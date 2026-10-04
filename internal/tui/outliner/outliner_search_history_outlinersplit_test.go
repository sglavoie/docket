package outliner

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// submitSearchQuery types a query and submits it through the real key cascade,
// so the ring is only ever fed the way a user feeds it.
func submitSearchQuery(outliner *OutlinerModel, query string) {
	outliner.SetSearchInputValue(query)
	outliner.SetSearchInputCursor(len([]rune(query)))
	outliner.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
}

func pressSearchKey(outliner *OutlinerModel, code rune) {
	outliner.HandleKeyMsg(tea.KeyPressMsg{Code: code})
}

func TestSearchHistoryRecallsSubmittedQueriesNewestFirst(t *testing.T) {
	outliner := searchHistoryOutliner()
	for _, query := range []string{"alpha", "beta", "gamma"} {
		outliner.ActivateSearch()
		outliner.SetSearchInputValue("")
		submitSearchQuery(outliner, query)
	}

	outliner.ActivateSearch()
	outliner.SetSearchInputValue("")
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
	outliner.SetSearchInputValue("")
	outliner.SetSearchInputValue("al")
	outliner.SetSearchInputCursor(len("al"))

	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "alpha" {
		t.Fatalf("recalled %q, want alpha", got)
	}
	pressSearchKey(outliner, tea.KeyDown)
	if got := outliner.SearchInputValue(); got != "al" {
		t.Fatalf("draft restored as %q, want %q", got, "al")
	}
	// Down outside a walk reports nothing, so it can never clear the input.
	pressSearchKey(outliner, tea.KeyDown)
	if got := outliner.SearchInputValue(); got != "al" {
		t.Fatalf("Down outside a walk changed the input to %q", got)
	}
}

func TestSearchHistoryRecordsOnlySubmittedQueries(t *testing.T) {
	outliner := searchHistoryOutliner()
	outliner.SetSearchInputValue("abandoned")
	outliner.SetSearchInputCursor(len("abandoned"))
	pressSearchKey(outliner, tea.KeyEscape)

	outliner.ActivateSearch()
	outliner.SetSearchInputValue("")
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

func TestSearchHistoryRestoresDraftCaret(t *testing.T) {
	for _, draft := range []string{"invoice", "café東京"} {
		t.Run(draft, func(t *testing.T) {
			outliner := searchHistoryOutliner()
			for _, suffix := range []string{" old", " new"} {
				outliner.ActivateSearch()
				submitSearchQuery(outliner, draft+suffix)
			}
			outliner.ActivateSearch()
			outliner.SetSearchInputValue(draft)
			cursor := len([]rune(draft)) - 1
			outliner.SetSearchInputCursor(cursor)
			for _, suffix := range []string{" new", " old"} {
				pressSearchKey(outliner, tea.KeyUp)
				if got := outliner.SearchInputCursor(); got != len([]rune(draft+suffix)) {
					t.Fatalf("recalled query caret = %d, want end", got)
				}
			}
			pressSearchKey(outliner, tea.KeyDown)
			pressSearchKey(outliner, tea.KeyDown)
			if outliner.SearchInputValue() != draft || outliner.SearchInputCursor() != cursor {
				t.Fatalf("restored %q at %d, want %q at %d", outliner.SearchInputValue(), outliner.SearchInputCursor(), draft, cursor)
			}
			outliner.HandleKeyMsg(tea.KeyPressMsg{Code: 'x', Text: "x"})
			runes := []rune(draft)
			want := string(runes[:cursor]) + "x" + string(runes[cursor:])
			if got := outliner.SearchInputValue(); got != want {
				t.Fatalf("typing after restore = %q, want %q", got, want)
			}
			pressSearchKey(outliner, tea.KeyUp)
			pressSearchKey(outliner, tea.KeyDown)
			if outliner.SearchInputValue() != want || outliner.SearchInputCursor() != cursor+1 {
				t.Fatal("unsuccessful recall moved the edited draft or its caret")
			}
		})
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
	for _, query := range []string{"alphax new", "alpha", "beta"} {
		outliner.ActivateSearch()
		outliner.SetSearchInputValue("")
		submitSearchQuery(outliner, query)
	}

	outliner.ActivateSearch()
	outliner.SetSearchInputValue("")
	pressSearchKey(outliner, tea.KeyUp)
	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "alpha" {
		t.Fatalf("walked to %q, want alpha", got)
	}
	outliner.HandleKeyMsg(tea.KeyPressMsg{Code: 'x', Text: "x"})
	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "alphax new" {
		t.Fatalf("after typing, Up recalled %q, want the newest matching query alphax new", got)
	}
}

// Reopening the input starts a fresh walk over the surviving ring.
func TestSearchHistoryReopeningResetsThePositionButKeepsTheRing(t *testing.T) {
	outliner := searchHistoryOutliner()
	for _, query := range []string{"alpha", "beta"} {
		outliner.ActivateSearch()
		outliner.SetSearchInputValue("")
		submitSearchQuery(outliner, query)
	}

	outliner.ActivateSearch()
	outliner.SetSearchInputValue("")
	pressSearchKey(outliner, tea.KeyUp)
	pressSearchKey(outliner, tea.KeyUp)
	pressSearchKey(outliner, tea.KeyEscape)

	outliner.ActivateSearch()
	outliner.SetSearchInputValue("")
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
	submitSearchQuery(outliner, "$doing")

	outliner.ActivateSearch()
	outliner.SetSearchInputValue("")
	outliner.SetSearchInputValue("$")
	outliner.SetSearchInputCursor(1)
	outliner.RefreshSearchAutocomplete()
	if !outliner.Search.Autocomplete.Active() {
		t.Fatal("fixture produced no suggestion pane")
	}

	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "$" {
		t.Fatalf("the pane lost Up to the walk: input = %q", got)
	}

	pressSearchKey(outliner, tea.KeyEscape)
	if outliner.Search.Autocomplete.Active() {
		t.Fatal("Esc left the suggestion pane open")
	}
	pressSearchKey(outliner, tea.KeyUp)
	if got := outliner.SearchInputValue(); got != "$doing" {
		t.Fatalf("after the pane closed, Up recalled %q, want $doing", got)
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
	outliner.SetSearchInputValue("")
	spec, ok = outliner.ContextualGuidance()
	if !ok {
		t.Fatal("search mode produced no guidance after a submission")
	}
	if labels := guidanceHintLabels(spec); !strings.Contains(labels, "↑/↓ recall query") {
		t.Fatalf("recall is not advertised: %s", labels)
	}
}

func TestSearchHistoryFiltersBothDirectionsByOriginalPrefix(t *testing.T) {
	var h outlinerSearchHistory
	for _, query := range []string{"invoice old", "other", "invoice new", "Invoice upper", "unrelated"} {
		h.record(query)
	}
	for _, want := range []string{"invoice new", "invoice old"} {
		if got, ok := h.older("invoice", 7); !ok || got != want {
			t.Fatalf("older = %q, %v; want %q", got, ok, want)
		}
	}
	if _, ok := h.older("invoice old", 11); ok {
		t.Fatal("walk wrapped past its oldest matching entry")
	}
	for _, want := range []string{"invoice new", "invoice"} {
		if got, ok := h.newer(); !ok || got != want {
			t.Fatalf("newer = %q, %v; want %q", got, ok, want)
		}
	}
	if _, ok := h.older("missing", 7); ok {
		t.Fatal("unmatched prefix recalled an unrelated query")
	}
	if _, ok := h.newer(); ok {
		t.Fatal("Down changed a draft after an unmatched recall")
	}
	h.resetRecall()
	if got, ok := h.older("Invoice", 7); !ok || got != "Invoice upper" {
		t.Fatalf("case-sensitive recall = %q, %v", got, ok)
	}
}
