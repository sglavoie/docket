package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
		parentID := fmt.Sprintf("beta-task-%03d", seq)
		parent, _ := models.NewTask(parentID, seq, models.TaskTypeTask, fmt.Sprintf("Setup pipeline %d", parentIdx+1))
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