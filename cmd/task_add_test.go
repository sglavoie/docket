package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskCreationWithParent(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create an epic as potential parent
	epic := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic Parent")
	project.Tasks = append(project.Tasks, epic)

	// Create a task that will be a child of the epic
	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Child Task")
	task.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, task)

	// Verify parent assignment
	if task.ParentID == nil {
		t.Error("task should have a parent ID")
	}
	if *task.ParentID != epic.ID {
		t.Errorf("task parent ID = %s, want %s", *task.ParentID, epic.ID)
	}

	// Build tree and verify relationships
	project.BuildTaskTree()
	if task.Parent == nil {
		t.Error("task should have Parent pointer after BuildTaskTree")
	}
	if task.Parent.ID != epic.ID {
		t.Errorf("task.Parent.ID = %s, want %s", task.Parent.ID, epic.ID)
	}
	if len(epic.Children) != 1 {
		t.Errorf("epic should have 1 child, got %d", len(epic.Children))
	}
}

func TestTaskTypeParentValidation(t *testing.T) {
	tests := []struct {
		name       string
		childType  models.TaskType
		parentType models.TaskType
		canBeChild bool
	}{
		{"task under epic", models.TaskTypeTask, models.TaskTypeEpic, true},
		{"story under epic", models.TaskTypeStory, models.TaskTypeEpic, true},
		{"bug under epic", models.TaskTypeBug, models.TaskTypeEpic, true},
		{"idea under epic", models.TaskTypeIdea, models.TaskTypeEpic, true},
		{"task under story", models.TaskTypeTask, models.TaskTypeStory, true},
		{"bug under story", models.TaskTypeBug, models.TaskTypeStory, true},
		{"epic under epic", models.TaskTypeEpic, models.TaskTypeEpic, false},
		{"story under story", models.TaskTypeStory, models.TaskTypeStory, false},
		{"epic under story", models.TaskTypeEpic, models.TaskTypeStory, false},
		{"story under task", models.TaskTypeStory, models.TaskTypeTask, false},
		{"epic under task", models.TaskTypeEpic, models.TaskTypeTask, false},
		{"task under task", models.TaskTypeTask, models.TaskTypeTask, false},
		{"task under bug", models.TaskTypeTask, models.TaskTypeBug, false},
		{"task under idea", models.TaskTypeTask, models.TaskTypeIdea, false},
		{"note under epic", models.TaskTypeNote, models.TaskTypeEpic, true},
		{"note under story", models.TaskTypeNote, models.TaskTypeStory, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.childType.CanBeChildOf(tt.parentType)
			if got != tt.canBeChild {
				t.Errorf("%s.CanBeChildOf(%s) = %v, want %v", tt.childType, tt.parentType, got, tt.canBeChild)
			}
		})
	}
}

func TestTaskPriorityValidation(t *testing.T) {
	task := testutil.NewTestTask("Test Task", models.TaskTypeTask)

	// Default priority should be 3
	testutil.AssertEqual(t, task.Priority, 3)

	// Valid priorities
	for p := 1; p <= 5; p++ {
		task.Priority = p
		testutil.AssertEqual(t, task.Priority, p)
	}
}

func TestTaskTagManagement(t *testing.T) {
	task := testutil.NewTestTask("Test Task", models.TaskTypeTask)

	// Initially no tags
	testutil.AssertEqual(t, len(task.Tags), 0)

	// Add tags
	task.AddTag("backend")
	task.AddTag("urgent")
	testutil.AssertEqual(t, len(task.Tags), 2)

	// Adding duplicate tag should not duplicate
	task.AddTag("backend")
	testutil.AssertEqual(t, len(task.Tags), 2)

	// Check tag existence
	if !task.HasTag("backend") {
		t.Error("task should have 'backend' tag")
	}
	if !task.HasTag("urgent") {
		t.Error("task should have 'urgent' tag")
	}
	if task.HasTag("frontend") {
		t.Error("task should not have 'frontend' tag")
	}

	// Remove tag
	task.RemoveTag("backend")
	testutil.AssertEqual(t, len(task.Tags), 1)
	if task.HasTag("backend") {
		t.Error("task should not have 'backend' tag after removal")
	}
}

func TestTaskSectionAssignment(t *testing.T) {
	project := testutil.NewTestProject("Test Project")
	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Test Task")

	// Default section should be Backlog
	testutil.AssertEqual(t, task.Section, models.SectionBacklog)

	// Assign to custom section
	task.Section = "Active"
	testutil.AssertEqual(t, task.Section, "Active")

	// Ensure section exists in project
	project.EnsureSection("Active")
	if !project.HasSection("Active") {
		t.Error("project should have 'Active' section after EnsureSection")
	}
}

func TestTaskShortIDAssignment(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Project starts with NextShortID = 1
	testutil.AssertEqual(t, project.NextShortID, 1)

	// Create first task
	id1 := project.GetNextShortID()
	testutil.AssertEqual(t, id1, 1)
	testutil.AssertEqual(t, project.NextShortID, 2)

	// Create second task
	id2 := project.GetNextShortID()
	testutil.AssertEqual(t, id2, 2)
	testutil.AssertEqual(t, project.NextShortID, 3)
}

func TestNewTaskDefaults(t *testing.T) {
	task := models.NewTask("test-id", 42, models.TaskTypeTask, "Test Title")

	testutil.AssertEqual(t, task.ID, "test-id")
	testutil.AssertEqual(t, task.ShortID, 42)
	testutil.AssertEqual(t, task.Type, models.TaskTypeTask)
	testutil.AssertEqual(t, task.Title, "Test Title")
	testutil.AssertEqual(t, task.Status, models.StatusTodo)
	testutil.AssertEqual(t, task.Priority, 3)
	testutil.AssertEqual(t, task.Section, models.SectionBacklog)
	testutil.AssertEqual(t, task.Description, "")

	if task.ParentID != nil {
		t.Error("new task should have nil ParentID")
	}
	if task.CompletedAt != nil {
		t.Error("new task should have nil CompletedAt")
	}
	if task.DueDate != nil {
		t.Error("new task should have nil DueDate")
	}
	if task.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if task.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestTaskTypeHeadingPrefix(t *testing.T) {
	tests := []struct {
		taskType models.TaskType
		prefix   string
	}{
		{models.TaskTypeEpic, "Epic"},
		{models.TaskTypeStory, "Story"},
		{models.TaskTypeTask, "Task"},
		{models.TaskTypeBug, "Bug"},
		{models.TaskTypeIdea, "Idea"},
		{models.TaskTypeNote, "Note"},
	}

	for _, tt := range tests {
		t.Run(string(tt.taskType), func(t *testing.T) {
			got := tt.taskType.HeadingPrefix()
			if got != tt.prefix {
				t.Errorf("TaskType(%q).HeadingPrefix() = %q, want %q", tt.taskType, got, tt.prefix)
			}
		})
	}
}

func TestTaskTypeIcons(t *testing.T) {
	// Each task type should have an icon
	for _, taskType := range models.AllTaskTypes() {
		icon := taskType.Icon()
		if icon == "" || icon == "?" {
			t.Errorf("TaskType(%q) should have a valid icon, got %q", taskType, icon)
		}
	}
}
