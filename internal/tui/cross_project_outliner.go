package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/status"
)

// crossProjectsLoadedMsg carries the result of an async cross-project load.
type crossProjectsLoadedMsg struct {
	projects    []*models.Project
	failedLoads []projectLoadFailure
	generation  int

	// focusProjectID is set only when this load was started to enter a parked
	// project focus. Only that load pushes a scope, and only it cancels outright
	// when the project cannot be read.
	focusProjectID string

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
	loadedProjectFilter string
}

// crossSpinnerDelay is how long an initial load runs before it animates a
// spinner, so a fast load never flashes one.
const crossSpinnerDelay = 200 * time.Millisecond

// crossSpinnerDelayMsg fires after crossSpinnerDelay to animate the initial-load
// spinner.
type crossSpinnerDelayMsg struct{}

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
		m.crossSpinner = chrome.NewSpinner()
	}
	// An ordinary reload reads the live project filter, which is not the candidate
	// filter a pending lens application is waiting on. That application can never
	// publish behind this load, so it is abandoned here rather than left to have
	// its result arrive against a model that has moved on.
	m.discardPendingLensApplication()
	m.crossLoading = true
	m.crossLoadGen++
	return tea.Batch(
		m.loadCrossProjectsCmd(),
		status.TickCmd(crossSpinnerDelay, func(time.Time) tea.Msg {
			return crossSpinnerDelayMsg{}
		}),
	)
}

// loadCrossProjectsCmd returns a Cmd that loads all active (context-filtered)
// projects from disk. Failed projects are skipped; their load errors are
// collected for a non-blocking status bar warning, matching the AllTasks
// continue-on-error pattern.
func (m *OutlinerModel) loadCrossProjectsCmd() tea.Cmd {
	gen := m.crossLoadGen
	runtime := m.runtime
	svc := m.projectService
	adopting := m.pendingParkedFocusID != ""
	focus := m.focusRequestForLoad()
	projectFilter := m.projectFilter.Clone()
	return func() tea.Msg {
		_, loaded, failed := LoadCrossScopeTasks(runtime, svc, CrossScopeOpts{
			ActiveOnly:     true,
			FocusProjectID: focus,
			ProjectFilter:  projectFilter,
		})
		msg := crossProjectsLoadedMsg{
			projects:            loaded,
			failedLoads:         failed,
			generation:          gen,
			loadedProjectFilter: query.ProjectFilterFingerprint(&projectFilter),
		}
		if adopting {
			msg.focusProjectID = focus
		} else {
			msg.retainedFocusProjectID = focus
		}
		return msg
	}
}

// focusRequestForLoad names the parked project the next load must admit. An
// in-flight request wins over the adopted one so a changed focus is not fought
// by the project it replaces; an adopted focus is re-requested by every later
// reload so an ordinary mutation refresh cannot drop the focused tree.
func (m *OutlinerModel) focusRequestForLoad() string {
	if m.pendingParkedFocusID != "" {
		return m.pendingParkedFocusID
	}
	return m.parkedFocusProjectID
}

// beginParkedProjectFocus starts the load that admits one parked project on top
// of the base workspace. The generation bump inside startCrossProjectLoad is
// what makes a repeated, changed, or abandoned request safe: only the newest
// request's result is ever accepted.
func (m *OutlinerModel) beginParkedProjectFocus(projectID string) tea.Cmd {
	if m == nil || m.project != nil || projectID == "" {
		return nil
	}
	m.pendingParkedFocusID = projectID
	return m.reloadCrossProjects()
}

// cancelParkedProjectFocus abandons an in-flight focus request. The base
// snapshot is untouched, so there is nothing to restore.
func (m *OutlinerModel) cancelParkedProjectFocus() {
	if m == nil || m.pendingParkedFocusID == "" {
		return
	}
	m.pendingParkedFocusID = ""
	m.cancelCrossProjectLoad()
}

// takeParkedFocusReload claims the reload owed after a parked focus was dropped
// synchronously. The Esc cascade drops the project from the live snapshot so no
// empty frame is rendered; this reload then re-reads the base workspace alone.
func (m *OutlinerModel) takeParkedFocusReload() tea.Cmd {
	if m == nil || !m.parkedFocusReloadPending {
		return nil
	}
	return m.reloadCrossProjects()
}

