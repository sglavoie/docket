package focus

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

func TestComputeCrossProjectScore(t *testing.T) {
	now := time.Now()
	project := models.NewProject(uuid.New().String(), "Test", "")

	t.Run("higher priority scores higher", func(t *testing.T) {
		p1 := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Critical")
		p1.Priority = 1
		p5 := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Low")
		p5.Priority = 5

		s1 := computeCrossProjectScore(p1, project, nil, now)
		s5 := computeCrossProjectScore(p5, project, nil, now)
		if s1 <= s5 {
			t.Errorf("priority 1 score (%d) should be > priority 5 score (%d)", s1, s5)
		}
	})

	t.Run("in-progress bonus", func(t *testing.T) {
		todo := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Todo")
		todo.Priority = 3

		inProgress := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "In Progress")
		inProgress.Priority = 3
		inProgress.Status = models.StatusInProgress

		sTodo := computeCrossProjectScore(todo, project, nil, now)
		sIP := computeCrossProjectScore(inProgress, project, nil, now)
		if sIP <= sTodo {
			t.Errorf("in-progress score (%d) should be > todo score (%d)", sIP, sTodo)
		}
	})

	t.Run("overdue due date boosts score", func(t *testing.T) {
		noDue := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "No due")
		noDue.Priority = 3

		overdue := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Overdue")
		overdue.Priority = 3
		yesterday := time.Now().AddDate(0, 0, -1)
		overdue.DueDate = &yesterday

		sNoDue := computeCrossProjectScore(noDue, project, nil, now)
		sOverdue := computeCrossProjectScore(overdue, project, nil, now)
		if sOverdue <= sNoDue {
			t.Errorf("overdue score (%d) should be > no-due score (%d)", sOverdue, sNoDue)
		}
	})

	t.Run("bug type bonus", func(t *testing.T) {
		task := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Task")
		task.Priority = 3

		bug := models.NewTask(uuid.New().String(), 2, models.TaskTypeBug, "Bug")
		bug.Priority = 3

		sTask := computeCrossProjectScore(task, project, nil, now)
		sBug := computeCrossProjectScore(bug, project, nil, now)
		if sBug <= sTask {
			t.Errorf("bug score (%d) should be > task score (%d)", sBug, sTask)
		}
	})
}

func TestRankCrossProjectTasks(t *testing.T) {
	t.Run("excludes dismissed tasks", func(t *testing.T) {
		project := models.NewProject(uuid.New().String(), "Test", "")
		t1 := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Task 1")
		t2 := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Task 2")

		tasks := []FocusTask{
			{Task: t1, Project: project},
			{Task: t2, Project: project},
		}
		dismissed := map[string]bool{t1.ID: true}

		ranked := RankCrossProjectTasks(tasks, dismissed, nil)
		if len(ranked) != 1 {
			t.Fatalf("expected 1 ranked task, got %d", len(ranked))
		}
		if ranked[0].Task.ID != t2.ID {
			t.Errorf("expected task 2, got %s", ranked[0].Task.Title)
		}
	})

	t.Run("balance factor penalizes repeated projects", func(t *testing.T) {
		p1 := models.NewProject(uuid.New().String(), "Project A", "")
		p2 := models.NewProject(uuid.New().String(), "Project B", "")

		// Create 3 same-priority tasks from p1 and 1 from p2
		t1a := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "P1 Task A")
		t1a.Priority = 2
		t1b := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "P1 Task B")
		t1b.Priority = 2
		t1c := models.NewTask(uuid.New().String(), 3, models.TaskTypeTask, "P1 Task C")
		t1c.Priority = 2
		t2a := models.NewTask(uuid.New().String(), 4, models.TaskTypeTask, "P2 Task A")
		t2a.Priority = 2

		tasks := []FocusTask{
			{Task: t1a, Project: p1},
			{Task: t1b, Project: p1},
			{Task: t1c, Project: p1},
			{Task: t2a, Project: p2},
		}

		ranked := RankCrossProjectTasks(tasks, nil, nil)
		if len(ranked) != 4 {
			t.Fatalf("expected 4 ranked tasks, got %d", len(ranked))
		}

		// P2's task should appear before P1's 3rd task due to balance penalty.
		p2Idx := -1
		for i, r := range ranked {
			if r.Project.ID == p2.ID {
				p2Idx = i
				break
			}
		}
		if p2Idx >= 3 {
			t.Errorf("p2 task should be interleaved (idx=%d), not last", p2Idx)
		}
	})

	t.Run("sorted descending by score", func(t *testing.T) {
		project := models.NewProject(uuid.New().String(), "Test", "")
		high := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "High")
		high.Priority = 1
		low := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Low")
		low.Priority = 5

		tasks := []FocusTask{
			{Task: low, Project: project},
			{Task: high, Project: project},
		}

		ranked := RankCrossProjectTasks(tasks, nil, nil)
		if len(ranked) < 2 {
			t.Fatalf("expected 2 ranked tasks, got %d", len(ranked))
		}
		if ranked[0].Score < ranked[1].Score {
			t.Errorf("first score (%d) should be >= second score (%d)", ranked[0].Score, ranked[1].Score)
		}
	})
}

