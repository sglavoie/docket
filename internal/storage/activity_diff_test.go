package storage

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestDiffTaskComments(t *testing.T) {
	old := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	new := models.NewTask("1", 1, models.TaskTypeTask, "Test")

	// Add a comment to new
	new.Comments = []*models.Comment{{Text: "hello", CreatedAt: time.Now()}}

	diffTask(old, new)

	found := findActivity(new, "comments")
	if found == nil {
		t.Fatal("expected comments activity entry")
	}
	if found.OldValue != "0" || found.NewValue != "1" {
		t.Errorf("expected 0→1, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskCommentsRemoved(t *testing.T) {
	old := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	old.Comments = []*models.Comment{{Text: "a", CreatedAt: time.Now()}, {Text: "b", CreatedAt: time.Now()}}
	new := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	new.Comments = []*models.Comment{{Text: "a", CreatedAt: time.Now()}}

	diffTask(old, new)

	found := findActivity(new, "comments")
	if found == nil {
		t.Fatal("expected comments activity entry")
	}
	if found.OldValue != "2" || found.NewValue != "1" {
		t.Errorf("expected 2→1, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskDependencies(t *testing.T) {
	old := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	new := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	new.DependsOn = []string{"dep-1"}

	diffTask(old, new)

	found := findActivity(new, "dependencies")
	if found == nil {
		t.Fatal("expected dependencies activity entry")
	}
}

func TestDiffTaskDependenciesUnchanged(t *testing.T) {
	old := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	old.DependsOn = []string{"dep-1"}
	new := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	new.DependsOn = []string{"dep-1"}

	diffTask(old, new)

	if findActivity(new, "dependencies") != nil {
		t.Fatal("expected no dependencies activity entry when unchanged")
	}
}

func TestDiffTaskRelated(t *testing.T) {
	old := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	new := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	new.RelatedTo = []string{"rel-1"}

	diffTask(old, new)

	found := findActivity(new, "related")
	if found == nil {
		t.Fatal("expected related activity entry")
	}
}

func TestDiffTaskAttachments(t *testing.T) {
	old := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	new := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	new.Attachments = []models.Attachment{{Filename: "file.txt", AddedAt: time.Now()}}

	diffTask(old, new)

	found := findActivity(new, "attachments")
	if found == nil {
		t.Fatal("expected attachments activity entry")
	}
	if found.OldValue != "0" || found.NewValue != "1" {
		t.Errorf("expected 0→1, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskParentID(t *testing.T) {
	old := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	new := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	parentID := "parent-1"
	new.ParentID = &parentID

	diffTask(old, new)

	found := findActivity(new, "parent")
	if found == nil {
		t.Fatal("expected parent activity entry")
	}
	if found.OldValue != "none" || found.NewValue != "parent-1" {
		t.Errorf("expected none→parent-1, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskParentIDUnchanged(t *testing.T) {
	parentID := "parent-1"
	old := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	old.ParentID = &parentID
	new := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	newParentID := "parent-1"
	new.ParentID = &newParentID

	diffTask(old, new)

	if findActivity(new, "parent") != nil {
		t.Fatal("expected no parent activity entry when unchanged")
	}
}

func TestDiffTaskNoChanges(t *testing.T) {
	old := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	new := models.NewTask("1", 1, models.TaskTypeTask, "Test")

	diffTask(old, new)

	if len(new.Activity) != 0 {
		t.Errorf("expected no activity entries, got %d", len(new.Activity))
	}
}

func findActivity(task *models.Task, field string) *models.ActivityEntry {
	for _, e := range task.Activity {
		if e.Field == field {
			return e
		}
	}
	return nil
}
