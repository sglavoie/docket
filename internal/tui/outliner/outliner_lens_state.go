package outliner

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/lens"
	"github.com/sglavoie/docket/internal/tui/query"
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
type LensState struct {
	Mode chrome.OutlinerModeID
	// scopeProjectID is the single project the lens reopens into, or empty when
	// the lens does not own the scope axis.
	ScopeProjectID string
	// facets is the canonical half of the narrowing: the one predicate the
	// Outliner holds, carried verbatim rather than lifted through the filter
	// carriers, which cannot express a clause's polarity or the dependency axis.
	Facets models.Query
	// query is the remainder the canonical vocabulary cannot spell, which the
	// textgrammar surfaces own.
	Query string
	// sort is the remembered treeSort rather than the active render sort: a
	// fixed-order mode leaves the active sort at its default while the user's
	// manual choice survives in treeSort, and that choice is what the lens owns.
	Sort query.SortState
}

// projectFilterState is the population cache the lens's canonical facets
// select. Load routing compares filters rather than queries, so the derivation
// lives here once instead of at each comparison.
func (s LensState) ProjectFilterState() query.ProjectFilterState {
	return query.ProjectFilterStateFromQuery(s.Facets)
}

// captureLensState reads every lens-owned axis off the live Outliner. The
// canonical query is re-canonicalized rather than aliased, which allocates a
// clause slice of its own, so a later facet edit cannot reach back into a
// captured definition.
func (m *OutlinerModel) CaptureLensState() LensState {
	if m == nil {
		return LensState{}
	}
	return LensState{
		Mode:           m.PrimaryMode,
		ScopeProjectID: m.capturedLensScope(),
		Facets:         m.FilterQuery.Canonical(),
		Query:          m.capturedLensQuery(),
		Sort:           m.TreeSort,
	}
}