// handleViewDidAppear is called when the cross-project outliner becomes the
// top view again (e.g., after returning from a drill-down). It triggers an
// eager reload from disk while preserving the current cursor position; the
// cursor is clamped to the new row count once the load completes.
func (m *OutlinerModel) handleViewDidAppear() tea.Cmd {
	if m.project != nil {
		// Only applies in cross-project mode.
		return nil
	}
	// Becoming visible again means the surface that asked for a parked focus was
	// dismissed before its load landed. Abandon it and reload the base workspace.
	m.cancelParkedProjectFocus()
	if m.crossLoading {
		return nil
	}
	savedCursor := m.Cursor
	cmd := m.reloadCrossProjects()
	m.Cursor = savedCursor
	return cmd
}

// handleCrossProjectsLoaded processes a completed load. Stale results
// (generation mismatch) are discarded. Successful results populate
// loadedProjects and rebuild rows. A status bar warning is returned for any
// failed projects.
func (m *OutlinerModel) handleCrossProjectsLoaded(msg crossProjectsLoadedMsg) tea.Cmd {
	if msg.generation != m.crossLoadGen {
		return nil // Stale result from a superseded load; discard.
	}
	if msg.focusProjectID != "" {
		m.pendingParkedFocusID = ""
		if detail, failed := parkedFocusFailureDetail(msg, msg.focusProjectID); failed {
			// The base snapshot stays exactly as it was: a focus that could not be
			// read must not disturb the workspace the user is still looking at.
			m.crossLoading = false
			m.crossSpinnerVisible = false
			m.crossReloadProjectID = ""
			m.crossReloadTaskID = ""
			return func() tea.Msg { return status.ErrorMsg{Err: errors.New(detail)} }
		}
	}
	fallbackCursor := m.Cursor
	fallbackScrollOffset := m.ViewScrollOffset
	m.discardDisplacedRelocationCapture()
	m.crossLoading = false
	m.crossSpinnerVisible = false
	m.setLoadedProjects(msg.projects)
	m.crossSnapshotReady = true
	m.recordLoadedPopulation(msg.loadedProjectFilter)
	m.pruneSelectionsOutsideLoadedProjects()
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	m.restoreCrossOutlinerCursorAfterReload(fallbackCursor, fallbackScrollOffset)
	switch {
	case msg.focusProjectID != "":
		// Push after the cursor is restored so the captured base frame holds the
		// position the user actually left, not a transient reload artifact.
		m.settleParkedFocusException(msg.focusProjectID)
		m.pushProjectScope(msg.focusProjectID)
	case msg.retainedFocusProjectID != "" && msg.retainedFocusProjectID == m.parkedFocusProjectID:
		// A reload that merely carried the adopted focus leaves it in place and
		// only re-decides the exception.
		m.settleParkedFocusException(msg.retainedFocusProjectID)
	}

	var cmds []tea.Cmd
	if jumpCmd := m.finishPendingOutlinerRelocation(); jumpCmd != nil {
		cmds = append(cmds, jumpCmd)
	}
	// A scoped project may have been archived or removed externally
	// while the reload was in flight; unwind to the nearest valid scope.
	if scopeMsg, repaired := m.reconcileProjectScope(); repaired {
		cmds = append(cmds, func() tea.Msg { return status.InfoStatusMsg(scopeMsg) })
	}

	if advisory := m.loadFailureAdvisory(msg.failedLoads); advisory != nil {
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
func parkedFocusFailureDetail(msg crossProjectsLoadedMsg, projectID string) (string, bool) {
	for _, failure := range msg.failedLoads {
		if failure.ProjectID == projectID {
			return failure.Detail, true
		}
	}
	for _, project := range msg.projects {
		if project != nil && project.ID == projectID {
			return "", false
		}
	}
	return "Could not open that project — it is no longer available", true
}

// crossProjectRebuildRows aggregates tasks from loadedProjects through the
// canonical mode pipeline. Called by rebuildRows() when m.project == nil.
//
// A memoization guard short-circuits rebuilding when mode, search/filter state,
// focus stack depth, and total task count are all unchanged from
// the previous call. The guard is bypassed when crossForceRebuild is true (set
// after mutations) or when a fresh load completes.
func (m *OutlinerModel) crossProjectRebuildRows() {
	m.syncJumpProjection()
	// Compute current guard inputs.
	taskCount := 0
	for _, proj := range m.loadedProjects {
		taskCount += len(proj.Tasks)
	}
	currentFingerprint := m.crossProjectRowsFingerprint()
	currentFocusDepth := len(m.crossFocusStack)

	// Short-circuit when all guard inputs match the previous call and no force
	// rebuild has been requested. This prevents redundant rebuilds on no-op
	// events like window resize or view reactivation.
	if !m.crossForceRebuild &&
		currentFingerprint == m.crossLastRowsFingerprint &&
		currentFocusDepth == m.crossLastFocusDepth &&
		taskCount == m.crossLastTaskCount {
		return
	}

	// Update guard state and consume the force flag.
	m.crossLastRowsFingerprint = currentFingerprint
	m.crossLastFocusDepth = currentFocusDepth
	m.crossLastTaskCount = taskCount
	m.crossForceRebuild = false
	// The reference index is a reading of the population this pass is about to
	// walk, so it is dropped here and nowhere else on this path: every force
	// flag and every fresh load reaches this line, and a pass the guard
	// short-circuited changed nothing the index could have gone stale against.
	m.invalidateDependencyIndex()

	m.rows = m.rows[:0]
	m.validateCrossFocusStack()
	// Validation may have shortened the stack after the guard captured its old
	// depth. Record the repaired depth so the next render remains a no-op.
	m.crossLastFocusDepth = len(m.crossFocusStack)
	if len(m.loadedProjects) == 0 {
		m.pruneInvisibleSelections()
		m.clampCursor()
		m.reseatNoteCursorAfterRebuild()
		return
	}

	m.buildCanonicalModeRows()
	m.reseatNoteCursorAfterRebuild()
}

// reseatNoteCursorAfterRebuild consumes the note identity an accepted
// note-index refresh captured and puts the cursor back on that note in the
// rebuilt rows. It runs after the row pipeline has already clamped, so a note
// that left the refreshed projection keeps that deterministic numeric landing
// rather than being moved again. The viewport is untouched either way:
// computeOutlinerViewport is the single window authority and follows the cursor
// on the next render.
//
// A memoized no-op rebuild returns before this point, which is what keeps the
// capture alive until the rebuild the force flag guarantees actually runs.
func (m *OutlinerModel) reseatNoteCursorAfterRebuild() {
	identity := m.noteCursorReseat
	m.noteCursorReseat = NoteRowIdentity{}
	if identity.Filename == "" {
		return
	}
	for index, row := range m.rows {
		if row.IsNoteRow && row.NoteRow == identity {
			m.Cursor = index
			return
		}
	}
}

func (m *OutlinerModel) crossProjectRowsFingerprint() string {
	commonParts := query.CommonFilterParts(&m.filter.CommonFilterState)

	var b strings.Builder
	fmt.Fprintf(&b, "search=%q|", m.SearchQuery())
	fmt.Fprintf(&b, "search-projection=%t|", m.hasSearchProjection())
	// The same query yields lanes or one flat list, so the presentation toggle is
	// a membership input here exactly as the query itself is.
	fmt.Fprintf(&b, "ranked-list=%t|", m.showsRankedList())
	// The lens query decides membership without changing any other guard input,
	// so retyping it must be able to invalidate the memoized rows on its own.
	fmt.Fprintf(&b, "lens=%q|", m.lensQuery)
	fmt.Fprintf(&b, "sort=%d:%t|", m.sort.Field, m.sort.Ascending)
	fmt.Fprintf(&b, "common=%q|", strings.Join(commonParts, "\x1f"))
	fmt.Fprintf(&b, "project-filter=%q|", query.ProjectFilterFingerprint(&m.projectFilter))
	fmt.Fprintf(&b, "loaded=%s|", loadedProjectIDsFingerprint(m.loadedProjects))
	fmt.Fprintf(&b, "scope=%q|", m.scopedProjectID)
	fmt.Fprintf(&b, "mode=%s|", m.primaryMode)
	fmt.Fprintf(&b, "zoom=%q|", m.zoom.key)
	fmt.Fprintf(&b, "today-fold=%t|next-fold=%t|", m.collapsedSections[todaySectionCollapseKey], m.collapsedSections[nextSectionCollapseKey])
	b.WriteString("today-plan=")
	if m.runtime != nil {
		for _, item := range m.runtime.Planning().GetToday() {
			fmt.Fprintf(&b, "%s:%s,", item.ProjectID, item.TaskID)
		}
		b.WriteString("|next-plan=")
		for _, item := range m.runtime.Planning().GetNext() {
			fmt.Fprintf(&b, "%s:%s,", item.ProjectID, item.TaskID)
		}
	}
	return b.String()
}

func loadedProjectIDsFingerprint(projects []*models.Project) string {
	if len(projects) == 0 {
		return ""
	}
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			ids = append(ids, "<nil>")
			continue
		}
		ids = append(ids, project.ID)
	}
	return strings.Join(ids, "\x1f")
}

