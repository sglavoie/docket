package outliner

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// Applying a named lens is one atomic, reversible act, and the routes below are
// the whole of that act: applying, publishing, dismissing, and retyping the
// narrowing query. The state they write lives in outlinerLensState, whose frame
// is what makes the application reversible.

// lensProjectsLoadedMsg carries the result of a load started for one lens
// application. It is deliberately distinct from crossProjectsLoadedMsg: an
// ordinary reload publishes its projects unconditionally, while this one
// publishes a whole lens or nothing at all.
type LensProjectsLoadedMsg struct {
	Projects    []*models.Project
	FailedLoads []ProjectLoadFailure
	Generation  int
}

// hasActiveLens reports whether a lens owns the current projection.
func (m *OutlinerModel) HasActiveLens() bool {
	return m != nil && m.Lens.hasActive()
}

// ApplyNamedLens installs a saved projection, or refuses without touching the
// current view. Every reason to refuse — a malformed definition, an unknown
// mode, a query the grammar cannot honor — is decided before a single axis is
// written, so a refusal is indistinguishable from never having asked.
func (m *OutlinerModel) ApplyNamedLens(lens config.NamedLens) (tea.Cmd, error) {
	if m == nil {
		return nil, errors.New("no outliner")
	}
	state, err := LensStateFromNamedLens(lens)
	if err != nil {
		return nil, err
	}
	if _, ok := chrome.OutlinerModeByID(state.Mode); !ok {
		return nil, fmt.Errorf("lens %q names an unknown mode", lens.Name)
	}
	if err := m.validateLensQuery(state.Query); err != nil {
		return nil, err
	}
	if err := m.lensScopeRefusal(lens.Name, state.ScopeProjectID); err != nil {
		return nil, err
	}

	// The baseline is the view this application replaces, so it is taken before
	// the scope push below moves it: one Esc owes the user the scope they were in,
	// not the one the lens just entered.
	origin := m.LensRestorationPoint()
	// The scope goes in before the load decision, never after it. Under a
	// single-project scope lensProjectLoadRequired is false, so a scoped lens
	// reads no population at all and its project facets are recorded inert for the
	// explicit scope exit to honor. Pushing afterwards would start a load the
	// scope immediately invalidates.
	if state.ScopeProjectID != "" {
		m.PushProjectScope(state.ScopeProjectID)
	}
	if m.lensProjectLoadRequired(state) {
		return m.beginLensProjectLoad(lens, state, origin), nil
	}
	m.PublishLens(lens, state, nil, origin)
	return nil, nil
}

// lensScopeRefusal decides whether the lens's scope can be entered at all,
// before a single axis is written. TaskWorkspaceProjects() is the one population
// the base Outliner scopes into and it is already non-archived and unparked, so
// that single membership test answers all three refusals; the catalog is read
// only to word the message and never to widen admission. Parked therefore fails
// closed in v1 — scopeState.parkedProjectID and settleParkedFocusException are the
// seam a later revision would reopen it through.
func (m *OutlinerModel) lensScopeRefusal(name, projectID string) error {
	if projectID == "" || m.baseWorkspaceAdmitsProject(projectID) {
		return nil
	}
	if m.Runtime != nil {
		if project := m.Runtime.GetProject(projectID); project != nil {
			switch {
			case project.Archived:
				return fmt.Errorf("lens %q opens in %q, which is archived", name, project.Name)
			case models.ProjectTagsParked(project.Tags):
				return fmt.Errorf("lens %q opens in %q, which is parked", name, project.Name)
			}
		}
	}
	return fmt.Errorf("lens %q opens in a project that no longer exists", name)
}

// validateLensQuery compiles the narrowing query the way the projection will,
// but throws the compilation away. Only a query the grammar recognizes and
// cannot honor refuses; an unrecognized one is ordinary prose, exactly as it is
// in slash search.
func (m *OutlinerModel) validateLensQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	_, predicates := m.compileSearchQuery(query, m.WorkModeScopeProjects())
	if guidance := predicates.GuidanceText(); guidance != "" {
		return errors.New(guidance)
	}
	return nil
}

