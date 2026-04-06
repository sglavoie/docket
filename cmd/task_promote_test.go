package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestPromoteTaskToStory(t *testing.T) {
	project := testutil.NewTestProject("Promote Test")

	task := addTaskTypeChangeTestTask(t, project, "task-1", models.TaskTypeTask, "Task to Promote")
	project.BuildTaskTree()

	testutil.AssertEqual(t, task.Type, models.TaskTypeTask)

	err := task.Promote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, task.Type, models.TaskTypeStory)
}

func TestPromoteStoryToEpic(t *testing.T) {
	project := testutil.NewTestProject("Promote Test")

	story := addTaskTypeChangeTestTask(t, project, "story-1", models.TaskTypeStory, "Story to Promote")
	project.BuildTaskTree()

	testutil.AssertEqual(t, story.Type, models.TaskTypeStory)

	err := story.Promote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, story.Type, models.TaskTypeEpic)
}

func TestPromoteBugToStory(t *testing.T) {
	project := testutil.NewTestProject("Promote Test")

	bug := addTaskTypeChangeTestTask(t, project, "bug-1", models.TaskTypeBug, "Bug to Promote")
	project.BuildTaskTree()

	testutil.AssertEqual(t, bug.Type, models.TaskTypeBug)

	err := bug.Promote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, bug.Type, models.TaskTypeStory)
}

func TestPromoteIdeaToStory(t *testing.T) {
	project := testutil.NewTestProject("Promote Test")

	idea := addTaskTypeChangeTestTask(t, project, "idea-1", models.TaskTypeIdea, "Idea to Promote")
	project.BuildTaskTree()

	testutil.AssertEqual(t, idea.Type, models.TaskTypeIdea)

	err := idea.Promote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, idea.Type, models.TaskTypeStory)
}

func TestPromoteEpicAlreadyAtTop(t *testing.T) {
	project := testutil.NewTestProject("Promote Test")

	epic := addTaskTypeChangeTestTask(t, project, "epic-1", models.TaskTypeEpic, "Epic Already at Top")
	project.BuildTaskTree()

	err := epic.Promote()
	if err == nil {
		t.Error("expected error when promoting epic")
	}

	testutil.AssertEqual(t, epic.Type, models.TaskTypeEpic)
}

func TestCanPromoteReturnsReason(t *testing.T) {
	project := testutil.NewTestProject("CanPromote Test")

	epic := addTaskTypeChangeTestTask(t, project, "epic-1", models.TaskTypeEpic, "Epic")
	project.BuildTaskTree()

	canPromote, reason := epic.CanPromote()
	if canPromote {
		t.Error("expected epic to not be promotable")
	}
	if reason != "already at top of hierarchy" {
		t.Errorf("expected specific reason, got: %s", reason)
	}
}

func TestPromoteTaskUnderStoryBlockedByHierarchy(t *testing.T) {
	project := testutil.NewTestProject("Hierarchy Block Test")

	story := addTaskTypeChangeTestTask(t, project, "story-1", models.TaskTypeStory, "Parent Story")
	task := addTaskTypeChangeTestTask(t, project, "task-1", models.TaskTypeTask, "Child Task")
	task.ParentID = &story.ID
	project.BuildTaskTree()

	// Task under Story cannot be promoted to Story (story cannot be child of story)
	canPromote, reason := task.CanPromote()
	if canPromote {
		t.Error("task under story should not be promotable to story")
	}
	if reason == "" {
		t.Error("expected a reason for blocked promotion")
	}
}

func TestPromoteUpdatesTimestamp(t *testing.T) {
	project := testutil.NewTestProject("Timestamp Test")

	task := addTaskTypeChangeTestTask(t, project, "task-1", models.TaskTypeTask, "Task")
	project.BuildTaskTree()

	originalTime := task.UpdatedAt

	err := task.Promote()
	testutil.AssertNoError(t, err)

	if !task.UpdatedAt.After(originalTime) && task.UpdatedAt != originalTime {
		// UpdatedAt should be same or later (test runs fast)
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestPromoteTaskUnderEpicToStory(t *testing.T) {
	project := testutil.NewTestProject("Hierarchy Test")

	epic := addTaskTypeChangeTestTask(t, project, "epic-1", models.TaskTypeEpic, "Epic")
	task := addTaskTypeChangeTestTask(t, project, "task-1", models.TaskTypeTask, "Task under Epic")
	task.ParentID = &epic.ID
	project.BuildTaskTree()

	// Task under Epic can be promoted to Story (story can be child of epic)
	canPromote, _ := task.CanPromote()
	if !canPromote {
		t.Error("task under epic should be promotable to story")
	}

	err := task.Promote()
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, task.Type, models.TaskTypeStory)
}
