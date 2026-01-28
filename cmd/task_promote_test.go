package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestPromoteTaskToStory(t *testing.T) {
	project := testutil.NewTestProject("Promote Test")

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task to Promote")
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	testutil.AssertEqual(t, task.Type, models.TaskTypeTask)

	err := task.Promote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, task.Type, models.TaskTypeStory)
}

func TestPromoteStoryToEpic(t *testing.T) {
	project := testutil.NewTestProject("Promote Test")

	story := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story to Promote")
	project.Tasks = append(project.Tasks, story)
	project.BuildTaskTree()

	testutil.AssertEqual(t, story.Type, models.TaskTypeStory)

	err := story.Promote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, story.Type, models.TaskTypeEpic)
}

func TestPromoteBugToStory(t *testing.T) {
	project := testutil.NewTestProject("Promote Test")

	bug := models.NewTask("bug-1", project.GetNextShortID(), models.TaskTypeBug, "Bug to Promote")
	project.Tasks = append(project.Tasks, bug)
	project.BuildTaskTree()

	testutil.AssertEqual(t, bug.Type, models.TaskTypeBug)

	err := bug.Promote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, bug.Type, models.TaskTypeStory)
}

func TestPromoteIdeaToStory(t *testing.T) {
	project := testutil.NewTestProject("Promote Test")

	idea := models.NewTask("idea-1", project.GetNextShortID(), models.TaskTypeIdea, "Idea to Promote")
	project.Tasks = append(project.Tasks, idea)
	project.BuildTaskTree()

	testutil.AssertEqual(t, idea.Type, models.TaskTypeIdea)

	err := idea.Promote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, idea.Type, models.TaskTypeStory)
}

func TestPromoteEpicAlreadyAtTop(t *testing.T) {
	project := testutil.NewTestProject("Promote Test")

	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic Already at Top")
	project.Tasks = append(project.Tasks, epic)
	project.BuildTaskTree()

	err := epic.Promote()
	if err == nil {
		t.Error("expected error when promoting epic")
	}

	testutil.AssertEqual(t, epic.Type, models.TaskTypeEpic)
}

func TestCanPromoteReturnsReason(t *testing.T) {
	project := testutil.NewTestProject("CanPromote Test")

	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	project.Tasks = append(project.Tasks, epic)
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

	story := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Parent Story")
	project.Tasks = append(project.Tasks, story)

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Child Task")
	task.ParentID = &story.ID
	project.Tasks = append(project.Tasks, task)
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

func TestDemoteEpicToStory(t *testing.T) {
	project := testutil.NewTestProject("Demote Test")

	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic to Demote")
	project.Tasks = append(project.Tasks, epic)
	project.BuildTaskTree()

	testutil.AssertEqual(t, epic.Type, models.TaskTypeEpic)

	err := epic.Demote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, epic.Type, models.TaskTypeStory)
}

func TestDemoteStoryToTask(t *testing.T) {
	project := testutil.NewTestProject("Demote Test")

	story := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story to Demote")
	project.Tasks = append(project.Tasks, story)
	project.BuildTaskTree()

	testutil.AssertEqual(t, story.Type, models.TaskTypeStory)

	err := story.Demote()
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, story.Type, models.TaskTypeTask)
}

func TestDemoteTaskAlreadyAtBottom(t *testing.T) {
	project := testutil.NewTestProject("Demote Test")

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task Already at Bottom")
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	err := task.Demote()
	if err == nil {
		t.Error("expected error when demoting task (already at bottom)")
	}

	testutil.AssertEqual(t, task.Type, models.TaskTypeTask)
}

func TestDemoteBugAlreadyAtBottom(t *testing.T) {
	project := testutil.NewTestProject("Demote Test")

	bug := models.NewTask("bug-1", project.GetNextShortID(), models.TaskTypeBug, "Bug Already at Bottom")
	project.Tasks = append(project.Tasks, bug)
	project.BuildTaskTree()

	err := bug.Demote()
	if err == nil {
		t.Error("expected error when demoting bug (already at bottom)")
	}

	testutil.AssertEqual(t, bug.Type, models.TaskTypeBug)
}

