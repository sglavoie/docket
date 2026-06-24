package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

func BenchmarkCrossProjectLoadAndRebuild(b *testing.B) {
	strPtr := func(s string) *string { return &s }

	alphaDir := b.TempDir()
	var alphaTasks []*models.Task
	seq := 1
	sections := []string{"", models.SectionNext, models.SectionInbox, models.SectionWaiting, models.SectionSomeday}
	for epicIdx := range 2 {
		epicID := fmt.Sprintf("alpha-epic-%03d", seq)
		epic, _ := models.NewTask(epicID, seq, models.TaskTypeEpic, fmt.Sprintf("Epic %d: Auth Module", epicIdx+1))
		seq++
		alphaTasks = append(alphaTasks, epic)

		storyID := fmt.Sprintf("alpha-story-%03d", seq)
		story, _ := models.NewTask(storyID, seq, models.TaskTypeStory, fmt.Sprintf("Story %d: Login Flow", epicIdx+1))
		story.ParentID = strPtr(epicID)
		seq++
		alphaTasks = append(alphaTasks, story)

		for leafIdx := range 3 {
			leafID := fmt.Sprintf("alpha-task-%03d", seq)
			leaf, _ := models.NewTask(leafID, seq, models.TaskTypeTask, fmt.Sprintf("Implement endpoint %d-%d", epicIdx+1, leafIdx+1))
			leaf.ParentID = strPtr(storyID)
			seq++
			alphaTasks = append(alphaTasks, leaf)
		}
	}
	for i := range 6 {
		id := fmt.Sprintf("alpha-task-%03d", seq)
		task, _ := models.NewTask(id, seq, models.TaskTypeTask, fmt.Sprintf("Alpha standalone %d", i+1))
		task.Section = sections[i%len(sections)]
		seq++
		alphaTasks = append(alphaTasks, task)
	}

	alpha := &models.Project{ID: "bench-alpha", Name: "Alpha", Path: alphaDir}
	alpha.Tasks = alphaTasks
	alpha.BuildTaskTree()
	if err := storage.SaveProject(alpha); err != nil {
		b.Fatalf("save alpha: %v", err)
	}

	betaDir := b.TempDir()
	var betaTasks []*models.Task
	seq = 1
	for parentIdx := range 2 {
		parentID := fmt.Sprintf("beta-story-%03d", seq)
		parent, _ := models.NewTask(parentID, seq, models.TaskTypeStory, fmt.Sprintf("Setup pipeline %d", parentIdx+1))
		seq++
		betaTasks = append(betaTasks, parent)

		for childIdx := range 3 {
			childID := fmt.Sprintf("beta-task-%03d", seq)
			child, _ := models.NewTask(childID, seq, models.TaskTypeTask, fmt.Sprintf("Configure step %d-%d", parentIdx+1, childIdx+1))
			child.ParentID = strPtr(parentID)
			seq++
			betaTasks = append(betaTasks, child)
		}
	}
	for i := range 12 {
		id := fmt.Sprintf("beta-task-%03d", seq)
		task, _ := models.NewTask(id, seq, models.TaskTypeTask, fmt.Sprintf("Beta standalone %d", i+1))
		task.Section = sections[i%len(sections)]
		seq++
		betaTasks = append(betaTasks, task)
	}

	beta := &models.Project{ID: "bench-beta", Name: "Beta", Path: betaDir}
	beta.Tasks = betaTasks
	beta.BuildTaskTree()
	if err := storage.SaveProject(beta); err != nil {
		b.Fatalf("save beta: %v", err)
	}

	b.ResetTimer()

	for b.Loop() {
		alphaLoad := &models.Project{ID: "bench-alpha", Name: "Alpha", Path: alphaDir}
		betaLoad := &models.Project{ID: "bench-beta", Name: "Beta", Path: betaDir}

		if err := storage.LoadProject(alphaLoad); err != nil {
			b.Fatalf("load alpha: %v", err)
		}
		if err := storage.LoadProject(betaLoad); err != nil {
			b.Fatalf("load beta: %v", err)
		}

		m := &OutlinerModel{
			collapsedSections: make(map[string]bool),
			expandedNodes:     make(map[string]bool),
			MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
			activePerspective: 1,
			loadedProjects:    []*models.Project{alphaLoad, betaLoad},
		}

		m.crossProjectRebuildRows()

		if len(m.rows) == 0 {
			b.Fatal("crossProjectRebuildRows produced no rows")
		}
	}
}

