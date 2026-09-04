package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/query"
)

// lensRoundTripID is a canonical UUID, which is what a stored definition must
// carry for NamedLens.Validate to accept it.
const lensRoundTripID = "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c01"

// lensRoundTripScopeID is the project a scoped definition names. It is a
// canonical UUID for the same reason the lens ID is: NamedLens.Validate holds
// the scope to one spelling.
const lensRoundTripScopeID = "2b7d5e91-3c48-4f16-8a05-c9e7f30d4b62"

// equal reports whether two states name the same projection. It reads the
// fingerprint rather than comparing fields, because the fingerprint is what
// production compares: lensModified and the replace re-base both hold the live
// state against a stored fingerprint string, never against another lensState.
// That is why this convenience lives with the assertions instead of beside
// fingerprint itself.
func (s lensState) equal(other lensState) bool {
	return s.fingerprint() == other.fingerprint()
}

// lensRoundTripFacets is a narrowing touching every kind of canonical clause a
// lens can hold: a value-carrying facet, a repeated one, the reserved unset
// token, a derived predicate, a negation, a population selector, and the
// dependency axis the retired filter carriers could not persist at all.
const lensRoundTripFacets = "!1 #api #zeta ##deep ##errand @week $waiting $doing " +
	"*pin =review ?stale ?deferred ?completed:30d -?leaf +#client +#0 +parked +archived:only"

func lensRoundTripBase(t *testing.T) lensState {
	t.Helper()
	facets, err := models.ParseQuery(lensRoundTripFacets)
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	return lensState{
		mode:           chrome.ModeAll,
		scopeProjectID: lensRoundTripScopeID,
		facets:         facets,
		query:          "#api ship",
		sort:           query.SortState{Field: query.SortByPriority, Ascending: false},
	}
}

// lensStateWithFacets replaces the canonical half of one state's narrowing.
func lensStateWithFacets(t *testing.T, state lensState, query string) lensState {
	t.Helper()
	facets, err := models.ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", query, err)
	}
	state.facets = facets
	return state
}

func TestLensStateRoundTripsEveryModeFacetPopulationAndSortAxis(t *testing.T) {
	base := lensRoundTripBase(t)

	variants := map[string]func(*lensState){}
	for _, mode := range chrome.OutlinerModeCatalog() {
		variants["mode:"+string(mode.ID)] = func(s *lensState) { s.mode = mode.ID }
	}
	// Every canonical field a lens may narrow on, one variant each, so an axis
	// that cannot survive the durable string fails here rather than silently
	// widening a restored lens.
	for _, facets := range []string{
		"", `"needle"`, "!2", "!0", "#api", "#api*", "##deep", "##0",
		"@overdue", "@today", "@week", "@2026-01-05", "@+3h", "@0", "@@2026-01-05",
		"$backlog", "$done $canceled", "*pin", "=review", "?stale", "?deferred",
		"?completed", "?completed:7d", "?parent", "?child", "?leaf", "?blocked", "?blocking",
		"-#api", "-!2", "-$doing", "-*pin", "-=review", "-?stale",
		"+#client", "+#0", "+parked", "+parked:only", "+archived", "+archived:only",
	} {
		variants["facets:"+facets] = func(s *lensState) { *s = lensStateWithFacets(t, *s, facets) }
	}
	for _, field := range []query.SortField{
		query.SortByDefault, query.SortByPriority, query.SortByDueDate, query.SortByCreatedAt, query.SortByUpdatedAt, query.SortByTitle,
	} {
		for _, ascending := range []bool{true, false} {
			name := "sort:" + lens.SortAxis.Token(field)
			if !ascending {
				name += ":desc"
			}
			variants[name] = func(s *lensState) { s.sort = query.SortState{Field: field, Ascending: ascending} }
		}
	}
	// The remainder is the half the canonical vocabulary cannot spell. It round
	// trips verbatim, including the case where it is the whole narrowing.
	variants["query:absent"] = func(s *lensState) { s.query = "" }
	variants["query:only"] = func(s *lensState) {
		*s = lensStateWithFacets(t, *s, "")
		s.query = "ship it"
	}
	// The unowned scope is a distinct record from a named one, and the empty
	// string is its only spelling, so it round trips as absence rather than as a
	// default token some axis table supplies.
	variants["scope:unowned"] = func(s *lensState) { s.scopeProjectID = "" }

	for name, mutate := range variants {
		t.Run(name, func(t *testing.T) {
			want := base
			mutate(&want)

			stored := want.namedLens(lensRoundTripID, "Round Trip")
			if err := stored.Validate(); err != nil {
				t.Fatalf("stored definition invalid: %v", err)
			}
			got, err := lensStateFromNamedLens(stored)
			if err != nil {
				t.Fatalf("lensStateFromNamedLens: %v", err)
			}
			if !got.equal(want) {
				t.Fatalf("round trip = %q, want %q", got.fingerprint(), want.fingerprint())
			}
		})
	}
}

