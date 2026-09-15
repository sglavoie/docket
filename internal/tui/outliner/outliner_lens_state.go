package outliner

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
// Comparison therefore belongs to fingerprint, never to two DTOs.
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

// Applying a named lens is one atomic, reversible act. The frame below is what
// makes it reversible: it holds the identity that was installed, the fingerprint
// the modified marker compares against, and the single restoration
// point Esc returns to. Switching lenses replaces the identity and keeps that
// restoration point, so one Esc always lands on the view the user actually left
// rather than on an intermediate lens they passed through.

// activeLensFrame is the one live lens. There is never a stack of them.
type activeLensFrame struct {
	LensID string
	Name   string

	// savedFingerprint is the canonical rendering of the definition as installed.
	// The marker compares fingerprints rather than states so map iteration order
	// can never report an unchanged lens as modified, and the fingerprint alone is
	// what a comparison ever needs: the record itself lives in machine state.
	SavedFingerprint string

	// origin is the view the first activation replaced. A switch carries it
	// forward untouched; nothing else may write it.
	Origin OutlinerViewSnapshot

	// pendingGeneration is the load generation of an application that has not
	// published yet, or zero. It exists so restoration can invalidate work still
	// in flight instead of letting it land on a view that already moved on.
	PendingGeneration int

	// projectFacetsAwaitingLoad marks a lens published without loading the
	// population its project facets select, which is what a scoped application
	// does: the scope had already fixed the population to one project. Project
	// facets are population selectors rather than row predicates, so those facets
	// are recorded and inert, and this is what tells the explicit scope exit that
	// it owes them a load before any unscoped row is shown.
	ProjectFacetsAwaitingLoad bool

	// loadedPopulationFingerprint names the project filter the live population was
	// actually read under, which is the provenance an owed load has to be decided
	// against. Equality between consecutive lens filters cannot decide it: a
	// scoped application writes its filter without reading a population, so the
	// next equivalent lens compares equal to a filter no population belongs to
	// and would clear a debt nothing has paid. It is meaningful only while
	// projectFacetsAwaitingLoad is set; a published population is its own
	// provenance.
	loadedPopulationFingerprint string
}

// pendingLensApplication is a lens whose project population is still loading.
// Nothing on the live model has changed yet: the candidate filter travels with
// the load, and the lens publishes only when its own generation comes back.
type pendingLensApplication struct {
	Generation int
	lens       config.NamedLens
	State      LensState

	// origin is the restoration point frozen when the load started, not when it
	// landed. An asynchronous application owes Esc the view the user actually
	// left, so navigating, searching, or filtering while the projects read must
	// not rewrite the baseline the application is going to install.
	Origin OutlinerViewSnapshot

	// scopeExitFrom names the project scope an explicit exit is leaving. When it
	// is set this application installs no lens: the one it belongs to is already
	// live, and only its inert project facets are being honored, so the result
	// publishes the population and drops that scope in one step. An empty string
	// is an ordinary application that publishes a lens.
	scopeExitFrom string
}

// outlinerLensState is the whole of the Outliner's named-lens facility: the one
// live frame, the application whose population is still being read, the
// narrowing query that frame installs and the compiled reading of it, the
// machine-local seam that remembers which lens to reopen with, and the input
// mode that retypes the query. Five concerns, one owner, because every one of
// them is the live lens seen from a different side — the pending application is
// the frame before it exists, the query is what the frame narrows with, the
// compilation is that query's reading, the seam is the frame outliving the
// process, and the edit mode is the query being retyped in place.
//
// forgetActiveLensFrame is the evidence: putting the lens down drops the frame,
// the pending application, the edit mode and the query in one act, and it has
// to, or a narrowing outlives the lens that installed it or an input keeps
// retyping a query nothing is reading.
//
// It is deliberately not lensState, which is one saved definition's axes —
// mode, scope, facets, query, sort — and lives in outliner_lens_state.go. That
// type describes a lens; this one describes the Outliner holding one. It is also
// deliberately not searchState: at most one of the two narrowing queries is ever
// live, which is what lets activeNarrowingGate answer for both, and
// compileActiveNarrowing writes each carrier's triple through its own sub-state
// rather than merging them.
//
// The zero value is inert: no frame, no application, no query, no gate, and a
// nil seam that reads as remembering nothing rather than as a missing
// dependency. That is what keeps a struct-literal test model and the Project
// Hub's preview painter working without a constructor.
type outlinerLensState struct {
	// The live lens and the application still reading its population. There is
	// never a stack of either: a switch replaces the definition and keeps the
	// original restoration point, so one Esc always returns to the view the user
	// actually left. Neither survives a restart or a global reset.
	Active  *activeLensFrame
	Pending *pendingLensApplication

	// The narrowing query and its compiled reading. query is the accepted
	// narrowing of the saved projection the view is showing; remainder is the free
	// text left after the sigil gate was lifted out of it and gate is that gate,
	// so scoring and highlighting never read a predicate token as prose;
	// narrowRelaxed records that the rows on screen were produced by a
	// case-insensitive retry. It is deliberately not the slash-search query: a
	// lens query narrows its own saved mode, facets, and sort, while ordinary
	// search replaces the whole projection and stays scope-wide.
	Query         string
	Gate          *query.SearchPredicates
	remainder     string
	NarrowRelaxed bool

	// persist records which lens the Outliner should reopen with, or clears that
	// pointer when passed the empty string. The shell injects it, so the Outliner
	// never learns where machine-local state is committed; a nil seam is inert and
	// simply remembers nothing.
	Persist func(id string) error
}

