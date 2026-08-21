package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
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
		m.crossSpinner = NewSpinner()
	}
	m.crossLoading = true
	m.crossLoadGen++
	return tea.Batch(
		m.loadCrossProjectsCmd(),
		tickCmd(crossSpinnerDelay, func(time.Time) tea.Msg {
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
	projectFilter := m.projectFilter.clone()
	return func() tea.Msg {
		_, loaded, failed := LoadCrossScopeTasks(runtime, svc, CrossScopeOpts{
			ActiveOnly:     true,
			FocusProjectID: focus,
			ProjectFilter:  projectFilter,
		})
		msg := crossProjectsLoadedMsg{
			projects:    loaded,
			failedLoads: failed,
			generation:  gen,
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
	savedCursor := m.cursor
	cmd := m.reloadCrossProjects()
	m.cursor = savedCursor
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
			return func() tea.Msg { return ErrorMsg{Err: errors.New(detail)} }
		}
	}
	fallbackCursor := m.cursor
	fallbackScrollOffset := m.viewScrollOffset
	// A user may keep navigating while the committed mutation's disk refresh is
	// in flight. Do not turn that later snapshot into a relocation jump unless
	// they are still following the task that caused it.
	if pending := m.relocationHistory.pending; pending != nil && m.currentTaskIdentity() != pending.Task {
		m.relocationHistory.pending = nil
	}
	m.crossLoading = false
	m.crossSpinnerVisible = false
	m.loadedProjects = msg.projects
	m.crossSnapshotReady = true
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
		cmds = append(cmds, func() tea.Msg { return InfoStatusMsg(scopeMsg) })
	}

	if len(msg.failedLoads) == 0 {
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	}
	count := len(msg.failedLoads)
	noun := "project"
	if count > 1 {
		noun = "projects"
	}
	warning := fmt.Sprintf("%d %s failed to load: %s", count, noun, msg.failedLoads[0].Detail)
	cmds = append(cmds, func() tea.Msg { return InfoStatusMsg(warning) })
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

	m.rows = m.rows[:0]
	m.validateCrossFocusStack()
	// Validation may have shortened the stack after the guard captured its old
	// depth. Record the repaired depth so the next render remains a no-op.
	m.crossLastFocusDepth = len(m.crossFocusStack)
	if len(m.loadedProjects) == 0 {
		m.pruneInvisibleSelections()
		m.clampCursor()
		return
	}

	m.buildCanonicalModeRows()
}

func (m *OutlinerModel) crossProjectRowsFingerprint() string {
	commonParts := commonFilterParts(&m.filter.CommonFilterState)

	var b strings.Builder
	fmt.Fprintf(&b, "search=%q|", m.SearchQuery())
	fmt.Fprintf(&b, "search-projection=%t|", m.hasSearchProjection())
	fmt.Fprintf(&b, "sort=%d:%t|", m.sort.Field, m.sort.Ascending)
	fmt.Fprintf(&b, "common=%q|", strings.Join(commonParts, "\x1f"))
	fmt.Fprintf(&b, "project-filter=%q|", projectFilterFingerprint(&m.projectFilter))
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

// crossProjectView renders the cross-project outliner. A cold load shows a
// centered loading state. Later reloads retain the last accepted snapshot until
// its replacement is ready, so rendering never passes through an empty frame.
func (m *OutlinerModel) crossProjectView() string {
	if m.crossLoading && !m.hasCrossProjectSnapshot() {
		return m.renderCrossLoadingState()
	}
	return m.renderCrossProjectContent()
}

// hasCrossProjectSnapshot reports whether there is stable content to render
// while a refresh is in flight. The loadedProjects fallback keeps older tests
// and callers that install an in-memory snapshot directly compatible.
func (m *OutlinerModel) hasCrossProjectSnapshot() bool {
	return m.crossSnapshotReady || len(m.loadedProjects) > 0
}

// renderCrossLoadingState renders a non-empty cold-load frame immediately,
// adding the animated Dot spinner only after its short anti-flicker delay.
func (m *OutlinerModel) renderCrossLoadingState() string {
	label := "Loading projects…"
	if m.crossSpinnerVisible {
		label = m.crossSpinner.View() + " " + label
	}
	centered := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(label)
	return centered
}

// renderCrossProjectContent renders the aggregated cross-project outliner tree.
// It delegates to the unified renderOutlinerTreeContent after rebuilding rows.
func (m *OutlinerModel) renderCrossProjectContent() string {
	m.crossProjectRebuildRows()
	return m.renderFullWidth()
}

// reloadCrossProjects triggers a new load, incrementing the generation counter
// so any in-flight stale result is discarded on arrival.
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
func (m *OutlinerModel) preserveCrossOutlinerCursorForReload() {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return
	}
	row := m.rows[m.cursor]
	if !row.isTaskRow() || row.task == nil || row.ProjectID == "" || row.task.ID == "" {
		return
	}
	m.crossReloadProjectID = row.ProjectID
	m.crossReloadTaskID = row.task.ID
}

// restoreCrossOutlinerCursorAfterReload consumes the captured cursor identity
// after the accepted load rebuilds rows. If the task disappeared or is hidden,
// focus falls back to the nearest task around the old numeric row, preferring a
// following task at equal distance. When no tasks remain, the ordinary navigable
// row clamp still permits an empty section or project header to receive focus.
func (m *OutlinerModel) restoreCrossOutlinerCursorAfterReload(fallbackCursor, fallbackScrollOffset int) {
	projectID, taskID := m.crossReloadProjectID, m.crossReloadTaskID
	headerProjectID := m.crossReloadHeaderProjectID
	m.crossReloadProjectID = ""
	m.crossReloadTaskID = ""
	m.crossReloadHeaderProjectID = ""
	m.viewScrollOffset = fallbackScrollOffset
	if headerProjectID != "" {
		if !m.moveCursorToProjectHeader(headerProjectID) {
			// The successor went away too, so the top of the projection — a section
			// header — is the only landing spot left that still means something.
			m.cursor = 0
			m.clampCursor()
		}
		return
	}
	if projectID == "" || taskID == "" {
		m.cursor = fallbackCursor
		m.clampCursor()
		return
	}
	if m.moveCursorToProjectTask(taskID, projectID) {
		return
	}
	m.moveCursorToNearestTask(fallbackCursor)
}

// projectHeaderAnchorAfterRemoval names the project sub-header the cursor should
// hold once the named projects' rows leave: the first surviving header below the
// topmost departing one, else the closest surviving header above it. With no
// survivor it returns a departing project's own ID, which stops resolving after
// the reload and therefore lands the cursor on the owning section header rather
// than an arbitrary row. The second result is false when none of the named
// projects has a header on screen at all.
func (m *OutlinerModel) projectHeaderAnchorAfterRemoval(projectIDs ...string) (string, bool) {
	leaving := make(map[string]bool, len(projectIDs))
	for _, projectID := range projectIDs {
		if projectID != "" {
			leaving[projectID] = true
		}
	}
	index := -1
	for candidate, row := range m.rows {
		if row.IsProjectHeader && leaving[row.ProjectID] {
			index = candidate
			break
		}
	}
	if index < 0 {
		return "", false
	}
	for _, row := range m.rows[index+1:] {
		if row.IsProjectHeader && row.ProjectID != "" && !leaving[row.ProjectID] {
			return row.ProjectID, true
		}
	}
	for candidate := index - 1; candidate >= 0; candidate-- {
		row := m.rows[candidate]
		if row.IsProjectHeader && row.ProjectID != "" && !leaving[row.ProjectID] {
			return row.ProjectID, true
		}
	}
	return m.rows[index].ProjectID, true
}

func (m *OutlinerModel) moveCursorToProjectHeader(projectID string) bool {
	for index, row := range m.rows {
		if row.IsProjectHeader && row.ProjectID == projectID {
			m.cursor = index
			m.clampCursor()
			return true
		}
	}
	return false
}

func (m *OutlinerModel) moveCursorToNearestTask(origin int) {
	if len(m.rows) == 0 {
		m.cursor = 0
		return
	}
	if origin < 0 {
		origin = 0
	}
	if origin >= len(m.rows) {
		origin = len(m.rows) - 1
	}
	for distance := 0; distance < len(m.rows); distance++ {
		next := origin + distance
		if next < len(m.rows) && m.rows[next].isTaskRow() && m.rows[next].task != nil {
			m.cursor = next
			return
		}
		previous := origin - distance
		if distance > 0 && previous >= 0 && m.rows[previous].isTaskRow() && m.rows[previous].task != nil {
			m.cursor = previous
			return
		}
	}
	m.cursor = origin
	m.clampCursor()
}

// cancelCrossProjectLoad settles an abandoned refresh and invalidates any
// result already in flight. The last accepted snapshot remains renderable.
func (m *OutlinerModel) cancelCrossProjectLoad() {
	if m == nil || m.project != nil {
		return
	}
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
	parts := projectFilterParts(&m.projectFilter)
	if !m.projectFilter.showsParked() {
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
	if m.primaryMode != ModeAll {
		label += ": " + m.activeOutlinerModeEntry().Label
	}
	return fmt.Sprintf("%s · %s · %d tasks", label, scopeName, taskCount)
}
