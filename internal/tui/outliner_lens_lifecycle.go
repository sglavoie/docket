package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
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

// lensProjectsLoadedMsg carries the result of a load started for one lens
// application. It is deliberately distinct from crossProjectsLoadedMsg: an
// ordinary reload publishes its projects unconditionally, while this one
// publishes a whole lens or nothing at all.
type lensProjectsLoadedMsg struct {
	projects    []*models.Project
	failedLoads []projectLoadFailure
	generation  int
}

// hasActiveLens reports whether a lens owns the current projection.
func (m *OutlinerModel) hasActiveLens() bool {
	return m != nil && m.activeLens != nil
}

// ApplyNamedLens installs a saved projection, or refuses without touching the
// current view. Every reason to refuse — a malformed definition, an unknown
// mode, a query the grammar cannot honor — is decided before a single axis is
// written, so a refusal is indistinguishable from never having asked.
func (m *OutlinerModel) ApplyNamedLens(lens config.NamedLens) (tea.Cmd, error) {
	if m == nil {
		return nil, errors.New("no outliner")
	}
	state, err := lensStateFromNamedLens(lens)
	if err != nil {
		return nil, err
	}
	if _, ok := OutlinerModeByID(state.mode); !ok {
		return nil, fmt.Errorf("lens %q names an unknown mode", lens.Name)
	}
	if err := m.validateLensQuery(state.query); err != nil {
		return nil, err
	}
	if err := m.lensScopeRefusal(lens.Name, state.scopeProjectID); err != nil {
		return nil, err
	}

	// The baseline is the view this application replaces, so it is taken before
	// the scope push below moves it: one Esc owes the user the scope they were in,
	// not the one the lens just entered.
	origin := m.lensRestorationPoint()
	// The scope goes in before the load decision, never after it. Under a
	// single-project scope lensProjectLoadRequired is false, so a scoped lens
	// reads no population at all and its project facets are recorded inert for the
	// explicit scope exit to honor. Pushing afterwards would start a load the
	// scope immediately invalidates.
	if state.scopeProjectID != "" {
		m.pushProjectScope(state.scopeProjectID)
	}
	if m.lensProjectLoadRequired(state) {
		return m.beginLensProjectLoad(lens, state, origin), nil
	}
	m.publishLens(lens, state, nil, origin)
	return nil, nil
}

