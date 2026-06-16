package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// getAllContexts returns a sorted list of unique contexts from loaded projects.
func (m *DashboardModel) getAllContexts() []string {
	contextSet := make(map[string]bool)
	for _, p := range m.projects {
		if p.Context != "" {
			contextSet[p.Context] = true
		}
	}
	contexts := make([]string, 0, len(contextSet))
	for ctx := range contextSet {
		contexts = append(contexts, ctx)
	}
	sort.Strings(contexts)
	return contexts
}

func (m *DashboardModel) matchesFilters(p *models.Project) bool {
	// Apply custom context filter first (list of specific project IDs)
	if m.filterCustomContext != "" {
		if !m.config.IsProjectInCustomContext(m.filterCustomContext, p.ID) {
			return false
		}
	} else if m.filterContext != "" && p.Context != m.filterContext {
		return false
	}

	// Then apply search filter
	if m.SearchQuery() == "" {
		return true
	}
	query := strings.TrimSpace(m.SearchQuery())

	if fuzzy.MatchTerms(p.Name, query) {
		return true
	}
	if fuzzy.MatchTerms(p.Context, query) {
		return true
	}
	if fuzzy.MatchTerms(p.Description, query) {
		return true
	}
	return false
}

func (m *DashboardModel) visibleProjects() []*models.Project {
	if m.SearchQuery() == "" && m.filterContext == "" && m.filterCustomContext == "" {
		return m.projects
	}
	var visible []*models.Project
	for _, p := range m.projects {
		if m.matchesFilters(p) {
			visible = append(visible, p)
		}
	}
	return visible
}

// renderFilterBar renders the search and context filter bar.
// Returns the rendered string and the number of chrome lines consumed.
func (m *DashboardModel) renderFilterBar() (string, int) {
	var b strings.Builder
	chromeHeight := 0
	hasSearch := m.IsSearchMode() || m.SearchQuery() != ""
	hasContext := m.filterContext != ""
	hasCustomContext := m.filterCustomContext != ""

	if m.IsSearchMode() {
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.SearchInputView())
		if hasCustomContext {
			b.WriteString(" ")
			b.WriteString(FilterBadgeStyle.Render(fmt.Sprintf("[+ %s]", m.filterCustomContext)))
		} else if hasContext {
			b.WriteString(" ")
			b.WriteString(FilterBadgeStyle.Render(fmt.Sprintf("[%s]", m.filterContext)))
		}
		b.WriteString("\n")
		chromeHeight++
	} else if hasSearch || hasContext || hasCustomContext {
		var parts []string
		if m.SearchQuery() != "" {
			parts = append(parts, MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery())))
		}
		if hasCustomContext {
			parts = append(parts, FilterBadgeStyle.Render(fmt.Sprintf("[+ %s]", m.filterCustomContext)))
		} else if hasContext {
			parts = append(parts, FilterBadgeStyle.Render(fmt.Sprintf("[%s]", m.filterContext)))
		}
		b.WriteString(strings.Join(parts, " · "))
		b.WriteString(" ")
		b.WriteString(HelpStyle.Render("(Esc clear)"))
		b.WriteString("\n")
		chromeHeight++
	}

	return b.String(), chromeHeight
}

func (m *DashboardModel) loadProjects() {
	if m.showArchived {
		m.projects = m.config.GetArchivedProjects()
	} else {
		m.projects = m.config.GetActiveProjects()
	}
	for _, p := range m.projects {
		if err := storage.LoadProject(p); err != nil {
			logging.Warn("failed to load project %s: %v", p.Name, err)
		}
	}

	// Evaluate project-level workflow rules for all loaded projects
	rules := m.config.GetEnabledWorkflowRules()
	if len(rules) > 0 {
		changed := false
		for _, p := range m.projects {
			if p.StatusIconSource == "user" {
				continue
			}
			projCtx := &workflow.ProjectEvaluationContext{
				Project: p,
				Tasks:   p.Tasks,
			}
			projResult := workflow.EvaluateProjectRules(projCtx, rules)
			if projResult.NewStatusIcon != "" && projResult.NewStatusIcon != p.StatusIcon {
				p.StatusIcon = projResult.NewStatusIcon
				p.StatusIconSource = "auto"
				changed = true
			}
		}
		if changed && m.configManager != nil {
			if err := m.configManager.Save(m.config); err != nil {
				logging.Warn("failed to save config: %v", err)
			}
		}
	}

	m.contexts = m.getAllContexts()
	m.cursor = ClampCursor(m.cursor, len(m.visibleProjects()))
	m.resetViewScrollOffset()
}

// GetHelpItems returns the help items for the dashboard view.
func (m *DashboardModel) GetHelpItems() []HelpItem {
	toggleArchiveDesc := "Show archived"
	if m.showArchived {
		toggleArchiveDesc = "Show active"
	}

	items := append([]HelpItem{}, GetKeybindingRegistry().GetGlobalBindings()...)
	for _, item := range m.buildKeyMap().HelpItems() {
		items = append(items, HelpItem{
			Key:         item.Key,
			Description: item.Description,
			Category:    item.Category,
		})
	}
	for i, item := range items {
		if item.Key == "a" {
			items[i].Description = toggleArchiveDesc
		}
	}
	return items
}