// capturedLensScope is the project scope a lens saved here can reopen into. The
// single-project Outliner is a scope no lens installs, and a parked focus is an
// exception to the base workspace rather than a scope the base Outliner can be
// asked for, so both are captured as no scope at all rather than as one every
// later application would have to refuse.
func (m *OutlinerModel) capturedLensScope() string {
	if m == nil || m.Project != nil {
		return ""
	}
	if !m.baseWorkspaceAdmitsProject(m.Scope.ProjectID) {
		return ""
	}
	return m.Scope.ProjectID
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
	if !m.HasActiveLens() && m.HasSearchProjection() && !m.IsSearchMode() {
		return m.SearchQuery()
	}
	return m.Lens.Query
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
func (m *OutlinerModel) ApplyLensState(state LensState) bool {
	mode, ok := chrome.OutlinerModeByID(state.Mode)
	if m == nil || !ok {
		return false
	}
	if state.ScopeProjectID != "" {
		m.PushProjectScope(state.ScopeProjectID)
	}
	m.PrimaryMode = state.Mode
	// The definition already holds the canonical predicate, so it is installed
	// directly and both carrier caches are re-derived from it.
	m.SetFilterQuery(state.Facets)
	m.SetLensQuery(state.Query)

	next := ApplyOutlinerModeTransition(
		OutlinerModeSwitch{TreeSort: state.Sort, ActiveSort: state.Sort},
		lensTreeOrderPrev,
		mode,
	)
	m.Sort = next.ActiveSort
	m.TreeSort = next.TreeSort
	return true
}

// namedLens renders the state as a durable definition under the given identity.
// Every axis is written with its canonical token rather than the empty
// omitted-default rendering, so one state has exactly one spelling here; the
// decoder still accepts the empty form config documents as that axis's default.
// Comparison therefore belongs to canonical state, never to unnormalized DTOs.
func (s LensState) NamedLens(id, name string) config.NamedLens {
	return config.NamedLens{
		ID:   id,
		Name: name,
		Mode: lens.ModeAxis.Token(s.Mode),
		// Scope has no token table: it is a project ID, and the empty string is
		// the unowned axis rather than a default the decoder has to name.
		Scope: s.ScopeProjectID,
		Query: models.JoinQueryString(s.Facets, s.Query),
		Sort: config.LensSort{
			Field:     lens.SortAxis.Token(s.Sort.Field),
			Ascending: s.Sort.Ascending,
		},
	}
}

// lensStateFromNamedLens decodes a durable definition. The whole record is
// validated first, so a hand-edited machine file refuses here rather than
// installing a projection nobody can name. The query is deliberately not parsed:
// it compiles at projection time, where an unparseable one keeps the ordinary
// invalid-query guidance instead of failing to load.
func LensStateFromNamedLens(saved config.NamedLens) (LensState, error) {
	if err := saved.Validate(); err != nil {
		return LensState{}, err
	}
	mode, err := lens.ModeAxis.Value(saved.Mode)
	if err != nil {
		return LensState{}, err
	}
	sortField, err := lens.SortAxis.Value(saved.Sort.Field)
	if err != nil {
		return LensState{}, err
	}
	facets, remainder := models.PartitionQueryString(saved.Query)
	return LensState{
		Mode: mode,
		// The scope is carried verbatim: Validate already held it to the canonical
		// UUID spelling, and whether that project can still be entered is a runtime
		// question ApplyNamedLens answers, never a decoding one.
		ScopeProjectID: saved.Scope,
		Facets:         facets,
		Query:          remainder,
		Sort:           query.SortState{Field: sortField, Ascending: saved.Sort.Ascending},
	}, nil
}

// fingerprint is the canonical rendering of one state. It renders the durable
// definition and joins its axes, so the comparison is over exactly the bytes a
// save would write: the query string is already canonical on the clause half
// and verbatim on the remainder, which is what keeps map iteration out of it and
// makes an unchanged lens read as unchanged. Every axis namedLens writes must
// appear here — an omitted one is silently unwatched, so an edit to it leaves
// the lens reading as unmodified.
func (s LensState) Fingerprint() string {
	saved := s.NamedLens("", "")
	return strings.Join([]string{
		saved.Mode,
		saved.Scope,
		saved.Query,
		saved.Sort.Field,
		strconv.FormatBool(saved.Sort.Ascending),
	}, "\x1f")
}

// ChangedAxes names the saved axes that differ, in reading order. Compare the
// canonical saved query as a whole so equivalent facets and typed tokens agree.
func (s LensState) ChangedAxes(saved LensState) []string {
	live, baseline := s.NamedLens("", ""), saved.NamedLens("", "")
	var changed []string
	if live.Mode != baseline.Mode {
		changed = append(changed, "mode")
	}
	if live.Query != baseline.Query {
		changed = append(changed, "query / filters")
	}
	if live.Sort != baseline.Sort {
		changed = append(changed, "sort")
	}
	if live.Scope != baseline.Scope {
		changed = append(changed, "project scope")
	}
	return changed
}

// LensReplacementChanges describes the saved and replacement values of changed axes.
func (view *OutlinerModel) LensReplacementChanges(saved, current LensState) []string {
	before, after := saved.NamedLens("", ""), current.NamedLens("", "")
	var lines []string
	add := func(label, old, next string) {
		lines = append(lines, fmt.Sprintf("%s: %s → %s", label, old, next))
	}
	if before.Mode != after.Mode {
		old, _ := chrome.OutlinerModeByID(saved.Mode)
		next, _ := chrome.OutlinerModeByID(current.Mode)
		add("Mode", old.Label, next.Label)
	}
	if before.Query != after.Query {
		describe := func(value string) string {
			if value == "" {
				return "none"
			}
			return value
		}
		add("Query / filters", describe(before.Query), describe(after.Query))
	}
	if before.Sort != after.Sort {
		describe := func(value query.SortState) string {
			direction := "↓"
			if value.Ascending {
				direction = "↑"
			}
			return value.Field.String() + " " + direction
		}
		add("Sort", describe(saved.Sort), describe(current.Sort))
	}
	if before.Scope != after.Scope {
		describe := func(id string) string {
			if id == "" {
				return "keep current scope"
			}
			if view.Runtime != nil {
				if project := view.Runtime.GetProject(id); project != nil {
					return project.Slug
				}
			}
			return id + " (unavailable)"
		}
		add("Project", describe(before.Scope), describe(after.Scope))
	}
	return lines
}
