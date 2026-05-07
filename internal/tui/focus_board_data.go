package tui

import (
	"sort"
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

	// Sort today rows: active first, then completed; within each group preserve dailyPlan order
	posMap := make(map[string]int, len(*m.dailyPlan))
	for i, ft := range *m.dailyPlan {
		posMap[ft.Task.ID] = i
	}
	sort.SliceStable(m.todayRows, func(i, j int) bool {
		iComplete := m.todayRows[i].Task.Status.IsComplete()
		jComplete := m.todayRows[j].Task.Status.IsComplete()
		if iComplete != jComplete {
			return !iComplete
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
}

// applyFilter filters rows based on the active tab.
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
	case boardTabActiveDue:
		m.filtered = make([]boardRow, len(m.rows))
		copy(m.filtered, m.rows)
	case boardTabNext:
		m.filtered = make([]boardRow, len(m.nextRows))
		copy(m.filtered, m.nextRows)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Priority < m.filtered[j].Priority
		})
	case boardTabBySource:
		m.filtered = make([]boardRow, len(m.rows))
		copy(m.filtered, m.rows)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Source < m.filtered[j].Source
		})
	case boardTabByPriority:
		m.filtered = make([]boardRow, len(m.rows))
		copy(m.filtered, m.rows)
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.filtered[i].Priority < m.filtered[j].Priority
		})
	case boardTabByScore:
		m.filtered = nil
		for _, r := range m.rows {
			profile := m.config.GetProjectScoringProfile(r.Project.ID)
			if profile != nil {
				m.filtered = append(m.filtered, r)
			}
		}
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

	// Apply sorting: user-selected field, or default score-descending on By Score tab
	if m.sort.Field != SortByDefault {
		m.applySorting()
	} else if m.tabs.Active == boardTabByScore {
		sort.SliceStable(m.filtered, func(i, j int) bool {
			return m.scorePercent(m.filtered[i]) > m.scorePercent(m.filtered[j])
		})
	}
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
			m.collectChildren(&expanded, r.Task, r.Project, 1, seen)
		}
	}
	m.todayRows = expanded
}

// collectChildren recursively appends child rows depth-first.
func (m *FocusBoardModel) collectChildren(out *[]boardRow, parent *models.Task, project *models.Project, depth int, seen map[string]bool) {
	for _, child := range parent.Children {
		if seen[child.ID] {
			continue
		}
		seen[child.ID] = true
		*out = append(*out, boardRow{
			Task:        child,
			Project:     project,
			Priority:    child.Priority,
			Source:      project.Name,
			IndentLevel: depth,
		})
		m.collectChildren(out, child, project, depth+1, seen)
	}
}
