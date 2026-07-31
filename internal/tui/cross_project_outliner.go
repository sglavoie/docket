package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// crossProjectsLoadedMsg carries the result of an async cross-project load.
type crossProjectsLoadedMsg struct {
	projects    []*models.Project
	failedLoads []projectLoadFailure
	generation  int
}

// crossSpinnerDelayMsg fires after 200ms to animate the initial-load spinner.
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
		func() tea.Msg {
			time.Sleep(200 * time.Millisecond)
			return crossSpinnerDelayMsg{}
		},
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
	return func() tea.Msg {
		_, loaded, failed := LoadCrossScopeTasks(runtime, svc, CrossScopeOpts{
			ActiveOnly: true,
		})
		return crossProjectsLoadedMsg{
			projects:    loaded,
			failedLoads: failed,
			generation:  gen,
		}
	}
}

// handleViewDidAppear is called when the cross-project outliner becomes the
// top view again (e.g., after returning from a drill-down). It triggers an
// eager reload from disk while preserving the current cursor position; the
// cursor is clamped to the new row count once the load completes.
func (m *OutlinerModel) handleViewDidAppear() tea.Cmd {
	if m.project != nil || m.crossLoading {
		// Only applies in cross-project mode.
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
	fallbackCursor := m.cursor
	fallbackScrollOffset := m.viewScrollOffset
	pinnedTagTabName, hasPinnedTagTab := m.consumeCrossOutlinerReloadPinnedTagTab()
	if !hasPinnedTagTab {
		pinnedTagTabName, _ = m.crossProjectPinnedTagTabBeforeLoad()
	}
	m.crossLoading = false
	m.crossSpinnerVisible = false
	m.loadedProjects = msg.projects
	m.crossSnapshotReady = true
	m.workspaceResolver = app.NewWorkspaceResolver(app.NewSnapshot(msg.projects, nil, nil))
	m.refreshContextScopeProjects()
	m.restoreCrossOutlinerPinnedTagTab(pinnedTagTabName)
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()
	m.restoreCrossOutlinerCursorAfterReload(fallbackCursor, fallbackScrollOffset)

	var cmds []tea.Cmd
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

// crossProjectRebuildRows aggregates tasks from loadedProjects through the
// canonical mode pipeline. Called by rebuildRows() when m.project == nil.
//
// A memoization guard short-circuits rebuilding when mode, search/filter state,
// zoom stack depth, and total task count are all unchanged from
// the previous call. The guard is bypassed when crossForceRebuild is true (set
// after mutations) or when a fresh load completes.
func (m *OutlinerModel) crossProjectRebuildRows() {
	// Compute current guard inputs.
	taskCount := 0
	for _, proj := range m.loadedProjects {
		taskCount += len(proj.Tasks)
	}
	currentFingerprint := m.crossProjectRowsFingerprint()
	currentZoomDepth := len(m.crossZoomStack)

	// Short-circuit when all guard inputs match the previous call and no force
	// rebuild has been requested. This prevents redundant rebuilds on no-op
	// events like window resize or view reactivation.
	if !m.crossForceRebuild &&
		currentFingerprint == m.crossLastRowsFingerprint &&
		currentZoomDepth == m.crossLastZoomDepth &&
		taskCount == m.crossLastTaskCount {
		return
	}

	// Update guard state and consume the force flag.
	m.crossLastRowsFingerprint = currentFingerprint
	m.crossLastZoomDepth = currentZoomDepth
	m.crossLastTaskCount = taskCount
	m.crossForceRebuild = false

	m.rows = m.rows[:0]
	if len(m.loadedProjects) == 0 {
		m.pruneInvisibleSelections()
		m.clampCursor()
		return
	}

	m.validateCrossZoomStack()

	m.buildCanonicalModeRows()
}

func (m *OutlinerModel) crossProjectRowsFingerprint() string {
	tabs := m.pinnedTagTabs()
	commonParts := commonFilterParts(&m.filter.CommonFilterState)

	var b strings.Builder
	fmt.Fprintf(&b, "search=%q|", m.SearchQuery())
	fmt.Fprintf(&b, "tab=%d:%q|", m.pinnedTagTabIndex(), m.activePinnedTagFilter())
	fmt.Fprintf(&b, "tabs=%q|", strings.Join(tabs, "\x1f"))
	fmt.Fprintf(&b, "sort=%d:%t|", m.sort.Field, m.sort.Ascending)
	fmt.Fprintf(&b, "common=%q|", strings.Join(commonParts, "\x1f"))
	fmt.Fprintf(&b, "context=%d:%q|", m.activeContext.Kind, m.activeContext.Name)
	fmt.Fprintf(&b, "loaded=%s|", loadedProjectIDsFingerprint(m.loadedProjects))
	fmt.Fprintf(&b, "scope=%q|", m.scopedProjectID)
	fmt.Fprintf(&b, "mode=%s|", m.primaryMode)
	fmt.Fprintf(&b, "today-fold=%t|", m.collapsedSections[todaySectionCollapseKey])
	b.WriteString("today-plan=")
	if m.runtime != nil {
		for _, item := range m.runtime.Planning().GetToday() {
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
	m.preserveCrossOutlinerPinnedTagTabForReload()
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
	m.crossReloadProjectID = ""
	m.crossReloadTaskID = ""
	m.viewScrollOffset = fallbackScrollOffset
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

func (m *OutlinerModel) moveCursorToNearestTask(origin int) bool {
	if len(m.rows) == 0 {
		m.cursor = 0
		return false
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
			return true
		}
		previous := origin - distance
		if distance > 0 && previous >= 0 && m.rows[previous].isTaskRow() && m.rows[previous].task != nil {
			m.cursor = previous
			return true
		}
	}
	m.cursor = origin
	m.clampCursor()
	return false
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
	m.crossReloadTabName = ""
	m.crossReloadTabSet = false
	m.crossReloadProjectID = ""
	m.crossReloadTaskID = ""
	m.crossReloadFromTop = false
}

// crossStatusBarLabel returns the status bar label for the global Outliner,
// formatted as 'Outliner · [context-name] · N tasks'.
// N counts only task rows, excluding section headers, project sub-headers,
// and separators.
func (m *OutlinerModel) crossStatusBarLabel() string {
	ctxName := m.contextScopeLabel()
	if m.isProjectScoped() {
		// Under a project scope the label names the scoped project (chain) rather
		// than the active context.
		ctxName = strings.Join(m.scopeChainNames(), scopeChainSeparator)
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
	return fmt.Sprintf("%s · %s · %d tasks", label, ctxName, taskCount)
}
