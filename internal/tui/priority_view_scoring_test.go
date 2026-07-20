package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
)

func TestComputeUrgencyScore(t *testing.T) {
	tests := []struct {
		name    string
		task    *models.Task
		inQueue bool
		wantMin int
		wantMax int
	}{
		{
			name: "priority 1 base score",
			task: func() *models.Task {
				task, err := models.NewTask("1", 1, models.TaskTypeTask, "High priority")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 1
				return task
			}(),
			wantMin: 5000,
			wantMax: 5000,
		},
		{
			name: "priority 5 base score",
			task: func() *models.Task {
				task, err := models.NewTask("2", 2, models.TaskTypeTask, "Low priority")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 5
				return task
			}(),
			wantMin: 1000,
			wantMax: 1000,
		},
		{
			name: "priority 3 base score",
			task: func() *models.Task {
				task, err := models.NewTask("3", 3, models.TaskTypeTask, "Medium priority")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				return task
			}(),
			wantMin: 3000,
			wantMax: 3000,
		},
		{
			name: "in progress bonus",
			task: func() *models.Task {
				task, err := models.NewTask("4", 4, models.TaskTypeTask, "In progress task")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				task.WorkflowState = models.StateInProgress
				return task
			}(),
			wantMin: 3500,
			wantMax: 3500,
		},
		{
			name: "focus queue bonus",
			task: func() *models.Task {
				task, err := models.NewTask("5", 5, models.TaskTypeTask, "Queued task")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				return task
			}(),
			inQueue: true,
			wantMin: 3200,
			wantMax: 3200,
		},
		{
			name: "combined bonuses",
			task: func() *models.Task {
				task, err := models.NewTask("6", 6, models.TaskTypeTask, "Queued in progress")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 1
				task.WorkflowState = models.StateInProgress
				return task
			}(),
			inQueue: true,
			wantMin: 5700,
			wantMax: 5700,
		},
		{
			name: "bug type bonus",
			task: func() *models.Task {
				task, err := models.NewTask("7", 7, models.TaskTypeBug, "Bug task")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				return task
			}(),
			wantMin: 3100,
			wantMax: 3100,
		},
		{
			name: "bug outranks task at same priority",
			task: func() *models.Task {
				task, err := models.NewTask("8", 8, models.TaskTypeBug, "Bug at priority 3")
				if err != nil {
					t.Fatal(err)
				}
				task.Priority = 3
				return task
			}(),
			wantMin: 3100,
			wantMax: 3100,
		},
		{
			name: "overdue adds points",
			task: func() *models.Task {
				task := newTestTaskWithDueDate(t, "Overdue task", -2)
				task.Priority = 3
				return task
			}(),
			wantMin: 3100,
			wantMax: 3100,
		},
		{
			name: "future due subtracts points",
			task: func() *models.Task {
				task := newTestTaskWithDueDate(t, "Future task", 2)
				task.Priority = 3
				return task
			}(),
			wantMin: 2900,
			wantMax: 2900,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeUrgencyScore(tt.task, tt.inQueue)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("computeUrgencyScore() = %d, want between %d and %d", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// suggestionCandidates mirrors computeSuggestion's candidate set (the
// actionable groups, excluding blocked/waiting) as FocusTasks so a test can
// independently rank them with the shared engine.
func suggestionCandidates(m *PriorityViewModel) []focus.FocusTask {
	var pts []*PriorityTask
	pts = append(pts, m.overdue...)
	pts = append(pts, m.today...)
	pts = append(pts, m.thisWeek...)
	pts = append(pts, m.later...)
	pts = append(pts, m.backlog...)

	fts := make([]focus.FocusTask, 0, len(pts))
	for _, pt := range pts {
		fts = append(fts, focus.FocusTask{Task: pt.Task, Project: m.project})
	}
	return fts
}

// TestPrioritySuggestionUsesSharedFocusRanking proves the Priority smart
// suggestion is the first non-dismissed task of the shared cross-project focus
// ranking (focus.RankCrossProjectTasks) rather than a divergent second
// heuristic — score and reason included — before and after a dismissal.
func TestPrioritySuggestionUsesSharedFocusRanking(t *testing.T) {
	project := newTestPriorityProject(t)

	mkTask := func(id string, priority int, taskType models.TaskType, title string) *models.Task {
		task, err := models.NewTask(id, priority, taskType, title)
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = priority
		return task
	}

	// A spread of eligible tasks with different priorities/types so the shared
	// ranker yields a strict ordering.
	high := mkTask("1", 1, models.TaskTypeBug, "Critical bug")
	mid := mkTask("2", 3, models.TaskTypeTask, "Medium task")
	low := mkTask("3", 5, models.TaskTypeTask, "Low task")

	project.Tasks = []*models.Task{low, mid, high}

	runtime := config.EmptyRuntimeState("")
	m := NewPriorityViewModel(project, runtime, nil)

	// Independently rank the same candidate set with the shared engine.
	ranked := focus.RankCrossProjectTasks(suggestionCandidates(m), nil, runtime)
	if len(ranked) == 0 {
		t.Fatal("expected the shared ranker to return candidates")
	}
	if m.suggestion == nil {
		t.Fatal("expected a smart suggestion")
	}
	if m.suggestion.Task.Task.ID != ranked[0].Task.ID {
		t.Errorf("suggestion task %q != first shared-ranked task %q",
			m.suggestion.Task.Task.ID, ranked[0].Task.ID)
	}
	if m.suggestion.Score != ranked[0].Score {
		t.Errorf("suggestion score %d != shared score %d", m.suggestion.Score, ranked[0].Score)
	}
	if m.suggestion.Reason != ranked[0].Reason {
		t.Errorf("suggestion reason %q != shared reason %q", m.suggestion.Reason, ranked[0].Reason)
	}

	// Dismissing the current suggestion must advance to the next non-dismissed
	// shared-ranked candidate.
	m.dismissSuggestion()
	rankedAfter := focus.RankCrossProjectTasks(suggestionCandidates(m), m.dismissedIDs, runtime)
	if len(rankedAfter) == 0 {
		t.Fatal("expected remaining candidates after dismiss")
	}
	if m.suggestion == nil {
		t.Fatal("expected a suggestion after dismiss")
	}
	if m.suggestion.Task.Task.ID != rankedAfter[0].Task.ID {
		t.Errorf("after dismiss, suggestion %q != first non-dismissed shared-ranked %q",
			m.suggestion.Task.Task.ID, rankedAfter[0].Task.ID)
	}
}

func TestCategorizeByDueDate(t *testing.T) {
	tests := []struct {
		name       string
		daysOffset *int
		want       PriorityGroup
	}{
		{"overdue yesterday", intPtr(-1), PriorityGroupOverdue},
		{"overdue 10 days ago", intPtr(-10), PriorityGroupOverdue},
		{"due today", intPtr(0), PriorityGroupToday},
		{"due tomorrow", intPtr(1), PriorityGroupThisWeek},
		{"due in 3 days", intPtr(3), PriorityGroupThisWeek},
		{"due in 6 days", intPtr(6), PriorityGroupThisWeek},
		{"due in 7 days", intPtr(7), PriorityGroupLater},
		{"due in 14 days", intPtr(14), PriorityGroupLater},
		{"no due date", nil, PriorityGroupBacklog},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
			if err != nil {
				t.Fatal(err)
			}
			if tt.daysOffset != nil {
				due := time.Now().AddDate(0, 0, *tt.daysOffset)
				task.DueDate = &due
			}
			got := CategorizeByDueDate(task)
			if got != tt.want {
				t.Errorf("categorizeByDueDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatRelativeDue(t *testing.T) {
	tests := []struct {
		name       string
		daysOffset *int
		wantMatch  string
	}{
		{"nil due date", nil, ""},
		{"yesterday", intPtr(-1), "yesterday"},
		{"2 days ago", intPtr(-2), "days ago"},
		{"10 days ago", intPtr(-10), "days ago"},
		{"today", intPtr(0), "today"},
		{"tomorrow", intPtr(1), "tomorrow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
			if err != nil {
				t.Fatal(err)
			}
			if tt.daysOffset != nil {
				due := time.Now().AddDate(0, 0, *tt.daysOffset)
				task.DueDate = &due
			}

			got := FormatRelativeDue(task.DueDate, false)
			if tt.wantMatch == "" {
				if got != "" {
					t.Errorf("FormatRelativeDue() = %q, want empty", got)
				}
				return
			}
			if got != tt.wantMatch && !strings.Contains(got, tt.wantMatch) {
				t.Errorf("FormatRelativeDue() = %q, want to contain %q", got, tt.wantMatch)
			}
		})
	}
}