func (m *OutlinerModel) PublishLens(lens config.NamedLens, state LensState, projects []*models.Project, origin OutlinerViewSnapshot) {
	// A lens that publishes without I/O supersedes one that is still loading. The
	// pending application can never publish behind it, so it is settled here; the
	// asynchronous path already cleared itself before calling in.
	m.discardPendingLensApplication()
	anchor := m.captureModeCursor()

	// Publishing without projects leaves the population exactly as it was, so
	// project facets that select a different one are recorded and inert. That is
	// what a scoped application does, and the frame below remembers it so the
	// explicit scope exit can honor them. The comparison is against the filter
	// the live population was actually read under, never against one a previous
	// scoped application merely wrote: an equivalent lens replacing an owed one
	// leaves that debt exactly where it was.
	candidate := state.ProjectFilterState()
	population := m.loadedPopulationFingerprint()
	facetsAwaitingLoad := projects == nil &&
		query.ProjectFilterFingerprint(&candidate) != population
	if !facetsAwaitingLoad {
		population = query.ProjectFilterFingerprint(&candidate)
	}

	// A lens replaces the projection, so an ordinary search projection cannot
	// survive underneath it. The baseline above already holds search's own
	// restoration point, which is what one Esc later hands back — and a lens that
	// search had suspended travels inside that same baseline, so applying a lens
	// from a temporary search leaves two Escs between here and the lens the slash
	// was pressed under.
	m.clearSearchProjectionWithoutRestore()

	if projects != nil {
		m.setLoadedProjects(projects)
		m.Population.setSnapshotReady(true)
	}
	// Focus, folds, selection, and Peek are the view the user is in, not the lens:
	// applyLensState writes only the lens-owned axes, scope among them.
	m.ApplyLensState(state)
	if projects != nil {
		m.pruneSelectionsOutsideLoadedProjects()
	}

	// The baseline the modified marker compares against takes the scope the view
	// is actually in rather than the one the record named. They are the same
	// whenever the lens owns the axis; when it does not, an unowned axis cannot be
	// diverged from, so the live scope is what a later push or pop is measured
	// against — and that push or pop is exactly what should mark the lens
	// modified.
	saved := state
	saved.ScopeProjectID = m.capturedLensScope()

	m.Lens.setActive(&activeLensFrame{
		LensID:                      lens.ID,
		Name:                        lens.Name,
		SavedFingerprint:            saved.Fingerprint(),
		Origin:                      origin,
		ProjectFacetsAwaitingLoad:   facetsAwaitingLoad,
		loadedPopulationFingerprint: population,
	})
	m.Population.Invalidate()
	m.RebuildRows()
	m.restoreSearchCursor(anchor)
	// This is the one place a lens becomes live, so it is the only honest place to
	// record which one to reopen with. Persisting from the application instead
	// would let a superseded asynchronous attempt name a lens the user never saw.
	m.noteActiveLens(lens.ID)
}

// noteActiveLens records the resume hint for the lens that just became live, or
// clears it when passed the empty string. The pointer is machine-local and is a
// hint rather than a record: the projection the user asked for is already
// installed, so a failed commit is reported and nothing else. The seam is
// injected by the shell and a model without one simply remembers nothing.
func (m *OutlinerModel) noteActiveLens(id string) {
	if m == nil {
		return
	}
	if err := m.Lens.remember(id); err != nil {
		m.SetTransientStatus("Could not remember the active lens: " + err.Error())
	}
}

// setPersistActiveLens injects the machine-local write the shell owns. It is one
// of the four seams an Outliner cannot build for itself; see
// wireOutlinerOperationExecutor.
func (m *OutlinerModel) SetPersistActiveLens(persist func(id string) error) {
	if m == nil {
		return
	}
	m.Lens.Persist = persist
}

// lensRestorationPoint returns the baseline the next Esc restores: the original
// one when a lens is already live, a fresh capture otherwise. This is the whole
// of the non-stacking rule.
func (m *OutlinerModel) LensRestorationPoint() OutlinerViewSnapshot {
	if m.Lens.hasActive() {
		return m.Lens.Active.Origin
	}
	return m.CaptureOutlinerViewSnapshot()
}

// restoreLensBaseline dismisses the lens and returns to the captured view. Any
// application still in flight is invalidated first, so a load that lands after
// this cannot reinstall a lens the user just left.
func (m *OutlinerModel) RestoreLensBaseline() {
	if m == nil || !m.Lens.hasActive() {
		return
	}
	m.closeMenuBar()
	frame := m.Lens.takeActive()
	m.discardPendingLensApplication()
	m.restoreOutlinerViewSnapshot(frame.Origin)
	m.noteActiveLens("")
}

