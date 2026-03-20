package format

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBuildHierarchy(t *testing.T) {
	tests := []struct {
		name         string
		parsedTasks  []*ParsedTask
		wantParents  map[string]*string // task ID -> parent ID
		wantNoParent []string           // task IDs that should have no parent
	}{
		{
			name: "simple hierarchy",
			parsedTasks: []*ParsedTask{
				{
					Task:  &models.Task{ID: "epic1", Type: models.TaskTypeEpic, Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "story1", Type: models.TaskTypeStory, Title: "Story 1"},
					Level: 3,
				},
				{
					Task:  &models.Task{ID: "task1", Type: models.TaskTypeTask, Title: "Task 1"},
					Level: 4,
				},
			},
			wantParents: map[string]*string{
				"story1": strPtr("epic1"),
				"task1":  strPtr("story1"),
			},
			wantNoParent: []string{"epic1"},
		},
		{
			name: "multiple siblings at same level",
			parsedTasks: []*ParsedTask{
				{
					Task:  &models.Task{ID: "epic1", Type: models.TaskTypeEpic, Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "story1", Type: models.TaskTypeStory, Title: "Story 1"},
					Level: 3,
				},
				{
					Task:  &models.Task{ID: "story2", Type: models.TaskTypeStory, Title: "Story 2"},
					Level: 3,
				},
				{
					Task:  &models.Task{ID: "story3", Type: models.TaskTypeStory, Title: "Story 3"},
					Level: 3,
				},
			},
			wantParents: map[string]*string{
				"story1": strPtr("epic1"),
				"story2": strPtr("epic1"),
				"story3": strPtr("epic1"),
			},
			wantNoParent: []string{"epic1"},
		},
		{
			name: "task with explicit parent ID is preserved",
			parsedTasks: []*ParsedTask{
				{
					Task:  &models.Task{ID: "epic1", Type: models.TaskTypeEpic, Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "story1", Type: models.TaskTypeStory, Title: "Story 1", ParentID: strPtr("other-epic")},
					Level: 3,
				},
			},
			wantParents: map[string]*string{
				"story1": strPtr("other-epic"), // Explicit parent preserved
			},
			wantNoParent: []string{"epic1"},
		},
		{
			name: "type restriction prevents invalid parent",
			parsedTasks: []*ParsedTask{
				{
					Task:  &models.Task{ID: "task1", Type: models.TaskTypeTask, Title: "Task 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "epic1", Type: models.TaskTypeEpic, Title: "Epic 1"},
					Level: 3,
				},
			},
			wantNoParent: []string{"task1", "epic1"}, // Epic can't be child of Task
		},
		{
			name: "level skip - find parent at multiple levels up",
			parsedTasks: []*ParsedTask{
				{
					Task:  &models.Task{ID: "epic1", Type: models.TaskTypeEpic, Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "task1", Type: models.TaskTypeTask, Title: "Task 1"},
					Level: 4, // Skip level 3
				},
			},
			wantParents: map[string]*string{
				"task1": strPtr("epic1"), // Should find epic at level 2
			},
			wantNoParent: []string{"epic1"},
		},
		{
			name: "complex multi-level hierarchy",
			parsedTasks: []*ParsedTask{
				{
					Task:  &models.Task{ID: "epic1", Type: models.TaskTypeEpic, Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "story1", Type: models.TaskTypeStory, Title: "Story 1"},
					Level: 3,
				},
				{
					Task:  &models.Task{ID: "task1", Type: models.TaskTypeTask, Title: "Task 1"},
					Level: 4,
				},
				{
					Task:  &models.Task{ID: "task2", Type: models.TaskTypeTask, Title: "Task 2"},
					Level: 4,
				},
				{
					Task:  &models.Task{ID: "story2", Type: models.TaskTypeStory, Title: "Story 2"},
					Level: 3,
				},
			},
			wantParents: map[string]*string{
				"story1": strPtr("epic1"),
				"task1":  strPtr("story1"),
				"task2":  strPtr("story1"),
				"story2": strPtr("epic1"),
			},
			wantNoParent: []string{"epic1"},
		},
		{
			name: "top-level tasks have no parent",
			parsedTasks: []*ParsedTask{
				{
					Task:  &models.Task{ID: "epic1", Type: models.TaskTypeEpic, Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "epic2", Type: models.TaskTypeEpic, Title: "Epic 2"},
					Level: 2,
				},
			},
			wantNoParent: []string{"epic1", "epic2"},
		},
		{
			name: "heading level skip (## → #####) is clamped to direct child",
			parsedTasks: []*ParsedTask{
				{
					Task:  &models.Task{ID: "epic1", Type: models.TaskTypeEpic, Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "task1", Type: models.TaskTypeTask, Title: "Task 1"},
					Level: 5, // jumps 3 levels — clamped to level 3 (prevLevel+1)
				},
			},
			wantParents: map[string]*string{
				"task1": strPtr("epic1"), // clamped level 3 finds epic1 at level 2
			},
			wantNoParent: []string{"epic1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, _ := BuildHierarchy(tt.parsedTasks)

			// Build a map of tasks by ID for easier lookup
			taskMap := make(map[string]*models.Task)
			for _, task := range tasks {
				taskMap[task.ID] = task
			}

			// Check tasks that should have parents
			for taskID, wantParentID := range tt.wantParents {
				task, ok := taskMap[taskID]
				if !ok {
					t.Errorf("task %s not found in result", taskID)
					continue
				}

				if task.ParentID == nil {
					t.Errorf("task %s: ParentID = nil, want %s", taskID, *wantParentID)
					continue
				}

				if *task.ParentID != *wantParentID {
					t.Errorf("task %s: ParentID = %s, want %s", taskID, *task.ParentID, *wantParentID)
				}
			}

			// Check tasks that should have no parent
			for _, taskID := range tt.wantNoParent {
				task, ok := taskMap[taskID]
				if !ok {
					t.Errorf("task %s not found in result", taskID)
					continue
				}

				if task.ParentID != nil {
					t.Errorf("task %s: ParentID = %s, want nil", taskID, *task.ParentID)
				}
			}
		})
	}
}

func TestBuildHierarchy_HeadingLevelSkipWarning(t *testing.T) {
	parsedTasks := []*ParsedTask{
		{Task: &models.Task{ID: "epic1", Type: models.TaskTypeEpic, Title: "Epic 1"}, Level: 2},
		{Task: &models.Task{ID: "task1", Type: models.TaskTypeTask, Title: "Task 1"}, Level: 5},
	}

	_, warnings := BuildHierarchy(parsedTasks)

	if len(warnings) == 0 {
		t.Fatal("expected a warning for heading level skip, got none")
	}
	found := false
	for _, w := range warnings {
		if w.Field == "heading_level" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning with field='heading_level', got: %+v", warnings)
	}
}

func TestBuildHierarchy_TypeRestrictions(t *testing.T) {
	// Test that type restrictions are properly enforced
	tests := []struct {
		name       string
		parentType models.TaskType
		childType  models.TaskType
		wantParent bool
	}{
		{
			name:       "Epic can contain Story",
			parentType: models.TaskTypeEpic,
			childType:  models.TaskTypeStory,
			wantParent: true,
		},
		{
			name:       "Epic can contain Task",
			parentType: models.TaskTypeEpic,
			childType:  models.TaskTypeTask,
			wantParent: true,
		},
		{
			name:       "Epic can contain Bug",
			parentType: models.TaskTypeEpic,
			childType:  models.TaskTypeBug,
			wantParent: true,
		},
		{
			name:       "Epic can contain Idea",
			parentType: models.TaskTypeEpic,
			childType:  models.TaskTypeIdea,
			wantParent: true,
		},
		{
			name:       "Story can contain Task",
			parentType: models.TaskTypeStory,
			childType:  models.TaskTypeTask,
			wantParent: true,
		},
		{
			name:       "Story can contain Bug",
			parentType: models.TaskTypeStory,
			childType:  models.TaskTypeBug,
			wantParent: true,
		},
		{
			name:       "Story cannot contain Epic",
			parentType: models.TaskTypeStory,
			childType:  models.TaskTypeEpic,
			wantParent: false,
		},
		{
			name:       "Story cannot contain Story",
			parentType: models.TaskTypeStory,
			childType:  models.TaskTypeStory,
			wantParent: false,
		},
		{
			name:       "Task cannot contain Task",
			parentType: models.TaskTypeTask,
			childType:  models.TaskTypeTask,
			wantParent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedTasks := []*ParsedTask{
				{
					Task:  &models.Task{ID: "parent", Type: tt.parentType, Title: "Parent"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "child", Type: tt.childType, Title: "Child"},
					Level: 3,
				},
			}

			tasks, _ := BuildHierarchy(parsedTasks)

			var child *models.Task
			for _, task := range tasks {
				if task.ID == "child" {
					child = task
					break
				}
			}

			if child == nil {
				t.Fatal("child task not found")
			}

			hasParent := child.ParentID != nil
			if hasParent != tt.wantParent {
				if tt.wantParent {
					t.Errorf("child should have parent but doesn't")
				} else {
					t.Errorf("child should not have parent but has: %s", *child.ParentID)
				}
			}

			if hasParent && tt.wantParent {
				if *child.ParentID != "parent" {
					t.Errorf("child.ParentID = %s, want 'parent'", *child.ParentID)
				}
			}
		})
	}
}
