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
	"github.com/sglavoie/tt/internal/tui/outliner"
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
func lensStateHoldsReview(state outliner.LensState) bool {
	return len(query.ClausesForField(state.Facets, models.QueryFieldReview)) == 1
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
			stored := want.NamedLens(lensRoundTripID, "Review "+name)
			if err := stored.Validate(); err != nil {
				t.Fatalf("stored definition invalid: %v", err)
			}
			for i, axis := range reviewFacetAxes {
				if got := strings.Contains(stored.Query, axis.token); got != present[i] {
					t.Fatalf("stored query %q carries %s = %v, want %v",
						stored.Query, axis.token, got, present[i])
				}
			}

			got, err := outliner.LensStateFromNamedLens(stored)
			if err != nil {
				t.Fatalf("lensStateFromNamedLens: %v", err)
			}
			if got.Fingerprint() != want.Fingerprint() {
				t.Fatalf("round trip = %q, want %q", got.Fingerprint(), want.Fingerprint())
			}
			if lensStateHoldsReview(got) != present[0] {
				t.Fatalf("restored review clause = %v, want %v", lensStateHoldsReview(got), present[0])
			}
			if other, clash := fingerprints[got.Fingerprint()]; clash {
				t.Fatalf("combination %q is indistinguishable from %q: %q", name, other, got.Fingerprint())
			}
			fingerprints[got.Fingerprint()] = name
		})
	}
}

// The record a restart reads is JSON on disk, not the in-memory DTO, so the
// axis is followed through the durable bytes as well as through the encoder.
func reviewFacetSurvivesDurableBytes(t *testing.T) {
	want := lensStateWithFacets(t, lensRoundTripBase(t), "=review *pin #review")
	raw, err := json.Marshal(want.NamedLens(lensRoundTripID, "Review Queue"))
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
	got, err := outliner.LensStateFromNamedLens(reloaded)
	if err != nil {
		t.Fatalf("lensStateFromNamedLens: %v", err)
	}
	if got.Fingerprint() != want.Fingerprint() {
		t.Fatalf("restart round trip = %q, want %q", got.Fingerprint(), want.Fingerprint())
	}
	if !lensStateHoldsReview(got) {
		t.Fatalf("restarted lens lost the review clause: %q", got.Facets.String())
	}
}

// fingerprint is the modified/unmodified oracle. An axis it cannot see leaves
// an edited lens reading as unmodified, which loses the edit with no prompt.
func reviewFacetIsWatchedByFingerprint(t *testing.T) {
	withoutReview := lensStateWithFacets(t, lensRoundTripBase(t), "*pin #review")
	withReview := lensStateWithFacets(t, lensRoundTripBase(t), "=review *pin #review")
	if withoutReview.Fingerprint() == withReview.Fingerprint() {
		t.Fatalf("adding review left the lens reading as unmodified: %q", withReview.Fingerprint())
	}

	reviewOnly := lensStateWithFacets(t, lensRoundTripBase(t), "=review")
	pinnedOnly := lensStateWithFacets(t, lensRoundTripBase(t), "*pin")
	taggedOnly := lensStateWithFacets(t, lensRoundTripBase(t), "#review")
	if reviewOnly.Fingerprint() == pinnedOnly.Fingerprint() {
		t.Fatal("review and pinned produced one fingerprint")
	}
	if reviewOnly.Fingerprint() == taggedOnly.Fingerprint() {
		t.Fatal("the review flag and a #review tag produced one fingerprint")
	}
}