func TestGetSuggestionReason(t *testing.T) {
	now := time.Now()
	project := models.NewProject(uuid.New().String(), "Test", "")

	t.Run("critical priority", func(t *testing.T) {
		task := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Critical")
		task.Priority = 1
		reason := getSuggestionReason(task, project, nil, now)
		if !containsSubstring(reason, "ritical priority") {
			t.Errorf("expected 'critical priority' in reason, got %q", reason)
		}
	})

	t.Run("overdue", func(t *testing.T) {
		task := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Overdue")
		task.Priority = 3
		yesterday := time.Now().AddDate(0, 0, -1)
		task.DueDate = &yesterday
		reason := getSuggestionReason(task, project, nil, now)
		if !containsSubstring(reason, "verdue") {
			t.Errorf("expected 'overdue' in reason, got %q", reason)
		}
	})

	t.Run("default reason for generic task", func(t *testing.T) {
		task := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Generic")
		task.Priority = 3
		task.CreatedAt = time.Now().AddDate(0, -1, 0)
		task.UpdatedAt = time.Now().AddDate(0, -1, 0)
		reason := getSuggestionReason(task, project, nil, now)
		if reason == "" {
			t.Error("expected non-empty reason")
		}
	})
}

func TestAllChildrenComplete(t *testing.T) {
	t.Run("no children returns true", func(t *testing.T) {
		task := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Leaf")
		if !allChildrenComplete(task) {
			t.Error("expected true for task with no children")
		}
	})

	t.Run("all done returns true", func(t *testing.T) {
		parent := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Parent")
		child := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Child")
		child.Status = models.StatusDone
		parent.Children = []*models.Task{child}
		if !allChildrenComplete(parent) {
			t.Error("expected true when all children are done")
		}
	})

	t.Run("incomplete child returns false", func(t *testing.T) {
		parent := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Parent")
		child := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Child")
		parent.Children = []*models.Task{child}
		if allChildrenComplete(parent) {
			t.Error("expected false when child is not complete")
		}
	})
}

