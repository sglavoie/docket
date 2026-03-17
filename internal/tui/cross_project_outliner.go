package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/fuzzy"
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

		// Apply search filter against task titles only (per req body). This
		// intentionally excludes tags, description, and comments — unlike
		// TaskMatchesSearch which matches all fields. The "consistent with
		// single-project" AC (ac-5d7b136df1a59124f0004eea4f72aa31) is
		// superseded by the explicit title-only wording in the requirement.
		if q := m.SearchQuery(); q != "" {
			tasks = filterTaskTree(tasks, func(t *models.Task) bool {
				return fuzzy.MatchTerms(t.Title, q)
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
func (m *OutlinerModel) renderCrossProjectContent() string {
	m.crossProjectRebuildRows()

	perspective := m.currentPerspective()
	presets := PerspectivePresets()
	perspCount := len(presets)
	prevIdx := (m.activePerspective + perspCount - 1) % perspCount
	nextIdx := (m.activePerspective + 1) % perspCount

	currentStyle := GetViewModeStyleForSection(perspective.Name)
	prevName := presets[prevIdx].Name
	nextName := presets[nextIdx].Name

	perspectiveLabel := "  " +
		MutedStyle.Render("‹ "+prevName) + " " +
		currentStyle.Render("@ "+perspective.Name) + " " +
		MutedStyle.Render(nextName+" ›")

	projectCount := len(m.loadedProjects)
	header := fmt.Sprintf("Cross-project outliner (%d project", projectCount)
	if projectCount != 1 {
		header += "s"
	}
	header += ")"

	var b strings.Builder
	b.WriteString(header)
	b.WriteString(perspectiveLabel)

	searchIndicator := ""
	if m.IsSearchMode() {
		searchIndicator = "  " + MutedStyle.Render("Search: ") + m.SearchInputView()
	} else if m.SearchQuery() != "" {
		searchIndicator = "  " + MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery()))
	}
	b.WriteString(searchIndicator)
	b.WriteString("\n")

	bc := m.crossProjectBreadcrumb(m.width)
	if bc != "" {
		b.WriteString(bc)
		b.WriteString("\n")
	}

	if len(m.rows) == 0 {
		if len(m.loadedProjects) == 0 {
			b.WriteString(MutedStyle.Render("No projects in current context. Press C to switch context."))
		} else {
			b.WriteString(MutedStyle.Render("No tasks found."))
		}
		return b.String()
	}

	headerLines := 1 // title/perspective line
	if bc != "" {
		headerLines++ // breadcrumb line
	}
	visibleRows := max(m.height-headerLines, 1)
	startIdx := calcScrollOffset(m.cursor, visibleRows)
	endIdx := min(startIdx+visibleRows, len(m.rows))

	for i := startIdx; i < endIdx; i++ {
		row := m.rows[i]
		selected := i == m.cursor

		if row.IsSeparator {
			b.WriteString("")
		} else if row.IsSectionHeader {
			displayName := row.SectionName
			if displayName == "" {
				displayName = "Active"
			}
			sectionStyle := GetViewModeStyleForSection(row.SectionName)
			indicator := "▾"
			if m.collapsedSections[row.effectiveCollapseKey()] {
				indicator = "▸"
			}
			cursorPrefix := "  "
			if selected {
				cursorPrefix = SelectionCursorStyle.Render(">") + " "
			}
			badge := sectionStyle.Render(fmt.Sprintf("@ %s", displayName))
			projectLabel := ""
			if row.ProjectID != "" {
				projectLabel = "  " + MutedStyle.Render("["+m.projectNameByID(row.ProjectID)+"]")
			}
			line := fmt.Sprintf("%s%s %s (%d)%s", cursorPrefix, MutedStyle.Render(indicator), badge, row.SectionCount, projectLabel)
			if selected {
				rowStyle := SelectionRowStyle
				if m.cursorFlash {
					rowStyle = CursorFlashRowStyle
				}
				line = rowStyle.Width(m.width).Render(line)
			}
			b.WriteString(line)
		} else {
			task := row.task
			foldIndicator := ""
			if len(task.Children) > 0 {
				if m.isExpanded(task.ID) {
					foldIndicator = MutedStyle.Render("▾") + " "
				} else {
					foldIndicator = MutedStyle.Render("▸") + " "
				}
			}

			projBadge := ""
			if row.ProjectID != "" {
				projName := m.projectNameByID(row.ProjectID)
				badgeStyle := lipgloss.NewStyle().Foreground(projectBadgeColor(projName))
				projBadge = badgeStyle.Render("["+projName+"]") + " "
			}

			typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
			statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

			// Build indicators block: shortID and safe metadata columns that
			// don't require m.project (which is nil in cross-project mode).
			var orgParts []string
			if m.columnConfig.IsVisible(ColShortID) {
				orgParts = append(orgParts, MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)))
			}
			orgGroup := JoinGroupParts(orgParts...)

			var lifecycleParts []string
			if m.columnConfig.ShouldShowPriority(task.Priority) {
				lifecycleParts = append(lifecycleParts, renderPriorityBadge(task.Priority))
			}
			if m.columnConfig.IsVisible(ColDueDate) {
				if dueStr := formatDueDate(task); dueStr != "" {
					lifecycleParts = append(lifecycleParts, dueStr)
				}
			}
			if m.columnConfig.IsVisible(ColSnooze) && task.SnoozedUntil != nil {
				snoozeStr := task.SnoozedUntil.Format("Jan 2")
				if task.SnoozedUntil.Year() != time.Now().Year() {
					snoozeStr = task.SnoozedUntil.Format("Jan 2 '06")
				}
				lifecycleParts = append(lifecycleParts, MutedStyle.Render("zzz "+snoozeStr))
			}
			if m.columnConfig.IsVisible(ColRecurrence) && task.IsRecurring() {
				recurStr := "↻"
				if task.Recurrence.Streak > 0 {
					recurStr = fmt.Sprintf("↻%d", task.Recurrence.Streak)
				}
				lifecycleParts = append(lifecycleParts, MutedStyle.Render(recurStr))
			}
			lifecycleGroup := JoinGroupParts(lifecycleParts...)

			var contentParts []string
			if m.columnConfig.IsVisible(ColTags) {
				if s := renderTruncatedTags(task.Tags, 2, 8); s != "" {
					contentParts = append(contentParts, s)
				}
			}
			if m.columnConfig.IsVisible(ColComments) {
				if s := renderCommentIndicator(task); s != "" {
					contentParts = append(contentParts, s)
				}
			}
			contentGroup := JoinGroupParts(contentParts...)

			indicatorStr := JoinIndicatorGroups(orgGroup, lifecycleGroup, contentGroup)
			indicators := indicatorStr
			if indicators != "" {
				indicators = " " + indicators
			}

			line := fmt.Sprintf("  %s%s%s%s %s%s%s",
				row.prefix, foldIndicator, projBadge, typeIcon, statusIcon, task.Title, indicators)

			if selected {
				rowStyle := SelectionRowStyle
				if m.cursorFlash {
					rowStyle = CursorFlashRowStyle
				}
				line = rowStyle.Width(m.width).Render(line)
			}
			b.WriteString(line)
		}

		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// reloadCrossProjects triggers a new load, incrementing the generation counter
// so any in-flight stale result is discarded on arrival.
func (m *OutlinerModel) reloadCrossProjects() tea.Cmd {
	m.loadedProjects = nil
	m.rows = m.rows[:0]
	return m.startCrossProjectLoad()
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
