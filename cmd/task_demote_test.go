package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestDemoteEpicToStory(t *testing.T) {
	project := testutil.NewTestProject("Demote Test")

	epic := addTaskTypeChangeTestTask(t, project, "epic-1", models.TaskTypeEpic, "Epic to Demote")
	project.BuildTaskTree()

	testutil.AssertEqual(t, epic.Type, models.TaskTypeEpic)

	err := epic.Demote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, epic.Type, models.TaskTypeStory)
}

func TestDemoteStoryToTask(t *testing.T) {
	project := testutil.NewTestProject("Demote Test")

	story := addTaskTypeChangeTestTask(t, project, "story-1", models.TaskTypeStory, "Story to Demote")
	project.BuildTaskTree()

	testutil.AssertEqual(t, story.Type, models.TaskTypeStory)

	err := story.Demote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, story.Type, models.TaskTypeTask)
}

func TestDemoteTaskAlreadyAtBottom(t *testing.T) {
	project := testutil.NewTestProject("Demote Test")

	task := addTaskTypeChangeTestTask(t, project, "task-1", models.TaskTypeTask, "Task Already at Bottom")
	project.BuildTaskTree()

	err := task.Demote()
	if err == nil {
		t.Error("expected error when demoting task (already at bottom)")
	}

	testutil.AssertEqual(t, task.Type, models.TaskTypeTask)
}

func TestDemoteBugAlreadyAtBottom(t *testing.T) {
	project := testutil.NewTestProject("Demote Test")

	bug := addTaskTypeChangeTestTask(t, project, "bug-1", models.TaskTypeBug, "Bug Already at Bottom")
	project.BuildTaskTree()

	err := bug.Demote()
	if err == nil {
		t.Error("expected error when demoting bug (already at bottom)")
	}

	testutil.AssertEqual(t, bug.Type, models.TaskTypeBug)
}

func TestDemoteIdeaAlreadyAtBottom(t *testing.T) {
	project := testutil.NewTestProject("Demote Test")

	idea := addTaskTypeChangeTestTask(t, project, "idea-1", models.TaskTypeIdea, "Idea Already at Bottom")
	project.BuildTaskTree()

	err := idea.Demote()
	if err == nil {
		t.Error("expected error when demoting idea (already at bottom)")
	}

	testutil.AssertEqual(t, idea.Type, models.TaskTypeIdea)
}

func TestCanDemoteReturnsReason(t *testing.T) {
	project := testutil.NewTestProject("CanDemote Test")

	task := addTaskTypeChangeTestTask(t, project, "task-1", models.TaskTypeTask, "Task")
	project.BuildTaskTree()

	canDemote, reason := task.CanDemote()
	if canDemote {
		t.Error("expected task to not be demotable")
	}
	if reason != "already at bottom of hierarchy" {
		t.Errorf("expected specific reason, got: %s", reason)
	}
}

func TestDemoteStoryWithChildrenBlockedByHierarchy(t *testing.T) {
	project := testutil.NewTestProject("Demote With Children Test")

	story := addTaskTypeChangeTestTask(t, project, "story-1", models.TaskTypeStory, "Story with Child")
	task := addTaskTypeChangeTestTask(t, project, "task-1", models.TaskTypeTask, "Child Task")
	task.ParentID = &story.ID
	project.BuildTaskTree()

	// Story with task children cannot demote to task (task cannot have task children)
	canDemote, reason := story.CanDemote()
	if canDemote {
		t.Error("story with children should not be demotable to task")
	}
	if reason == "" {
		t.Error("expected a reason for blocked demotion")
	}
}

func TestDemoteUpdatesTimestamp(t *testing.T) {
	project := testutil.NewTestProject("Timestamp Test")

	epic := addTaskTypeChangeTestTask(t, project, "epic-1", models.TaskTypeEpic, "Epic")
	project.BuildTaskTree()

	originalTime := epic.UpdatedAt

	err := epic.Demote()
	testutil.AssertNoError(t, err)

	if !epic.UpdatedAt.After(originalTime) && epic.UpdatedAt != originalTime {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestDemoteStoryUnderEpicToTask(t *testing.T) {
	project := testutil.NewTestProject("Hierarchy Test")

	epic := addTaskTypeChangeTestTask(t, project, "epic-1", models.TaskTypeEpic, "Epic")
	story := addTaskTypeChangeTestTask(t, project, "story-1", models.TaskTypeStory, "Story under Epic")
	story.ParentID = &epic.ID
	project.BuildTaskTree()

	// Story under Epic can be demoted to Task (task can be child of epic)
	canDemote, _ := story.CanDemote()
	if !canDemote {
		t.Error("story under epic should be demotable to task")
	}

	err := story.Demote()
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, story.Type, models.TaskTypeTask)
}

func TestEpicWithStoryChildrenCannotDemote(t *testing.T) {
	project := testutil.NewTestProject("Epic Demote Block Test")

	epic := addTaskTypeChangeTestTask(t, project, "epic-1", models.TaskTypeEpic, "Epic with Story Child")
	story := addTaskTypeChangeTestTask(t, project, "story-1", models.TaskTypeStory, "Story Child")
	story.ParentID = &epic.ID
	project.BuildTaskTree()

	// Epic with story children cannot demote to story (story cannot have story children)
	canDemote, reason := epic.CanDemote()
	if canDemote {
		t.Error("epic with story children should not be demotable")
	}
	if reason == "" {
		t.Error("expected a reason for blocked demotion")
	}
}