func TestScoringFactors(t *testing.T) {
	now := time.Now()
	project := models.NewProject(uuid.New().String(), "Test", "")

	baseTask := func() *models.Task {
		task := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Base")
		task.Priority = 3
		task.CreatedAt = time.Now().Add(-48 * time.Hour)
		task.UpdatedAt = time.Now().Add(-48 * time.Hour)
		return task
	}

	baseScore := computeCrossProjectScore(baseTask(), project, nil, now)

	t.Run("leaf task bonus", func(t *testing.T) {
		leaf := baseTask()
		leafScore := computeCrossProjectScore(leaf, project, nil, now)
		container := baseTask()
		child := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Child")
		container.Children = []*models.Task{child}
		containerScore := computeCrossProjectScore(container, project, nil, now)
		if leafScore <= containerScore {
			t.Errorf("leaf score (%d) should be > container score (%d)", leafScore, containerScore)
		}
	})

	t.Run("container penalty with incomplete children", func(t *testing.T) {
		container := baseTask()
		child := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Child")
		container.Children = []*models.Task{child}
		containerScore := computeCrossProjectScore(container, project, nil, now)
		if containerScore >= baseScore {
			t.Errorf("container score (%d) should be < leaf base score (%d)", containerScore, baseScore)
		}
	})

	t.Run("section scoring ordering", func(t *testing.T) {
		next := baseTask()
		next.Section = models.SectionNext
		nextScore := computeCrossProjectScore(next, project, nil, now)

		inbox := baseTask()
		inboxScore := computeCrossProjectScore(inbox, project, nil, now)

		waiting := baseTask()
		waiting.Section = models.SectionWaiting
		waitingScore := computeCrossProjectScore(waiting, project, nil, now)

		someday := baseTask()
		someday.Section = models.SectionSomeday
		somedayScore := computeCrossProjectScore(someday, project, nil, now)

		if nextScore <= inboxScore {
			t.Errorf("Next score (%d) should be > Inbox score (%d)", nextScore, inboxScore)
		}
		if inboxScore <= waitingScore {
			t.Errorf("Inbox score (%d) should be > Waiting score (%d)", inboxScore, waitingScore)
		}
		if waitingScore <= somedayScore {
			t.Errorf("Waiting score (%d) should be > Someday score (%d)", waitingScore, somedayScore)
		}
	})

	t.Run("unblocking bonus", func(t *testing.T) {
		blocker := baseTask()
		project.Tasks = []*models.Task{blocker}

		blocked := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Blocked")
		blocked.DependsOn = []string{blocker.ID}
		project.Tasks = append(project.Tasks, blocked)
		project.BuildTaskTree()

		blockerScore := computeCrossProjectScore(blocker, project, nil, now)
		nonBlocker := baseTask()
		nonBlockerScore := computeCrossProjectScore(nonBlocker, project, nil, now)
		if blockerScore <= nonBlockerScore {
			t.Errorf("blocker score (%d) should be > non-blocker score (%d)", blockerScore, nonBlockerScore)
		}
		// Clean up
		project.Tasks = nil
	})

	t.Run("active flag bonus", func(t *testing.T) {
		active := baseTask()
		active.Active = true
		activeScore := computeCrossProjectScore(active, project, nil, now)
		if activeScore <= baseScore {
			t.Errorf("active score (%d) should be > base score (%d)", activeScore, baseScore)
		}
	})

	t.Run("review status bonus", func(t *testing.T) {
		review := baseTask()
		review.Status = models.StatusReview
		reviewScore := computeCrossProjectScore(review, project, nil, now)
		if reviewScore <= baseScore {
			t.Errorf("review score (%d) should be > base score (%d)", reviewScore, baseScore)
		}
	})

	t.Run("composite scoring profile integration", func(t *testing.T) {
		cfg := config.NewConfig()
		profile := scoring.ScoringProfile{
			ID:      "test-profile",
			Name:    "Test",
			Formula: scoring.FormulaWeightedSum,
			Dimensions: []scoring.ScoringDimension{
				{
					ID:      "value",
					Name:    "Value",
					Min:     0,
					Max:     10,
					Default: 0,
					Weight:  1.0,
					Source:  scoring.SourceManual,
				},
			},
		}
		cfg.ScoringProfiles = []scoring.ScoringProfile{profile}
		cfg.ProjectScoringProfile = map[string]string{project.ID: "test-profile"}

		scored := baseTask()
		scored.ScoreDimensions = map[string]int{"value": 8}
		scoredScore := computeCrossProjectScore(scored, project, cfg, now)

		unscored := baseTask()
		unscoredScore := computeCrossProjectScore(unscored, project, cfg, now)

		if scoredScore <= unscoredScore {
			t.Errorf("scored task (%d) should be > unscored task (%d)", scoredScore, unscoredScore)
		}
	})
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