func TestDemoteIdeaAlreadyAtBottom(t *testing.T) {
	project := testutil.NewTestProject("Demote Test")

	idea := models.NewTask("idea-1", project.GetNextShortID(), models.TaskTypeIdea, "Idea Already at Bottom")
	project.Tasks = append(project.Tasks, idea)
	project.BuildTaskTree()

	err := idea.Demote()
	if err == nil {
		t.Error("expected error when demoting idea (already at bottom)")
	}

	testutil.AssertEqual(t, idea.Type, models.TaskTypeIdea)
}

func TestCanDemoteReturnsReason(t *testing.T) {
	project := testutil.NewTestProject("CanDemote Test")

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task")
	project.Tasks = append(project.Tasks, task)
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

	story := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story with Child")
	project.Tasks = append(project.Tasks, story)

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Child Task")
	task.ParentID = &story.ID
	project.Tasks = append(project.Tasks, task)
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

func TestCycleLeafTypeRightTaskToBug(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task to Cycle")
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	testutil.AssertEqual(t, task.Type, models.TaskTypeTask)

	err := task.CycleLeafType(false) // right
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, task.Type, models.TaskTypeBug)
}

func TestCycleLeafTypeRightBugToIdea(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	bug := models.NewTask("bug-1", project.GetNextShortID(), models.TaskTypeBug, "Bug to Cycle")
	project.Tasks = append(project.Tasks, bug)
	project.BuildTaskTree()

	testutil.AssertEqual(t, bug.Type, models.TaskTypeBug)

	err := bug.CycleLeafType(false) // right
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, bug.Type, models.TaskTypeIdea)
}

func TestCycleLeafTypeRightIdeaToTask(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	idea := models.NewTask("idea-1", project.GetNextShortID(), models.TaskTypeIdea, "Idea to Cycle")
	project.Tasks = append(project.Tasks, idea)
	project.BuildTaskTree()

	testutil.AssertEqual(t, idea.Type, models.TaskTypeIdea)

	err := idea.CycleLeafType(false) // right
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, idea.Type, models.TaskTypeTask)
}

func TestCycleLeafTypeLeftTaskToIdea(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task to Cycle Left")
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	testutil.AssertEqual(t, task.Type, models.TaskTypeTask)

	err := task.CycleLeafType(true) // left
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, task.Type, models.TaskTypeIdea)
}

func TestCycleLeafTypeLeftIdeaToBug(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	idea := models.NewTask("idea-1", project.GetNextShortID(), models.TaskTypeIdea, "Idea to Cycle Left")
	project.Tasks = append(project.Tasks, idea)
	project.BuildTaskTree()

	testutil.AssertEqual(t, idea.Type, models.TaskTypeIdea)

	err := idea.CycleLeafType(true) // left
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, idea.Type, models.TaskTypeBug)
}

func TestCycleLeafTypeLeftBugToTask(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	bug := models.NewTask("bug-1", project.GetNextShortID(), models.TaskTypeBug, "Bug to Cycle Left")
	project.Tasks = append(project.Tasks, bug)
	project.BuildTaskTree()

	testutil.AssertEqual(t, bug.Type, models.TaskTypeBug)

	err := bug.CycleLeafType(true) // left
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, bug.Type, models.TaskTypeTask)
}

func TestCycleLeafTypeFullCycleRight(t *testing.T) {
	project := testutil.NewTestProject("Full Cycle Test")

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task Full Cycle")
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	// Task -> Bug -> Idea -> Task
	err := task.CycleLeafType(false)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, task.Type, models.TaskTypeBug)

	err = task.CycleLeafType(false)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, task.Type, models.TaskTypeIdea)

	err = task.CycleLeafType(false)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, task.Type, models.TaskTypeTask)
}

