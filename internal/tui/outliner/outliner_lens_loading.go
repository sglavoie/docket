package outliner

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/status"
)

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
		loaded, failed := LoadCrossScopeTasks(runtime, service, CrossScopeOpts{
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