// hasActive reports whether a lens owns the current projection.
func (s *outlinerLensState) hasActive() bool { return s != nil && s.Active != nil }

// activeIs reports that the live lens is the one named. Every mutation of a
// saved definition asks this before touching the frame, because a rename, a
// replace, or a delete of some other lens must leave the live one alone.
func (s *outlinerLensState) ActiveIs(id string) bool {
	return s.hasActive() && s.Active.LensID == id
}

// activeID names the live lens for the picker's own highlight, or the empty
// string when no lens owns the view.
func (s *outlinerLensState) ActiveID() string {
	if !s.hasActive() {
		return ""
	}
	return s.Active.LensID
}

// setActive installs the frame one publication produced. It is the only write of
// the live lens; every other route here either hands it back or puts it down.
func (s *outlinerLensState) setActive(frame *activeLensFrame) {
	if s == nil {
		return
	}
	s.Active = frame
}

// takeActive hands the frame back and puts the lens down in one act, so the
// restoration it carries cannot be replayed by whatever installs next.
func (s *outlinerLensState) takeActive() *activeLensFrame {
	if s == nil {
		return nil
	}
	frame := s.Active
	s.Active = nil
	return frame
}

// clearActive drops the frame without handing it back, for the resets that are
// replacing the whole projection rather than returning to what it replaced.
func (s *outlinerLensState) clearActive() {
	if s == nil {
		return
	}
	s.Active = nil
}

// renameActive carries a renamed definition onto the live frame. The identity
// test is here rather than at the call site so a mutation of some other lens is
// a no-op by construction.
func (s *outlinerLensState) RenameActive(id, name string) {
	if !s.ActiveIs(id) {
		return
	}
	s.Active.Name = name
}

// rebaseActive re-bases the live frame on a definition that was just overwritten
// with the view, which is what clears the modified marker: the view and the
// record now agree.
func (s *outlinerLensState) RebaseActive(id, fingerprint string) {
	if !s.ActiveIs(id) {
		return
	}
	s.Active.SavedFingerprint = fingerprint
}

// hasPending reports that an application is still reading its population.
func (s *outlinerLensState) hasPending() bool { return s != nil && s.Pending != nil }

// arm records the application one load belongs to and mirrors its generation
// onto the live frame. The two are one act: the frame's pendingGeneration exists
// only to name the application in flight, so an application armed without it
// would leave a restoration unable to invalidate the work it supersedes.
func (s *outlinerLensState) arm(pending *pendingLensApplication) {
	if s == nil {
		return
	}
	s.Pending = pending
	if s.Active != nil && pending != nil {
		s.Active.PendingGeneration = pending.Generation
	}
}

// clearPending puts the application down and, with it, the generation the frame
// was mirroring. arm above is the same act in reverse; nothing may drop one
// without the other, or the frame keeps naming a load that already settled.
func (s *outlinerLensState) clearPending() {
	if s == nil {
		return
	}
	s.Pending = nil
	if s.Active != nil {
		s.Active.PendingGeneration = 0
	}
}

// supersededGeneration reports that the live frame no longer names the
// generation a result came back under, which is a lens dismissed or replaced
// while its load ran. A view with no frame supersedes nothing: the application
// that is landing may be the one about to install the first.
func (s *outlinerLensState) supersededGeneration(generation int) bool {
	return s.hasActive() && s.Active.PendingGeneration != generation
}

// awaitedPopulation is the project filter the live population was actually read
// under, and whether the live lens is still owed the load its project facets
// select. The two are returned together because the fingerprint is meaningful
// only while that debt stands: a published population is its own provenance.
func (s *outlinerLensState) awaitedPopulation() (string, bool) {
	if !s.hasActive() || !s.Active.ProjectFacetsAwaitingLoad {
		return "", false
	}
	return s.Active.loadedPopulationFingerprint, true
}

// recordPopulation is where a landed population tells the live frame what it is.
// The debt is exactly the disagreement between the filter the read answered to
// and the one the model now holds, so both are written from one comparison
// rather than by two callers that could disagree about it.
func (s *outlinerLensState) recordPopulation(read, live string) {
	if !s.hasActive() {
		return
	}
	s.Active.loadedPopulationFingerprint = read
	s.Active.ProjectFacetsAwaitingLoad = read != live
}

// remember commits the resume pointer through the shell's seam, or reports
// success without one. A nil seam is the whole of "this model remembers
// nothing": the caller reports a real failure and never a missing dependency.
func (s *outlinerLensState) remember(id string) error {
	if s == nil || s.Persist == nil {
		return nil
	}
	return s.Persist(id)
}

// setQuery installs the narrowing query and discards the compilation of the
// previous one, because a reading of a query that is gone would badge and
// highlight rows the live query never matched.
func (s *outlinerLensState) setQuery(narrowing string) {
	if s == nil {
		return
	}
	s.Query = strings.TrimSpace(narrowing)
	s.setCompilation("", nil, false)
}

// setCompilation installs the reading of the query the visible rows were
// produced under. The three are one act for the same reason search's are: a
// remainder without the gate it was lifted from would score the sigils as prose,
// and a case reading without the pass that took it would highlight rows that
// pass never matched.
func (s *outlinerLensState) setCompilation(remainder string, gate *query.SearchPredicates, relaxed bool) {
	if s == nil {
		return
	}
	s.remainder = remainder
	s.Gate = gate
	s.NarrowRelaxed = relaxed
}
