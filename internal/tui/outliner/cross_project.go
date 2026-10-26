package outliner

import (
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/status"
)

// crossProjectsLoadedMsg carries the result of an async cross-project load.
type CrossProjectsLoadedMsg struct {
	Projects    []*models.Project
	FailedLoads []ProjectLoadFailure
	Generation  int

	// focusProjectID is set only when this load was started to enter a parked
	// project focus. Only that load pushes a scope, and only it cancels outright
	// when the project cannot be read.
	FocusProjectID string

	// retainedFocusProjectID names the already-adopted focus a steady-state
	// reload carried on top of the base workspace. It leaves the focus exactly
	// where it is and exists so the handler can re-decide whether the exception
	// that admits the project is still required.
	retainedFocusProjectID string

	// loadedProjectFilter is the fingerprint of the project filter this read was
	// actually issued under. It travels with the result because the live filter is
	// not always the one the population answers to: a lens applied under a project
	// scope writes its facets without loading, so a reload started before it lands
	// afterwards carrying the older population. The provenance has to be the
	// filter the read used, never the one the model happens to hold when it
	// arrives.
	LoadedProjectFilter string
}

// crossSpinnerDelay is how long an initial load runs before it animates a
// spinner, so a fast load never flashes one.
const crossSpinnerDelay = 200 * time.Millisecond

// crossSpinnerDelayMsg fires after crossSpinnerDelay to animate the initial-load
// spinner.
type CrossSpinnerDelayMsg struct{}

// crossSpinnerTickMsg wraps a spinner tick so it can be distinguished from
// the global save spinner's ticks.
type crossSpinnerTickMsg struct {
	inner tea.Msg
}

// crossSpinnerTickFwd wraps a spinner Cmd so its ticks are delivered as
// crossSpinnerTickMsg rather than bare spinner.TickMsg.
func crossSpinnerTickFwd(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		return crossSpinnerTickMsg{inner: msg}
	}
}

// startCrossProjectLoad sets loading state and returns a Cmd that begins async
// loading of all context-filtered projects plus a 200ms initial-spinner delay.
func (m *OutlinerModel) startCrossProjectLoad() tea.Cmd {
	if !m.hasCrossProjectSnapshot() {
		m.Population.resetSpinner()
	}
	// An ordinary reload reads the live project filter, which is not the candidate
	// filter a pending lens application is waiting on. That application can never
	// publish behind this load, so it is abandoned here rather than left to have
	// its result arrive against a model that has moved on.
	m.discardPendingLensApplication()
	m.Population.beginLoad()
	return tea.Batch(
		m.LoadCrossProjectsCmd(),
		status.TickCmd(crossSpinnerDelay, func(time.Time) tea.Msg {
			return CrossSpinnerDelayMsg{}
		}),
	)
}

// loadCrossProjectsCmd returns a Cmd that loads all active (context-filtered)
// projects from disk. Failed projects are skipped; their load errors are
// collected for a non-blocking status bar warning, matching the AllTasks
// continue-on-error pattern.
func (m *OutlinerModel) LoadCrossProjectsCmd() tea.Cmd {
	gen := m.Population.LoadGen
	runtime := m.Runtime
	svc := m.ProjectService
	adopting := m.Scope.PendingParkedID != ""
	focus := m.Scope.FocusRequest()
	projectFilter := m.ProjectFilter.Clone()
	return func() tea.Msg {
		loaded, failed := LoadCrossScopeTasks(runtime, svc, CrossScopeOpts{
			ActiveOnly:     true,
			FocusProjectID: focus,
			ProjectFilter:  projectFilter,
		})
		msg := CrossProjectsLoadedMsg{
			Projects:            loaded,
			FailedLoads:         failed,
			Generation:          gen,
			LoadedProjectFilter: query.ProjectFilterFingerprint(&projectFilter),
		}
		if adopting {
			msg.FocusProjectID = focus
		} else {
			msg.retainedFocusProjectID = focus
		}
		return msg
	}
}