// The carrier lift is the direction that used to lose the axis: the f= facet
// writes CommonFilterState.Review, and a capture reads the query the carriers
// were adopted into.
func reviewFacetCapturedFromLiveView(t *testing.T) {
	outlinerView := scopeTestOutliner(primaryRowsProject("alpha"))
	outlinerView.Filter.Review = true
	outlinerView.Filter.Pinned = true
	outlinerView.Filter.Tags = map[string]bool{"review": true}
	outlinerView.AdoptFilterCarriers()

	captured := outlinerView.CaptureLensState()
	if !lensStateHoldsReview(captured) {
		t.Fatalf("captured facets dropped the review flag: %q", captured.Facets.String())
	}
	if want := "#review *pin =review"; captured.Facets.String() != want {
		t.Fatalf("captured facets = %q, want %q", captured.Facets.String(), want)
	}

	restored, err := outliner.LensStateFromNamedLens(captured.NamedLens(lensRoundTripID, "Review"))
	if err != nil {
		t.Fatalf("lensStateFromNamedLens: %v", err)
	}
	reopened := scopeTestOutliner(primaryRowsProject("alpha"))
	if !reopened.ApplyLensState(restored) {
		t.Fatal("applyLensState refused the definition")
	}
	if !reopened.Filter.Review {
		t.Fatal("applying the lens left the review facet off")
	}
	if !reopened.Filter.Pinned || !reopened.Filter.Tags["review"] {
		t.Fatalf("applying the lens disturbed the axes beside review: %+v", reopened.Filter.CommonFilterState)
	}
	if reopened.CaptureLensState().Fingerprint() != captured.Fingerprint() {
		t.Fatalf("recaptured %q after applying %q",
			reopened.CaptureLensState().Fingerprint(), captured.Fingerprint())
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

	out.Filter.Review = true
	out.AdoptFilterCarriers()
	if !out.LensModified() {
		t.Fatal("adding the review facet must mark the lens modified")
	}

	if result := executeLens(model, lens.OperationLensReplace, operation.LensEntity(starter.ID), "", true); result.Err != nil {
		t.Fatalf("replace lens: %v", result.Err)
	}
	replaced := lensNamed(t, model, starter.Name)
	if !strings.Contains(replaced.Query, "=review") {
		t.Fatalf("replaced query = %q, want the review facet the view carried", replaced.Query)
	}
	if out.LensModified() {
		t.Fatal("replacing the active lens must clear the modified marker")
	}

	out.Filter.Review = false
	out.AdoptFilterCarriers()
	if !out.LensModified() {
		t.Fatal("dropping the review facet must mark the lens modified")
	}
	if result := executeLens(model, lens.OperationLensReplace, operation.LensEntity(starter.ID), "", true); result.Err != nil {
		t.Fatalf("replace lens: %v", result.Err)
	}
	if again := lensNamed(t, model, starter.Name); strings.Contains(again.Query, "=review") {
		t.Fatalf("replaced query = %q, want the review facet dropped", again.Query)
	}
	if out.LensModified() {
		t.Fatal("replacing the active lens must clear the modified marker")
	}
}

func TestCaptureLensStateReadsRememberedTreeSortAndDetachesCollections(t *testing.T) {
	outlinerView := scopeTestOutliner(primaryRowsProject("alpha"))
	outlinerView.PrimaryMode = chrome.ModeDue
	outlinerView.SetLensQuery("  ship it  ")
	outlinerView.Filter.Tags = map[string]bool{"api": true}
	outlinerView.ProjectFilter.Tags = map[string]bool{"client": true}
	outlinerView.AdoptFilterCarriers()
	// Due owns its order, so the active sort is default while the user's manual
	// choice survives in treeSort. The lens owns the remembered one.
	outlinerView.Sort = query.NewSortState()
	outlinerView.TreeSort = query.SortState{Field: query.SortByTitle, Ascending: false}

	captured := outlinerView.CaptureLensState()
	if captured.Mode != chrome.ModeDue || captured.Query != "ship it" {
		t.Fatalf("captured mode/query = %s/%q", captured.Mode, captured.Query)
	}
	if captured.Sort != (query.SortState{Field: query.SortByTitle, Ascending: false}) {
		t.Fatalf("captured sort = %+v, want the remembered tree sort", captured.Sort)
	}
	if want := "#api +#client"; captured.Facets.String() != want {
		t.Fatalf("captured facets = %q, want %q", captured.Facets.String(), want)
	}

	before := captured.Facets.String()
	outlinerView.Filter.Tags["late"] = true
	outlinerView.ProjectFilter.Tags["late"] = true
	outlinerView.AdoptFilterCarriers()
	if captured.Facets.String() != before {
		t.Fatalf("a later facet edit reached into the captured definition: %q", captured.Facets.String())
	}
}

func TestApplyLensStateInstallsManualSortAndRemembersItInFixedOrderModes(t *testing.T) {
	saved := query.SortState{Field: query.SortByUpdatedAt, Ascending: false}
	for _, mode := range chrome.OutlinerModeCatalog() {
		t.Run(string(mode.ID), func(t *testing.T) {
			outlinerView := scopeTestOutliner(primaryRowsProject("alpha"))
			outlinerView.Sort = query.SortState{Field: query.SortByTitle, Ascending: true}
			outlinerView.TreeSort = query.SortState{Field: query.SortByTitle, Ascending: true}

			state := outliner.LensState{Mode: mode.ID, Sort: saved}
			if !outlinerView.ApplyLensState(state) {
				t.Fatal("applyLensState refused a catalog mode")
			}
			if outlinerView.PrimaryMode != mode.ID {
				t.Fatalf("mode = %s, want %s", outlinerView.PrimaryMode, mode.ID)
			}
			if outlinerView.TreeSort != saved {
				t.Fatalf("treeSort = %+v, want the saved manual sort %+v", outlinerView.TreeSort, saved)
			}
			wantActive := saved
			if mode.FixedOrder() {
				wantActive = query.NewSortState()
			}
			if outlinerView.Sort != wantActive {
				t.Fatalf("%s active sort = %+v, want %+v", mode.ID, outlinerView.Sort, wantActive)
			}
		})
	}

	outlinerView := scopeTestOutliner(primaryRowsProject("alpha"))
	if outlinerView.ApplyLensState(outliner.LensState{Mode: chrome.OutlinerModeID("archive")}) {
		t.Fatal("applyLensState accepted a mode outside the catalog")
	}
	if outlinerView.PrimaryMode != chrome.ModeAll {
		t.Fatalf("a refused lens changed the mode to %s", outlinerView.PrimaryMode)
	}
}

func TestApplyLensStateDetachesFilterCollectionsFromTheDefinition(t *testing.T) {
	outlinerView := scopeTestOutliner(primaryRowsProject("alpha"))
	state := lensRoundTripBase(t)
	// The scope axis is only readable back when the base workspace admits the
	// project it names: that population is the one scope the Outliner can be asked
	// for, so it is the one a capture reports.
	state.ScopeProjectID = "alpha"
	if !outlinerView.ApplyLensState(state) {
		t.Fatal("applyLensState refused the definition")
	}
	if outlinerView.Scope.ProjectID != "alpha" {
		t.Fatalf("applyLensState did not enter the lens's scope, got %q", outlinerView.Scope.ProjectID)
	}
	if outlinerView.CaptureLensState().Fingerprint() != state.Fingerprint() {
		t.Fatalf("captured %q after applying %q", outlinerView.CaptureLensState().Fingerprint(), state.Fingerprint())
	}

	before := state.Facets.String()
	outlinerView.Filter.Tags["live"] = true
	outlinerView.ProjectFilter.Tags["live"] = true
	outlinerView.AdoptFilterCarriers()
	if state.Facets.String() != before {
		t.Fatalf("a live facet edit mutated the applied definition: %q", state.Facets.String())
	}
}
