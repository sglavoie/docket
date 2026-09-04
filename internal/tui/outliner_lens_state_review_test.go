package tui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
)

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
	return len(query.ClausesForField(state.facets, models.QueryFieldReview)) == 1
}

// Review must survive every hop a lens takes: capture off a live view, the
// durable encode/decode, the bytes a restart reads back, the replacement that
// re-bases a live lens, and the fingerprint that decides whether the lens reads
// as modified. Each hop is asserted with Pinned and a #review tag held fixed
// beside it, so an encoder that lifts one of those three and drops another
// fails here rather than quietly widening a saved projection.
func TestLensStateRoundTripsReviewFacet(t *testing.T) {
	t.Run("encode and decode carry review independently of pinned and tags", reviewFacetSurvivesEncodeDecode)
	t.Run("durable bytes carry review across a restart", reviewFacetSurvivesDurableBytes)
	t.Run("fingerprint watches review alone", reviewFacetIsWatchedByFingerprint)
	t.Run("a live view captures and reapplies the review facet", reviewFacetCapturedFromLiveView)
	t.Run("replacing a lens records and clears review", reviewFacetRecordedAndClearedByReplace)
}

func reviewFacetSurvivesEncodeDecode(t *testing.T) {
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
}

// The record a restart reads is JSON on disk, not the in-memory DTO, so the
// axis is followed through the durable bytes as well as through the encoder.
func reviewFacetSurvivesDurableBytes(t *testing.T) {
	want := lensStateWithFacets(t, lensRoundTripBase(t), "=review *pin #review")
	raw, err := json.Marshal(want.namedLens(lensRoundTripID, "Review Queue"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), "=review") {
		t.Fatalf("persisted bytes dropped the review facet: %s", raw)
	}

	var reloaded config.NamedLens
	if err = json.Unmarshal(raw, &reloaded); err != nil {
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
}

// fingerprint is the modified/unmodified oracle. An axis it cannot see leaves
// an edited lens reading as unmodified, which loses the edit with no prompt.
func reviewFacetIsWatchedByFingerprint(t *testing.T) {
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
}

// The carrier lift is the direction that used to lose the axis: the f= facet
// writes CommonFilterState.Review, and a capture reads the query the carriers
// were adopted into.
func reviewFacetCapturedFromLiveView(t *testing.T) {
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
}

// Replace overwrites the saved record from the live view, so it is the hop
// that persists a review narrowing the user added after opening the lens —
// and the hop that must drop it again when they take it back off.
func reviewFacetRecordedAndClearedByReplace(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	starter := lensNamed(t, model, config.StarterNamedLenses()[0].Name)
	applyLensOrFatal(t, out, starter)

	out.filter.Review = true
	out.adoptFilterCarriers()
	if !out.lensModified() {
		t.Fatal("adding the review facet must mark the lens modified")
	}

	if result := executeLens(model, lens.OperationLensReplace, operation.LensEntity(starter.ID), "", true); result.Err != nil {
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
	if result := executeLens(model, lens.OperationLensReplace, operation.LensEntity(starter.ID), "", true); result.Err != nil {
		t.Fatalf("replace lens: %v", result.Err)
	}
	if again := lensNamed(t, model, starter.Name); strings.Contains(again.Query, "=review") {
		t.Fatalf("replaced query = %q, want the review facet dropped", again.Query)
	}
	if out.lensModified() {
		t.Fatal("replacing the active lens must clear the modified marker")
	}
}

func TestCaptureLensStateReadsRememberedTreeSortAndDetachesCollections(t *testing.T) {
	outliner := scopeTestOutliner(primaryRowsProject("alpha"))
	outliner.primaryMode = chrome.ModeDue
	outliner.setLensQuery("  ship it  ")
	outliner.filter.Tags = map[string]bool{"api": true}
	outliner.projectFilter.Tags = map[string]bool{"client": true}
	outliner.adoptFilterCarriers()
	// Due owns its order, so the active sort is default while the user's manual
	// choice survives in treeSort. The lens owns the remembered one.
	outliner.sort = query.NewSortState()
	outliner.treeSort = query.SortState{Field: query.SortByTitle, Ascending: false}

	captured := outliner.captureLensState()
	if captured.mode != chrome.ModeDue || captured.query != "ship it" {
		t.Fatalf("captured mode/query = %s/%q", captured.mode, captured.query)
	}
	if captured.sort != (query.SortState{Field: query.SortByTitle, Ascending: false}) {
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
	saved := query.SortState{Field: query.SortByUpdatedAt, Ascending: false}
	for _, mode := range chrome.OutlinerModeCatalog() {
		t.Run(string(mode.ID), func(t *testing.T) {
			outliner := scopeTestOutliner(primaryRowsProject("alpha"))
			outliner.sort = query.SortState{Field: query.SortByTitle, Ascending: true}
			outliner.treeSort = query.SortState{Field: query.SortByTitle, Ascending: true}

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
				wantActive = query.NewSortState()
			}
			if outliner.sort != wantActive {
				t.Fatalf("%s active sort = %+v, want %+v", mode.ID, outliner.sort, wantActive)
			}
		})
	}

	outliner := scopeTestOutliner(primaryRowsProject("alpha"))
	if outliner.applyLensState(lensState{mode: chrome.OutlinerModeID("archive")}) {
		t.Fatal("applyLensState accepted a mode outside the catalog")
	}
	if outliner.primaryMode != chrome.ModeAll {
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
	if outliner.scope.projectID != "alpha" {
		t.Fatalf("applyLensState did not enter the lens's scope, got %q", outliner.scope.projectID)
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
