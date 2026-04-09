package tui

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
)

// loadItems loads Action tasks and aggregates active/due tasks across all projects.
func (m *FocusBoardModel) loadItems() {
	m.rows = nil
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

			// Include if active or due soon
			if task.Status.IsComplete() || task.IsSnoozed() {
				continue
			}

			// Collect Next-section tasks (empty section defaults to Next)
			if task.Section == "" || task.Section == models.SectionNext {
				m.nextRows = append(m.nextRows, row)
			}

			isActive := task.Active
			isDueSoon := task.DueDate != nil && util.StartOfDay(*task.DueDate).Before(today.AddDate(0, 0, 1))
			if isActive || isDueSoon {
				m.rows = append(m.rows, row)
			}
		}
	}

	// Sort today rows: active first (by priority), then completed (by priority)
	sort.SliceStable(m.todayRows, func(i, j int) bool {
		iComplete := m.todayRows[i].Task.Status.IsComplete()
		jComplete := m.todayRows[j].Task.Status.IsComplete()
		if iComplete != jComplete {
			return !iComplete
		}
		return m.todayRows[i].Priority < m.todayRows[j].Priority
	})

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
	case boardTabActions:
		m.filtered = nil
		for _, r := range m.rows {
			if r.isAction() {
				m.filtered = append(m.filtered, r)
			}
		}
	case boardTabActiveDue:
		m.filtered = nil
		for _, r := range m.rows {
			if r.isTask() {
				m.filtered = append(m.filtered, r)
			}
		}
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
