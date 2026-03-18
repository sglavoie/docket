package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
)

// crossProjectsLoadedMsg carries the result of an async cross-project load.
type crossProjectsLoadedMsg struct {
	projects    []*models.Project
	failedNames []string
	generation  int
}

// crossSpinnerDelayMsg fires after 200ms to reveal the loading spinner.
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

// startCrossProjectLoad sets loading state and returns a Cmd that begins
// async loading of all context-filtered projects and a 200ms spinner delay.
func (m *OutlinerModel) startCrossProjectLoad() tea.Cmd {
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
// projects from disk. Failed projects are skipped; their names are collected
// for a non-blocking status bar warning, matching the AllTasks continue-on-error
// pattern.
func (m *OutlinerModel) loadCrossProjectsCmd() tea.Cmd {
	gen := m.crossLoadGen
	cfg := m.config
	svc := m.projectService
	return func() tea.Msg {
		projects := cfg.GetActiveProjects()
		var loaded []*models.Project
		var failed []string
		for _, p := range projects {
			if err := svc.Load(p); err != nil {
				failed = append(failed, p.Name)
				continue
			}
			loaded = append(loaded, p)
		}
		return crossProjectsLoadedMsg{
			projects:    loaded,
			failedNames: failed,
			generation:  gen,
		}
	}
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
	m.crossLoading = false
	m.crossSpinnerVisible = false
	m.loadedProjects = msg.projects
	m.crossForceRebuild = true
	m.crossProjectRebuildRows()

	if len(msg.failedNames) == 0 {
		return nil
	}
	warning := "Cross-project outliner: failed to load: " + strings.Join(msg.failedNames, ", ")
	return func() tea.Msg { return StatusMsg(warning) }
}

// crossProjectRebuildRows aggregates tasks from all loadedProjects into m.rows,
// applying the active perspective filter and, for the All perspective, grouping
// by GTD section with collapsible headers. Called by rebuildRows() when
// m.project == nil.
//
// A memoization guard short-circuits rebuilding when perspective, search
// filter string, zoom stack depth, and total task count are all unchanged from
// the previous call. The guard is bypassed when crossForceRebuild is true (set
// after mutations) or when a fresh load completes.
func (m *OutlinerModel) crossProjectRebuildRows() {
	// Compute current guard inputs.
	taskCount := 0
	for _, proj := range m.loadedProjects {
		taskCount += len(proj.Tasks)
	}
	currentPerspective := m.activePerspective
	currentFilter := m.SearchQuery()
	currentZoomDepth := len(m.crossZoomStack)

	// Short-circuit when all guard inputs match the previous call and no force
	// rebuild has been requested. This prevents redundant rebuilds on no-op
	// events like window resize or view reactivation.
	if !m.crossForceRebuild &&
		currentPerspective == m.crossLastPerspective &&
		currentFilter == m.crossLastFilterStr &&
		currentZoomDepth == m.crossLastZoomDepth &&
		taskCount == m.crossLastTaskCount {
		return
	}

	// Update guard state and consume the force flag.
	m.crossLastPerspective = currentPerspective
	m.crossLastFilterStr = currentFilter
	m.crossLastZoomDepth = currentZoomDepth
	m.crossLastTaskCount = taskCount
	m.crossForceRebuild = false

	m.rows = m.rows[:0]
	if len(m.loadedProjects) == 0 {
		m.clampCursor()
		return
	}

	m.validateCrossZoomStack()

	p := m.currentPerspective()

	// Collect per-project task groups, applying per-project perspective filter.
	// Grouping preserves project identity so ProjectID can be set on task rows.
	var groups []projectTasksGroup
	for _, proj := range m.loadedProjects {
		roots := proj.GetRootTasks()

		// Scope to zoomed task's children for cross-project zoom.
		if len(m.crossZoomStack) > 0 {
			top := m.crossZoomStack[len(m.crossZoomStack)-1]
			if top.ProjectID == proj.ID {
				if t := proj.GetTaskByID(top.TaskID); t != nil {
					roots = t.Children
				}
			} else {
				// Skip projects that are not the zoomed project when zoomed.
				continue
			}
		}

		tasks := filterTaskTree(roots, p.Filter)

		if q := m.SearchQuery(); q != "" {
			tasks = filterTaskTree(tasks, func(t *models.Task) bool {
				return matchesSearch(t, q)
			})
		}

		if len(tasks) == 0 {
			continue
		}
		groups = append(groups, projectTasksGroup{projectID: proj.ID, tasks: tasks})
	}

	if p.Name == "All" {
		m.buildCrossProjectRowsWithSectionHeaders(groups)
	} else {
		m.buildProjectGroupedRows(groups)
	}
	m.clampCursor()
}

// crossProjectView renders the cross-project outliner. It shows a centered
// loading spinner while projects are being loaded, then delegates to
// renderCrossProjectContent once loading is complete.
func (m *OutlinerModel) crossProjectView() string {
	if m.crossSpinnerVisible {
		return m.renderCrossSpinner()
	}
	if m.crossLoading {
		// Load started but 200ms hasn't elapsed yet — render empty frame.
		return ""
	}
	return m.renderCrossProjectContent()
}

// renderCrossSpinner renders a centered loading spinner using the Dot style.
func (m *OutlinerModel) renderCrossSpinner() string {
	spinnerView := m.crossSpinner.View()
	label := spinnerView + " Loading projects…"
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
	return m.renderOutlinerTreeContent(m.width)
}

// reloadCrossProjects triggers a new load, incrementing the generation counter
// so any in-flight stale result is discarded on arrival.
func (m *OutlinerModel) reloadCrossProjects() tea.Cmd {
	m.loadedProjects = nil
	m.rows = m.rows[:0]
	return m.startCrossProjectLoad()
}

// crossMergedTaskSlice returns a single flat slice of all tasks from every
// loaded project. Callers use this to call BuildTagList once on the union of
// all visible projects' tasks rather than merging per-project results.
func (m *OutlinerModel) crossMergedTaskSlice() []*models.Task {
	var total int
	for _, p := range m.loadedProjects {
		total += len(p.Tasks)
	}
	merged := make([]*models.Task, 0, total)
	for _, p := range m.loadedProjects {
		merged = append(merged, p.Tasks...)
	}
	return merged
}

// crossStatusBarLabel returns the status bar label for cross-project mode,
// formatted as 'Cross-Outliner · [context-name] · N tasks'.
// N counts only task rows, excluding section headers and separators.
func (m *OutlinerModel) crossStatusBarLabel() string {
	ctxName := "All"
	if m.config != nil {
		if m.config.FilterCustomContext != "" {
			ctxName = "+" + m.config.FilterCustomContext
		} else if m.config.FilterContext != "" {
			ctxName = m.config.FilterContext
		}
	}
	taskCount := 0
	for _, row := range m.rows {
		if !row.IsSeparator && !row.IsSectionHeader {
			taskCount++
		}
	}
	return fmt.Sprintf("Cross-Outliner · %s · %d tasks", ctxName, taskCount)
}

// Ensure spinner package is used (crossSpinner field type is SpinnerModel which
// wraps bubbles/spinner; the Dot spinner style is selected in NewSpinner).
var _ = spinner.Dot
