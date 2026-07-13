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
// alert lines. Both renderGlobalAlerts and compact dashboard info cards consume
// this. Lines are emitted in strict urgency tiers so the most important signal
// always wins the first row:
//
//  1. Urgent action: triage, overdue, due today.
//  2. Today progress: planned vs done today.
//  3. Awareness: blocked tasks and active blockers (merged).
//  4. Summary: completed this week, global notes, scoring suggestion.
//
// Per-project inbox counts are surfaced by the project row indicator, so the
// alert panel no longer repeats them as a separate "Inbox hotspots" line.
//
//nolint:gocyclo // The branch count mirrors the dashboard's ordered alert tiers.
func (m *DashboardModel) computeGlobalAlertLines(selectedProject *models.Project) []string {
	var totalTriage, totalOverdue, totalDueToday, totalBlocked, totalBlockers int
	var totalCompletedThisWeek, totalCompletedToday, todayPlannedCount int

	now := time.Now()
	today := util.StartOfDay(now)
	todayPlanKeys := m.todayPlannedTaskKeys(today)
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
		if p == nil {
			continue
		}
		totalOverdue += p.CountOverdueTasks()
		totalBlocked += p.CountBlockedTasks()
		totalBlockers += storage.GetActiveBlockerCount(p)

		for _, task := range p.Tasks {
			if task == nil {
				continue
			}
			if todayPlanKeys[planKey(task.ID, p.ID)] {
				todayPlannedCount++
			}
			if task.DueDate != nil && !task.IsComplete() && util.StartOfDay(*task.DueDate).Equal(today) {
				totalDueToday++
			}
			if task.CompletedAt != nil {
				if task.CompletedAt.After(sevenDaysAgo) {
					totalCompletedThisWeek++
				}
				if util.StartOfDay(*task.CompletedAt).Equal(today) {
					totalCompletedToday++
				}
			}
			if isProcessableInboxTask(task) {
				totalTriage++
			}
		}
	}

	var lines []string

	// Tier 1: urgent action.
	if totalTriage > 0 {
		line := fmt.Sprintf("%s %d to triage · ctrl+i", icons.Get("inbox"), totalTriage)
		if selectedProject != nil {
			if selectedInbox := countProjectInboxTasks(selectedProject); selectedInbox > 0 {
				line = fmt.Sprintf("%s %d to triage · O for %s (%d) · ctrl+i",
					icons.Get("inbox"), totalTriage, selectedProject.Name, selectedInbox)
			}
		}
		lines = append(lines, DueSoonStyle.Render(line))
	}
	if totalOverdue > 0 {
		lines = append(lines, OverdueStyle.Render(fmt.Sprintf("%s %d overdue", icons.Get("overdue"), totalOverdue)))
	}
	if totalDueToday > 0 {
		lines = append(lines, DueSoonStyle.Render(fmt.Sprintf("%s %d due today", icons.Get("due_today"), totalDueToday)))
	}

	// Tier 2: today progress.
	if todayPlannedCount > 0 || totalCompletedToday > 0 {
		var parts []string
		if todayPlannedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d planned", todayPlannedCount))
		}
		if totalCompletedToday > 0 {
			parts = append(parts, fmt.Sprintf("%d done", totalCompletedToday))
		}
		lines = append(lines, InfoStatusStyle.Render(fmt.Sprintf("Today: %s", strings.Join(parts, " · "))))
	}

	// Tier 3: awareness — blocked and active blockers share one muted row.
	if totalBlocked > 0 || totalBlockers > 0 {
		var parts []string
		if totalBlocked > 0 {
			parts = append(parts, fmt.Sprintf("%s %d blocked", icons.Get("blocked_dot"), totalBlocked))
		}
		if totalBlockers > 0 {
			parts = append(parts, fmt.Sprintf("%s %d active blockers", icons.Get("blockers"), totalBlockers))
		}
		lines = append(lines, MutedStyle.Render(strings.Join(parts, " · ")))
	}

	// Tier 4: summary.
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

func (m *DashboardModel) todayPlannedTaskKeys(today time.Time) map[string]bool {
	if m == nil || m.config == nil {
		return nil
	}
	todayKey := today.Format(util.DateFormat)
	keys := make(map[string]bool)
	for _, item := range m.config.GetDailyPlan() {
		if item.TaskID == "" || item.ProjectID == "" || item.PlannedDate != todayKey {
			continue
		}
		keys[planKey(item.TaskID, item.ProjectID)] = true
	}
	return keys
}
