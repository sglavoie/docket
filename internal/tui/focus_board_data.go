package tui

import (
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
)

// loadItems loads and aggregates active/due tasks across all projects.
// It cancels any active edit/add mode because the underlying data (m.filtered)
// is about to be rebuilt, which would make the stale editIdx point at the
// wrong row or go out of bounds.
func (m *FocusBoardModel) loadItems() {
	m.resetViewScrollOffset()
	if m.editMode {
		m.editMode = false
		m.editInput.Blur()
	}
	if m.addMode {
		m.addMode = false
		m.addInput.Blur()
		m.ac.Clear()
	}
	m.rows = nil
	m.allRows = nil
	m.nextRows = nil
	m.closedRows = nil
	m.todayRows = nil

	now := time.Now()
	today := util.StartOfDay(now)

	// Iterate all loadable projects, including the visible system Inbox and any
	// legacy internal projects that still participate in cross-project counts.
	_, projects, _ := LoadCrossScopeTasks(m.runtime, m.projectService, CrossScopeOpts{})
	for _, project := range projects {
		for _, task := range project.Tasks {
			row := boardRow{
				Task:     task,
				Project:  project,
				Priority: task.Priority,
				Source:   project.Name,
			}

			// Today tab: include daily-planned tasks regardless of completion status
			if m.dailyPlanIDs[task.ID] {
				m.todayRows = append(m.todayRows, row)
			}

			// Collect closed tasks for the Closed tab
			if task.IsTerminalState() {
				m.closedRows = append(m.closedRows, row)
			}

			// Skip completed/snoozed for active tabs
			if task.IsTerminalState() || task.IsSnoozed() {
				continue
			}

			// All non-complete, non-snoozed tasks for the "All" tab
			m.allRows = append(m.allRows, row)

			// Collect Next-bucket tasks (the active ready/in_progress bucket
			// shares the Next tab, matching the legacy empty-section default).
			if bucket := taskSectionBucket(task); bucket == "" || bucket == models.SectionNext {
				m.nextRows = append(m.nextRows, row)
			}

			isQueued := m.queueIDs[task.ID]
			isDueSoon := task.DueDate != nil && util.StartOfDay(*task.DueDate).Before(today.AddDate(0, 0, 1))
			if isQueued || isDueSoon {
				m.rows = append(m.rows, row)
			}
		}
	}

	// Sort today rows by state activeness, then by dailyPlan position within same state group
	posMap := make(map[string]int, len(*m.dailyPlan))
	for i, ft := range *m.dailyPlan {
		posMap[ft.Task.ID] = i
	}
	sort.SliceStable(m.todayRows, func(i, j int) bool {
		iOrd := TodayStateOrder(m.todayRows[i].Task.State())
		jOrd := TodayStateOrder(m.todayRows[j].Task.State())
		if iOrd != jOrd {
			return iOrd < jOrd
		}
		return posMap[m.todayRows[i].Task.ID] < posMap[m.todayRows[j].Task.ID]
	})

	m.expandTodayChildren()

	// Sort by priority ascending, then preserve original order within same priority
	sort.SliceStable(m.rows, func(i, j int) bool {
		if m.rows[i].Priority != m.rows[j].Priority {
			return m.rows[i].Priority < m.rows[j].Priority
		}
		return false
	})

	m.applyFilter()
	m.initBoardCaches()

	if !m.initialTabSet {
		m.initialTabSet = true
		if len(m.dailyPlanIDs) == 0 {
			m.tabs.Active = boardTabNext
			m.applyFilter()
		}
	}
}

// completedTodayCount returns the number of today's planned tasks that are complete.
func (m *FocusBoardModel) completedTodayCount() int {
	count := 0
	for _, row := range m.todayRows {
		if row.Task.IsTerminalState() {
			count++
		}
	}
	return count
}

// applyFilter filters rows based on the active tab.
func (m *FocusBoardModel) applyFilter() {
	m.resetViewScrollOffset()
	switch m.tabs.Active {
	case boardTabToday:
		m.filtered = make([]boardRow, len(m.todayRows))
		copy(m.filtered, m.todayRows)
	case boardTabNext:
		m.filtered = make([]boardRow, len(m.nextRows))
		copy(m.filtered, m.nextRows)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Priority < m.filtered[j].Priority
		})
	case boardTabClosed:
		m.filtered = make([]boardRow, len(m.closedRows))
		copy(m.filtered, m.closedRows)
	default:
		m.filtered = make([]boardRow, len(m.rows))
		copy(m.filtered, m.rows)
	}

	// Apply common filters and search
	if hasActiveCommonFilters(&m.filter) || m.searchQuery != "" {
		var passed []boardRow
		for _, r := range m.filtered {
			if hasActiveCommonFilters(&m.filter) && !passesCommonFilters(r.Task, &m.filter, r.Project, m.runtime.Automation()) {
				continue
			}
			if m.searchQuery != "" && !m.matchesSearch(r) {
				continue
			}
			passed = append(passed, r)
		}
		m.filtered = passed
	}

	// Perspective lens AND-composes on top of the tab/search/filter result.
	if perspFilter := PerspectiveLensFilter(m.activePerspective); perspFilter != nil {
		var passed []boardRow
		for _, r := range m.filtered {
			if r.Task != nil && perspFilter(r.Task) {
				passed = append(passed, r)
			}
		}
		m.filtered = passed
	}

	// Apply sorting
	if m.sort.Field != SortByDefault {
		m.applySorting()
	}
}