// beginParkedProjectFocus starts the load that admits one parked project on top
// of the base workspace. The generation bump inside startCrossProjectLoad is
// what makes a repeated, changed, or abandoned request safe: only the newest
// request's result is ever accepted.
func (m *OutlinerModel) BeginParkedProjectFocus(projectID string) tea.Cmd {
	if m == nil || m.Project != nil || projectID == "" {
		return nil
	}
	m.Scope.requestParked(projectID)
	return m.ReloadCrossProjects()
}

// cancelParkedProjectFocus abandons an in-flight focus request. The base
// snapshot is untouched, so there is nothing to restore.
func (m *OutlinerModel) cancelParkedProjectFocus() {
	if m == nil || m.Scope.PendingParkedID == "" {
		return
	}
	m.Scope.clearPendingParked()
	m.CancelCrossProjectLoad()
}

// takeParkedFocusReload claims the reload owed after a parked focus was dropped
// synchronously. The Esc cascade drops the project from the live snapshot so no
// empty frame is rendered; this reload then re-reads the base workspace alone.
func (m *OutlinerModel) TakeParkedFocusReload() tea.Cmd {
	if m == nil || !m.Scope.ParkedReloadPending {
		return nil
	}
	return m.ReloadCrossProjects()
}

// handleViewDidAppear is called when the cross-project outliner becomes the
// top view again (e.g., after returning from a drill-down). It triggers an
// eager reload from disk while preserving the current cursor position; the
// cursor is clamped to the new row count once the load completes.
func (m *OutlinerModel) handleViewDidAppear() tea.Cmd {
	if m.Project != nil {
		// Only applies in cross-project mode.
		return nil
	}
	// Becoming visible again means the surface that asked for a parked focus was
	// dismissed before its load landed. Abandon it and reload the base workspace.
	m.cancelParkedProjectFocus()
	if m.Population.Loading {
		return nil
	}
	savedCursor := m.Cursor
	cmd := m.ReloadCrossProjects()
	m.Cursor = savedCursor
	return cmd
}

