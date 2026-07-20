package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
)

// computeSuggestion selects the smart suggestion as the first non-dismissed
// task of the shared cross-project focus ranking. Using
// focus.RankCrossProjectTasks (the same engine that backs the Planner Backlog
// and the By-Score tab) guarantees the Priority suggestion recommends the same
// task the shared ordering ranks first — there is no second scoring or reason
// heuristic in the TUI package. The candidate set is the actionable groups
// (overdue, today, this week, later, backlog), excluding blocked/waiting tasks.
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

	byID := make(map[string]*PriorityTask, len(candidates))
	focusTasks := make([]focus.FocusTask, 0, len(candidates))
	for _, pt := range candidates {
		byID[pt.Task.ID] = pt
		focusTasks = append(focusTasks, focus.FocusTask{Task: pt.Task, Project: m.project})
	}

	// RankCrossProjectTasks drops dismissed IDs and sorts by the shared score;
	// the first survivor whose task is in our candidate set is the suggestion.
	ranked := focus.RankCrossProjectTasks(focusTasks, m.dismissedIDs, m.runtime)
	for _, sft := range ranked {
		pt := byID[sft.Task.ID]
		if pt == nil {
			continue
		}
		m.suggestion = &SuggestedTask{
			Task:   pt,
			Score:  sft.Score,
			Reason: sft.Reason,
		}
		return
	}
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
