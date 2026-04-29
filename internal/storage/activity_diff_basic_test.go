package storage

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestDiffTaskComments(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	newTask.Comments = []*models.Comment{{Text: "hello", CreatedAt: time.Now()}}

	diffTask(old, newTask)

	found := findActivity(newTask, "comments")
	if found == nil {
		t.Fatal("expected comments activity entry")
	}
	if found.OldValue != "0" || found.NewValue != "1" {
		t.Errorf("expected 0→1, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskCommentsRemoved(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.Comments = []*models.Comment{{Text: "a", CreatedAt: time.Now()}, {Text: "b", CreatedAt: time.Now()}}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.Comments = []*models.Comment{{Text: "a", CreatedAt: time.Now()}}

	diffTask(old, newTask)

	found := findActivity(newTask, "comments")
	if found == nil {
		t.Fatal("expected comments activity entry")
	}
	if found.OldValue != "2" || found.NewValue != "1" {
		t.Errorf("expected 2→1, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskDependencies(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.DependsOn = []string{"dep-1"}

	diffTask(old, newTask)

	found := findActivity(newTask, "dependencies")
	if found == nil {
		t.Fatal("expected dependencies activity entry")
	}
}

func TestDiffTaskDependenciesUnchanged(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.DependsOn = []string{"dep-1"}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.DependsOn = []string{"dep-1"}

	diffTask(old, newTask)

	if findActivity(newTask, "dependencies") != nil {
		t.Fatal("expected no dependencies activity entry when unchanged")
	}
}

func TestDiffTaskRelated(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.RelatedTo = []string{"rel-1"}

	diffTask(old, newTask)

	found := findActivity(newTask, "related")
	if found == nil {
		t.Fatal("expected related activity entry")
	}
}

func TestDiffTaskAttachments(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask.Attachments = []models.Attachment{{Filename: "file.txt", AddedAt: time.Now()}}

	diffTask(old, newTask)

	found := findActivity(newTask, "attachments")
	if found == nil {
		t.Fatal("expected attachments activity entry")
	}
	if found.OldValue != "0" || found.NewValue != "1" {
		t.Errorf("expected 0→1, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskParentID(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	parentID := "parent-1"
	newTask.ParentID = &parentID

	diffTask(old, newTask)

	found := findActivity(newTask, "parent")
	if found == nil {
		t.Fatal("expected parent activity entry")
	}
	if found.OldValue != "none" || found.NewValue != "parent-1" {
		t.Errorf("expected none→parent-1, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskParentIDUnchanged(t *testing.T) {
	parentID := "parent-1"
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.ParentID = &parentID
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newParentID := "parent-1"
	newTask.ParentID = &newParentID

	diffTask(old, newTask)

	if findActivity(newTask, "parent") != nil {
		t.Fatal("expected no parent activity entry when unchanged")
	}
}

func TestDiffTaskNoChanges(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	diffTask(old, newTask)

	if len(newTask.Activity) != 0 {
		t.Errorf("expected no activity entries, got %d", len(newTask.Activity))
	}
}