func TestLensStateDecodesOmittedTokensAsAxisDefaultsAndRefusesUnknownSpellings(t *testing.T) {
	stored := config.NamedLens{
		ID:   lensRoundTripID,
		Name: "Defaults",
		Mode: config.LensModeInbox,
	}
	got, err := lensStateFromNamedLens(stored)
	if err != nil {
		t.Fatalf("lensStateFromNamedLens: %v", err)
	}
	want := lensState{mode: chrome.ModeInbox, sort: query.SortState{Field: query.SortByDefault}}
	if !got.equal(want) {
		t.Fatalf("omitted tokens decoded to %q, want axis defaults %q", got.fingerprint(), want.fingerprint())
	}

	for name, mutate := range map[string]func(*config.NamedLens){
		"mode":     func(l *config.NamedLens) { l.Mode = "" },
		"sort":     func(l *config.NamedLens) { l.Sort.Field = "manual" },
		"scope":    func(l *config.NamedLens) { l.Scope = "Alpha" },
		"oversize": func(l *config.NamedLens) { l.Query = strings.Repeat("a", config.MaxNamedLensQueryRunes+1) },
	} {
		t.Run("malformed/"+name, func(t *testing.T) {
			broken := stored
			mutate(&broken)
			if _, err := lensStateFromNamedLens(broken); err == nil {
				t.Fatalf("malformed %s token was accepted", name)
			}
		})
	}

	// A narrowing token the canonical vocabulary cannot spell is not malformed:
	// it is the remainder the textgrammar surfaces own, and it decodes rather
	// than refusing, because compiling it belongs to projection time.
	t.Run("unparseable narrowing decodes as the remainder", func(t *testing.T) {
		carried := stored
		carried.Query = "@someday needle"
		state, err := lensStateFromNamedLens(carried)
		if err != nil {
			t.Fatalf("lensStateFromNamedLens: %v", err)
		}
		if state.query != "@someday needle" {
			t.Fatalf("remainder = %q, want the verbatim narrowing", state.query)
		}
		if !state.facets.IsEmpty() {
			t.Fatalf("unparseable tokens became clauses: %q", state.facets.String())
		}
	})
}

// The canonical half is rendered in canonical order, so the same narrowing
// spelled in two orders is one record: nothing about the order a facet was
// chosen in can make an unchanged saved lens read as modified.
func TestLensStateFingerprintIsIndependentOfFacetSpellingOrder(t *testing.T) {
	first := lensStateWithFacets(t, lensRoundTripBase(t), "#zeta #api $waiting !1")
	second := lensStateWithFacets(t, lensRoundTripBase(t), "!1 $waiting #api #zeta")

	if first.fingerprint() != second.fingerprint() {
		t.Fatalf("fingerprints differ by spelling order:\n%q\n%q",
			first.fingerprint(), second.fingerprint())
	}
	if !first.equal(second) {
		t.Fatal("equal disagreed with identical fingerprints")
	}

	// A canonical token typed into the remainder is lifted into the clause half
	// on the way out, so the two halves cannot disagree about where a facet is
	// and a freshly applied lens cannot read as modified.
	split := lensStateWithFacets(t, lensRoundTripBase(t), "#api $waiting")
	split.query = "!1 #zeta ship"
	if split.fingerprint() != first.fingerprint() {
		t.Fatalf("a facet in the remainder changed the record:\n%q\n%q",
			split.fingerprint(), first.fingerprint())
	}

	if lensStateWithFacets(t, first, "#api $waiting !1").equal(first) {
		t.Fatal("dropping a tag facet did not change the fingerprint")
	}
}

