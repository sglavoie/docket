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
	new, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

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
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.Comments = []*models.Comment{{Text: "a", CreatedAt: time.Now()}, {Text: "b", CreatedAt: time.Now()}}
	new, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
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
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	new, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	new.DependsOn = []string{"dep-1"}

	diffTask(old, new)

	found := findActivity(new, "dependencies")
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
	new, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	new.DependsOn = []string{"dep-1"}

	diffTask(old, new)

	if findActivity(new, "dependencies") != nil {
		t.Fatal("expected no dependencies activity entry when unchanged")
	}
}

func TestDiffTaskRelated(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	new, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	new.RelatedTo = []string{"rel-1"}

	diffTask(old, new)

	found := findActivity(new, "related")
	if found == nil {
		t.Fatal("expected related activity entry")
	}
}

func TestDiffTaskAttachments(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	new, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
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
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	new, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
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
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.ParentID = &parentID
	new, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newParentID := "parent-1"
	new.ParentID = &newParentID

	diffTask(old, new)

	if findActivity(new, "parent") != nil {
		t.Fatal("expected no parent activity entry when unchanged")
	}
}

func TestDiffTaskNoChanges(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	new, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	diffTask(old, new)

	if len(new.Activity) != 0 {
		t.Errorf("expected no activity entries, got %d", len(new.Activity))
	}
}

func TestDiffTaskRecurrenceAdded(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}

	diffTask(old, newT)

	found := findActivity(newT, "recurrence")
	if found == nil {
		t.Fatal("expected recurrence activity entry")
	}
	if found.OldValue != "none" || found.NewValue != "daily" {
		t.Errorf("expected none→daily, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskRecurrenceRemoved(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly}
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	diffTask(old, newT)

	found := findActivity(newT, "recurrence")
	if found == nil {
		t.Fatal("expected recurrence activity entry")
	}
	if found.OldValue != "weekly" || found.NewValue != "none" {
		t.Errorf("expected weekly→none, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskRecurrenceUnchanged(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}

	diffTask(old, newT)

	if findActivity(newT, "recurrence") != nil {
		t.Fatal("expected no recurrence activity entry when unchanged")
	}
}

func TestDiffTaskChecklistAdded(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT.Checklist = []models.ChecklistItem{
		{Text: "item 1", Done: false},
		{Text: "item 2", Done: true},
	}

	diffTask(old, newT)

	found := findActivity(newT, "checklist")
	if found == nil {
		t.Fatal("expected checklist activity entry")
	}
	if found.OldValue != "0/0" || found.NewValue != "1/2" {
		t.Errorf("expected 0/0→1/2, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskChecklistItemCompleted(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.Checklist = []models.ChecklistItem{
		{Text: "item 1", Done: false},
		{Text: "item 2", Done: false},
	}
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT.Checklist = []models.ChecklistItem{
		{Text: "item 1", Done: true},
		{Text: "item 2", Done: false},
	}

	diffTask(old, newT)

	found := findActivity(newT, "checklist")
	if found == nil {
		t.Fatal("expected checklist activity entry")
	}
	if found.OldValue != "0/2" || found.NewValue != "1/2" {
		t.Errorf("expected 0/2→1/2, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskChecklistUnchanged(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.Checklist = []models.ChecklistItem{{Text: "item", Done: true}}
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT.Checklist = []models.ChecklistItem{{Text: "item", Done: true}}

	diffTask(old, newT)

	if findActivity(newT, "checklist") != nil {
		t.Fatal("expected no checklist activity entry when unchanged")
	}
}

func TestDiffTaskScoreDimensions(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT.ScoreDimensions = map[string]int{"impact": 5}

	diffTask(old, newT)

	found := findActivity(newT, "score")
	if found == nil {
		t.Fatal("expected score activity entry")
	}
}

func TestDiffTaskScoreDimensionsUnchanged(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	old.ScoreDimensions = map[string]int{"impact": 5}
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT.ScoreDimensions = map[string]int{"impact": 5}

	diffTask(old, newT)

	if findActivity(newT, "score") != nil {
		t.Fatal("expected no score activity entry when unchanged")
	}
}

func TestDiffTaskCustomSection(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT.CustomSection = "sprint-1"

	diffTask(old, newT)

	found := findActivity(newT, "custom_section")
	if found == nil {
		t.Fatal("expected custom_section activity entry")
	}
	if found.OldValue != "none" || found.NewValue != "sprint-1" {
		t.Errorf("expected none→sprint-1, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskURL(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT.URL = "https://example.com"

	diffTask(old, newT)

	found := findActivity(newT, "url")
	if found == nil {
		t.Fatal("expected url activity entry")
	}
	if found.OldValue != "none" || found.NewValue != "https://example.com" {
		t.Errorf("expected none→https://example.com, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffTaskSource(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	newT.Source = "github"

	diffTask(old, newT)

	found := findActivity(newT, "source")
	if found == nil {
		t.Fatal("expected source activity entry")
	}
	if found.OldValue != "none" || found.NewValue != "github" {
		t.Errorf("expected none→github, got %s→%s", found.OldValue, found.NewValue)
	}
}

func TestDiffAndRecordActivity_PrunesExcessEntries(t *testing.T) {
	// Create "old" snapshot with a task that already has many activity entries
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	// Create "new" snapshot of the same task with 60 pre-existing entries
	newT, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	for i := range 60 {
		newT.Activity = append(newT.Activity, &models.ActivityEntry{
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Field:     "status",
			OldValue:  "old",
			NewValue:  "new",
		})
	}

	// Change status to trigger one more activity entry
	old.Status = models.StatusTodo
	newT.Status = models.StatusInProgress

	maxEntries := 10
	DiffAndRecordActivity([]*models.Task{old}, []*models.Task{newT}, maxEntries)

	// Should be capped at maxEntries
	if len(newT.Activity) != maxEntries {
		t.Errorf("expected %d activity entries after pruning, got %d", maxEntries, len(newT.Activity))
	}

	// The newest entry should be the status change we just triggered
	newest := newT.Activity[len(newT.Activity)-1]
	if newest.Field != "status" || newest.NewValue != "in_progress" {
		t.Errorf("expected newest entry to be status change, got field=%q new=%q", newest.Field, newest.NewValue)
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
