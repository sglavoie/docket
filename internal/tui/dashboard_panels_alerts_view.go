package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// renderGlobalAlerts aggregates alert counts across all active projects.
func (m *DashboardModel) renderGlobalAlerts(_ int, selectedProject *models.Project) string {
	lines := m.computeGlobalAlertLines(selectedProject)
	if len(lines) == 0 {
		return MutedStyle.Render("No alerts")
	}
	return strings.Join(lines, "\n")
}

// computeGlobalAlertLines walks all active projects and produces the rendered
// alert lines. Both renderGlobalAlerts and compact dashboard info cards consume this.
func (m *DashboardModel) computeGlobalAlertLines(selectedProject *models.Project) []string {
	var totalTriage, totalOverdue, totalDueToday, totalBlocked, totalBlockers, totalCompletedThisWeek, todayPlannedCount int
	var inboxProjects []string

	now := time.Now()
	today := util.StartOfDay(now)
	sevenDaysAgo := now.AddDate(0, 0, -7)

	activeProjects := m.projects
	if m.showArchived {
		activeProjects = m.config.GetActiveProjects()
		for _, p := range activeProjects {
			if err := storage.LoadProject(p); err != nil {
				logging.Warn("failed to load project %s: %v", p.Name, err)
			}
		}
	}

	for _, p := range activeProjects {
		totalOverdue += p.CountOverdueTasks()
		totalBlocked += p.CountBlockedTasks()
		totalBlockers += storage.GetActiveBlockerCount(p)

		for _, task := range p.Tasks {
			if task == nil {
				continue
			}
			if task.Section == models.SectionNext && !task.Status.IsComplete() {
				todayPlannedCount++
			}
			if task.DueDate != nil && !task.Status.IsComplete() && util.StartOfDay(*task.DueDate).Equal(today) {
				totalDueToday++
			}
			if task.CompletedAt != nil && task.CompletedAt.After(sevenDaysAgo) {
				totalCompletedThisWeek++
			}
		}

		inboxCount := 0
		for _, task := range p.Tasks {
			if isProcessableInboxTask(task) {
				inboxCount++
			}
		}
		totalTriage += inboxCount
		if inboxCount >= 5 {
			inboxProjects = append(inboxProjects, fmt.Sprintf("%s(%d)", p.Name, inboxCount))
		}
	}

	var lines []string
	if totalTriage > 0 {
		lines = append(lines, DueSoonStyle.Render(fmt.Sprintf("%s %d to triage · ctrl+i", icons.Get("inbox"), totalTriage)))
	}
	if selectedProject != nil {
		if selectedInbox := countProjectInboxTasks(selectedProject); selectedInbox > 0 {
			lines = append(lines, DueSoonStyle.Render(fmt.Sprintf("%s Press O to triage %s (%d)",
				icons.Get("inbox"), selectedProject.Name, selectedInbox)))
		}
	}
	if totalOverdue > 0 {
		lines = append(lines, OverdueStyle.Render(fmt.Sprintf("%s %d overdue", icons.Get("overdue"), totalOverdue)))
	}
	if totalDueToday > 0 {
		lines = append(lines, DueSoonStyle.Render(fmt.Sprintf("%s %d due today", icons.Get("due_today"), totalDueToday)))
	}
	if totalBlocked > 0 {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("%s %d blocked", icons.Get("blocked_dot"), totalBlocked)))
	}
	if totalBlockers > 0 {
		lines = append(lines, DueSoonStyle.Render(fmt.Sprintf("%s %d active blockers", icons.Get("blockers"), totalBlockers)))
	}
	if len(inboxProjects) > 0 {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("%s Inbox hotspots: %s", icons.Get("inbox"), strings.Join(inboxProjects, ", "))))
	}
	if totalCompletedThisWeek > 0 {
		lines = append(lines, SuccessStyle.Render(fmt.Sprintf("%s %d completed this week", IconChecklist(), totalCompletedThisWeek)))
	}

	globalNotes, _ := storage.ListNotesFrom(storage.GlobalNotesDir(m.config.DataDir))
	if len(globalNotes) > 0 {
		lines = append(lines, MutedStyle.Render(fmt.Sprintf("%s %d global notes", icons.Get("notes"), len(globalNotes))))
	}

	if todayPlannedCount == 0 && selectedProject != nil {
		profile := m.config.GetProjectScoringProfile(selectedProject.ID)
		if suggested := bestScoredTask(selectedProject, profile); suggested != nil {
			lines = append(lines, renderSuggestedTask(suggested))
		}
	}

	return lines
}