// lensScopeRefusal decides whether the lens's scope can be entered at all,
// before a single axis is written. TaskWorkspaceProjects() is the one population
// the base Outliner scopes into and it is already non-archived and unparked, so
// that single membership test answers all three refusals; the catalog is read
// only to word the message and never to widen admission. Parked therefore fails
// closed in v1 — parkedFocusProjectID and settleParkedFocusException are the
// seam a later revision would reopen it through.
func (m *OutlinerModel) lensScopeRefusal(name, projectID string) error {
	if projectID == "" || m.baseWorkspaceAdmitsProject(projectID) {
		return nil
	}
	if m.runtime != nil {
		if project := m.runtime.GetProject(projectID); project != nil {
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
	_, predicates := m.compileSearchQuery(query, m.workModeScopeProjects())
	if guidance := predicates.guidanceText(); guidance != "" {
		return errors.New(guidance)
	}
	return nil
}

// lensProjectLoadRequired reports whether installing the lens changes which
// projects are loaded at all. Under a single-project scope it never does: the
// scope already fixed the population to one project, so the lens's project
// facets are recorded on the model and stay inert until the scope is left.
func (m *OutlinerModel) lensProjectLoadRequired(state lensState) bool {
	if m == nil || m.project != nil || m.isProjectScoped() {
		return false
	}
	candidate := state.projectFilter
	return projectFilterFingerprint(&candidate) != m.loadedPopulationFingerprint()
}

// loadedPopulationFingerprint names the project filter the live population was
// read under. It is the live filter unless a lens is still owed a load, in which
// case the live filter is one the population never answered to and the frame
// holds the real provenance.
func (m *OutlinerModel) loadedPopulationFingerprint() string {
	if m.activeLens != nil && m.activeLens.projectFacetsAwaitingLoad {
		return m.activeLens.loadedPopulationFingerprint
	}
	return projectFilterFingerprint(&m.projectFilter)
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
	if m == nil || m.activeLens == nil {
		return
	}
	m.activeLens.loadedPopulationFingerprint = fingerprint
	m.activeLens.projectFacetsAwaitingLoad = fingerprint != projectFilterFingerprint(&m.projectFilter)
}

// beginLensProjectLoad starts a load against the candidate filter without
// writing it to the model. The baseline arrives already frozen, taken before a
// byte was read and before any scope was entered: the restoration point belongs
// to the view the application replaced, and nothing the user does while the load
// runs may move it.
func (m *OutlinerModel) beginLensProjectLoad(
	lens config.NamedLens,
	state lensState,
	origin outlinerViewSnapshot,
) tea.Cmd {
	return m.startLensProjectLoad(&pendingLensApplication{
		lens:   lens,
		state:  state,
		origin: origin,
	})
}

// beginLensScopeExit defers an explicit scope exit behind the load the live
// lens's project facets finally decide. A lens applied under a project scope
// recorded those facets without loading, because the scope had already fixed the
// population to one project; leaving that scope is the moment they select a
// population, so the exit waits on the projects it asked for rather than
// showing unscoped rows built from the population the scope replaced. It reports
// whether it took the exit; false leaves the ordinary immediate clear in place.
func (m *OutlinerModel) beginLensScopeExit() (tea.Cmd, bool) {
	if m == nil || m.project != nil || !m.isProjectScoped() {
		return nil, false
	}
	if m.activeLens == nil || !m.activeLens.projectFacetsAwaitingLoad {
		return nil, false
	}
	// Leaving the scope supersedes any parked focus still being requested: that
	// request would otherwise win focusRequestForLoad and have its project
	// appended to this population as an exception, while the settlement below only
	// ever evaluates the adopted one. Canceling it here, before the read selects
	// its focus, is what keeps the population the exit installs exactly the one
	// the live facets chose.
	m.cancelParkedProjectFocus()
	// The live state is what the facets are, marker and all: this application
	// publishes a population, never a definition, so it must read the filter the
	// user can currently see rather than the one the record was saved with.
	return m.startLensProjectLoad(&pendingLensApplication{
		state:         m.captureLensState(),
		origin:        m.lensRestorationPoint(),
		scopeExitFrom: m.scopedProjectID,
	}), true
}

// startLensProjectLoad arms one application and returns its read. The generation
// bump is what makes the load safe to abandon: a later reload, a later lens, or a
// restoration all supersede it, and a superseded result publishes nothing.
func (m *OutlinerModel) startLensProjectLoad(pending *pendingLensApplication) tea.Cmd {
	m.discardPendingLensApplication()
	m.crossLoadGen++
	m.crossLoading = true
	pending.generation = m.crossLoadGen
	m.pendingLens = pending
	if m.activeLens != nil {
		m.activeLens.pendingGeneration = pending.generation
	}

	generation := pending.generation
	runtime := m.runtime
	service := m.projectService
	focus := m.focusRequestForLoad()
	projectFilter := pending.state.projectFilter.clone()
	return func() tea.Msg {
		_, loaded, failed := LoadCrossScopeTasks(runtime, service, CrossScopeOpts{
			ActiveOnly:     true,
			FocusProjectID: focus,
			ProjectFilter:  projectFilter,
		})
		return lensProjectsLoadedMsg{
			projects:    loaded,
			failedLoads: failed,
			generation:  generation,
		}
	}
}

// handleLensProjectsLoaded publishes the matching application or discards the
// result. It reports whether the lens was published so the shell knows when the
// snapshot it caches has actually been replaced.
func (m *OutlinerModel) handleLensProjectsLoaded(msg lensProjectsLoadedMsg) (tea.Cmd, bool) {
	pending := m.pendingLens
	if pending == nil || msg.generation != pending.generation {
		// The result belongs to an application that is no longer the pending one,
		// so whatever is pending now owns the loading state and must keep it.
		// Nothing was published, and the view the user is looking at is already
		// the right one.
		return nil, false
	}
	if msg.generation != m.crossLoadGen || (m.activeLens != nil && m.activeLens.pendingGeneration != msg.generation) {
		// This application was superseded or dismissed while its load ran. It
		// publishes nothing, but it is still the one holding the loading state, so
		// settle that here rather than leaving the model armed for a result that
		// has already arrived.
		m.settlePendingLensApplication()
		return nil, false
	}
	m.settlePendingLensApplication()

	if len(msg.projects) == 0 && len(msg.failedLoads) > 0 {
		// Every project the lens asked for failed to read. Publishing here would
		// install a lens over an empty population and destroy the restoration point
		// in the same step, so the application is abandoned instead.
		detail := msg.failedLoads[0].Detail
		return func() tea.Msg { return ErrorMsg{Err: errors.New(detail)} }, false
	}

	if pending.scopeExitFrom != "" {
		if !m.finishLensScopeExit(pending, msg.projects) {
			return nil, false
		}
	} else {
		m.publishLens(pending.lens, pending.state, msg.projects, pending.origin)
	}
	if len(msg.failedLoads) == 0 {
		return nil, true
	}
	// A partial load is publishable: the lens holds the projects that read, and
	// the ones that did not are reported the way every other load reports them.
	count := len(msg.failedLoads)
	noun := "project"
	if count > 1 {
		noun = "projects"
	}
	warning := fmt.Sprintf("%d %s failed to load: %s", count, noun, msg.failedLoads[0].Detail)
	return func() tea.Msg { return InfoStatusMsg(warning) }, true
}

// finishLensScopeExit installs the population the live lens's project facets
// select and drops the scope in the same step. The frame is deliberately left
// alone: the lens is the one already applied, so its identity, its saved
// fingerprint, and the pre-lens restoration point all belong to the application
// that installed it and none of them is being replaced here. It reports whether
// the exit was still the one to take.
func (m *OutlinerModel) finishLensScopeExit(pending *pendingLensApplication, projects []*models.Project) bool {
	if m.scopedProjectID != pending.scopeExitFrom {
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
	m.crossSnapshotReady = true
	// The provenance is the filter this exit read under, not the one the model
	// holds now: the two are the same on every route that reaches here, and
	// recording the read's own filter is what keeps that an observation rather
	// than an assumption.
	m.recordLoadedPopulation(projectFilterFingerprint(&pending.state.projectFilter))
	m.pruneSelectionsOutsideLoadedProjects()
	if !m.clearProjectScope() {
		// Defensive: the scope owns the rebuild, so there is one to do only when it
		// declined to restore anything.
		m.crossForceRebuild = true
		m.crossProjectRebuildRows()
		m.clampCursor()
	}
	return true
}

// publishLens is the only place a lens becomes live. Nil projects mean the
// population is unchanged and the snapshot in hand stands. The origin is
// supplied rather than captured here: an asynchronous application froze its
// baseline before its I/O started, and this must install that one.
func (m *OutlinerModel) publishLens(lens config.NamedLens, state lensState, projects []*models.Project, origin outlinerViewSnapshot) {
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
	candidate := state.projectFilter
	population := m.loadedPopulationFingerprint()
	facetsAwaitingLoad := projects == nil &&
		projectFilterFingerprint(&candidate) != population
	if !facetsAwaitingLoad {
		population = projectFilterFingerprint(&candidate)
	}

	// A lens replaces the projection, so an ordinary search projection cannot
	// survive underneath it. The baseline above already holds search's own
	// restoration point, which is what one Esc later hands back.
	m.clearSearchProjectionWithoutRestore()
	m.endLensQueryEdit()

	if projects != nil {
		m.setLoadedProjects(projects)
		m.crossSnapshotReady = true
	}
	// Focus, folds, selection, and Peek are the view the user is in, not the lens:
	// applyLensState writes only the lens-owned axes, scope among them.
	m.applyLensState(state)
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
	saved.scopeProjectID = m.capturedLensScope()

	m.activeLens = &activeLensFrame{
		lensID:                      lens.ID,
		name:                        lens.Name,
		savedFingerprint:            saved.fingerprint(),
		origin:                      origin,
		projectFacetsAwaitingLoad:   facetsAwaitingLoad,
		loadedPopulationFingerprint: population,
	}
	m.crossForceRebuild = true
	m.rebuildRows()
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
	if m == nil || m.persistActiveLens == nil {
		return
	}
	if err := m.persistActiveLens(id); err != nil {
		m.setTransientStatus("Could not remember the active lens: " + err.Error())
	}
}

// lensRestorationPoint returns the baseline the next Esc restores: the original
// one when a lens is already live, a fresh capture otherwise. This is the whole
// of the non-stacking rule.
func (m *OutlinerModel) lensRestorationPoint() outlinerViewSnapshot {
	if m.activeLens != nil {
		return m.activeLens.origin
	}
	return m.captureOutlinerViewSnapshot()
}

// restoreLensBaseline dismisses the lens and returns to the captured view. Any
// application still in flight is invalidated first, so a load that lands after
// this cannot reinstall a lens the user just left.
func (m *OutlinerModel) restoreLensBaseline() {
	if m == nil || m.activeLens == nil {
		return
	}
	frame := m.activeLens
	m.activeLens = nil
	m.endLensQueryEdit()
	m.discardPendingLensApplication()
	m.restoreOutlinerViewSnapshot(frame.origin)
	m.noteActiveLens("")
}

// discardPendingLensApplication abandons an unpublished application. The
// generation bump is what rejects the result already on its way back; it is
// taken only when something is actually pending, so an ordinary reload in flight
// beside it is left alone.
func (m *OutlinerModel) discardPendingLensApplication() {
	if m == nil || m.pendingLens == nil {
		return
	}
	m.crossLoadGen++
	m.settlePendingLensApplication()
}

// settlePendingLensApplication clears the state one application armed without
// bumping the generation. It is what a result that has already come back needs:
// there is nothing left in flight to invalidate, only loading state to put down.
func (m *OutlinerModel) settlePendingLensApplication() {
	if m == nil || m.pendingLens == nil {
		return
	}
	m.pendingLens = nil
	m.crossLoading = false
	m.crossSpinnerVisible = false
	if m.activeLens != nil {
		m.activeLens.pendingGeneration = 0
	}
}

// clearActiveLensWithoutRestore drops the frame without returning to its
// baseline. It belongs to the resets that are replacing the whole projection
// anyway, never to a dismissal the user asked for.
func (m *OutlinerModel) clearActiveLensWithoutRestore() {
	if m == nil {
		return
	}
	m.activeLens = nil
	m.discardPendingLensApplication()
	m.endLensQueryEdit()
	m.setLensQuery("")
	m.noteActiveLens("")
}

// lensModified reports whether the live view has diverged from the definition
// that was installed. Only the lens-owned axes are compared, so moving the
// cursor, folding a section, selecting rows, and opening Peek all leave an
// unmodified lens unmodified. Scope is the one carve-out from "navigating": it
// is a lens-owned axis, so entering or leaving a project scope under a live lens
// does mark it modified, and replacing the lens from the current view is what
// adopts the scope the user moved to.
func (m *OutlinerModel) lensModified() bool {
	if m == nil || m.activeLens == nil {
		return false
	}
	return m.captureLensState().fingerprint() != m.activeLens.savedFingerprint
}

// activeLensLabel is the operational-title segment: the lens name, marked when
// the view has diverged from it.
func (m *OutlinerModel) activeLensLabel() string {
	if m == nil || m.activeLens == nil {
		return ""
	}
	label := "Lens: " + m.activeLens.name
	if m.lensModified() {
		label += "*"
	}
	return label
}

// editingLensQuery reports whether the search input currently holds the lens's
// narrowing query rather than an ordinary search query.
func (m *OutlinerModel) editingLensQuery() bool {
	return m != nil && m.lensQueryEditing && m.IsSearchMode()
}

// beginLensQueryEdit points `/` at the narrowing query. No search projection is
// created: the lens keeps deciding the mode, facets, and order, and the query
// only decides which of its rows survive.
func (m *OutlinerModel) beginLensQueryEdit() tea.Cmd {
	if m.lensQueryEditing {
		return nil
	}
	m.lensQueryEditing = true
	m.lensQueryEditOrigin = m.lensQuery
	// Suggestions describe the ordinary search grammar's candidate set and its
	// projection, neither of which exists here.
	m.searchAutocomplete.clear()
	m.SetSearchInputValue(m.lensQuery)
	m.SetSearchInputCursor(len([]rune(m.lensQuery)))
	cmd := m.BaseSearchableView.ActivateSearch()
	// The ordinary query stays empty throughout, so no consumer of it — scoring,
	// the work-mode predicate, the empty-state cause — ever sees the lens's query
	// and applies it a second time.
	m.BaseSearchableView.SetSearchQuery("")
	return cmd
}

// handleLensQueryKey is the input loop for that edit. Enter accepts the typed
// query, Esc restores the one the edit opened on, and every other key retypes
// the narrowing live.
func (m *OutlinerModel) handleLensQueryKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cancelLensQueryEdit()
		return m, nil
	case "enter":
		m.commitLensQueryEdit()
		return m, nil
	}
	_, cmd, changed := m.HandleSearchInput(msg)
	m.BaseSearchableView.SetSearchQuery("")
	if changed {
		m.setLensQuery(m.SearchInputValue())
		m.crossForceRebuild = true
		rebuildOutlinerPreservingCursor(m)
	}
	return m, cmd
}

func (m *OutlinerModel) commitLensQueryEdit() {
	if !m.lensQueryEditing {
		return
	}
	query := m.SearchInputValue()
	m.endLensQueryEdit()
	m.setLensQuery(query)
	m.crossForceRebuild = true
	rebuildOutlinerPreservingCursor(m)
}

func (m *OutlinerModel) cancelLensQueryEdit() {
	if !m.lensQueryEditing {
		return
	}
	origin := m.lensQueryEditOrigin
	m.endLensQueryEdit()
	m.setLensQuery(origin)
	m.crossForceRebuild = true
	rebuildOutlinerPreservingCursor(m)
}

// endLensQueryEdit leaves the input mode without deciding the query. Both the
// commit and the cancel above name that decision themselves.
func (m *OutlinerModel) endLensQueryEdit() {
	if m == nil || !m.lensQueryEditing {
		return
	}
	m.lensQueryEditing = false
	m.lensQueryEditOrigin = ""
	m.searchAutocomplete.clear()
	m.DeactivateSearch()
}
