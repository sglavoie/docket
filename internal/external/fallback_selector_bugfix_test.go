package external

import (
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// TestFallbackMultiSelectTasks_Deduplicates verifies that entering the same
// item number multiple times (e.g. "1,1,2") selects each task at most once.
// Without dedup, bulk operations would double-apply to the duplicated task.
func TestFallbackMultiSelectTasks_Deduplicates(t *testing.T) {
	tasks := make([]*models.Task, 3)
	for i := range tasks {
		tsk, err := models.NewTask(uuid.New().String(), i+1, models.TaskTypeTask, "task")
		if err != nil {
			t.Fatalf("new task: %v", err)
		}
		tasks[i] = tsk
	}

	// Pipe "1,1,2" through stdin.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	if _, err := w.WriteString("1,1,2\n"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()

	selected, err := FallbackMultiSelectTasks(tasks, "pick:")
	if err != nil {
		t.Fatalf("FallbackMultiSelectTasks: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 unique selections (dedup), got %d (%v)", len(selected), selected)
	}
	if selected[0] != tasks[0] || selected[1] != tasks[1] {
		t.Errorf("expected tasks[0] and tasks[1], got %v", selected)
	}
}
