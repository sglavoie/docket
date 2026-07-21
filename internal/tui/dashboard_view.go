package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

func (m *DashboardModel) View() tea.View {
	var body strings.Builder
	filterBar, chromeHeight := m.renderFilterBar()
	body.WriteString(filterBar)
	body.WriteString(m.renderHome(m.visibleProjects(), chromeHeight))
	return tea.NewView(body.String())
}

func (m *DashboardModel) projectCardTitle(visible []*models.Project) string {
	if m.showArchived {
		return fmt.Sprintf("Archived (%d)", len(visible))
	}
	title := fmt.Sprintf("Projects (%d)", len(visible))
	if streak := m.runtime.Planning().GetStreakCount(); streak > 0 && isStreakActive(m.runtime.Planning().GetStreakLastDate()) {
		title += fmt.Sprintf("  %s %dd", icons.Get("streak"), streak)
	}
	return title
}

func isStreakActive(date string) bool {
	if date == "" {
		return false
	}
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	return date == today || date == yesterday
}
