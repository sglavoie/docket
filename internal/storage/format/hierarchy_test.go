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
					Task:  &models.Task{ID: "epic1", Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "story1", Title: "Story 1"},
					Level: 3,
				},
				{
					Task:  &models.Task{ID: "task1", Title: "Task 1"},
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
					Task:  &models.Task{ID: "epic1", Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "story1", Title: "Story 1"},
					Level: 3,
				},
				{
					Task:  &models.Task{ID: "story2", Title: "Story 2"},
					Level: 3,
				},
				{
					Task:  &models.Task{ID: "story3", Title: "Story 3"},
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
					Task:  &models.Task{ID: "epic1", Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "story1", Title: "Story 1", ParentID: strPtr("other-epic")},
					Level: 3,
				},
			},
			wantParents: map[string]*string{
				"story1": strPtr("other-epic"), // Explicit parent preserved
			},
			wantNoParent: []string{"epic1"},
		},
		{
			name: "generic hierarchy permits parent",
			parsedTasks: []*ParsedTask{
				{
					Task:  &models.Task{ID: "task1", Title: "Task 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "epic1", Title: "Epic 1"},
					Level: 3,
				},
			},
			wantParents:  map[string]*string{"epic1": strPtr("task1")},
			wantNoParent: []string{"task1"},
		},
		{
			name: "level skip - find parent at multiple levels up",
			parsedTasks: []*ParsedTask{
				{
					Task:  &models.Task{ID: "epic1", Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "task1", Title: "Task 1"},
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
					Task:  &models.Task{ID: "epic1", Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "story1", Title: "Story 1"},
					Level: 3,
				},
				{
					Task:  &models.Task{ID: "task1", Title: "Task 1"},
					Level: 4,
				},
				{
					Task:  &models.Task{ID: "task2", Title: "Task 2"},
					Level: 4,
				},
				{
					Task:  &models.Task{ID: "story2", Title: "Story 2"},
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
					Task:  &models.Task{ID: "epic1", Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "epic2", Title: "Epic 2"},
					Level: 2,
				},
			},
			wantNoParent: []string{"epic1", "epic2"},
		},
		{
			name: "heading level skip (## → #####) is clamped to direct child",
			parsedTasks: []*ParsedTask{
				{
					Task:  &models.Task{ID: "epic1", Title: "Epic 1"},
					Level: 2,
				},
				{
					Task:  &models.Task{ID: "task1", Title: "Task 1"},
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
		{Task: &models.Task{ID: "epic1", Title: "Epic 1"}, Level: 2},
		{Task: &models.Task{ID: "task1", Title: "Task 1"}, Level: 5},
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

// TestBuildHierarchy_ExplicitParentClearsStaleStack ensures that a task with
// an explicit ParentID does not leave stale deeper-level entries in the
// internal level stack. Previously, a level-2 task with a preset parent
// skipped the cleanup loop, so a later level-N task at the same indentation
// would attach to an unrelated sibling's child from an earlier group.
func TestBuildHierarchy_ExplicitParentClearsStaleStack(t *testing.T) {
	parsedTasks := []*ParsedTask{
		{Task: &models.Task{ID: "epic1", Title: "Epic 1"}, Level: 2},
		{Task: &models.Task{ID: "story1", Title: "Story 1"}, Level: 3},
		{Task: &models.Task{ID: "epic2", Title: "Epic 2", ParentID: strPtr("external-epic")}, Level: 2},
		{Task: &models.Task{ID: "task1", Title: "Task 1"}, Level: 3},
	}

	tasks, _ := BuildHierarchy(parsedTasks)

	var task1 *models.Task
	for _, t := range tasks {
		if t.ID == "task1" {
			task1 = t
			break
		}
	}
	if task1 == nil {
		t.Fatal("task1 not found")
	}
	if task1.ParentID == nil {
		t.Fatal("task1 should have parent epic2, got nil")
	}
	if *task1.ParentID != "epic2" {
		t.Errorf("task1 should be child of epic2 (the most recent level-2 task), got %s", *task1.ParentID)
	}
}
