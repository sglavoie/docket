package tui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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
		mode:           ModeAll,
		scopeProjectID: lensRoundTripScopeID,
		facets:         facets,
		query:          "#api ship",
		sort:           SortState{Field: SortByPriority, Ascending: false},
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
	for _, mode := range OutlinerModeCatalog() {
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
	for _, field := range []SortField{
		SortByDefault, SortByPriority, SortByDueDate, SortByCreatedAt, SortByUpdatedAt, SortByTitle,
	} {
		for _, ascending := range []bool{true, false} {
			name := "sort:" + lensSortAxis.token(field)
			if !ascending {
				name += ":desc"
			}
			variants[name] = func(s *lensState) { s.sort = SortState{Field: field, Ascending: ascending} }
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
	want := lensState{mode: ModeInbox, sort: SortState{Field: SortByDefault}}
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
		"mode": func(s *lensState) { s.mode = ModeInbox },
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
		"sort field":    func(s *lensState) { s.sort.Field = SortByTitle },
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

// reviewFacetAxes are the three narrowing axes a review-flagged lens used to be
// confused with itself. Review is a stored boolean beside Pinned, and before the
// canonical query owned the narrowing the lens encoder wrote Pinned and dropped
// Review outright, so a save or a replace silently widened a review-narrowed
// lens and the fingerprint could not see the axis at all. Holding all three
// apart is what proves the surviving carrier is symmetric: Pinned is Review's
// structural twin, and #review is the tag spelling the stored flag replaced.
var reviewFacetAxes = []struct {
	name  string
	token string
}{
	{"review", "=review"},
	{"pinned", "*pin"},
	{"tag", "#review"},
}

// reviewFacetCombination spells one subset of those axes as a canonical query,
// naming the subset for the subtest and reporting which axes it holds.
func reviewFacetCombination(mask int) (name string, query string, present []bool) {
	present = make([]bool, len(reviewFacetAxes))
	tokens := make([]string, 0, len(reviewFacetAxes))
	names := make([]string, 0, len(reviewFacetAxes))
	for i, axis := range reviewFacetAxes {
		if mask&(1<<i) == 0 {
			continue
		}
		present[i] = true
		tokens = append(tokens, axis.token)
		names = append(names, axis.name)
	}
	if len(names) == 0 {
		return "none", "", present
	}
	return strings.Join(names, "+"), strings.Join(tokens, " "), present
}

// lensStateHoldsReview reports whether the canonical half carries the review
// clause. It reads the clause rather than the rendered string so a tag named
// "review" cannot be mistaken for the flag.
func lensStateHoldsReview(state lensState) bool {
	return len(queryClausesForField(state.facets, models.QueryFieldReview)) == 1
}

// Review must survive every hop a lens takes: capture off a live view, the
// durable encode/decode, the bytes a restart reads back, the replacement that
// re-bases a live lens, and the fingerprint that decides whether the lens reads
// as modified. Each hop is asserted with Pinned and a #review tag held fixed
// beside it, so an encoder that lifts one of those three and drops another
// fails here rather than quietly widening a saved projection.
func TestLensStateRoundTripsReviewFacet(t *testing.T) {
	t.Run("encode and decode carry review independently of pinned and tags", func(t *testing.T) {
		fingerprints := map[string]string{}
		for mask := range 1 << len(reviewFacetAxes) {
			name, query, present := reviewFacetCombination(mask)
			t.Run(name, func(t *testing.T) {
				want := lensStateWithFacets(t, lensRoundTripBase(t), query)
				stored := want.namedLens(lensRoundTripID, "Review "+name)
				if err := stored.Validate(); err != nil {
					t.Fatalf("stored definition invalid: %v", err)
				}
				for i, axis := range reviewFacetAxes {
					if got := strings.Contains(stored.Query, axis.token); got != present[i] {
						t.Fatalf("stored query %q carries %s = %v, want %v",
							stored.Query, axis.token, got, present[i])
					}
				}

				got, err := lensStateFromNamedLens(stored)
				if err != nil {
					t.Fatalf("lensStateFromNamedLens: %v", err)
				}
				if !got.equal(want) {
					t.Fatalf("round trip = %q, want %q", got.fingerprint(), want.fingerprint())
				}
				if lensStateHoldsReview(got) != present[0] {
					t.Fatalf("restored review clause = %v, want %v", lensStateHoldsReview(got), present[0])
				}
				if other, clash := fingerprints[got.fingerprint()]; clash {
					t.Fatalf("combination %q is indistinguishable from %q: %q", name, other, got.fingerprint())
				}
				fingerprints[got.fingerprint()] = name
			})
		}
	})

	// The record a restart reads is JSON on disk, not the in-memory DTO, so the
	// axis is followed through the durable bytes as well as through the encoder.
	t.Run("durable bytes carry review across a restart", func(t *testing.T) {
		want := lensStateWithFacets(t, lensRoundTripBase(t), "=review *pin #review")
		raw, err := json.Marshal(want.namedLens(lensRoundTripID, "Review Queue"))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(raw), "=review") {
			t.Fatalf("persisted bytes dropped the review facet: %s", raw)
		}

		var reloaded config.NamedLens
		if err := json.Unmarshal(raw, &reloaded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		got, err := lensStateFromNamedLens(reloaded)
		if err != nil {
			t.Fatalf("lensStateFromNamedLens: %v", err)
		}
		if !got.equal(want) {
			t.Fatalf("restart round trip = %q, want %q", got.fingerprint(), want.fingerprint())
		}
		if !lensStateHoldsReview(got) {
			t.Fatalf("restarted lens lost the review clause: %q", got.facets.String())
		}
	})

	// fingerprint is the modified/unmodified oracle. An axis it cannot see leaves
	// an edited lens reading as unmodified, which loses the edit with no prompt.
	t.Run("fingerprint watches review alone", func(t *testing.T) {
		withoutReview := lensStateWithFacets(t, lensRoundTripBase(t), "*pin #review")
		withReview := lensStateWithFacets(t, lensRoundTripBase(t), "=review *pin #review")
		if withoutReview.fingerprint() == withReview.fingerprint() {
			t.Fatalf("adding review left the lens reading as unmodified: %q", withReview.fingerprint())
		}

		reviewOnly := lensStateWithFacets(t, lensRoundTripBase(t), "=review")
		pinnedOnly := lensStateWithFacets(t, lensRoundTripBase(t), "*pin")
		taggedOnly := lensStateWithFacets(t, lensRoundTripBase(t), "#review")
		if reviewOnly.fingerprint() == pinnedOnly.fingerprint() {
			t.Fatal("review and pinned produced one fingerprint")
		}
		if reviewOnly.fingerprint() == taggedOnly.fingerprint() {
			t.Fatal("the review flag and a #review tag produced one fingerprint")
		}
	})

	// The carrier lift is the direction that used to lose the axis: the f= facet
	// writes CommonFilterState.Review, and a capture reads the query the carriers
	// were adopted into.
	t.Run("a live view captures and reapplies the review facet", func(t *testing.T) {
		outliner := scopeTestOutliner(primaryRowsProject("alpha"))
		outliner.filter.Review = true
		outliner.filter.Pinned = true
		outliner.filter.Tags = map[string]bool{"review": true}
		outliner.adoptFilterCarriers()

		captured := outliner.captureLensState()
		if !lensStateHoldsReview(captured) {
			t.Fatalf("captured facets dropped the review flag: %q", captured.facets.String())
		}
		if want := "#review *pin =review"; captured.facets.String() != want {
			t.Fatalf("captured facets = %q, want %q", captured.facets.String(), want)
		}

		restored, err := lensStateFromNamedLens(captured.namedLens(lensRoundTripID, "Review"))
		if err != nil {
			t.Fatalf("lensStateFromNamedLens: %v", err)
		}
		reopened := scopeTestOutliner(primaryRowsProject("alpha"))
		if !reopened.applyLensState(restored) {
			t.Fatal("applyLensState refused the definition")
		}
		if !reopened.filter.Review {
			t.Fatal("applying the lens left the review facet off")
		}
		if !reopened.filter.Pinned || !reopened.filter.Tags["review"] {
			t.Fatalf("applying the lens disturbed the axes beside review: %+v", reopened.filter.CommonFilterState)
		}
		if !reopened.captureLensState().equal(captured) {
			t.Fatalf("recaptured %q after applying %q",
				reopened.captureLensState().fingerprint(), captured.fingerprint())
		}
	})

	// Replace overwrites the saved record from the live view, so it is the hop
	// that persists a review narrowing the user added after opening the lens —
	// and the hop that must drop it again when they take it back off.
	t.Run("replacing a lens records and clears review", func(t *testing.T) {
		model := lensOperationApp(t)
		out := model.crossOutliner
		starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
		applyLensOrFatal(t, out, starter)

		out.filter.Review = true
		out.adoptFilterCarriers()
		if !out.lensModified() {
			t.Fatal("adding the review facet must mark the lens modified")
		}

		if result := executeLens(model, operationLensReplace, LensEntity(starter.ID), "", true); result.Err != nil {
			t.Fatalf("replace lens: %v", result.Err)
		}
		replaced := lensNamed(t, model, starter.Name)
		if !strings.Contains(replaced.Query, "=review") {
			t.Fatalf("replaced query = %q, want the review facet the view carried", replaced.Query)
		}
		if out.lensModified() {
			t.Fatal("replacing the active lens must clear the modified marker")
		}

		out.filter.Review = false
		out.adoptFilterCarriers()
		if !out.lensModified() {
			t.Fatal("dropping the review facet must mark the lens modified")
		}
		if result := executeLens(model, operationLensReplace, LensEntity(starter.ID), "", true); result.Err != nil {
			t.Fatalf("replace lens: %v", result.Err)
		}
		if again := lensNamed(t, model, starter.Name); strings.Contains(again.Query, "=review") {
			t.Fatalf("replaced query = %q, want the review facet dropped", again.Query)
		}
		if out.lensModified() {
			t.Fatal("replacing the active lens must clear the modified marker")
		}
	})
}

func TestCaptureLensStateReadsRememberedTreeSortAndDetachesCollections(t *testing.T) {
	outliner := scopeTestOutliner(primaryRowsProject("alpha"))
	outliner.primaryMode = ModeDue
	outliner.setLensQuery("  ship it  ")
	outliner.filter.Tags = map[string]bool{"api": true}
	outliner.projectFilter.Tags = map[string]bool{"client": true}
	outliner.adoptFilterCarriers()
	// Due owns its order, so the active sort is default while the user's manual
	// choice survives in treeSort. The lens owns the remembered one.
	outliner.sort = NewSortState()
	outliner.treeSort = SortState{Field: SortByTitle, Ascending: false}

	captured := outliner.captureLensState()
	if captured.mode != ModeDue || captured.query != "ship it" {
		t.Fatalf("captured mode/query = %s/%q", captured.mode, captured.query)
	}
	if captured.sort != (SortState{Field: SortByTitle, Ascending: false}) {
		t.Fatalf("captured sort = %+v, want the remembered tree sort", captured.sort)
	}
	if want := "#api +#client"; captured.facets.String() != want {
		t.Fatalf("captured facets = %q, want %q", captured.facets.String(), want)
	}

	before := captured.facets.String()
	outliner.filter.Tags["late"] = true
	outliner.projectFilter.Tags["late"] = true
	outliner.adoptFilterCarriers()
	if captured.facets.String() != before {
		t.Fatalf("a later facet edit reached into the captured definition: %q", captured.facets.String())
	}
}

func TestApplyLensStateInstallsManualSortAndRemembersItInFixedOrderModes(t *testing.T) {
	saved := SortState{Field: SortByUpdatedAt, Ascending: false}
	for _, mode := range OutlinerModeCatalog() {
		t.Run(string(mode.ID), func(t *testing.T) {
			outliner := scopeTestOutliner(primaryRowsProject("alpha"))
			outliner.sort = SortState{Field: SortByTitle, Ascending: true}
			outliner.treeSort = SortState{Field: SortByTitle, Ascending: true}

			state := lensState{mode: mode.ID, sort: saved}
			if !outliner.applyLensState(state) {
				t.Fatal("applyLensState refused a catalog mode")
			}
			if outliner.primaryMode != mode.ID {
				t.Fatalf("mode = %s, want %s", outliner.primaryMode, mode.ID)
			}
			if outliner.treeSort != saved {
				t.Fatalf("treeSort = %+v, want the saved manual sort %+v", outliner.treeSort, saved)
			}
			wantActive := saved
			if mode.FixedOrder() {
				wantActive = NewSortState()
			}
			if outliner.sort != wantActive {
				t.Fatalf("%s active sort = %+v, want %+v", mode.ID, outliner.sort, wantActive)
			}
		})
	}

	outliner := scopeTestOutliner(primaryRowsProject("alpha"))
	if outliner.applyLensState(lensState{mode: OutlinerModeID("archive")}) {
		t.Fatal("applyLensState accepted a mode outside the catalog")
	}
	if outliner.primaryMode != ModeAll {
		t.Fatalf("a refused lens changed the mode to %s", outliner.primaryMode)
	}
}

func TestApplyLensStateDetachesFilterCollectionsFromTheDefinition(t *testing.T) {
	outliner := scopeTestOutliner(primaryRowsProject("alpha"))
	state := lensRoundTripBase(t)
	// The scope axis is only readable back when the base workspace admits the
	// project it names: that population is the one scope the Outliner can be asked
	// for, so it is the one a capture reports.
	state.scopeProjectID = "alpha"
	if !outliner.applyLensState(state) {
		t.Fatal("applyLensState refused the definition")
	}
	if outliner.scopedProjectID != "alpha" {
		t.Fatalf("applyLensState did not enter the lens's scope, got %q", outliner.scopedProjectID)
	}
	if !outliner.captureLensState().equal(state) {
		t.Fatalf("captured %q after applying %q", outliner.captureLensState().fingerprint(), state.fingerprint())
	}

	before := state.facets.String()
	outliner.filter.Tags["live"] = true
	outliner.projectFilter.Tags["live"] = true
	outliner.adoptFilterCarriers()
	if state.facets.String() != before {
		t.Fatalf("a live facet edit mutated the applied definition: %q", state.facets.String())
	}
}
