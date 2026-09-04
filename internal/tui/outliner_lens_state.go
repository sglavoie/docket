package tui

import (
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/query"
)

// A named lens is a saved Outliner projection: the mode it opens, the narrowing
// it applies, the project scope it may own, and the manual sort it restores.
// This file owns the Outliner-side representation of those axes and is the only
// place the Outliner converts between them and a durable definition. The token
// tables it converts through are internal/tui/lens's, so no other TUI code reads
// a persisted spelling and no config code learns a TUI enum.
//
// The narrowing persists as one string holding both halves of it: the canonical
// clauses models.Query owns, and whatever richer spelling the user-facing
// grammar accepts and that vocabulary does not. models.JoinQueryString and
// models.PartitionQueryString are the only split, so a facet chosen from a
// picker and the same narrowing typed into slash search save as the same bytes.

// lensState is the complete set of lens-owned Outliner axes. Scope is one of
// them, but it is the one axis a lens may decline to own: an empty
// scopeProjectID means the definition leaves the view's scope exactly as it
// found it, which is deliberately not the same as forcing the global
// population. Everything absent here belongs to the view the user is in rather
// than to the lens: folds, focus, zoom, selection, cursor, and viewport are all
// deliberately excluded.
type lensState struct {
	mode chrome.OutlinerModeID
	// scopeProjectID is the single project the lens reopens into, or empty when
	// the lens does not own the scope axis.
	scopeProjectID string
	// facets is the canonical half of the narrowing: the one predicate the
	// Outliner holds, carried verbatim rather than lifted through the filter
	// carriers, which cannot express a clause's polarity or the dependency axis.
	facets models.Query
	// query is the remainder the canonical vocabulary cannot spell, which the
	// textgrammar surfaces own.
	query string
	// sort is the remembered treeSort rather than the active render sort: a
	// fixed-order mode leaves the active sort at its default while the user's
	// manual choice survives in treeSort, and that choice is what the lens owns.
	sort query.SortState
}

// projectFilterState is the population cache the lens's canonical facets
// select. Load routing compares filters rather than queries, so the derivation
// lives here once instead of at each comparison.
func (s lensState) projectFilterState() query.ProjectFilterState {
	return query.ProjectFilterStateFromQuery(s.facets)
}

// captureLensState reads every lens-owned axis off the live Outliner. The
// canonical query is re-canonicalized rather than aliased, which allocates a
// clause slice of its own, so a later facet edit cannot reach back into a
// captured definition.
func (m *OutlinerModel) captureLensState() lensState {
	if m == nil {
		return lensState{}
	}
	return lensState{
		mode:           m.primaryMode,
		scopeProjectID: m.capturedLensScope(),
		facets:         m.filterQuery.Canonical(),
		query:          m.capturedLensQuery(),
		sort:           m.treeSort,
	}
}

// capturedLensScope is the project scope a lens saved here can reopen into. The
// single-project Outliner is a scope no lens installs, and a parked focus is an
// exception to the base workspace rather than a scope the base Outliner can be
// asked for, so both are captured as no scope at all rather than as one every
// later application would have to refuse.
func (m *OutlinerModel) capturedLensScope() string {
	if m == nil || m.project != nil {
		return ""
	}
	if !m.baseWorkspaceAdmitsProject(m.scopedProjectID) {
		return ""
	}
	return m.scopedProjectID
}

// capturedLensQuery is the accepted query the current view is narrowed by,
// whichever facility owns it. Under a lens that is the lens's own narrowing
// query. Outside one an accepted ordinary-search projection owns the visible
// rows, so saving records the query that produced them rather than an empty
// axis; a search input still open has accepted nothing and is not captured.
// Only the query axis crosses over: the projection itself still cannot survive
// underneath a lens, and publishLens drops it in favor of the baseline that
// hands it back on the way out.
func (m *OutlinerModel) capturedLensQuery() string {
	if !m.hasActiveLens() && m.hasSearchProjection() && !m.IsSearchMode() {
		return m.SearchQuery()
	}
	return m.lensQuery
}

// lensTreeOrderPrev is the synthetic mode a lens is applied "out of". A lens
// stores its manual sort the way a tree-order mode holds it, so installing one
// is exactly the reconciliation ApplyOutlinerModeTransition performs when
// leaving a tree-order mode: manual-sort modes take the sort as their active
// order, and fixed-order modes keep their own while remembering it in treeSort.
var lensTreeOrderPrev = chrome.OutlinerMode{OrderPolicy: chrome.OrderPolicyTree}

