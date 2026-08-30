package tui

import (
	"slices"
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

func lensRoundTripBase(t *testing.T) lensState {
	t.Helper()
	states, err := models.NewWorkflowStateSet(models.StateWaiting, models.StateDoing)
	if err != nil {
		t.Fatalf("NewWorkflowStateSet: %v", err)
	}
	base := lensState{
		mode:           ModeAll,
		scopeProjectID: lensRoundTripScopeID,
		query:          "#api ship",
		filter:         NewFilterState(),
		projectFilter:  newProjectFilterState(),
		sort:           NewSortState(),
	}
	base.filter.setStateFilter(states, true)
	base.filter.Priority = models.PriorityHigh
	base.filter.Tags = map[string]bool{"zeta": true, "api": true}
	base.filter.Contexts = map[string]bool{"errand": true, "deep": true}
	base.filter.setCompletionAgeFilter(CompletionAge30Days)
	base.filter.Stale = true
	base.filter.Deferred = true
	base.filter.Pinned = true
	base.projectFilter.Tags = map[string]bool{"client": true}
	base.projectFilter.Untagged = true
	base.projectFilter.ParkedMode = projectPopulationInclude
	base.projectFilter.ArchivedMode = projectPopulationOnly
	base.sort = SortState{Field: SortByPriority, Ascending: false}
	return base
}

func TestLensStateRoundTripsEveryModeFacetPopulationAndSortAxis(t *testing.T) {
	base := lensRoundTripBase(t)

	variants := map[string]func(*lensState){}
	for _, mode := range OutlinerModeCatalog() {
		variants["mode:"+string(mode.ID)] = func(s *lensState) { s.mode = mode.ID }
	}
	for _, due := range []DueDateFilter{DueDateAll, DueDateOverdue, DueDateToday, DueDateThisWeek, DueDateNone} {
		variants["due:"+lensDueAxis.token(due)] = func(s *lensState) { s.filter.DueDate = due }
	}
	for _, hierarchy := range []HierarchyFilter{
		HierarchyAll, HierarchyParent, HierarchyChild, HierarchyRoot, HierarchyLeaf,
	} {
		variants["hierarchy:"+lensHierarchyAxis.token(hierarchy)] = func(s *lensState) { s.filter.Hierarchy = hierarchy }
	}
	for _, age := range []CompletionAgeFilter{
		CompletionAgeAll, CompletionAge7Days, CompletionAge30Days, CompletionAge90Days,
	} {
		variants["age:"+lensCompletionAgeAxis.token(age)] = func(s *lensState) { s.filter.setCompletionAgeFilter(age) }
	}
	for _, population := range []projectPopulationMode{
		projectPopulationExclude, projectPopulationInclude, projectPopulationOnly,
	} {
		variants["parked:"+lensPopulationAxis.token(population)] = func(s *lensState) {
			s.projectFilter.ParkedMode = population
		}
		variants["archived:"+lensPopulationAxis.token(population)] = func(s *lensState) {
			s.projectFilter.ArchivedMode = population
		}
	}
	for _, field := range []SortField{
		SortByDefault, SortByPriority, SortByDueDate, SortByCreatedAt, SortByStatus, SortByUpdatedAt, SortByTitle,
	} {
		for _, ascending := range []bool{true, false} {
			name := "sort:" + lensSortAxis.token(field)
			if !ascending {
				name += ":desc"
			}
			variants[name] = func(s *lensState) { s.sort = SortState{Field: field, Ascending: ascending} }
		}
	}
	// The two activation flags are axes of their own: an absent predicate and an
	// active one that admits everything must not collapse into the same record.
	variants["states:absent"] = func(s *lensState) { s.filter.setStateFilter(models.WorkflowStateSet{}, false) }
	variants["states:empty-active"] = func(s *lensState) { s.filter.setStateFilter(models.WorkflowStateSet{}, true) }
	variants["age:inactive"] = func(s *lensState) {
		s.filter.CompletionAge = CompletionAge90Days
		s.filter.completionAgeSet = false
	}
	// The context facet needs no activation flag: its two halves are a
	// vocabulary and the gap beside it, and each is independently absent or
	// present, so all four combinations must round trip distinctly.
	variants["contexts:absent"] = func(s *lensState) {
		s.filter.Contexts = map[string]bool{}
		s.filter.NoContext = false
	}
	variants["contexts:gap-only"] = func(s *lensState) {
		s.filter.Contexts = map[string]bool{}
		s.filter.NoContext = true
	}
	variants["contexts:names-and-gap"] = func(s *lensState) { s.filter.NoContext = true }
	// The unowned scope is a distinct record from a named one, and the empty
	// string is its only spelling, so it round trips as absence rather than as a
	// default token some axis table supplies.
	variants["scope:unowned"] = func(s *lensState) { s.scopeProjectID = "" }

	for name, mutate := range variants {
		t.Run(name, func(t *testing.T) {
			want := base
			want.filter = copyFilterState(base.filter)
			want.projectFilter = base.projectFilter.clone()
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
	want := lensState{
		mode:          ModeInbox,
		filter:        NewFilterState(),
		projectFilter: newProjectFilterState(),
		sort:          SortState{Field: SortByDefault},
	}
	if !got.equal(want) {
		t.Fatalf("omitted tokens decoded to %q, want axis defaults %q", got.fingerprint(), want.fingerprint())
	}

	for name, mutate := range map[string]func(*config.NamedLens){
		"mode":     func(l *config.NamedLens) { l.Mode = "" },
		"due":      func(l *config.NamedLens) { l.Tasks.DueDate = "someday" },
		"age":      func(l *config.NamedLens) { l.Tasks.CompletionAge = "14d" },
		"context":  func(l *config.NamedLens) { l.Tasks.Contexts = []string{"Not A Context"} },
		"sort":     func(l *config.NamedLens) { l.Sort.Field = "manual" },
		"archived": func(l *config.NamedLens) { l.Projects.ArchivedMode = "always" },
	} {
		t.Run("malformed/"+name, func(t *testing.T) {
			broken := stored
			mutate(&broken)
			if _, err := lensStateFromNamedLens(broken); err == nil {
				t.Fatalf("malformed %s token was accepted", name)
			}
		})
	}
}

func TestLensStateFingerprintIsDeterministicAcrossMapIterationOrder(t *testing.T) {
	first := lensRoundTripBase(t)
	second := lensRoundTripBase(t)
	second.filter.Tags = map[string]bool{}
	for _, tag := range []string{"zeta", "api"} {
		second.filter.Tags[tag] = true
	}
	// An unselected key is not part of the projection and must not reach the
	// fingerprint through the map it happens to sit in.
	second.filter.Tags["dropped"] = false

	states, err := models.NewWorkflowStateSet(models.StateDoing, models.StateWaiting)
	if err != nil {
		t.Fatalf("NewWorkflowStateSet: %v", err)
	}
	second.filter.setStateFilter(states, true)

	for range 20 {
		if first.fingerprint() != second.fingerprint() {
			t.Fatalf("fingerprints differ across iteration order:\n%q\n%q",
				first.fingerprint(), second.fingerprint())
		}
	}
	if !first.equal(second) {
		t.Fatal("equal disagreed with identical fingerprints")
	}

	modified := first
	modified.filter = copyFilterState(first.filter)
	modified.filter.Tags["extra"] = true
	if modified.equal(first) {
		t.Fatal("an added tag facet did not change the fingerprint")
	}

	stored := first.namedLens(lensRoundTripID, "Sorted")
	if !slices.Equal(stored.Tasks.Tags, []string{"api", "zeta"}) {
		t.Fatalf("stored task tags = %v, want sorted", stored.Tasks.Tags)
	}
	if !slices.Equal(stored.Tasks.States, []string{string(models.StateDoing), string(models.StateWaiting)}) {
		t.Fatalf("stored states = %v, want canonical lifecycle order", stored.Tasks.States)
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
		"states":        func(s *lensState) { s.filter.setStateFilter(models.WorkflowStateSet{}, true) },
		"states set":    func(s *lensState) { s.filter.setStateFilter(models.WorkflowStateSet{}, false) },
		"priority":      func(s *lensState) { s.filter.Priority = models.PriorityLow },
		"tags":          func(s *lensState) { s.filter.Tags["extra"] = true },
		"contexts":      func(s *lensState) { s.filter.Contexts["extra"] = true },
		"no context":    func(s *lensState) { s.filter.NoContext = !s.filter.NoContext },
		"due":           func(s *lensState) { s.filter.DueDate = DueDateOverdue },
		"age":           func(s *lensState) { s.filter.setCompletionAgeFilter(CompletionAge7Days) },
		"age set":       func(s *lensState) { s.filter.completionAgeSet = !s.filter.completionAgeSet },
		"hierarchy":     func(s *lensState) { s.filter.Hierarchy = HierarchyLeaf },
		"stale":         func(s *lensState) { s.filter.Stale = !s.filter.Stale },
		"deferred":      func(s *lensState) { s.filter.Deferred = !s.filter.Deferred },
		"pinned":        func(s *lensState) { s.filter.Pinned = !s.filter.Pinned },
		"project tags":  func(s *lensState) { s.projectFilter.Tags["extra"] = true },
		"untagged":      func(s *lensState) { s.projectFilter.Untagged = !s.projectFilter.Untagged },
		"parked":        func(s *lensState) { s.projectFilter.ParkedMode = projectPopulationExclude },
		"archived":      func(s *lensState) { s.projectFilter.ArchivedMode = projectPopulationInclude },
		"sort field":    func(s *lensState) { s.sort.Field = SortByTitle },
		"sort ordering": func(s *lensState) { s.sort.Ascending = !s.sort.Ascending },
	}

	base := lensRoundTripBase(t)
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			edited := base
			edited.filter = copyFilterState(base.filter)
			edited.projectFilter = base.projectFilter.clone()
			mutate(&edited)
			if edited.fingerprint() == base.fingerprint() {
				t.Fatalf("editing %s left the lens reading as unmodified: %q", name, base.fingerprint())
			}
		})
	}
}

// The context facet's names are emitted sorted, beside the tags they are not,
// so map iteration order can never make an unchanged context lens read as
// modified and a context can never be mistaken for a tag in the record.
func TestLensStateStoresContextsSortedAndApartFromTags(t *testing.T) {
	state := lensRoundTripBase(t)
	state.filter.NoContext = true
	stored := state.namedLens(lensRoundTripID, "Contexts")

	if !slices.Equal(stored.Tasks.Contexts, []string{"deep", "errand"}) {
		t.Fatalf("stored contexts = %v, want sorted", stored.Tasks.Contexts)
	}
	if !slices.Equal(stored.Tasks.Tags, []string{"api", "zeta"}) {
		t.Fatalf("the context facet leaked into the tag facet: %v", stored.Tasks.Tags)
	}
	if !stored.Tasks.NoContext {
		t.Fatal("the no-context half was not persisted")
	}

	got, err := lensStateFromNamedLens(stored)
	if err != nil {
		t.Fatalf("lensStateFromNamedLens: %v", err)
	}
	if !got.filter.Contexts["deep"] || !got.filter.Contexts["errand"] || !got.filter.NoContext {
		t.Fatalf("context facet did not restore: %+v noContext=%v", got.filter.Contexts, got.filter.NoContext)
	}
	if got.filter.Contexts["api"] {
		t.Fatal("a tag was restored as a context")
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
	if !waiting.filter.hasStateFilter() || !waiting.filter.States.Contains(models.StateWaiting) {
		t.Fatalf("Waiting starter lost its state facet: %+v", waiting.filter.States.Values())
	}
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

	outliner.filter.Tags["late"] = true
	outliner.projectFilter.Tags["late"] = true
	if captured.filter.Tags["late"] || captured.projectFilter.Tags["late"] {
		t.Fatal("a later facet edit reached into the captured definition")
	}
}

func TestApplyLensStateInstallsManualSortAndRemembersItInFixedOrderModes(t *testing.T) {
	saved := SortState{Field: SortByUpdatedAt, Ascending: false}
	for _, mode := range OutlinerModeCatalog() {
		t.Run(string(mode.ID), func(t *testing.T) {
			outliner := scopeTestOutliner(primaryRowsProject("alpha"))
			outliner.sort = SortState{Field: SortByTitle, Ascending: true}
			outliner.treeSort = SortState{Field: SortByTitle, Ascending: true}

			state := lensState{
				mode:          mode.ID,
				filter:        NewFilterState(),
				projectFilter: newProjectFilterState(),
				sort:          saved,
			}
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

	outliner.filter.Tags["live"] = true
	outliner.projectFilter.Tags["live"] = true
	if state.filter.Tags["live"] || state.projectFilter.Tags["live"] {
		t.Fatal("a live facet edit mutated the applied definition")
	}
}
