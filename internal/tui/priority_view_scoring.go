package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
)

// computeSuggestionScore calculates the suggestion score for a task.
// Builds on the shared urgency score with additional factors for
// actionability. Higher scores = more recommended as the suggested next task.
func (m *PriorityViewModel) computeSuggestionScore(task *models.Task) int {
	score := ComputeUrgencyScore(task, m.queueIDs[task.ID])

	// Leaf task bonus: prefer actionable leaf tasks over containers
	if task.IsLeaf() {
		score += 800
	}

	// Container penalty: penalize tasks with incomplete children
	if len(task.Children) > 0 && !m.allChildrenComplete(task) {
		score -= 1000
	}

	// Task age: older tasks get a small boost to avoid stagnation
	daysSinceCreated := util.DaysBetween(task.CreatedAt, time.Now())
	ageBonus := min(
		// 30 points per week
		daysSinceCreated/7,
		// Cap at 300 points
		10)
	score += ageBonus * 30

	// Recent momentum: boost tasks with recent activity
	daysSinceUpdated := util.DaysBetween(task.UpdatedAt, time.Now())
	if daysSinceUpdated == 0 {
		score += 400 // Updated today
	} else if daysSinceUpdated <= 3 {
		score += 200 // Updated within 3 days
	}

	// Unblocking bonus: prefer tasks that unblock others
	blockedByThisCount := len(m.project.GetBlockedByTasks(task))
	unblockingBonus := min(blockedByThisCount,
		// Cap at 3
		3)
	score += unblockingBonus * 150

	// State-bucket scoring: Next is boosted, Waiting/Review/Standby/Someday penalized
	switch taskSectionBucket(task) {
	case models.SectionNext:
		score += 100
	case models.SectionWaiting:
		score -= 100
	case models.SectionReview:
		score -= 100
	case models.SectionStandby:
		score -= 200
	case models.SectionSomeday:
		score -= 300
	}

	return score
}

// allChildrenComplete returns true if all children of the task are complete.
func (m *PriorityViewModel) allChildrenComplete(task *models.Task) bool {
	for _, child := range task.Children {
		if !child.IsTerminalState() {
			return false
		}
	}
	return true
}

// computeSuggestion finds the highest-scoring non-dismissed actionable task.
func (m *PriorityViewModel) computeSuggestion() {
	m.suggestion = nil

	// Collect all actionable tasks (not blocked, not completed)
	var candidates []*PriorityTask
	candidates = append(candidates, m.overdue...)
	candidates = append(candidates, m.today...)
	candidates = append(candidates, m.thisWeek...)
	candidates = append(candidates, m.later...)
	candidates = append(candidates, m.backlog...)

	if len(candidates) == 0 {
		return
	}

	// Find highest-scoring non-dismissed task
	var best *PriorityTask
	bestScore := -999999

	for _, pt := range candidates {
		if m.dismissedIDs[pt.Task.ID] {
			continue
		}

		suggestionScore := m.computeSuggestionScore(pt.Task)
		if suggestionScore > bestScore {
			bestScore = suggestionScore
			best = pt
		}
	}

	if best == nil {
		return
	}

	m.suggestion = &SuggestedTask{
		Task:   best,
		Score:  bestScore,
		Reason: m.getSuggestionReason(best.Task),
	}
}

// getSuggestionReason generates a human-readable explanation for why a task is suggested.
func (m *PriorityViewModel) getSuggestionReason(task *models.Task) string {
	var reasons []string

	// Priority
	switch task.Priority {
	case 1:
		reasons = append(reasons, "critical priority")
	case 2:
		reasons = append(reasons, "high priority")
	}

	// Due date status
	if task.DueDate != nil {
		now := time.Now()
		today := util.StartOfDay(now)
		dueDay := util.StartOfDay(*task.DueDate)

		if dueDay.Before(today) {
			reasons = append(reasons, "overdue")
		} else if dueDay.Equal(today) {
			reasons = append(reasons, "due today")
		} else if dueDay.Before(today.AddDate(0, 0, 7)) {
			reasons = append(reasons, "due this week")
		}
	}

	// In-progress state
	if task.IsInState(models.StateInProgress) {
		reasons = append(reasons, "in progress")
	}

	// Focus Queue membership (successor of the legacy Active flag)
	if m.queueIDs[task.ID] {
		reasons = append(reasons, "in focus queue")
	}

	// Unblocking count
	blockedByThisCount := len(m.project.GetBlockedByTasks(task))
	if blockedByThisCount > 0 {
		if blockedByThisCount == 1 {
			reasons = append(reasons, "unblocks 1 task")
		} else {
			reasons = append(reasons, fmt.Sprintf("unblocks %d tasks", blockedByThisCount))
		}
	}

	// Momentum
	daysSinceUpdated := util.DaysBetween(task.UpdatedAt, time.Now())
	if daysSinceUpdated == 0 && !task.IsInState(models.StateInProgress) {
		reasons = append(reasons, "recent activity")
	}

	// Composite score from scoring profile
	if m.runtime != nil {
		if profile := m.runtime.Automation().GetProjectScoringProfile(m.project.ID); profile != nil {
			pct := scoring.NormalizedScore(task, profile)
			if int(pct) > 0 {
				reasons = append(reasons, fmt.Sprintf("composite score %d/%d", int(pct), scoring.MaxNormalizedScore(profile)))
			}
		}
	}

	// If no specific reasons, provide a generic one
	if len(reasons) == 0 {
		return "Ready to work on"
	}

	// Join reasons with " + "
	var result strings.Builder
	for i, r := range reasons {
		if i > 0 {
			result.WriteString(" + ")
		}
		// Capitalize first reason
		if i == 0 {
			result.WriteString(strings.ToUpper(r[:1]) + r[1:])
		} else {
			result.WriteString(r)
		}
	}
	return result.String()
}

// dismissSuggestion adds the current suggestion to the dismissed set.
func (m *PriorityViewModel) dismissSuggestion() {
	if m.suggestion == nil {
		return
	}
	m.dismissedIDs[m.suggestion.Task.Task.ID] = true
	m.computeSuggestion()
}

// startSuggestedTask sets the suggested task to in_progress or jumps to it if already started.
func (m *PriorityViewModel) startSuggestedTask() (tea.Model, tea.Cmd) {
	if m.suggestion == nil {
		return m, nil
	}

	task := m.suggestion.Task.Task

	// If already in progress, just jump to it in the list
	if task.IsInState(models.StateInProgress) {
		m.jumpToTask(task)
		return m, nil
	}

	if reason := task.CanTransitionTo(models.StatusInProgress); reason != "" {
		m.setStatus(reason)
		return m, nil
	}

	// Commit the start through the application-layer single-task coordinator; the
	// shell's transaction-result handler reloads the project and refreshes the
	// priority view (reloadData + clampCursor), so no inline reload is needed.
	cmd := m.taskItem.Transition(task.ID, models.TargetStateForStatus(models.StatusInProgress))
	msg := cmd()
	if em, ok := msg.(TransactionErrorMsg); ok {
		m.SetError(em.Err)
		return m, nil
	}
	return m, func() tea.Msg { return msg }
}
