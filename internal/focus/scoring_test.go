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
	project, err := models.NewProject(uuid.New().String(), "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("higher priority scores higher", func(t *testing.T) {
		p1, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Critical")
		if err != nil {
			t.Fatal(err)
		}
		p1.Priority = 1
		p5, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Low")
		if err != nil {
			t.Fatal(err)
		}
		p5.Priority = 5

		s1 := computeCrossProjectScore(p1, project, nil, now)
		s5 := computeCrossProjectScore(p5, project, nil, now)
		if s1 <= s5 {
			t.Errorf("priority 1 score (%d) should be > priority 5 score (%d)", s1, s5)
		}
	})

	t.Run("malformed priority is clamped", func(t *testing.T) {
		critical, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Critical")
		if err != nil {
			t.Fatal(err)
		}
		critical.Priority = models.PriorityMin

		tooHigh, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Too high")
		if err != nil {
			t.Fatal(err)
		}
		tooHigh.Priority = models.PriorityMin - 1

		low, err := models.NewTask(uuid.New().String(), 3, models.TaskTypeTask, "Low")
		if err != nil {
			t.Fatal(err)
		}
		low.Priority = models.PriorityMax

		tooLow, err := models.NewTask(uuid.New().String(), 4, models.TaskTypeTask, "Too low")
		if err != nil {
			t.Fatal(err)
		}
		tooLow.Priority = models.PriorityMax + 1

		criticalScore := computeCrossProjectScore(critical, project, nil, now)
		tooHighScore := computeCrossProjectScore(tooHigh, project, nil, now)
		if tooHighScore != criticalScore {
			t.Errorf("priority below min should score like priority %d: got %d, want %d", models.PriorityMin, tooHighScore, criticalScore)
		}

		lowScore := computeCrossProjectScore(low, project, nil, now)
		tooLowScore := computeCrossProjectScore(tooLow, project, nil, now)
		if tooLowScore != lowScore {
			t.Errorf("priority above max should score like priority %d: got %d, want %d", models.PriorityMax, tooLowScore, lowScore)
		}
	})

	t.Run("in-progress bonus", func(t *testing.T) {
		todo, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Todo")
		if err != nil {
			t.Fatal(err)
		}
		todo.Priority = 3

		inProgress, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "In Progress")
		if err != nil {
			t.Fatal(err)
		}
		inProgress.Priority = 3
		inProgress.Status = models.StatusInProgress

		sTodo := computeCrossProjectScore(todo, project, nil, now)
		sIP := computeCrossProjectScore(inProgress, project, nil, now)
		if sIP <= sTodo {
			t.Errorf("in-progress score (%d) should be > todo score (%d)", sIP, sTodo)
		}
	})

	t.Run("overdue due date boosts score", func(t *testing.T) {
		noDue, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "No due")
		if err != nil {
			t.Fatal(err)
		}
		noDue.Priority = 3

		overdue, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Overdue")
		if err != nil {
			t.Fatal(err)
		}
		overdue.Priority = 3
		yesterday := time.Now().AddDate(0, 0, -1)
		overdue.DueDate = &yesterday

		sNoDue := computeCrossProjectScore(noDue, project, nil, now)
		sOverdue := computeCrossProjectScore(overdue, project, nil, now)
		if sOverdue <= sNoDue {
			t.Errorf("overdue score (%d) should be > no-due score (%d)", sOverdue, sNoDue)
		}
	})

	t.Run("upcoming due dates boost score instead of penalizing", func(t *testing.T) {
		baseTask := func(title string) *models.Task {
			task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
			if err != nil {
				t.Fatal(err)
			}
			task.Priority = 3
			task.CreatedAt = now
			task.UpdatedAt = now
			return task
		}

		noDue := baseTask("No due")
		tomorrow := baseTask("Tomorrow")
		tomorrowDue := now.AddDate(0, 0, 1)
		tomorrow.DueDate = &tomorrowDue
		nextWeek := baseTask("Next week")
		nextWeekDue := now.AddDate(0, 0, 7)
		nextWeek.DueDate = &nextWeekDue
		farFuture := baseTask("Far future")
		farFutureDue := now.AddDate(0, 0, 30)
		farFuture.DueDate = &farFutureDue

		noDueScore := computeCrossProjectScore(noDue, project, nil, now)
		tomorrowScore := computeCrossProjectScore(tomorrow, project, nil, now)
		nextWeekScore := computeCrossProjectScore(nextWeek, project, nil, now)
		farFutureScore := computeCrossProjectScore(farFuture, project, nil, now)

		if tomorrowScore <= noDueScore {
			t.Errorf("tomorrow due score (%d) should be > no-due score (%d)", tomorrowScore, noDueScore)
		}
		if nextWeekScore <= noDueScore {
			t.Errorf("next-week due score (%d) should be > no-due score (%d)", nextWeekScore, noDueScore)
		}
		if farFutureScore != noDueScore {
			t.Errorf("far-future due score (%d) should be neutral like no due (%d)", farFutureScore, noDueScore)
		}
	})

	t.Run("bug type bonus", func(t *testing.T) {
		task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Task")
		if err != nil {
			t.Fatal(err)
		}
		task.Priority = 3

		bug, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeBug, "Bug")
		if err != nil {
			t.Fatal(err)
		}
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
		project, err := models.NewProject(uuid.New().String(), "Test", "")
		if err != nil {
			t.Fatal(err)
		}
		t1, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Task 1")
		if err != nil {
			t.Fatal(err)
		}
		t2, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Task 2")
		if err != nil {
			t.Fatal(err)
		}

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
		p1, err := models.NewProject(uuid.New().String(), "Project A", "")
		if err != nil {
			t.Fatal(err)
		}
		p2, err := models.NewProject(uuid.New().String(), "Project B", "")
		if err != nil {
			t.Fatal(err)
		}

		// Create 3 same-priority tasks from p1 and 1 from p2
		t1a, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "P1 Task A")
		if err != nil {
			t.Fatal(err)
		}
		t1a.Priority = 2
		t1b, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "P1 Task B")
		if err != nil {
			t.Fatal(err)
		}
		t1b.Priority = 2
		t1c, err := models.NewTask(uuid.New().String(), 3, models.TaskTypeTask, "P1 Task C")
		if err != nil {
			t.Fatal(err)
		}
		t1c.Priority = 2
		t2a, err := models.NewTask(uuid.New().String(), 4, models.TaskTypeTask, "P2 Task A")
		if err != nil {
			t.Fatal(err)
		}
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
		project, err := models.NewProject(uuid.New().String(), "Test", "")
		if err != nil {
			t.Fatal(err)
		}
		high, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "High")
		if err != nil {
			t.Fatal(err)
		}
		high.Priority = 1
		low, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Low")
		if err != nil {
			t.Fatal(err)
		}
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