// discardPendingLensApplication abandons an unpublished application. The
// generation bump is what rejects the result already on its way back; it is
// taken only when something is actually pending, so an ordinary reload in flight
// beside it is left alone.
func (m *OutlinerModel) discardPendingLensApplication() {
	if m == nil || !m.Lens.hasPending() {
		return
	}
	m.Population.supersede()
	m.settlePendingLensApplication()
}

// settlePendingLensApplication clears the state one application armed without
// bumping the generation. It is what a result that has already come back needs:
// there is nothing left in flight to invalidate, only loading state to put down.
func (m *OutlinerModel) settlePendingLensApplication() {
	if m == nil || !m.Lens.hasPending() {
		return
	}
	m.Lens.clearPending()
	m.Population.settleLoad()
}

// clearActiveLensWithoutRestore drops the frame without returning to its
// baseline. It belongs to the resets that are replacing the whole projection
// anyway, never to a dismissal the user asked for.
func (m *OutlinerModel) ClearActiveLensWithoutRestore() {
	if m == nil {
		return
	}
	m.forgetActiveLensFrame()
	m.noteActiveLens("")
}

// forgetActiveLensFrame drops the live frame and the narrowing it installed
// while leaving the machine-local resume pointer exactly as it was. It is the
// half of clearActiveLensWithoutRestore that a reset of the view axes alone
// wants: the lens stops being applied now and still opens at the next launch,
// which is the difference between putting a projection down and forgetting it.
func (m *OutlinerModel) forgetActiveLensFrame() {
	if m == nil {
		return
	}
	m.Lens.clearActive()
	m.discardPendingLensApplication()
	m.SetLensQuery("")
	// A lens a temporary search suspended is forgotten on the same terms. The
	// resets that reach here are replacing the projection outright, and a
	// suspension left behind would have the next Esc reinstate a lens over a view
	// it was never suspended under. suspendActiveLensForSearch calls this before
	// it records its own frame, so the one suspension that must survive does.
	m.Search.suspendedLens = nil
}

// lensModified reports whether the live view has diverged from the definition
// that was installed. Only the lens-owned axes are compared, so moving the
// cursor, folding a section, selecting rows, and opening Peek all leave an
// unmodified lens unmodified. Scope is the one carve-out from "navigating": it
// is a lens-owned axis, so entering or leaving a project scope under a live lens
// does mark it modified, and replacing the lens from the current view is what
// adopts the scope the user moved to.
func (m *OutlinerModel) LensModified() bool {
	if m == nil || !m.Lens.hasActive() {
		return false
	}
	return m.CaptureLensState().Fingerprint() != m.Lens.Active.SavedFingerprint
}

// activeLensLabel is the operational-title segment: the lens name, marked when
// the view has diverged from it.
//
// A lens a temporary search suspended keeps that segment and says so. It is the
// one place the suspension is visible, and it has to be: the mode, the query,
// and the rows on screen are all the search's, so a title that fell silent would
// read as a lens the slash had dismissed rather than one the next Esc puts back.
// The divergence marker is the one read while the lens was still live, because
// the suspended definition is not what the current view would be compared with.
func (m *OutlinerModel) ActiveLensLabel() string {
	if m == nil {
		return ""
	}
	if suspended := m.Search.suspendedLens; suspended != nil && suspended.lens != nil && !m.Lens.hasActive() {
		label := "Lens: " + suspended.lens.Name
		if suspended.modified {
			label += "*"
		}
		return label + " (suspended)"
	}
	if !m.Lens.hasActive() {
		return ""
	}
	label := "Lens: " + m.Lens.Active.Name
	if m.LensModified() {
		label += "*"
	}
	return label
}

// lensProjectLoadRequired reports whether installing the lens changes which
// projects are loaded at all. Under a single-project scope it never does: the
// scope already fixed the population to one project, so the lens's project
// facets are recorded on the model and stay inert until the scope is left.
func (m *OutlinerModel) lensProjectLoadRequired(state LensState) bool {
	if m == nil || m.Project != nil || m.IsProjectScoped() {
		return false
	}
	candidate := state.ProjectFilterState()
	return query.ProjectFilterFingerprint(&candidate) != m.loadedPopulationFingerprint()
}