// handleCrossProjectsLoaded processes a completed load. Stale results
// (generation mismatch) are discarded. Successful results populate
// the population and rebuild rows. A status bar warning is returned for any
// failed projects.
func (m *OutlinerModel) HandleCrossProjectsLoaded(msg CrossProjectsLoadedMsg) tea.Cmd {
	if msg.Generation != m.Population.LoadGen {
		return nil // Stale result from a superseded load; discard.
	}
	if msg.FocusProjectID != "" {
		if msg.FocusProjectID != m.Scope.PendingParkedID {
			// The request was abandoned while its read ran — a scope push, a lens, or
			// a relationship hop replaced the projection it was for. This population
			// was read with the focused project appended as an exception the live view
			// no longer holds, so publishing it would open a parked project's tasks in
			// a view nobody asked for and push a scope over wherever the user now is.
			// Nothing is published; the read is put down, and the capture it was
			// carrying goes with it, because a read that replaced no rows must not move
			// a cursor later.
			m.Population.settleLoad()
			m.Population.clearReloadCursor()
			return nil
		}
		m.Scope.clearPendingParked()
		if detail, failed := parkedFocusFailureDetail(msg, msg.FocusProjectID); failed {
			// The base snapshot stays exactly as it was: a focus that could not be
			// read must not disturb the workspace the user is still looking at.
			m.Population.settleLoad()
			m.Population.clearReloadCursor()
			return func() tea.Msg { return status.ErrorMsg{Err: errors.New(detail)} }
		}
	}
	fallbackCursor := m.Cursor
	fallbackScrollOffset := m.ViewScrollOffset
	m.DiscardDisplacedRelocationCapture()
	m.Population.settleLoad()
	// A reload a commit started usually answers with the very population the
	// commit's own rebuild already drew, since that rebuild read the projects the
	// commit reloaded through the same loader. Only a population the rows were not
	// built from forces a rebuild; the guard still compares the view either way.
	republished := m.Population.rebuiltFromSame(msg.Projects)
	m.setLoadedProjects(msg.Projects)
	// Both halves of this read's answer are recorded together: the projects it
	// opened and the projects it could not. The generation check above is what
	// makes them honest — a superseded read publishes neither half, so a failure it
	// reported can never withdraw a project an accepted read opened — and the
	// pairing is what keeps the record per project, since a read that asked for
	// neither half of a project says nothing about it.
	m.Population.recordLoadFailures(msg.Projects, msg.FailedLoads)
	m.Population.setSnapshotReady(true)
	m.recordLoadedPopulation(msg.LoadedProjectFilter)
	m.pruneSelectionsOutsideLoadedProjects()
	if !republished {
		m.Population.Invalidate()
	}
	m.CrossProjectRebuildRows()
	m.RestoreCrossOutlinerCursorAfterReload(fallbackCursor, fallbackScrollOffset)
	switch {
	case msg.FocusProjectID != "":
		// Push after the cursor is restored so the captured base frame holds the
		// position the user actually left, not a transient reload artifact.
		m.settleParkedFocusException(msg.FocusProjectID)
		m.PushProjectScope(msg.FocusProjectID)
	case msg.retainedFocusProjectID != "" && msg.retainedFocusProjectID == m.Scope.ParkedProjectID:
		// A reload that merely carried the adopted focus leaves it in place and
		// only re-decides the exception.
		m.settleParkedFocusException(msg.retainedFocusProjectID)
	}

	var cmds []tea.Cmd
	if jumpCmd := m.FinishPendingOutlinerRelocation(); jumpCmd != nil {
		cmds = append(cmds, jumpCmd)
	}
	// A scoped project may have been archived or removed externally
	// while the reload was in flight; unwind to the nearest valid scope.
	if scopeMsg, repaired := m.reconcileProjectScope(); repaired {
		cmds = append(cmds, func() tea.Msg { return status.InfoStatusMsg(scopeMsg) })
	}

	if advisory := m.LoadFailureAdvisory(msg.FailedLoads); advisory != nil {
		cmds = append(cmds, advisory)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// parkedFocusFailureDetail reports why an explicit focus request cannot be
// honored. Ordinary cross-project loads treat a per-project failure as a
// non-fatal diagnostic, but the focused project is the whole point of this
// load: if it did not parse, or was archived or removed before the load ran,
// the request is canceled rather than silently opening an empty scope.
func parkedFocusFailureDetail(msg CrossProjectsLoadedMsg, projectID string) (string, bool) {
	for _, failure := range msg.FailedLoads {
		if failure.ProjectID == projectID {
			return failure.Detail, true
		}
	}
	for _, project := range msg.Projects {
		if project != nil && project.ID == projectID {
			return "", false
		}
	}
	return "Could not open that project — it is no longer available", true
}

func (m *OutlinerModel) ReloadCrossProjects() tea.Cmd {
	m.Scope.clearParkedReload()
	if !m.Population.consumeReloadFromTop() {
		m.preserveCrossOutlinerCursorForReload()
	}
	return m.startCrossProjectLoad()
}

// cancelCrossProjectLoad settles an abandoned refresh and invalidates any
// result already in flight. The last accepted snapshot remains renderable.
func (m *OutlinerModel) CancelCrossProjectLoad() {
	if m == nil || m.Project != nil {
		return
	}
	// An abandoned refresh takes any pending lens application with it: both are
	// waiting on the same in-flight read.
	m.discardPendingLensApplication()
	if m.Population.Loading {
		m.Population.supersede()
	}
	m.Population.settleLoad()
	m.Population.clearReloadTarget()
}

// crossResolveProject returns the loaded project matching the given ID by
// scanning m.population.projects. Returns nil when not found.
func (m *OutlinerModel) crossResolveProject(projectID string) *models.Project {
	for _, p := range m.Population.Projects {
		if p.ID == projectID {
			return p
		}
	}
	return nil
}

// handleCrossProjectReparent applies a ParentSelectedMsg in cross-project mode
// through the single-task coordinator (app Reparent). The application layer owns
// the parent-existence, type-compatibility, and cycle guards and aborts an
// invalid reparent before any commit, so no undo snapshot is recorded for a
// rejected change. A nil ParentID promotes the task to a root.
func (m *OutlinerModel) handleCrossProjectReparent(msg listsurface.ParentSelectedMsg) (tea.Model, tea.Cmd) {
	return m, m.taskItem.Reparent(msg.ReparentTask, msg.ParentID)
}