func (m *OutlinerModel) reloadCrossProjects() tea.Cmd {
	m.parkedFocusReloadPending = false
	if m.crossReloadFromTop {
		m.crossReloadProjectID = ""
		m.crossReloadTaskID = ""
		m.crossReloadFromTop = false
	} else {
		m.preserveCrossOutlinerCursorForReload()
	}
	return m.startCrossProjectLoad()
}

// preserveCrossOutlinerCursorForReload captures the selected task by project and
// task ID before reloadCrossProjects starts replacing the snapshot. If a
// superseding reload starts after the task has already disappeared, keep the
// existing target for the newest accepted load instead of losing the user's
// selection.
func (m *OutlinerModel) cancelCrossProjectLoad() {
	if m == nil || m.project != nil {
		return
	}
	// An abandoned refresh takes any pending lens application with it: both are
	// waiting on the same in-flight read.
	m.discardPendingLensApplication()
	if m.crossLoading {
		m.crossLoadGen++
	}
	m.crossLoading = false
	m.crossSpinnerVisible = false
	m.crossReloadProjectID = ""
	m.crossReloadTaskID = ""
	m.crossReloadHeaderProjectID = ""
	m.crossReloadFromTop = false
}

// parkedProjectCount counts the non-archived projects the parked tag keeps out
// of the task workspace.
func (m *OutlinerModel) parkedProjectCount() int {
	if m.runtime == nil {
		return 0
	}
	parked := 0
	for _, project := range m.runtime.AvailableProjects() {
		if project != nil && models.ProjectTagsParked(project.Tags) {
			parked++
		}
	}
	return parked
}

