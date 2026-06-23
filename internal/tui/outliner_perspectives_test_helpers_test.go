package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// buildTestProject creates a project with a tree of tasks for perspective testing.
// The tree structure:
//
//	epic (todo, priority 3, tags: ["backend"])
//	  ├── storyA (in_progress, priority 1)
//	  │   ├── taskA1 (todo, priority 1)
//	  │   └── taskA2 (done, priority 2)
//	  └── storyB (blocked, priority 4)
//	      └── taskB1 (in_review, priority 5)
//	bugRoot (todo, priority 2, no tags, no parent)
func buildTestProject() *models.Project {
	epic := &models.Task{
		ID:       "epic",
		ShortID:  1,
		Type:     models.TaskTypeEpic,
		Title:    "Epic",
		Status:   models.StatusTodo,
		Priority: 3,
		Tags:     []string{"backend"},
		Children: []*models.Task{},
	}
	storyA := &models.Task{
		ID:       "storyA",
		ShortID:  2,
		Type:     models.TaskTypeStory,
		Title:    "Story A",
		Status:   models.StatusInProgress,
		Priority: 1,
		Tags:     []string{"frontend"},
		Children: []*models.Task{},
		ParentID: &epic.ID,
	}
	taskA1 := &models.Task{
		ID:       "taskA1",
		ShortID:  3,
		Type:     models.TaskTypeTask,
		Title:    "Task A1",
		Status:   models.StatusTodo,
		Priority: 1,
		Tags:     []string{},
		Children: []*models.Task{},
		ParentID: &storyA.ID,
	}
	taskA2 := &models.Task{
		ID:       "taskA2",
		ShortID:  4,
		Type:     models.TaskTypeTask,
		Title:    "Task A2",
		Status:   models.StatusDone,
		Priority: 2,
		Tags:     []string{},
		Children: []*models.Task{},
		ParentID: &storyA.ID,
	}
	storyB := &models.Task{
		ID:       "storyB",
		ShortID:  5,
		Type:     models.TaskTypeStory,
		Title:    "Story B",
		Status:   models.StatusBlocked,
		Priority: 4,
		Tags:     []string{"ops"},
		Children: []*models.Task{},
		ParentID: &epic.ID,
	}
	taskB1 := &models.Task{
		ID:       "taskB1",
		ShortID:  6,
		Type:     models.TaskTypeTask,
		Title:    "Task B1",
		Status:   models.StatusReview,
		Priority: 5,
		Tags:     []string{},
		Children: []*models.Task{},
		ParentID: &storyB.ID,
	}
	bugRoot := &models.Task{
		ID:       "bugRoot",
		ShortID:  7,
		Type:     models.TaskTypeBug,
		Title:    "Bug Root",
		Status:   models.StatusTodo,
		Priority: 2,
		Tags:     nil,
		Children: []*models.Task{},
	}

	storyA.Children = []*models.Task{taskA1, taskA2}
	storyB.Children = []*models.Task{taskB1}
	epic.Children = []*models.Task{storyA, storyB}

	return &models.Project{
		Name:  "Test",
		Tasks: []*models.Task{epic, storyA, taskA1, taskA2, storyB, taskB1, bugRoot},
	}
}

func rowTitles(rows []outlinerRow) []string {
	var titles []string
	for _, r := range rows {
		if r.IsSeparator {
			continue
		}
		if r.IsProjectHeader {
			// Project sub-headers are synthetic, render-only rows in the
			// All perspective; treat them as visual-only for title-based
			// assertions so existing tests stay focused on section
			// headers and task rows.
			continue
		}
		if r.IsSectionHeader {
			displayName := r.SectionName
			if displayName == "" {
				displayName = "Active"
			}
			titles = append(titles, "["+displayName+"]")
		} else {
			titles = append(titles, r.task.Title)
		}
	}
	return titles
}

func assertRowTitles(t *testing.T, got []outlinerRow, want []string) {
	t.Helper()
	titles := rowTitles(got)
	if len(titles) != len(want) {
		t.Fatalf("expected %d rows %v, got %d rows %v", len(want), want, len(titles), titles)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Errorf("row %d: expected %q, got %q", i, want[i], titles[i])
		}
	}
}
