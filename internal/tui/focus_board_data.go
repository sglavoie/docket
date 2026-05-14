package tui

import (
	"fmt"
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

	// Iterate all loadable projects (including internal ones like action-inbox)
	projects := m.config.GetAllLoadableProjects()
	for _, project := range projects {
		if project.Sandbox {
			continue
		}
		if err := m.projectService.Load(project); err != nil {
			continue
		}
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
			if task.Status.IsComplete() {
				m.closedRows = append(m.closedRows, row)
			}

			// Skip completed/snoozed for active tabs
			if task.Status.IsComplete() || task.IsSnoozed() {
				continue
			}

			// All non-complete, non-snoozed tasks for the "All" tab
			m.allRows = append(m.allRows, row)

			// Collect Next-section tasks (empty section defaults to Next).
			// Exclude tasks with a custom section — they belong to that
			// custom section, not "Next", even though task.Section is "".
			if task.CustomSection == "" && (task.Section == "" || task.Section == models.SectionNext) {
				m.nextRows = append(m.nextRows, row)
			}

			isActive := task.Active
			isDueSoon := task.DueDate != nil && util.StartOfDay(*task.DueDate).Before(today.AddDate(0, 0, 1))
			if isActive || isDueSoon {
				m.rows = append(m.rows, row)
			}
		}
	}

	// Sort today rows by status activeness, then by dailyPlan position within same status group
	posMap := make(map[string]int, len(*m.dailyPlan))
	for i, ft := range *m.dailyPlan {
		posMap[ft.Task.ID] = i
	}
	sort.SliceStable(m.todayRows, func(i, j int) bool {
		iOrd := TodayStatusOrder(m.todayRows[i].Task.Status)
		jOrd := TodayStatusOrder(m.todayRows[j].Task.Status)
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
			m.tabs.Active = boardTabAll
			m.applyFilter()
		}
	}
}

// completedTodayCount returns the number of today's planned tasks that are complete.
func (m *FocusBoardModel) completedTodayCount() int {
	count := 0
	for _, row := range m.todayRows {
		if row.Task.Status.IsComplete() {
			count++
		}
	}
	return count
}

// applyFilter filters rows based on the active tab, then applies groupBy headers for the All tab.
func (m *FocusBoardModel) applyFilter() {
	switch m.tabs.Active {
	case boardTabToday:
		m.filtered = make([]boardRow, len(m.todayRows))
		copy(m.filtered, m.todayRows)
	case boardTabAll:
		m.filtered = make([]boardRow, len(m.allRows))
		copy(m.filtered, m.allRows)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Priority < m.filtered[j].Priority
		})
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
			if hasActiveCommonFilters(&m.filter) && !passesCommonFilters(r.Task, &m.filter, r.Project.ID, m.config) {
				continue
			}
			if m.searchQuery != "" && !m.matchesSearch(r) {
				continue
			}
			passed = append(passed, r)
		}
		m.filtered = passed
	}

	// Apply sorting
	if m.sort.Field != SortByDefault {
		m.applySorting()
	}

	// Insert group-header rows for the All tab when a groupBy mode is active
	if m.tabs.Active == boardTabAll && m.groupBy != boardGroupByNone {
		m.filtered = m.insertGroupHeaders(m.filtered)
	}
}

// insertGroupHeaders sorts filtered rows by the active groupBy key and inserts
// non-selectable header rows at group boundaries.
func (m *FocusBoardModel) insertGroupHeaders(rows []boardRow) []boardRow {
	if len(rows) == 0 {
		return rows
	}

	keyOf := func(r boardRow) string {
		switch m.groupBy {
		case boardGroupBySource:
			if r.Source == "" {
				return "(no source)"
			}
			return r.Source
		case boardGroupByPriority:
			return fmt.Sprintf("P%d", r.Priority)
		case boardGroupByScore:
			pct := m.scorePercent(r)
			switch {
			case pct >= 80:
				return "High score (≥80%)"
			case pct >= 50:
				return "Medium score (50–79%)"
			default:
				return "Low score (<50%)"
			}
		}
		return ""
	}

	// Sort by group key, stable so within-group order is preserved
	sort.SliceStable(rows, func(i, j int) bool {
		ki, kj := keyOf(rows[i]), keyOf(rows[j])
		if m.groupBy == boardGroupByScore {
			// Score groups: high first
			return m.scorePercent(rows[i]) > m.scorePercent(rows[j])
		}
		return ki < kj
	})

	var out []boardRow
	var lastKey string
	for _, r := range rows {
		k := keyOf(r)
		if k != lastKey {
			out = append(out, boardRow{GroupHeader: k})
			lastKey = k
		}
		out = append(out, r)
	}
	return out
}