func TestCycleLeafTypeEpicFails(t *testing.T) {
	project := testutil.NewTestProject("Cycle Epic Test")

	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic Cannot Cycle")
	project.Tasks = append(project.Tasks, epic)
	project.BuildTaskTree()

	err := epic.CycleLeafType(false)
	if err == nil {
		t.Error("expected error when cycling non-leaf type")
	}

	testutil.AssertEqual(t, epic.Type, models.TaskTypeEpic)
}

func TestCycleLeafTypeStoryFails(t *testing.T) {
	project := testutil.NewTestProject("Cycle Story Test")

	story := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story Cannot Cycle")
	project.Tasks = append(project.Tasks, story)
	project.BuildTaskTree()

	err := story.CycleLeafType(false)
	if err == nil {
		t.Error("expected error when cycling non-leaf type")
	}

	testutil.AssertEqual(t, story.Type, models.TaskTypeStory)
}

func TestCanCycleLeafTypeReturnsReason(t *testing.T) {
	project := testutil.NewTestProject("CanCycle Test")

	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	project.Tasks = append(project.Tasks, epic)
	project.BuildTaskTree()

	canCycle, reason := epic.CanCycleLeafType(false)
	if canCycle {
		t.Error("expected epic to not be cyclable")
	}
	if reason != "not a leaf type" {
		t.Errorf("expected specific reason, got: %s", reason)
	}
}

func TestPromoteUpdatesTimestamp(t *testing.T) {
	project := testutil.NewTestProject("Timestamp Test")

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task")
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	originalTime := task.UpdatedAt

	err := task.Promote()
	testutil.AssertNoError(t, err)

	if !task.UpdatedAt.After(originalTime) && task.UpdatedAt != originalTime {
		// UpdatedAt should be same or later (test runs fast)
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestDemoteUpdatesTimestamp(t *testing.T) {
	project := testutil.NewTestProject("Timestamp Test")

	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	project.Tasks = append(project.Tasks, epic)
	project.BuildTaskTree()

	originalTime := epic.UpdatedAt

	err := epic.Demote()
	testutil.AssertNoError(t, err)

	if !epic.UpdatedAt.After(originalTime) && epic.UpdatedAt != originalTime {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestCycleLeafTypeUpdatesTimestamp(t *testing.T) {
	project := testutil.NewTestProject("Timestamp Test")

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task")
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	originalTime := task.UpdatedAt

	err := task.CycleLeafType(false)
	testutil.AssertNoError(t, err)

	if !task.UpdatedAt.After(originalTime) && task.UpdatedAt != originalTime {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestPromoteTaskUnderEpicToStory(t *testing.T) {
	project := testutil.NewTestProject("Hierarchy Test")

	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	project.Tasks = append(project.Tasks, epic)

	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task under Epic")
	task.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, task)
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

func TestDemoteStoryUnderEpicToTask(t *testing.T) {
	project := testutil.NewTestProject("Hierarchy Test")

	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	project.Tasks = append(project.Tasks, epic)

	story := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story under Epic")
	story.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, story)
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

func TestCycleIdeaUnderEpicToTaskToBug(t *testing.T) {
	project := testutil.NewTestProject("Hierarchy Cycle Test")

	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	project.Tasks = append(project.Tasks, epic)

	idea := models.NewTask("idea-1", project.GetNextShortID(), models.TaskTypeIdea, "Idea under Epic")
	idea.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, idea)
	project.BuildTaskTree()

	// Idea -> Task (right cycle)
	err := idea.CycleLeafType(false)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, idea.Type, models.TaskTypeTask)

	// Task -> Bug (right cycle)
	err = idea.CycleLeafType(false)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, idea.Type, models.TaskTypeBug)
}

func TestEpicWithStoryChildrenCannotDemote(t *testing.T) {
	project := testutil.NewTestProject("Epic Demote Block Test")

	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic with Story Child")
	project.Tasks = append(project.Tasks, epic)

	story := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story Child")
	story.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, story)
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