// applyLensState installs a saved projection, reporting whether it named a
// catalog mode. Only the lens-owned axes are written; folds, focus, zoom,
// selection, and cursor belong to the view the user is in. The scope is written
// through pushProjectScope rather than by assigning the field, so the frame one
// Esc pops to is captured by the same helper every other scope push uses; a lens
// that names no scope leaves the current one alone. The push comes first,
// before a lens-owned axis is written, so the frame it captures holds the view
// the user was in rather than the projection being installed over it.
func (m *OutlinerModel) applyLensState(state lensState) bool {
	mode, ok := chrome.OutlinerModeByID(state.mode)
	if m == nil || !ok {
		return false
	}
	if state.scopeProjectID != "" {
		m.pushProjectScope(state.scopeProjectID)
	}
	m.primaryMode = state.mode
	// The definition already holds the canonical predicate, so it is installed
	// directly and both carrier caches are re-derived from it.
	m.setFilterQuery(state.facets)
	m.setLensQuery(state.query)

	next := ApplyOutlinerModeTransition(
		OutlinerModeSwitch{TreeSort: state.sort, ActiveSort: state.sort},
		lensTreeOrderPrev,
		mode,
	)
	m.sort = next.ActiveSort
	m.treeSort = next.TreeSort
	return true
}

// namedLens renders the state as a durable definition under the given identity.
// Every axis is written with its canonical token rather than the empty
// omitted-default rendering, so one state has exactly one spelling here; the
// decoder still accepts the empty form config documents as that axis's default.
// Comparison therefore belongs to fingerprint, never to two DTOs.
func (s lensState) namedLens(id, name string) config.NamedLens {
	return config.NamedLens{
		ID:   id,
		Name: name,
		Mode: lens.ModeAxis.Token(s.mode),
		// Scope has no token table: it is a project ID, and the empty string is
		// the unowned axis rather than a default the decoder has to name.
		Scope: s.scopeProjectID,
		Query: models.JoinQueryString(s.facets, s.query),
		Sort: config.LensSort{
			Field:     lens.SortAxis.Token(s.sort.Field),
			Ascending: s.sort.Ascending,
		},
	}
}

// lensStateFromNamedLens decodes a durable definition. The whole record is
// validated first, so a hand-edited machine file refuses here rather than
// installing a projection nobody can name. The query is deliberately not parsed:
// it compiles at projection time, where an unparseable one keeps the ordinary
// invalid-query guidance instead of failing to load.
func lensStateFromNamedLens(saved config.NamedLens) (lensState, error) {
	if err := saved.Validate(); err != nil {
		return lensState{}, err
	}
	mode, err := lens.ModeAxis.Value(saved.Mode)
	if err != nil {
		return lensState{}, err
	}
	sortField, err := lens.SortAxis.Value(saved.Sort.Field)
	if err != nil {
		return lensState{}, err
	}
	facets, remainder := models.PartitionQueryString(saved.Query)
	return lensState{
		mode: mode,
		// The scope is carried verbatim: Validate already held it to the canonical
		// UUID spelling, and whether that project can still be entered is a runtime
		// question ApplyNamedLens answers, never a decoding one.
		scopeProjectID: saved.Scope,
		facets:         facets,
		query:          remainder,
		sort:           query.SortState{Field: sortField, Ascending: saved.Sort.Ascending},
	}, nil
}

// fingerprint is the canonical rendering of one state. It renders the durable
// definition and joins its axes, so the comparison is over exactly the bytes a
// save would write: the query string is already canonical on the clause half
// and verbatim on the remainder, which is what keeps map iteration out of it and
// makes an unchanged lens read as unchanged. Every axis namedLens writes must
// appear here — an omitted one is silently unwatched, so an edit to it leaves
// the lens reading as unmodified.
func (s lensState) fingerprint() string {
	saved := s.namedLens("", "")
	return strings.Join([]string{
		saved.Mode,
		saved.Scope,
		saved.Query,
		saved.Sort.Field,
		strconv.FormatBool(saved.Sort.Ascending),
	}, "\x1f")
}