// loadedPopulationFingerprint names the project filter the live population was
// read under. It is the live filter unless a lens is still owed a load, in which
// case the live filter is one the population never answered to and the frame
// holds the real provenance.
func (m *OutlinerModel) loadedPopulationFingerprint() string {
	if fingerprint, owed := m.Lens.awaitedPopulation(); owed {
		return fingerprint
	}
	return query.ProjectFilterFingerprint(&m.ProjectFilter)
}

// recordLoadedPopulation is where a landed population tells the live lens what
// it is. Every successful load names the filter it was actually read under, so
// the frame stops carrying a provenance the population moved away from: an
// ordinary reload that happens to answer the lens's own facets pays the owed
// load, and one that answers a different filter leaves the debt exactly where it
// was. A debt is never stranded by the recomputation below, because the only way
// the live filter can move is a route that reloads, and that reload settles this
// again when it lands.
func (m *OutlinerModel) recordLoadedPopulation(fingerprint string) {
	if m == nil {
		return
	}
	m.Lens.recordPopulation(fingerprint, query.ProjectFilterFingerprint(&m.ProjectFilter))
}

// beginLensProjectLoad starts a load against the candidate filter without
// writing it to the model. The baseline arrives already frozen, taken before a
// byte was read and before any scope was entered: the restoration point belongs
// to the view the application replaced, and nothing the user does while the load
// runs may move it.
func (m *OutlinerModel) beginLensProjectLoad(
	lens config.NamedLens,
	state LensState,
	origin OutlinerViewSnapshot,
) tea.Cmd {
	return m.startLensProjectLoad(&pendingLensApplication{
		lens:   lens,
		State:  state,
		Origin: origin,
	})
}

// beginLensScopeExit defers an explicit scope exit behind the load the live
// lens's project facets finally decide. A lens applied under a project scope
// recorded those facets without loading, because the scope had already fixed the
// population to one project; leaving that scope is the moment they select a
// population, so the exit waits on the projects it asked for rather than
// showing unscoped rows built from the population the scope replaced. It reports
// whether it took the exit; false leaves the ordinary immediate clear in place.
func (m *OutlinerModel) BeginLensScopeExit() (tea.Cmd, bool) {
	if m == nil || m.Project != nil || !m.IsProjectScoped() {
		return nil, false
	}
	if _, owed := m.Lens.awaitedPopulation(); !owed {
		return nil, false
	}
	// Leaving the scope supersedes any parked focus still being requested: that
	// request would otherwise win scopeState.focusRequest and have its project
	// appended to this population as an exception, while the settlement below only
	// ever evaluates the adopted one. Canceling it here, before the read selects
	// its focus, is what keeps the population the exit installs exactly the one
	// the live facets chose.
	m.cancelParkedProjectFocus()
	// The live state is what the facets are, marker and all: this application
	// publishes a population, never a definition, so it must read the filter the
	// user can currently see rather than the one the record was saved with.
	return m.startLensProjectLoad(&pendingLensApplication{
		State:         m.CaptureLensState(),
		Origin:        m.LensRestorationPoint(),
		scopeExitFrom: m.Scope.ProjectID,
	}), true
}

// startLensProjectLoad arms one application and returns its read. The generation
// bump is what makes the load safe to abandon: a later reload, a later lens, or a
// restoration all supersede it, and a superseded result publishes nothing.
func (m *OutlinerModel) startLensProjectLoad(pending *pendingLensApplication) tea.Cmd {
	m.discardPendingLensApplication()
	pending.Generation = m.Population.beginLoad()
	m.Lens.arm(pending)

	generation := pending.Generation
	runtime := m.Runtime
	service := m.ProjectService
	focus := m.Scope.FocusRequest()
	projectFilter := pending.State.ProjectFilterState()
	return func() tea.Msg {
		_, loaded, failed := LoadCrossScopeTasks(runtime, service, CrossScopeOpts{
			ActiveOnly:     true,
			FocusProjectID: focus,
			ProjectFilter:  projectFilter,
		})
		return LensProjectsLoadedMsg{
			Projects:    loaded,
			FailedLoads: failed,
			Generation:  generation,
		}
	}
}

