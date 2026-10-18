package outliner

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
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

// PublishLens is the only place a lens becomes live. Nil projects mean the
// population is unchanged and the snapshot in hand stands. The origin is
// supplied rather than captured here: an asynchronous application froze its
// baseline before its I/O started, and this must install that one.
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
		SavedState:                  saved,
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
	return m.CaptureLensState().Fingerprint() != m.Lens.Active.SavedState.Fingerprint()
}

// LensChangedAxes explains the same comparison the modified marker reads.
func (m *OutlinerModel) LensChangedAxes() []string {
	if m == nil || !m.Lens.hasActive() {
		return nil
	}
	return m.CaptureLensState().ChangedAxes(m.Lens.Active.SavedState)
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