// applySorting sorts the filtered rows based on the current sort field.
func (m *FocusBoardModel) applySorting() {
	for start := 0; start < len(m.filtered); {
		for start < len(m.filtered) && !m.filtered[start].isTask() {
			start++
		}
		end := start
		for end < len(m.filtered) && m.filtered[end].isTask() {
			end++
		}
		if end-start > 1 {
			rows := m.filtered[start:end]
			sort.SliceStable(rows, func(i, j int) bool {
				return m.focusBoardSortLess(rows[i], rows[j])
			})
		}
		start = end
	}
}

func (m *FocusBoardModel) focusBoardSortLess(ri, rj boardRow) bool {
	if ri.Task == nil || rj.Task == nil || ri.Project == nil || rj.Project == nil {
		return false
	}
	if m.sort.Field == SortByScore {
		si, sj := m.scorePercent(ri), m.scorePercent(rj)
		if si != sj {
			cmp := -1
			if si < sj {
				cmp = 1
			}
			return ApplySortDirection(cmp, m.sort.Ascending)
		}
		return false
	}
	cmp := CompareTasksCrossProject(
		ri.Task, rj.Task, m.sort.Field,
		ri.Project.ID, rj.Project.ID, m.runtime.Automation(),
	)
	return ApplySortDirection(cmp, m.sort.Ascending)
}

// scorePercent returns the normalized score percentage for a task row.
func (m *FocusBoardModel) scorePercent(r boardRow) float64 {
	profile := m.runtime.Automation().GetProjectScoringProfile(r.Project.ID)
	if profile == nil {
		return 0
	}
	score := scoring.ComputeScore(r.Task, profile)
	return scoring.ScorePercent(score, profile)
}

// rebuildTodayTree re-flattens the Today tree in-memory (no disk reload) after a
// fold-depth change. It recovers the base rows (ParentID == "") from the current
// todayRows, resets todayRows to that base, re-expands children under the current
// childDepthLimit, then reapplies the active filter and clamps the cursor.
func (m *FocusBoardModel) rebuildTodayTree() {
	var base []boardRow
	for _, r := range m.todayRows {
		if r.ParentID == "" {
			base = append(base, r)
		}
	}
	m.todayRows = base
	m.expandTodayChildren()
	m.applyFilter()
	m.clampCursor()
}

// expandTodayChildren inserts descendants of explicitly planned items into todayRows.
func (m *FocusBoardModel) expandTodayChildren() {
	seen := make(map[string]bool, len(m.todayRows))
	for _, r := range m.todayRows {
		seen[r.Task.ID] = true
	}

	var expanded []boardRow
	for _, r := range m.todayRows {
		expanded = append(expanded, r)
		if m.dailyPlanIDs[r.Task.ID] {
			m.collectChildren(&expanded, r.Task, r.Project, nil, seen)
		}
	}
	m.todayRows = expanded
}

// collectChildren recursively appends child rows depth-first with tree connectors.
func (m *FocusBoardModel) collectChildren(out *[]boardRow, parent *models.Task, project *models.Project, ancestorIsLast []bool, seen map[string]bool) {
	if !m.isParentExpanded(parent.ID, len(ancestorIsLast)) {
		return
	}
	visible := make([]*models.Task, 0, len(parent.Children))
	for _, child := range parent.Children {
		if !seen[child.ID] {
			visible = append(visible, child)
		}
	}
	sort.SliceStable(visible, func(i, j int) bool {
		return !visible[i].IsTerminalState() && visible[j].IsTerminalState()
	})
	for i, child := range visible {
		seen[child.ID] = true
		isLast := i == len(visible)-1
		*out = append(*out, boardRow{
			Task:       child,
			Project:    project,
			Priority:   child.Priority,
			Source:     project.Name,
			TreePrefix: buildBoardTreePrefix(ancestorIsLast, isLast),
			ParentID:   parent.ID,
			Depth:      len(ancestorIsLast) + 1,
		})
		next := make([]bool, len(ancestorIsLast)+1)
		copy(next, ancestorIsLast)
		next[len(ancestorIsLast)] = isLast
		m.collectChildren(out, child, project, next, seen)
	}
}

// isParentExpanded reports whether a parent task at the given depth should
// have its children rendered in the Today tab.
func (m *FocusBoardModel) isParentExpanded(id string, depth int) bool {
	if v, ok := m.foldOverrides[id]; ok {
		return !v
	}
	if m.childDepthLimit < 0 {
		return true
	}
	return depth < m.childDepthLimit
}

// buildBoardTreePrefix builds a tree prefix with shorter horizontal lines
// (one fewer ─) to fit the focus board's compact layout.
func buildBoardTreePrefix(ancestorIsLast []bool, isLast bool) string {
	var b strings.Builder
	b.WriteByte(' ')
	for _, last := range ancestorIsLast {
		if last {
			b.WriteString("     ")
		} else {
			b.WriteString("│    ")
		}
	}
	if isLast {
		b.WriteString("└─── ")
	} else {
		b.WriteString("├─── ")
	}
	return b.String()
}
