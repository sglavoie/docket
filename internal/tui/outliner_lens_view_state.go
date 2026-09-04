package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/query"
)

// Applying a named lens is one atomic, reversible act. The frame below is what
// makes it reversible: it holds the identity that was installed, the fingerprint
// the modified marker compares against, and the single restoration
// point Esc returns to. Switching lenses replaces the identity and keeps that
// restoration point, so one Esc always lands on the view the user actually left
// rather than on an intermediate lens they passed through.

// activeLensFrame is the one live lens. There is never a stack of them.
type activeLensFrame struct {
	lensID string
	name   string

	// savedFingerprint is the canonical rendering of the definition as installed.
	// The marker compares fingerprints rather than states so map iteration order
	// can never report an unchanged lens as modified, and the fingerprint alone is
	// what a comparison ever needs: the record itself lives in machine state.
	savedFingerprint string

	// origin is the view the first activation replaced. A switch carries it
	// forward untouched; nothing else may write it.
	origin outlinerViewSnapshot

	// pendingGeneration is the load generation of an application that has not
	// published yet, or zero. It exists so restoration can invalidate work still
	// in flight instead of letting it land on a view that already moved on.
	pendingGeneration int

	// projectFacetsAwaitingLoad marks a lens published without loading the
	// population its project facets select, which is what a scoped application
	// does: the scope had already fixed the population to one project. Project
	// facets are population selectors rather than row predicates, so those facets
	// are recorded and inert, and this is what tells the explicit scope exit that
	// it owes them a load before any unscoped row is shown.
	projectFacetsAwaitingLoad bool

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
	generation int
	lens       config.NamedLens
	state      lensState

	// origin is the restoration point frozen when the load started, not when it
	// landed. An asynchronous application owes Esc the view the user actually
	// left, so navigating, searching, or filtering while the projects read must
	// not rewrite the baseline the application is going to install.
	origin outlinerViewSnapshot

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
	active  *activeLensFrame
	pending *pendingLensApplication

	// The narrowing query and its compiled reading. query is the accepted
	// narrowing of the saved projection the view is showing; remainder is the free
	// text left after the sigil gate was lifted out of it and gate is that gate,
	// so scoring and highlighting never read a predicate token as prose;
	// narrowRelaxed records that the rows on screen were produced by a
	// case-insensitive retry. It is deliberately not the slash-search query: a
	// lens query narrows its own saved mode, facets, and sort, while ordinary
	// search replaces the whole projection and stays scope-wide.
	query         string
	gate          *query.SearchPredicates
	remainder     string
	narrowRelaxed bool

	// persist records which lens the Outliner should reopen with, or clears that
	// pointer when passed the empty string. The shell injects it, so the Outliner
	// never learns where machine-local state is committed; a nil seam is inert and
	// simply remembers nothing.
	persist func(id string) error

	// The input mode. queryEditing points the shared search input at the
	// narrowing query instead of the ordinary search query, and queryEditOrigin is
	// the query Esc restores when that edit is abandoned.
	queryEditing    bool
	queryEditOrigin string
}

// hasActive reports whether a lens owns the current projection.
func (s *outlinerLensState) hasActive() bool { return s != nil && s.active != nil }

// activeIs reports that the live lens is the one named. Every mutation of a
// saved definition asks this before touching the frame, because a rename, a
// replace, or a delete of some other lens must leave the live one alone.
func (s *outlinerLensState) activeIs(id string) bool {
	return s.hasActive() && s.active.lensID == id
}

// activeID names the live lens for the picker's own highlight, or the empty
// string when no lens owns the view.
func (s *outlinerLensState) activeID() string {
	if !s.hasActive() {
		return ""
	}
	return s.active.lensID
}

// setActive installs the frame one publication produced. It is the only write of
// the live lens; every other route here either hands it back or puts it down.
func (s *outlinerLensState) setActive(frame *activeLensFrame) {
	if s == nil {
		return
	}
	s.active = frame
}

// takeActive hands the frame back and puts the lens down in one act, so the
// restoration it carries cannot be replayed by whatever installs next.
func (s *outlinerLensState) takeActive() *activeLensFrame {
	if s == nil {
		return nil
	}
	frame := s.active
	s.active = nil
	return frame
}

// clearActive drops the frame without handing it back, for the resets that are
// replacing the whole projection rather than returning to what it replaced.
func (s *outlinerLensState) clearActive() {
	if s == nil {
		return
	}
	s.active = nil
}