func BenchmarkCrossProjectRowBuilding(b *testing.B) {
	sections := []string{
		"", models.SectionInbox, models.SectionNext,
		models.SectionWaiting, models.SectionReview,
		models.SectionStandby, models.SectionSomeday,
	}

	const numProjects = 10
	const tasksPerProject = 100

	groups := make([]projectTasksGroup, numProjects)
	for p := range numProjects {
		projID := fmt.Sprintf("bench-proj-%02d", p)
		tasks := make([]*models.Task, 0, tasksPerProject)

		seq := 1
		for parentIdx := range 10 {
			parentID := fmt.Sprintf("%s-task-%03d", projID, seq)
			parent := makeTask(parentID, fmt.Sprintf("Parent %d-%d", p, parentIdx), sections[seq%len(sections)])
			parent.Status = models.StatusInProgress
			seq++
			tasks = append(tasks, parent)

			for childIdx := range 9 {
				childID := fmt.Sprintf("%s-task-%03d", projID, seq)
				child := makeTask(childID, fmt.Sprintf("Child %d-%d-%d", p, parentIdx, childIdx), sections[seq%len(sections)])
				child.Status = models.StatusInProgress
				seq++
				tasks = append(tasks, child)
			}
		}

		groups[p] = projectTasksGroup{projectID: projID, tasks: tasks}
	}

	b.Run("AllPerspective", func(b *testing.B) {
		for b.Loop() {
			m := &OutlinerModel{
				collapsedSections: make(map[string]bool),
				expandedNodes:     make(map[string]bool),
			}
			m.buildCrossProjectRowsWithSectionHeaders(groups)
			if len(m.rows) == 0 {
				b.Fatal("buildCrossProjectRowsWithSectionHeaders produced no rows")
			}
		}
	})

	b.Run("NonAllPerspective", func(b *testing.B) {
		for b.Loop() {
			m := &OutlinerModel{
				collapsedSections: make(map[string]bool),
				expandedNodes:     make(map[string]bool),
			}
			m.buildProjectGroupedRows(groups)
			if len(m.rows) == 0 {
				b.Fatal("buildProjectGroupedRows produced no rows")
			}
		}
	})
}

// BenchmarkRecommend exercises the cockpit's recommendation engine on a
// realistic cross-project payload. Today, Dashboard, and Pipeline Hub all
// invoke Recommend on every render, so this is the guard for cross-project
// loading cost called out in session 14. The benchmark seeds enough variety
// (inbox, planned-today, overdue, blocked, stale, recent) that no shortcut
// path dominates the run.
func BenchmarkRecommend(b *testing.B) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	today := util.StartOfDay(now)
	todayKey := today.Format(util.DateFormat)
	yesterdayKey := today.Add(-24 * time.Hour).Format(util.DateFormat)

	const numProjects = 10
	const tasksPerProject = 100

	sections := []string{
		"", models.SectionInbox, models.SectionNext,
		models.SectionWaiting, models.SectionReview,
	}

	projects := make([]*models.Project, numProjects)
	var plan []config.DailyPlanItem

	for p := range numProjects {
		projID := fmt.Sprintf("bench-rec-%02d", p)
		project := &models.Project{ID: projID, Name: fmt.Sprintf("Project %d", p)}
		tasks := make([]*models.Task, 0, tasksPerProject)

		for i := range tasksPerProject {
			id := fmt.Sprintf("%s-task-%04d", projID, i)
			task := makeTask(id, fmt.Sprintf("Task %d-%d", p, i), sections[i%len(sections)])
			task.Status = models.StatusInProgress
			task.UpdatedAt = now.Add(-time.Duration(i%30) * 24 * time.Hour)
			task.CreatedAt = task.UpdatedAt

			switch i % 7 {
			case 0:
				due := today.Add(-48 * time.Hour)
				task.DueDate = &due
			case 1:
				due := today
				task.DueDate = &due
			case 2:
				task.Priority = 1
			case 3:
				task.Status = models.StatusBlocked
			case 4:
				plan = append(plan, config.DailyPlanItem{
					ProjectID:   projID,
					TaskID:      id,
					PlannedDate: todayKey,
				})
			case 5:
				plan = append(plan, config.DailyPlanItem{
					ProjectID:   projID,
					TaskID:      id,
					PlannedDate: yesterdayKey,
				})
			}

			tasks = append(tasks, task)
		}

		project.Tasks = tasks
		project.BuildTaskTree()
		projects[p] = project
	}

	input := RecommendationEngineInput{
		Projects:  projects,
		DailyPlan: plan,
		Now:       now,
	}

	b.ResetTimer()

	for b.Loop() {
		got := Recommend(input)
		if got.Kind == RecommendationKindNone {
			b.Fatal("Recommend returned RecommendationKindNone for non-empty input")
		}
	}
}