// handleLensProjectsLoaded publishes the matching application or discards the
// result. It reports whether the lens was published so the shell knows when the
// snapshot it caches has actually been replaced.
func (m *OutlinerModel) HandleLensProjectsLoaded(msg LensProjectsLoadedMsg) (tea.Cmd, bool) {
	pending := m.Lens.Pending
	if pending == nil || msg.Generation != pending.Generation {
		// The result belongs to an application that is no longer the pending one,
		// so whatever is pending now owns the loading state and must keep it.
		// Nothing was published, and the view the user is looking at is already
		// the right one.
		return nil, false
	}
	if msg.Generation != m.Population.LoadGen || m.Lens.supersededGeneration(msg.Generation) {
		// This application was superseded or dismissed while its load ran. It
		// publishes nothing, but it is still the one holding the loading state, so
		// settle that here rather than leaving the model armed for a result that
		// has already arrived.
		m.settlePendingLensApplication()
		return nil, false
	}
	m.settlePendingLensApplication()

	if len(msg.Projects) == 0 && len(msg.FailedLoads) > 0 {
		// Every project the lens asked for failed to read. Publishing here would
		// install a lens over an empty population and destroy the restoration point
		// in the same step, so the application is abandoned instead.
		detail := msg.FailedLoads[0].Detail
		return func() tea.Msg { return status.ErrorMsg{Err: errors.New(detail)} }, false
	}

	if pending.scopeExitFrom != "" {
		if !m.finishLensScopeExit(pending, msg.Projects) {
			return nil, false
		}
	} else {
		m.PublishLens(pending.lens, pending.State, msg.Projects, pending.Origin)
	}
	// Both routes above install this read's population, so this read answers about
	// the projects it asked for: one it could not open is one no later restoration
	// may hand captured tasks back for, and one it did open is a failure expired.
	// It answers about nothing else — a lens that narrows the population to a
	// single project observes nothing about the projects it left out. The guard is
	// the pairing itself: an application that published a projection over the
	// population already standing attempted no project, and leaves every answer
	// with the read that produced it.
	if msg.Projects != nil {
		m.Population.recordLoadFailures(msg.Projects, msg.FailedLoads)
	}
	// A partial load is publishable: the lens holds the projects that read, and
	// the ones that did not are reported the way every other load reports them.
	return m.LoadFailureAdvisory(msg.FailedLoads), true
}

// finishLensScopeExit installs the population the live lens's project facets
// select and drops the scope in the same step. The frame is deliberately left
// alone: the lens is the one already applied, so its identity, its saved
// fingerprint, and the pre-lens restoration point all belong to the application
// that installed it and none of them is being replaced here. It reports whether
// the exit was still the one to take.
func (m *OutlinerModel) finishLensScopeExit(pending *pendingLensApplication, projects []*models.Project) bool {
	if m.Scope.ProjectID != pending.scopeExitFrom {
		// The scope moved while the read ran. Replacing one project scope with
		// another keeps the facets inert, so the population loaded here is dropped
		// and the exit is left to whichever scope the user is in now.
		return false
	}
	// A parked focus is an exception to the base workspace, and this population is
	// not that workspace: it was read under the lens's own project filter. Settling
	// the exception here is what keeps the scope drop below from shedding a parked
	// project the facets selected, and what leaves nothing owed either way — the
	// step that made this route's correctness depend on which of the exit's two
	// messages the runtime happened to deliver first.
	projects = m.settleParkedFocusAgainstPopulation(projects)
	m.setLoadedProjects(projects)
	m.Population.setSnapshotReady(true)
	// The provenance is the filter this exit read under, not the one the model
	// holds now: the two are the same on every route that reaches here, and
	// recording the read's own filter is what keeps that an observation rather
	// than an assumption.
	readFilter := pending.State.ProjectFilterState()
	m.recordLoadedPopulation(query.ProjectFilterFingerprint(&readFilter))
	m.pruneSelectionsOutsideLoadedProjects()
	if !m.ClearProjectScope() {
		// Defensive: the scope owns the rebuild, so there is one to do only when it
		// declined to restore anything.
		m.Population.Invalidate()
		m.CrossProjectRebuildRows()
		m.ClampCursor()
	}
	return true
}

