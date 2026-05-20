package tui

import (
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *RecentActivityModel) getPersister() Persister           { return defaultPersister }
func (m *RecentActivityModel) getConfig() *config.Config         { return m.config }
func (m *RecentActivityModel) getConfigManager() *config.Manager { return m.configManager }
func (m *RecentActivityModel) reloadData()                       { m.loadTasks() }
func (m *RecentActivityModel) setErr(err error)                  { m.setStatus(err.Error()) }

// getTaskRef returns the focused task and its project for queue toggling.
func (m *RecentActivityModel) getTaskRef() (*models.Task, *models.Project) {
	if !CursorInBounds(m.cursor, len(m.filtered)) {
		return nil, nil
	}
	cpt := m.filtered[m.cursor]
	return cpt.Task, cpt.Project
}

// loadTasks aggregates tasks from all active projects, sorted by UpdatedAt desc.
func (m *RecentActivityModel) loadTasks() {
	m.tasks = nil
	limit := m.config.GetRecentActivityLimit()

	if m.projectScope != nil {
		for _, task := range m.projectScope.Tasks {
			m.tasks = append(m.tasks, CrossProjectTask{Task: task, Project: m.projectScope})
		}
	} else {
		projects := m.config.GetActiveProjects()
		for _, project := range projects {
			if err := storage.LoadProject(project); err != nil {
				continue
			}
			for _, task := range project.Tasks {
				m.tasks = append(m.tasks, CrossProjectTask{Task: task, Project: project})
			}
		}
	}

	// Sort by UpdatedAt descending, breaking ties on Task.ID to keep ordering
	// deterministic before the cap-at-limit truncation below. Without the
	// tiebreak, tasks updated in the same millisecond (common during bulk
	// imports) could be partially retained in arbitrary order.
	sort.Slice(m.tasks, func(i, j int) bool {
		ti, tj := m.tasks[i].Task.UpdatedAt, m.tasks[j].Task.UpdatedAt
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return m.tasks[i].Task.ID < m.tasks[j].Task.ID
	})

	// Cap at limit
	if limit > 0 && len(m.tasks) > limit {
		m.tasks = m.tasks[:limit]
	}

	m.applyFilter()
}

func (m *RecentActivityModel) applyFilter() {
	// First filter by tab and common filters
	var tabFiltered []CrossProjectTask
	hasFilters := hasActiveCommonFilters(&m.filter)
	for _, dt := range m.tasks {
		switch m.tabs.Active {
		case recentActivityTabActive:
			if dt.Task.Status.IsComplete() {
				continue
			}
		case recentActivityTabCompleted:
			if !dt.Task.Status.IsComplete() {
				continue
			}
		}
		if hasFilters && !passesCommonFilters(dt.Task, &m.filter, dt.Project.ID, m.config) {
			continue
		}
		tabFiltered = append(tabFiltered, dt)
	}

	// Then apply search
	query := strings.ToLower(m.SearchQuery())
	if query == "" && !m.IsSearchMode() {
		query = strings.ToLower(m.SearchInputValue())
	}
	if query == "" {
		m.filtered = tabFiltered
		return
	}
	m.filtered = nil
	for _, dt := range tabFiltered {
		title := strings.ToLower(dt.Task.Title)
		project := strings.ToLower(dt.Project.Name)
		if strings.Contains(title, query) || strings.Contains(project, query) {
			m.filtered = append(m.filtered, dt)
		}
	}
}

// HasActiveSearch returns true when search is active or a query is set.
func (m *RecentActivityModel) HasActiveSearch() bool {
	return m.IsSearchMode() || m.SearchQuery() != ""
}

// HasTextInput returns true when search input is active.
func (m *RecentActivityModel) HasTextInput() bool {
	return m.DefaultHasTextInput()
}