func TestAllChildrenComplete(t *testing.T) {
	t.Run("no children returns true", func(t *testing.T) {
		task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Leaf")
		if err != nil {
			t.Fatal(err)
		}
		if !allChildrenComplete(task) {
			t.Error("expected true for task with no children")
		}
	})

	t.Run("all done returns true", func(t *testing.T) {
		parent, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Parent")
		if err != nil {
			t.Fatal(err)
		}
		child, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Child")
		if err != nil {
			t.Fatal(err)
		}
		child.Status = models.StatusDone
		parent.Children = []*models.Task{child}
		if !allChildrenComplete(parent) {
			t.Error("expected true when all children are done")
		}
	})

	t.Run("incomplete child returns false", func(t *testing.T) {
		parent, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Parent")
		if err != nil {
			t.Fatal(err)
		}
		child, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Child")
		if err != nil {
			t.Fatal(err)
		}
		parent.Children = []*models.Task{child}
		if allChildrenComplete(parent) {
			t.Error("expected false when child is not complete")
		}
	})
}

func TestScoringFactors(t *testing.T) {
	now := time.Now()
	project, err := models.NewProject(uuid.New().String(), "Test", "")
	if err != nil {
		t.Fatal(err)
	}

	baseTask := func() *models.Task {
		task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Base")
		if err != nil {
			t.Fatal(err)
		}
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
		child, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Child")
		if err != nil {
			t.Fatal(err)
		}
		container.Children = []*models.Task{child}
		containerScore := computeCrossProjectScore(container, project, nil, now)
		if leafScore <= containerScore {
			t.Errorf("leaf score (%d) should be > container score (%d)", leafScore, containerScore)
		}
	})

	t.Run("container penalty with incomplete children", func(t *testing.T) {
		container := baseTask()
		child, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Child")
		if err != nil {
			t.Fatal(err)
		}
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

		review := baseTask()
		review.Section = models.SectionReview
		reviewScore := computeCrossProjectScore(review, project, nil, now)

		standby := baseTask()
		standby.Section = models.SectionStandby
		standbyScore := computeCrossProjectScore(standby, project, nil, now)

		someday := baseTask()
		someday.Section = models.SectionSomeday
		somedayScore := computeCrossProjectScore(someday, project, nil, now)

		if nextScore <= inboxScore {
			t.Errorf("Next score (%d) should be > Inbox score (%d)", nextScore, inboxScore)
		}
		if inboxScore <= waitingScore {
			t.Errorf("Inbox score (%d) should be > Waiting score (%d)", inboxScore, waitingScore)
		}
		if reviewScore != waitingScore {
			t.Errorf("Review score (%d) should equal Waiting score (%d)", reviewScore, waitingScore)
		}
		if waitingScore <= standbyScore {
			t.Errorf("Waiting score (%d) should be > Standby score (%d)", waitingScore, standbyScore)
		}
		if standbyScore <= somedayScore {
			t.Errorf("Standby score (%d) should be > Someday score (%d)", standbyScore, somedayScore)
		}
	})

	t.Run("unblocking bonus", func(t *testing.T) {
		blocker := baseTask()
		project.Tasks = []*models.Task{blocker}

		blocked, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Blocked")
		if err != nil {
			t.Fatal(err)
		}
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

	t.Run("unblocking bonus excludes completed dependents", func(t *testing.T) {
		// Regression: GetBlockedByTasks returns all dependents regardless of
		// status, but a completed dependent does not need unblocking. The
		// score and reason must only count open dependents.
		blockerProject, err := models.NewProject(uuid.New().String(), "Blocker", "")
		if err != nil {
			t.Fatal(err)
		}

		blocker := baseTask()
		// Two dependents: one done, one open.
		doneDep, err := models.NewTask(uuid.New().String(), 2, models.TaskTypeTask, "Done dep")
		if err != nil {
			t.Fatal(err)
		}
		doneDep.DependsOn = []string{blocker.ID}
		doneDep.Status = models.StatusDone

		openDep, err := models.NewTask(uuid.New().String(), 3, models.TaskTypeTask, "Open dep")
		if err != nil {
			t.Fatal(err)
		}
		openDep.DependsOn = []string{blocker.ID}

		blockerProject.Tasks = []*models.Task{blocker, doneDep, openDep}
		blockerProject.BuildTaskTree()

		gotScore := computeCrossProjectScore(blocker, blockerProject, nil, now)

		// Compare against the same blocker with only the open dep (no done dep).
		blockerProject2, err := models.NewProject(uuid.New().String(), "Blocker2", "")
		if err != nil {
			t.Fatal(err)
		}
		blocker2 := baseTask()
		openDep2, err := models.NewTask(uuid.New().String(), 3, models.TaskTypeTask, "Open dep 2")
		if err != nil {
			t.Fatal(err)
		}
		openDep2.DependsOn = []string{blocker2.ID}
		blockerProject2.Tasks = []*models.Task{blocker2, openDep2}
		blockerProject2.BuildTaskTree()
		baselineScore := computeCrossProjectScore(blocker2, blockerProject2, nil, now)

		if gotScore != baselineScore {
			t.Errorf("score with done+open dep (%d) should equal score with just open dep (%d); completed dependents must not contribute to unblocking bonus", gotScore, baselineScore)
		}

		reason := getSuggestionReason(blocker, blockerProject, nil, now)
		if !containsSubstring(reason, "nblocks 1 task") {
			t.Errorf("expected 'unblocks 1 task' (only the open dep), got %q", reason)
		}
		if containsSubstring(reason, "unblocks 2 tasks") || containsSubstring(reason, "Unblocks 2 tasks") {
			t.Errorf("must not count completed dependent, got %q", reason)
		}

		// Sanity: blocker with only completed dependents should not list
		// "unblocks" at all, since no work is being freed.
		soloProject, err := models.NewProject(uuid.New().String(), "Solo", "")
		if err != nil {
			t.Fatal(err)
		}
		soloBlocker := baseTask()
		soloDoneDep, err := models.NewTask(uuid.New().String(), 4, models.TaskTypeTask, "Solo done dep")
		if err != nil {
			t.Fatal(err)
		}
		soloDoneDep.DependsOn = []string{soloBlocker.ID}
		soloDoneDep.Status = models.StatusDone
		soloProject.Tasks = []*models.Task{soloBlocker, soloDoneDep}
		soloProject.BuildTaskTree()

		soloReason := getSuggestionReason(soloBlocker, soloProject, nil, now)
		if containsSubstring(soloReason, "unblocks") {
			t.Errorf("blocker with only completed dependents should not list 'unblocks', got %q", soloReason)
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