// applySorting sorts the filtered rows based on the current sort field.
func (m *FocusBoardModel) applySorting() {
	sort.SliceStable(m.filtered, func(i, j int) bool {
		ri, rj := m.filtered[i], m.filtered[j]
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
			ri.Project.ID, rj.Project.ID, m.config,
		)
		return ApplySortDirection(cmp, m.sort.Ascending)
	})
}

// scorePercent returns the normalized score percentage for a task row.
func (m *FocusBoardModel) scorePercent(r boardRow) float64 {
	profile := m.config.GetProjectScoringProfile(r.Project.ID)
	if profile == nil {
		return 0
	}
	score := scoring.ComputeScore(r.Task, profile)
	return scoring.ScorePercent(score, profile)
}

// rebuildTodayChildren strips auto-expanded children and re-expands with the current depth limit.
func (m *FocusBoardModel) rebuildTodayChildren() {
	var roots []boardRow
	for _, r := range m.todayRows {
		if r.TreePrefix == "" {
			roots = append(roots, r)
		}
	}
	m.todayRows = roots
	m.expandTodayChildren()
	m.applyFilter()
	m.clampCursor()
}

// toggleFoldAtCursor toggles the collapsed state of the parent task at or containing the cursor.
func (m *FocusBoardModel) toggleFoldAtCursor() {
	row := m.getRowAtCursor()
	if row == nil || row.Task == nil {
		return
	}
	var parentID string
	if row.TreePrefix == "" {
		if len(row.Task.Children) == 0 {
			return
		}
		parentID = row.Task.ID
	} else {
		for _, r := range m.filtered {
			if r.TreePrefix == "" && r.Task != nil {
				parentID = r.Task.ID
			}
			if r.Task != nil && r.Task.ID == row.Task.ID {
				break
			}
		}
	}
	if parentID == "" {
		return
	}
	if m.collapsedParents[parentID] {
		delete(m.collapsedParents, parentID)
	} else {
		m.collapsedParents[parentID] = true
	}
	m.rebuildTodayChildren()
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
	if m.childDepthLimit >= 0 && len(ancestorIsLast) >= m.childDepthLimit {
		return
	}
	if m.collapsedParents[parent.ID] {
		return
	}
	visible := make([]*models.Task, 0, len(parent.Children))
	for _, child := range parent.Children {
		if !seen[child.ID] {
			visible = append(visible, child)
		}
	}
	for i, child := range visible {
		seen[child.ID] = true
		isLast := i == len(visible)-1
		*out = append(*out, boardRow{
			Task:       child,
			Project:    project,
			Priority:   child.Priority,
			Source:     project.Name,
			TreePrefix: buildBoardTreePrefix(ancestorIsLast, isLast),
		})
		next := make([]bool, len(ancestorIsLast)+1)
		copy(next, ancestorIsLast)
		next[len(ancestorIsLast)] = isLast
		m.collectChildren(out, child, project, next, seen)
	}
}

// parentRowIndices returns the indices of rows in m.filtered that are parent
// tasks (TreePrefix == ""), used by n/N navigation to jump between parents.
func (m *FocusBoardModel) parentRowIndices() []int {
	var indices []int
	for i, row := range m.filtered {
		if row.TreePrefix == "" {
			indices = append(indices, i)
		}
	}
	return indices
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
