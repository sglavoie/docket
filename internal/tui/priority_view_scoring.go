package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/storage"
)

// computeUrgencyScore calculates the urgency score for a task.
// Higher scores = more urgent.
func (m *PriorityViewModel) computeUrgencyScore(task *models.Task) int {
	score := 0

	// Priority weight: priority 1 = 5000, priority 5 = 1000
	priorityWeight := (6 - task.Priority) * 1000
	score += priorityWeight

	// Bug type bonus: Bugs at the same priority rank higher
	if task.Type == models.TaskTypeBug {
		score += 100
	}

	// Due date proximity: overdue adds points, future subtracts
	if task.DueDate != nil {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())
		daysUntil := int(dueDay.Sub(today).Hours() / 24)
		score += (-daysUntil) * 50
	}

	// In-progress bonus
	if task.Status == models.StatusInProgress {
		score += 500
	}

	// Review bonus (less than in-progress)
	if task.Status == models.StatusReview {
		score += 250
	}

	// Active flag bonus
	if task.Active {
		score += 200
	}

	return score
}

// computeSuggestionScore calculates the suggestion score for a task.
// Builds on urgencyScore with additional factors for actionability.
// Higher scores = more recommended as the suggested next task.
func (m *PriorityViewModel) computeSuggestionScore(task *models.Task) int {
	// Start with base urgency score
	score := m.computeUrgencyScore(task)

	// Leaf task bonus: prefer actionable leaf tasks over containers
	if task.IsLeaf() {
		score += 800
	}

	// Container penalty: penalize tasks with incomplete children
	if len(task.Children) > 0 && !m.allChildrenComplete(task) {
		score -= 1000
	}

	// Task age: older tasks get a small boost to avoid stagnation
	daysSinceCreated := int(time.Since(task.CreatedAt).Hours() / 24)
	ageBonus := daysSinceCreated / 7 // 30 points per week
	if ageBonus > 10 {
		ageBonus = 10 // Cap at 300 points
	}
	score += ageBonus * 30

	// Recent momentum: boost tasks with recent activity
	daysSinceUpdated := int(time.Since(task.UpdatedAt).Hours() / 24)
	if daysSinceUpdated == 0 {
		score += 400 // Updated today
	} else if daysSinceUpdated <= 3 {
		score += 200 // Updated within 3 days
	}

	// Unblocking bonus: prefer tasks that unblock others
	blockedByThisCount := len(m.project.GetBlockedByTasks(task))
	unblockingBonus := blockedByThisCount
	if unblockingBonus > 3 {
		unblockingBonus = 3 // Cap at 3
	}
	score += unblockingBonus * 150

	// Section penalty: deprioritize Next, Waiting, and Someday section tasks
	switch task.Section {
	case models.SectionNext:
		score -= 100
	case models.SectionWaiting:
		score -= 200
	case models.SectionSomeday:
		score -= 300
	}

	return score
}

// allChildrenComplete returns true if all children of the task are complete.
func (m *PriorityViewModel) allChildrenComplete(task *models.Task) bool {
	for _, child := range task.Children {
		if !child.Status.IsComplete() {
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
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

		if dueDay.Before(today) {
			reasons = append(reasons, "overdue")
		} else if dueDay.Equal(today) {
			reasons = append(reasons, "due today")
		} else if dueDay.Before(today.AddDate(0, 0, 7)) {
			reasons = append(reasons, "due this week")
		}
	}

	// In-progress status
	if task.Status == models.StatusInProgress {
		reasons = append(reasons, "in progress")
	}

	// Active flag
	if task.Active {
		reasons = append(reasons, "flagged active")
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
	daysSinceUpdated := int(time.Since(task.UpdatedAt).Hours() / 24)
	if daysSinceUpdated == 0 && task.Status != models.StatusInProgress {
		reasons = append(reasons, "recent activity")
	}

	// Composite score from scoring profile
	if m.config != nil {
		if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
			score := scoring.ComputeScore(task, profile)
			if int(score) > 0 {
				reasons = append(reasons, fmt.Sprintf("composite score %d", int(score)))
			}
		}
	}

	// If no specific reasons, provide a generic one
	if len(reasons) == 0 {
		return "Ready to work on"
	}

	// Join reasons with " + "
	result := ""
	for i, r := range reasons {
		if i > 0 {
			result += " + "
		}
		// Capitalize first reason
		if i == 0 {
			result += strings.ToUpper(r[:1]) + r[1:]
		} else {
			result += r
		}
	}
	return result
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
	if task.Status == models.StatusInProgress {
		// Find the task's position in the list and move cursor there
		m.jumpToTask(task)
		return m, nil
	}

	// Otherwise, start the task
	if err := storage.BackupProject(m.project); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
	}

	task.Start()

	if err := storage.SaveProject(m.project); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to save: %w", err)}
		}
	}

	// Reload and refresh
	_ = storage.LoadProject(m.project)
	m.loadActionableTasks()
	m.clampCursor()

	return m, func() tea.Msg {
		return TaskStatusChangedMsg{Task: task}
	}
}

// categorizeByDueDate determines which urgency group a task belongs to.
func (m *PriorityViewModel) categorizeByDueDate(task *models.Task) PriorityGroup {
	if task.DueDate == nil {
		return PriorityGroupBacklog
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

	if dueDay.Before(today) {
		return PriorityGroupOverdue
	}
	if dueDay.Equal(today) {
		return PriorityGroupToday
	}
	weekEnd := today.AddDate(0, 0, 7)
	if dueDay.Before(weekEnd) {
		return PriorityGroupThisWeek
	}
	return PriorityGroupLater
}
