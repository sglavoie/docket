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
	if m.lensProjectLoadRequired(state) {
		return m.beginLensProjectLoad(lens, state), nil
	}
	m.publishLens(lens, state, nil, m.lensRestorationPoint())
	return nil, nil
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
	return projectFilterFingerprint(&candidate) != projectFilterFingerprint(&m.projectFilter)
}

// beginLensProjectLoad starts a load against the candidate filter without
// writing it to the model. The generation bump is what makes the load safe to
// abandon: a later reload, a later lens, or a restoration all supersede it, and
// a superseded result publishes nothing.
func (m *OutlinerModel) beginLensProjectLoad(lens config.NamedLens, state lensState) tea.Cmd {
	// The baseline is frozen here, before a byte is read: the restoration point
	// belongs to the view the application replaced, and nothing the user does
	// while the load runs may move it.
	origin := m.lensRestorationPoint()
	m.discardPendingLensApplication()
	m.crossLoadGen++
	m.crossLoading = true
	m.pendingLens = &pendingLensApplication{
		generation: m.crossLoadGen,
		lens:       lens,
		state:      state,
		origin:     origin,
	}
	if m.activeLens != nil {
		m.activeLens.pendingGeneration = m.pendingLens.generation
	}

	generation := m.pendingLens.generation
	runtime := m.runtime
	service := m.projectService
	focus := m.focusRequestForLoad()
	projectFilter := state.projectFilter.clone()
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

	m.publishLens(pending.lens, pending.state, msg.projects, pending.origin)
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

	// A lens replaces the projection, so an ordinary search projection cannot
	// survive underneath it. The baseline above already holds search's own
	// restoration point, which is what one Esc later hands back.
	m.clearSearchProjectionWithoutRestore()
	m.endLensQueryEdit()

	if projects != nil {
		m.loadedProjects = projects
		m.crossSnapshotReady = true
	}
	// Scope, focus, folds, selection, and Peek are the view the user is in, not
	// the lens: applyLensState writes only the lens-owned axes.
	m.applyLensState(state)
	if projects != nil {
		m.pruneSelectionsOutsideLoadedProjects()
	}

	m.activeLens = &activeLensFrame{
		lensID:           lens.ID,
		name:             lens.Name,
		savedFingerprint: state.fingerprint(),
		origin:           origin,
	}
	m.crossForceRebuild = true
	m.rebuildRows()
	m.restoreSearchCursor(anchor)
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
}

// lensModified reports whether the live view has diverged from the definition
// that was installed. Only the lens-owned axes are compared, so moving the
// cursor, folding a section, selecting rows, opening Peek, and navigating all
// leave an unmodified lens unmodified.
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
