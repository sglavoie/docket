package storage

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func Test_diffAndRecordActivity_PrunesExcessEntries(t *testing.T) {
	old, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}

	newTask, err := models.NewTask("1", 1, models.TaskTypeTask, "Test")
	if err != nil {
		t.Fatal(err)
	}
	for i := range 60 {
		newTask.Activity = append(newTask.Activity, &models.ActivityEntry{
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			Field:     "status",
			OldValue:  "old",
			NewValue:  "new",
		})
	}

	old.Status = models.StatusTodo
	newTask.Status = models.StatusInProgress

	maxEntries := 10
	diffAndRecordActivity([]*models.Task{old}, []*models.Task{newTask}, maxEntries)

	if len(newTask.Activity) != maxEntries {
		t.Errorf("expected %d activity entries after pruning, got %d", maxEntries, len(newTask.Activity))
	}

	newest := newTask.Activity[len(newTask.Activity)-1]
	if newest.Field != "status" || newest.NewValue != "in_progress" {
		t.Errorf("expected newest entry to be status change, got field=%q new=%q", newest.Field, newest.NewValue)
	}
}