// publishLens is the only place a lens becomes live. Nil projects mean the
// population is unchanged and the snapshot in hand stands. The origin is
// supplied rather than captured here: an asynchronous application froze its
// baseline before its I/O started, and this must install that one.

// A narrowing query narrows the projection its own mode, facets, scope, and sort
// already produced: the order stands, the rows are a subset of what the mode was
// already showing, and the query only decides which of them survive. Both
// carriers that hold free text compile through here — a lens's saved query and
// the slash-search input — because a query is a clause, not a mode. The flat
// ranked list is a presentation of the same rows and is reached by toggling it,
// never by typing.

// queryNarrowing is one compiled narrowing query. retained is what the
// projection may still admit — every match plus the ancestors that give a match
// its hierarchy context — and matched is the attribution source for the actual
// hits alone, so a retained ancestor carries no cue it did not earn.
type queryNarrowing struct {
	Active   bool
	retained map[taskref.TaskIdentity]bool
	matched  map[taskref.TaskIdentity]MatchSource
}

// narrowingCompilation is everything compiling one narrowing query produced. The
// carrier that asked for it records the remainder, gate, and case mode where its
// own chrome reads them; the narrowing itself is what the projection consumes.
type narrowingCompilation struct {
	narrowing queryNarrowing
	remainder string
	Gate      *query.SearchPredicates
	relaxed   bool
}

// setLensQuery installs the narrowing query and discards the compilation of the
// previous one. Nothing is rebuilt here: the next projection compiles it.
func (m *OutlinerModel) SetLensQuery(query string) {
	if m == nil {
		return
	}
	m.Lens.setQuery(query)
}

// lensGuidanceText returns the refusal a recognized-but-unhonorable lens query
// produced, or the empty string. It is the same guidance ordinary search shows,
// because both queries compile through the same grammar.
func (m *OutlinerModel) LensGuidanceText() string {
	if m == nil {
		return ""
	}
	return m.Lens.Gate.GuidanceText()
}

// lensTextQuery is the free-text remainder of the lens query, which is what
// scoring and highlighting must read: the raw query would score its own sigils
// as prose.
func (m *OutlinerModel) LensTextQuery() string {
	if m == nil {
		return ""
	}
	return m.Lens.remainder
}

// lensCaseSensitive reports the case mode the visible rows were actually
// produced under, so a highlight can never disagree with the narrowing.
func (m *OutlinerModel) LensCaseSensitive() bool {
	if m == nil || m.Lens.NarrowRelaxed {
		return false
	}
	return fuzzy.CaseSensitiveQuery(m.LensTextQuery())
}

// compileActiveNarrowing compiles whichever narrowing query the view holds and
// records the compilation where that carrier's chrome reads it. A lens owns the
// query whenever it holds one, because slash search under a lens edits the
// lens's query rather than opening a projection of its own; otherwise the search
// input owns it. At most one is ever non-empty, which is why one gate answers
// for both, and reading the query rather than the lens pointer is what keeps
// that true for a carrier seeded outside the lens lifecycle.
func (m *OutlinerModel) compileActiveNarrowing(projects []*models.Project) queryNarrowing {
	if m == nil {
		return queryNarrowing{}
	}
	if m.Lens.Query != "" {
		compiled := m.compileNarrowingQuery(m.Lens.Query, projects)
		m.Lens.setCompilation(compiled.remainder, compiled.Gate, compiled.relaxed)
		return compiled.narrowing
	}
	query := ""
	if m.HasSearchProjection() {
		query = m.SearchQuery()
	}
	compiled := m.compileNarrowingQuery(query, projects)
	m.Search.setCompilation(compiled.remainder, compiled.Gate, compiled.relaxed)
	return compiled.narrowing
}

