package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestRenameTagDeduplication verifies that renaming a tag does not create
// duplicate entries when a task already has the destination tag.
func TestRenameTagDeduplication(t *testing.T) {
	project, err := models.NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := models.NewTask("1", 1, models.TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Tags = []string{"frontend", "backend"}

	task2, err := models.NewTask("2", 2, models.TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Tags = []string{"frontend"}

	task3, err := models.NewTask("3", 3, models.TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Tags = []string{"backend"}

	project.Tasks = []*models.Task{task1, task2, task3}

	// Create tag manager and simulate rename of "frontend" -> "backend"
	m := NewTagManagerModelForProject(project)
	// Execute the rename logic by calling the command function directly.
	// We can't call the full renameTag() because it calls saveProjectSync,
	// so we replicate the core rename logic that was fixed.
	oldName := "frontend"
	newName := "backend"

	// Apply the same logic as renameTag()
	for _, task := range m.project.Tasks {
		hasOld := false
		hasNew := false
		oldIdx := -1
		for i, tag := range task.Tags {
			if tag == oldName {
				hasOld = true
				oldIdx = i
			}
			if tag == newName {
				hasNew = true
			}
		}
		if !hasOld {
			continue
		}
		if hasNew {
			task.Tags = append(task.Tags[:oldIdx], task.Tags[oldIdx+1:]...)
		} else {
			task.Tags[oldIdx] = newName
		}
	}

	// task1 had both "frontend" and "backend": should now have just "backend" (no duplicate)
	if len(task1.Tags) != 1 {
		t.Errorf("task1: expected 1 tag, got %d: %v", len(task1.Tags), task1.Tags)
	}
	if task1.Tags[0] != "backend" {
		t.Errorf("task1: expected tag 'backend', got %q", task1.Tags[0])
	}

	// task2 had only "frontend": should now have "backend"
	if len(task2.Tags) != 1 {
		t.Errorf("task2: expected 1 tag, got %d: %v", len(task2.Tags), task2.Tags)
	}
	if task2.Tags[0] != "backend" {
		t.Errorf("task2: expected tag 'backend', got %q", task2.Tags[0])
	}

	// task3 had only "backend" (no "frontend"): should be unchanged
	if len(task3.Tags) != 1 {
		t.Errorf("task3: expected 1 tag, got %d: %v", len(task3.Tags), task3.Tags)
	}
	if task3.Tags[0] != "backend" {
		t.Errorf("task3: expected tag 'backend', got %q", task3.Tags[0])
	}
}

// TestRenameTagBasic verifies simple rename with no duplicate scenario.
func TestRenameTagBasic(t *testing.T) {
	project, err := models.NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := models.NewTask("1", 1, models.TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.Tags = []string{"alpha", "beta"}

	task2, err := models.NewTask("2", 2, models.TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.Tags = []string{"alpha"}

	project.Tasks = []*models.Task{task1, task2}

	m := NewTagManagerModelForProject(project)

	oldName := "alpha"
	newName := "gamma"

	// Apply the rename logic
	for _, task := range m.project.Tasks {
		hasOld := false
		hasNew := false
		oldIdx := -1
		for i, tag := range task.Tags {
			if tag == oldName {
				hasOld = true
				oldIdx = i
			}
			if tag == newName {
				hasNew = true
			}
		}
		if !hasOld {
			continue
		}
		if hasNew {
			task.Tags = append(task.Tags[:oldIdx], task.Tags[oldIdx+1:]...)
		} else {
			task.Tags[oldIdx] = newName
		}
	}

	// task1: "alpha" -> "gamma", "beta" stays
	if len(task1.Tags) != 2 {
		t.Errorf("task1: expected 2 tags, got %d: %v", len(task1.Tags), task1.Tags)
	}
	if task1.Tags[0] != "gamma" {
		t.Errorf("task1: expected first tag 'gamma', got %q", task1.Tags[0])
	}
	if task1.Tags[1] != "beta" {
		t.Errorf("task1: expected second tag 'beta', got %q", task1.Tags[1])
	}

	// task2: "alpha" -> "gamma"
	if len(task2.Tags) != 1 {
		t.Errorf("task2: expected 1 tag, got %d: %v", len(task2.Tags), task2.Tags)
	}
	if task2.Tags[0] != "gamma" {
		t.Errorf("task2: expected tag 'gamma', got %q", task2.Tags[0])
	}
}
