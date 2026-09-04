package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
)

// lensProjectLoadRequired reports whether installing the lens changes which
// projects are loaded at all. Under a single-project scope it never does: the
// scope already fixed the population to one project, so the lens's project
// facets are recorded on the model and stay inert until the scope is left.
func (m *OutlinerModel) lensProjectLoadRequired(state lensState) bool {
	if m == nil || m.project != nil || m.isProjectScoped() {
		return false
	}
	candidate := state.projectFilterState()
	return query.ProjectFilterFingerprint(&candidate) != m.loadedPopulationFingerprint()
}

// loadedPopulationFingerprint names the project filter the live population was
// read under. It is the live filter unless a lens is still owed a load, in which
// case the live filter is one the population never answered to and the frame
// holds the real provenance.
func (m *OutlinerModel) loadedPopulationFingerprint() string {
	if fingerprint, owed := m.lens.awaitedPopulation(); owed {
		return fingerprint
	}
	return query.ProjectFilterFingerprint(&m.projectFilter)
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
	m.lens.recordPopulation(fingerprint, query.ProjectFilterFingerprint(&m.projectFilter))
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
	if _, owed := m.lens.awaitedPopulation(); !owed {
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
		state:         m.captureLensState(),
		origin:        m.lensRestorationPoint(),
		scopeExitFrom: m.scope.projectID,
	}), true
}

// startLensProjectLoad arms one application and returns its read. The generation
// bump is what makes the load safe to abandon: a later reload, a later lens, or a
// restoration all supersede it, and a superseded result publishes nothing.
func (m *OutlinerModel) startLensProjectLoad(pending *pendingLensApplication) tea.Cmd {
	m.discardPendingLensApplication()
	pending.generation = m.population.beginLoad()
	m.lens.arm(pending)

	generation := pending.generation
	runtime := m.runtime
	service := m.projectService
	focus := m.scope.focusRequest()
	projectFilter := pending.state.projectFilterState()
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
	pending := m.lens.pending
	if pending == nil || msg.generation != pending.generation {
		// The result belongs to an application that is no longer the pending one,
		// so whatever is pending now owns the loading state and must keep it.
		// Nothing was published, and the view the user is looking at is already
		// the right one.
		return nil, false
	}
	if msg.generation != m.population.loadGen || m.lens.supersededGeneration(msg.generation) {
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
		return func() tea.Msg { return status.ErrorMsg{Err: errors.New(detail)} }, false
	}

	if pending.scopeExitFrom != "" {
		if !m.finishLensScopeExit(pending, msg.projects) {
			return nil, false
		}
	} else {
		m.publishLens(pending.lens, pending.state, msg.projects, pending.origin)
	}
	// A partial load is publishable: the lens holds the projects that read, and
	// the ones that did not are reported the way every other load reports them.
	return m.loadFailureAdvisory(msg.failedLoads), true
}

// finishLensScopeExit installs the population the live lens's project facets
// select and drops the scope in the same step. The frame is deliberately left
// alone: the lens is the one already applied, so its identity, its saved
// fingerprint, and the pre-lens restoration point all belong to the application
// that installed it and none of them is being replaced here. It reports whether
// the exit was still the one to take.
func (m *OutlinerModel) finishLensScopeExit(pending *pendingLensApplication, projects []*models.Project) bool {
	if m.scope.projectID != pending.scopeExitFrom {
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
	m.population.setSnapshotReady(true)
	// The provenance is the filter this exit read under, not the one the model
	// holds now: the two are the same on every route that reaches here, and
	// recording the read's own filter is what keeps that an observation rather
	// than an assumption.
	readFilter := pending.state.projectFilterState()
	m.recordLoadedPopulation(query.ProjectFilterFingerprint(&readFilter))
	m.pruneSelectionsOutsideLoadedProjects()
	if !m.clearProjectScope() {
		// Defensive: the scope owns the rebuild, so there is one to do only when it
		// declined to restore anything.
		m.population.invalidate()
		m.crossProjectRebuildRows()
		m.clampCursor()
	}
	return true
}

// publishLens is the only place a lens becomes live. Nil projects mean the
// population is unchanged and the snapshot in hand stands. The origin is
// supplied rather than captured here: an asynchronous application froze its
// baseline before its I/O started, and this must install that one.
