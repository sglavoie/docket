package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestCycleLeafTypeRightTaskToBug(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	task := addTaskTypeChangeTestTask(t, project, "task-1", models.TaskTypeTask, "Task to Cycle")
	project.BuildTaskTree()

	testutil.AssertEqual(t, task.Type, models.TaskTypeTask)

	err := task.CycleLeafType(false)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, task.Type, models.TaskTypeBug)
}

func TestCycleLeafTypeRightBugToIdea(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	bug := addTaskTypeChangeTestTask(t, project, "bug-1", models.TaskTypeBug, "Bug to Cycle")
	project.BuildTaskTree()

	testutil.AssertEqual(t, bug.Type, models.TaskTypeBug)

	err := bug.CycleLeafType(false)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, bug.Type, models.TaskTypeIdea)
}

func TestCycleLeafTypeRightIdeaToTask(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	idea := addTaskTypeChangeTestTask(t, project, "idea-1", models.TaskTypeIdea, "Idea to Cycle")
	project.BuildTaskTree()

	testutil.AssertEqual(t, idea.Type, models.TaskTypeIdea)

	err := idea.CycleLeafType(false)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, idea.Type, models.TaskTypeTask)
}

func TestCycleLeafTypeLeftTaskToIdea(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	task := addTaskTypeChangeTestTask(t, project, "task-1", models.TaskTypeTask, "Task to Cycle Left")
	project.BuildTaskTree()

	testutil.AssertEqual(t, task.Type, models.TaskTypeTask)

	err := task.CycleLeafType(true)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, task.Type, models.TaskTypeIdea)
}

func TestCycleLeafTypeLeftIdeaToBug(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	idea := addTaskTypeChangeTestTask(t, project, "idea-1", models.TaskTypeIdea, "Idea to Cycle Left")
	project.BuildTaskTree()

	testutil.AssertEqual(t, idea.Type, models.TaskTypeIdea)

	err := idea.CycleLeafType(true)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, idea.Type, models.TaskTypeBug)
}

func TestCycleLeafTypeLeftBugToTask(t *testing.T) {
	project := testutil.NewTestProject("Cycle Test")

	bug := addTaskTypeChangeTestTask(t, project, "bug-1", models.TaskTypeBug, "Bug to Cycle Left")
	project.BuildTaskTree()

	testutil.AssertEqual(t, bug.Type, models.TaskTypeBug)

	err := bug.CycleLeafType(true)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, bug.Type, models.TaskTypeTask)
}

func TestCycleLeafTypeFullCycleRight(t *testing.T) {
	project := testutil.NewTestProject("Full Cycle Test")

	task := addTaskTypeChangeTestTask(t, project, "task-1", models.TaskTypeTask, "Task Full Cycle")
	project.BuildTaskTree()

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

	epic := addTaskTypeChangeTestTask(t, project, "epic-1", models.TaskTypeEpic, "Epic Cannot Cycle")
	project.BuildTaskTree()

	err := epic.CycleLeafType(false)
	if err == nil {
		t.Error("expected error when cycling non-leaf type")
	}

	testutil.AssertEqual(t, epic.Type, models.TaskTypeEpic)
}

func TestCycleLeafTypeStoryFails(t *testing.T) {
	project := testutil.NewTestProject("Cycle Story Test")

	story := addTaskTypeChangeTestTask(t, project, "story-1", models.TaskTypeStory, "Story Cannot Cycle")
	project.BuildTaskTree()

	err := story.CycleLeafType(false)
	if err == nil {
		t.Error("expected error when cycling non-leaf type")
	}

	testutil.AssertEqual(t, story.Type, models.TaskTypeStory)
}

func TestCanCycleLeafTypeReturnsReason(t *testing.T) {
	project := testutil.NewTestProject("CanCycle Test")

	epic := addTaskTypeChangeTestTask(t, project, "epic-1", models.TaskTypeEpic, "Epic")
	project.BuildTaskTree()

	canCycle, reason := epic.CanCycleLeafType(false)
	if canCycle {
		t.Error("expected epic to not be cyclable")
	}
	if reason != "not a leaf type" {
		t.Errorf("expected specific reason, got: %s", reason)
	}
}

func TestCycleLeafTypeUpdatesTimestamp(t *testing.T) {
	project := testutil.NewTestProject("Timestamp Test")

	task := addTaskTypeChangeTestTask(t, project, "task-1", models.TaskTypeTask, "Task")
	project.BuildTaskTree()

	originalTime := task.UpdatedAt

	err := task.CycleLeafType(false)
	testutil.AssertNoError(t, err)

	if !task.UpdatedAt.After(originalTime) && task.UpdatedAt != originalTime {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestCycleIdeaUnderEpicToTaskToBug(t *testing.T) {
	project := testutil.NewTestProject("Hierarchy Cycle Test")

	epic := addTaskTypeChangeTestTask(t, project, "epic-1", models.TaskTypeEpic, "Epic")
	idea := addTaskTypeChangeTestTask(t, project, "idea-1", models.TaskTypeIdea, "Idea under Epic")
	idea.ParentID = &epic.ID
	project.BuildTaskTree()

	err := idea.CycleLeafType(false)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, idea.Type, models.TaskTypeTask)

	err = idea.CycleLeafType(false)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, idea.Type, models.TaskTypeBug)
}
