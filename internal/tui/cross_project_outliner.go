package tui

import (
	"fmt"
	"sort"
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
// projects from disk. Failed projects are skipped; their load errors are
// collected for a non-blocking status bar warning plus message-log details,
// matching the AllTasks continue-on-error pattern.
func (m *OutlinerModel) loadCrossProjectsCmd() tea.Cmd {
	gen := m.crossLoadGen
	cfg := m.config
	svc := m.projectService
	return func() tea.Msg {
		cycleOnly := cfg.CrossOutlinerCycleOnly != nil && *cfg.CrossOutlinerCycleOnly
		var ctxKey string
		if cycleOnly {
			ctxKey = cfg.FilterContext
			if cfg.FilterCustomContext != "" {
				ctxKey = "+" + cfg.FilterCustomContext
			}
		}
		_, loaded, failed := LoadCrossScopeTasks(cfg, svc, CrossScopeOpts{
			ActiveOnly:        true,
			CycleProjectsOnly: cycleOnly,
			ContextKey:        ctxKey,
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
	m.resetViewScrollOffset()
	m.crossProjectRebuildRows()

	if len(msg.failedLoads) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, failure := range msg.failedLoads {
		f := failure // capture for closure
		cmds = append(cmds, func() tea.Msg {
			return AppendToMessageLogMsg{
				Message: "Cross-project outliner: " + f.Detail,
				Level:   MessageLogLevelError,
			}
		})
	}
	count := len(msg.failedLoads)
	noun := "project"
	if count > 1 {
		noun = "projects"
	}
	warning := fmt.Sprintf("%d %s failed to load — see message log (Alt+L)", count, noun)
	cmds = append(cmds, func() tea.Msg { return InfoStatusMsg(warning) })
	return tea.Batch(cmds...)
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
//
//nolint:gocyclo // cross-project row rebuild has inherent branching
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
		m.pruneInvisibleSelections()
		m.clampCursor()
		return
	}

	m.validateCrossZoomStack()

	p := m.currentPerspective()

	// Collect per-project task groups, applying per-project perspective filter.
	// Grouping preserves project identity so ProjectID can be set on task rows.
	var groups []projectTasksGroup
	for _, proj := range m.loadedProjects {
		if len(m.filterProjects) > 0 && !m.filterProjects[proj.ID] {
			continue
		}

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

		filter := p.Filter
		if sf := crossProjectSectionFilter(p); sf != nil {
			filter = sf
		}
		tasks := filterTaskTree(roots, filter)

		if q := m.SearchQuery(); q != "" {
			tasks = filterTaskTree(tasks, func(t *models.Task) bool {
				return matchesSearch(t, q)
			})
		}

		if hasActiveCommonFilters(&m.filter.CommonFilterState) {
			tasks = filterTaskTree(tasks, func(t *models.Task) bool {
				return passesCommonFilters(t, &m.filter.CommonFilterState, proj.ID, m.config)
			})
		}

		if m.activeCriteriaFilter != nil {
			tasks = filterTaskTree(tasks, func(t *models.Task) bool {
				return passesSavedFilterCriteria(t, proj, m.activeCriteriaFilter, m.config)
			})
		}

		if len(m.filter.Sections) > 0 {
			tasks = filterTaskTree(tasks, func(t *models.Task) bool {
				return m.filter.Sections[t.Section] || m.filter.Sections[t.CustomSection]
			})
		}

		if m.filter.Actionable {
			tasks = filterTaskTree(tasks, func(t *models.Task) bool {
				if proj.HasIncompleteDependencies(t) {
					return false
				}
				if t.Status == models.StatusBlocked {
					return false
				}
				if t.Section == models.SectionInbox || t.Section == models.SectionNext ||
					t.Section == models.SectionWaiting || t.Section == models.SectionSomeday ||
					t.Section == models.SectionReview || t.Section == models.SectionStandby {
					return false
				}
				return true
			})
		}

		if len(tasks) == 0 {
			continue
		}
		groups = append(groups, projectTasksGroup{projectID: proj.ID, tasks: tasks})
	}

	if m.sort.Field != SortByDefault {
		m.sortCrossProjectGroups(groups)
	}

	if p.Name == "All" {
		m.buildCrossProjectRowsWithSectionHeaders(groups)
	} else {
		m.buildProjectGroupedRows(groups)
	}
	m.pruneInvisibleSelections()
	m.clampCursor()
}

// sortCrossProjectGroups sorts each project group's tasks in place using the
// current sort state. Each group is sorted independently using the group's
// project ID for score resolution.
func (m *OutlinerModel) sortCrossProjectGroups(groups []projectTasksGroup) {
	for i := range groups {
		groups[i].tasks = m.sortTaskTreeForProject(groups[i].tasks, groups[i].projectID)
	}
}

// sortTaskTreeForProject returns a sorted copy of the task tree using the
// given projectID for cross-project score comparison. Like sortTaskTree, it
// recurses into children to sort at every hierarchy level.
func (m *OutlinerModel) sortTaskTreeForProject(tasks []*models.Task, projectID string) []*models.Task {
	sorted := make([]*models.Task, len(tasks))
	copy(sorted, tasks)
	sort.SliceStable(sorted, func(i, j int) bool {
		cmp := CompareTasksCrossProject(sorted[i], sorted[j], m.sort.Field, projectID, projectID, m.config)
		return ApplySortDirection(cmp, m.sort.Ascending)
	})
	for idx, task := range sorted {
		if len(task.Children) > 0 {
			clone := *task
			clone.Children = m.sortTaskTreeForProject(task.Children, projectID)
			sorted[idx] = &clone
		}
	}
	return sorted
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
	m.resetViewScrollOffset()
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

// crossProjectSectionFilter returns a Section-based filter for perspectives
// that map to GTD sections in cross-project mode. Returns nil for perspectives
// that should use their computed Filter (All, Snoozed, Completed).
func crossProjectSectionFilter(p Perspective) func(t *models.Task) bool {
	switch p.Name {
	case "All", "Snoozed", "Completed":
		return nil
	case "Active":
		return func(t *models.Task) bool {
			return t.InSection("") && !t.Status.IsComplete() && !t.IsSnoozed()
		}
	default:
		section := p.Name
		return func(t *models.Task) bool {
			return t.InSection(section) && !t.Status.IsComplete() && !t.IsSnoozed()
		}
	}
}