// renameActive carries a renamed definition onto the live frame. The identity
// test is here rather than at the call site so a mutation of some other lens is
// a no-op by construction.
func (s *outlinerLensState) renameActive(id, name string) {
	if !s.activeIs(id) {
		return
	}
	s.active.name = name
}

// rebaseActive re-bases the live frame on a definition that was just overwritten
// with the view, which is what clears the modified marker: the view and the
// record now agree.
func (s *outlinerLensState) rebaseActive(id, fingerprint string) {
	if !s.activeIs(id) {
		return
	}
	s.active.savedFingerprint = fingerprint
}

// hasPending reports that an application is still reading its population.
func (s *outlinerLensState) hasPending() bool { return s != nil && s.pending != nil }

// arm records the application one load belongs to and mirrors its generation
// onto the live frame. The two are one act: the frame's pendingGeneration exists
// only to name the application in flight, so an application armed without it
// would leave a restoration unable to invalidate the work it supersedes.
func (s *outlinerLensState) arm(pending *pendingLensApplication) {
	if s == nil {
		return
	}
	s.pending = pending
	if s.active != nil && pending != nil {
		s.active.pendingGeneration = pending.generation
	}
}

// clearPending puts the application down and, with it, the generation the frame
// was mirroring. arm above is the same act in reverse; nothing may drop one
// without the other, or the frame keeps naming a load that already settled.
func (s *outlinerLensState) clearPending() {
	if s == nil {
		return
	}
	s.pending = nil
	if s.active != nil {
		s.active.pendingGeneration = 0
	}
}

// supersededGeneration reports that the live frame no longer names the
// generation a result came back under, which is a lens dismissed or replaced
// while its load ran. A view with no frame supersedes nothing: the application
// that is landing may be the one about to install the first.
func (s *outlinerLensState) supersededGeneration(generation int) bool {
	return s.hasActive() && s.active.pendingGeneration != generation
}

// awaitedPopulation is the project filter the live population was actually read
// under, and whether the live lens is still owed the load its project facets
// select. The two are returned together because the fingerprint is meaningful
// only while that debt stands: a published population is its own provenance.
func (s *outlinerLensState) awaitedPopulation() (string, bool) {
	if !s.hasActive() || !s.active.projectFacetsAwaitingLoad {
		return "", false
	}
	return s.active.loadedPopulationFingerprint, true
}

// recordPopulation is where a landed population tells the live frame what it is.
// The debt is exactly the disagreement between the filter the read answered to
// and the one the model now holds, so both are written from one comparison
// rather than by two callers that could disagree about it.
func (s *outlinerLensState) recordPopulation(read, live string) {
	if !s.hasActive() {
		return
	}
	s.active.loadedPopulationFingerprint = read
	s.active.projectFacetsAwaitingLoad = read != live
}

// remember commits the resume pointer through the shell's seam, or reports
// success without one. A nil seam is the whole of "this model remembers
// nothing": the caller reports a real failure and never a missing dependency.
func (s *outlinerLensState) remember(id string) error {
	if s == nil || s.persist == nil {
		return nil
	}
	return s.persist(id)
}

// setQuery installs the narrowing query and discards the compilation of the
// previous one, because a reading of a query that is gone would badge and
// highlight rows the live query never matched.
func (s *outlinerLensState) setQuery(narrowing string) {
	if s == nil {
		return
	}
	s.query = strings.TrimSpace(narrowing)
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
	s.gate = gate
	s.narrowRelaxed = relaxed
}

// beginQueryEdit points the shared input at the narrowing query and freezes the
// query an abandoned edit restores, reporting whether the mode was actually
// opened. Capturing the origin is part of arming it: an edit opened without one
// has nothing to hand back.
func (s *outlinerLensState) beginQueryEdit() bool {
	if s == nil || s.queryEditing {
		return false
	}
	s.queryEditing = true
	s.queryEditOrigin = s.query
	return true
}

// endQueryEdit leaves the input mode without deciding the query, reporting
// whether there was an edit to leave. The commit and the cancel above it each
// name that decision themselves.
func (s *outlinerLensState) endQueryEdit() bool {
	if s == nil || !s.queryEditing {
		return false
	}
	s.queryEditing = false
	s.queryEditOrigin = ""
	return true
}