// compileNarrowingQuery parses one query through the shared search grammar and
// resolves the identities the projection may keep. An empty query compiles to an
// inactive narrowing rather than to a gate that admits everything, so a view with
// no query does no scoring work at all.
func (m *OutlinerModel) compileNarrowingQuery(query string, projects []*models.Project) narrowingCompilation {
	if m == nil || strings.TrimSpace(query) == "" {
		return narrowingCompilation{}
	}

	remainder, predicates := m.compileSearchQuery(query, projects)
	compiled := narrowingCompilation{
		remainder: remainder,
		Gate:      predicates,
		narrowing: queryNarrowing{
			Active:   true,
			retained: make(map[taskref.TaskIdentity]bool),
			matched:  make(map[taskref.TaskIdentity]MatchSource),
		},
	}
	if predicates.Invalid() {
		// The guidance stands and nothing is admitted.
		return compiled
	}

	matcher := fuzzy.SmartCase(remainder)
	matches := searchProjectionMatchesWithin(projects, remainder, nil, matcher, predicates)
	if len(matches) == 0 && matcher.CaseSensitive() {
		matches = searchProjectionMatchesWithin(projects, remainder, nil, fuzzy.NewMatcher(false), predicates)
		compiled.relaxed = len(matches) > 0
	}

	for _, match := range matches {
		identity := taskref.For(match.task, match.project)
		if !identity.Valid() {
			continue
		}
		compiled.narrowing.matched[identity] = match.source
		compiled.narrowing.retained[identity] = true
		retainNarrowingAncestors(compiled.narrowing.retained, match.task, match.project)
	}
	return compiled
}

// retainNarrowingAncestors admits the canonical ancestry of one match. It follows
// stored ParentID links through the authoritative project rather than a row's
// own Children, which the filter pipeline may have rewritten.
func retainNarrowingAncestors(retained map[taskref.TaskIdentity]bool, task *models.Task, project *models.Project) {
	if project == nil {
		return
	}
	seen := make(map[string]bool)
	for current := task; current != nil && current.ParentID != nil; {
		parentID := *current.ParentID
		if seen[parentID] {
			return
		}
		seen[parentID] = true
		parent := project.GetTaskByID(parentID)
		if parent == nil {
			return
		}
		retained[taskref.For(parent, project)] = true
		current = parent
	}
}

// constrainModeFilterToNarrowing composes the narrowing over the mode input's
// own task predicate. Narrowing is applied as one more facet rather than as a
// separate pass, so mode membership, the saved facets, the section emitters,
// the project grouping, and the saved order all keep deciding the rows: an
// emptied section or project group simply stops being emitted.
//
// An ancestor is retained for the context it gives a match, so it is retained
// against the matches that actually survive the rest of the gate rather than
// against every match the query found. A facet that removes the only hit in a
// branch therefore removes the branch, instead of leaving a parent standing over
// nothing.
func (m *OutlinerModel) constrainModeFilterToNarrowing(
	base func(*models.Task, *models.Project) bool,
	narrowing queryNarrowing,
	projects []*models.Project,
) func(*models.Task, *models.Project) bool {
	if !narrowing.Active {
		return base
	}
	retained := narrowing.retained
	if base != nil {
		retained = retainedUnderBaseFilter(narrowing, base, projects)
	}
	return func(task *models.Task, project *models.Project) bool {
		if base != nil && !base(task, project) {
			return false
		}
		if project == nil {
			project = m.Project
		}
		return retained[taskref.For(task, project)]
	}
}

// retainedUnderBaseFilter recomputes the retained closure over the matches the
// base predicate still admits. It walks the authoritative projects rather than
// the compiled sets because ancestry is a stored fact and only the projects
// carry it.
func retainedUnderBaseFilter(
	narrowing queryNarrowing,
	base func(*models.Task, *models.Project) bool,
	projects []*models.Project,
) map[taskref.TaskIdentity]bool {
	retained := make(map[taskref.TaskIdentity]bool, len(narrowing.retained))
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			identity := taskref.For(task, project)
			if !identity.Valid() {
				continue
			}
			if _, matched := narrowing.matched[identity]; !matched {
				continue
			}
			if !base(task, project) {
				continue
			}
			retained[identity] = true
			retainNarrowingAncestors(retained, task, project)
		}
	}
	return retained
}

// attributeNarrowingMatches stamps the match cue on the rows that actually
// matched. Attribution is recomputed with membership on every rebuild, so a
// mutation that moves a hit from one field to another moves its cue too, and an
// ancestor retained only for context keeps matchSourceNone.
func (m *OutlinerModel) attributeNarrowingMatches(narrowing queryNarrowing) {
	if !narrowing.Active || len(narrowing.matched) == 0 {
		return
	}
	for index := range m.Rows {
		row := m.Rows[index]
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		if source, ok := narrowing.matched[m.sectionRowIdentity(row)]; ok {
			m.Rows[index].MatchSource = source
		}
	}
}