// fingerprint is the modified/unmodified oracle: an axis namedLens writes but
// fingerprint omits is silently unwatched, so editing it leaves a saved lens
// reading as unmodified and the edit is lost without a prompt. This walks every
// lens-owned axis one at a time and requires each to move the fingerprint, so a
// future axis cannot be added to namedLens alone.
func TestLensStateFingerprintWatchesEveryLensOwnedAxis(t *testing.T) {
	mutations := map[string]func(*lensState){
		"mode": func(s *lensState) { s.mode = chrome.ModeInbox },
		// Scope is lens-owned, so a push or a pop under a live lens has to read as
		// a divergence; dropping it is the unowned axis, which must read as one too.
		"scope":         func(s *lensState) { s.scopeProjectID = lensRoundTripID },
		"scope dropped": func(s *lensState) { s.scopeProjectID = "" },
		"query":         func(s *lensState) { s.query = "something else" },
		"query dropped": func(s *lensState) { s.query = "" },
		"facet added":   func(s *lensState) { *s = lensStateWithFacets(t, *s, lensRoundTripFacets+" ?parent") },
		"facet dropped": func(s *lensState) { *s = lensStateWithFacets(t, *s, "!1") },
		"facet value":   func(s *lensState) { *s = lensStateWithFacets(t, *s, "!2") },
		"facet polarity": func(s *lensState) {
			*s = lensStateWithFacets(t, *s, strings.Replace(lensRoundTripFacets, "#api", "-#api", 1))
		},
		"sort field":    func(s *lensState) { s.sort.Field = query.SortByTitle },
		"sort ordering": func(s *lensState) { s.sort.Ascending = !s.sort.Ascending },
	}

	base := lensRoundTripBase(t)
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			edited := base
			mutate(&edited)
			if edited.fingerprint() == base.fingerprint() {
				t.Fatalf("editing %s left the lens reading as unmodified: %q", name, base.fingerprint())
			}
		})
	}
}

// The two halves of the durable string stay apart in the record: canonical
// clauses first, in canonical order, and the remainder verbatim behind them.
func TestLensStateStoresCanonicalClausesAheadOfTheRemainder(t *testing.T) {
	state := lensStateWithFacets(t, lensRoundTripBase(t), "#zeta #api ##deep")
	state.query = "ship it"
	stored := state.namedLens(lensRoundTripID, "Halves")

	if want := "#api #zeta ##deep ship it"; stored.Query != want {
		t.Fatalf("stored query = %q, want %q", stored.Query, want)
	}

	got, err := lensStateFromNamedLens(stored)
	if err != nil {
		t.Fatalf("lensStateFromNamedLens: %v", err)
	}
	if want := "#api #zeta ##deep"; got.facets.String() != want {
		t.Fatalf("restored facets = %q, want %q", got.facets.String(), want)
	}
	if got.query != "ship it" {
		t.Fatalf("restored remainder = %q, want the verbatim prose", got.query)
	}
}

func TestLensStateStarterRecordsConvertAndRoundTrip(t *testing.T) {
	for _, starter := range config.StarterNamedLenses() {
		t.Run(starter.Name, func(t *testing.T) {
			state, err := lensStateFromNamedLens(starter)
			if err != nil {
				t.Fatalf("lensStateFromNamedLens(%q): %v", starter.Name, err)
			}
			again, err := lensStateFromNamedLens(state.namedLens(starter.ID, starter.Name))
			if err != nil {
				t.Fatalf("re-encoded starter refused: %v", err)
			}
			if !again.equal(state) {
				t.Fatalf("starter %q did not survive re-encoding", starter.Name)
			}
		})
	}

	waiting, err := lensStateFromNamedLens(config.StarterNamedLenses()[0])
	if err != nil {
		t.Fatalf("lensStateFromNamedLens: %v", err)
	}
	if waiting.facets.IsEmpty() {
		t.Fatal("Waiting starter carries no facets at all")
	}
	if !strings.Contains(waiting.facets.String(), "$"+string(models.StateWaiting)) {
		t.Fatalf("Waiting starter lost its state facet: %q", waiting.facets.String())
	}
}