// baseScopeChromeLabel names the unscoped population the Outliner is actually
// reading: the base label, the active F facets, and the count of parked projects
// still hidden. The count is dropped once the facets admit parked projects,
// because nothing is being hidden to report.
func (m *OutlinerModel) baseScopeChromeLabel() string {
	parts := query.ProjectFilterParts(&m.projectFilter)
	if !m.projectFilter.ShowsParked() {
		if parked := m.parkedProjectCount(); parked > 0 {
			parts = append(parts, fmt.Sprintf("%d parked", parked))
		}
	}
	if len(parts) == 0 {
		return activeProjectsScopeLabel
	}
	return activeProjectsScopeLabel + scopeChromeSeparator + strings.Join(parts, scopeChromeSeparator)
}

// crossStatusBarLabel returns the status bar label for the global Outliner,
// formatted as 'Outliner · [scope-name] · N tasks'.
// N counts only task rows, excluding section headers, project sub-headers,
// and separators.
func (m *OutlinerModel) crossStatusBarLabel() string {
	scopeName := m.baseScopeChromeLabel()
	if m.isProjectScoped() {
		// Under a project scope the label names the scoped project chain.
		scopeName = strings.Join(m.scopeChainNames(), scopeChainSeparator)
	}
	taskCount := 0
	for _, row := range m.rows {
		if row.isTaskRow() {
			taskCount++
		}
	}
	label := "Outliner"
	if m.primaryMode != chrome.ModeAll {
		label += ": " + m.activeOutlinerModeEntry().Label
	}
	return fmt.Sprintf("%s · %s · %d tasks", label, scopeName, taskCount)
}
