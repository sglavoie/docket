package external

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBuildProjectFzfInput(t *testing.T) {
	projects := []*models.Project{
		{Name: "project-a", Context: "work"},
		{Name: "project-b", Context: "personal"},
	}

	// Simulate the input building logic from FzfSelectProject
	var lines []string
	for _, p := range projects {
		line := fmt.Sprintf("%s\t%s\t%d tasks", p.Name, p.Context, p.CountTasks())
		lines = append(lines, line)
	}
	input := strings.Join(lines, "\n")

	t.Run("formats project lines correctly", func(t *testing.T) {
		if !strings.Contains(input, "project-a\twork\t0 tasks") {
			t.Error("expected first project line with name, context, and task count")
		}
		if !strings.Contains(input, "project-b\tpersonal\t0 tasks") {
			t.Error("expected second project line with name, context, and task count")
		}
	})

	t.Run("uses tab as delimiter", func(t *testing.T) {
		firstLine := strings.Split(input, "\n")[0]
		parts := strings.Split(firstLine, "\t")
		if len(parts) != 3 {
			t.Errorf("expected 3 tab-separated parts, got %d", len(parts))
		}
	})
}

func TestBuildTaskFzfInput(t *testing.T) {
	tasks := []*models.Task{
		{ShortID: 1, Type: models.TaskTypeTask, Status: models.StatusTodo, Title: "First task"},
		{ShortID: 2, Type: models.TaskTypeBug, Status: models.StatusDone, Title: "Bug fix"},
	}

	// Simulate the input building logic from FzfSelectTask
	var lines []string
	for _, task := range tasks {
		line := fmt.Sprintf("#%d\t%s\t%s\t%s",
			task.ShortID,
			task.Type.Icon(),
			task.Status.Icon(),
			task.Title)
		lines = append(lines, line)
	}
	input := strings.Join(lines, "\n")

	t.Run("formats task lines correctly", func(t *testing.T) {
		if !strings.Contains(input, "#1\t") {
			t.Error("expected task line to start with #1")
		}
		if !strings.Contains(input, "#2\t") {
			t.Error("expected task line to start with #2")
		}
		if !strings.Contains(input, "First task") {
			t.Error("expected first task title in output")
		}
		if !strings.Contains(input, "Bug fix") {
			t.Error("expected second task title in output")
		}
	})

	t.Run("uses tab as delimiter", func(t *testing.T) {
		firstLine := strings.Split(input, "\n")[0]
		parts := strings.Split(firstLine, "\t")
		if len(parts) != 4 {
			t.Errorf("expected 4 tab-separated parts, got %d", len(parts))
		}
	})

	t.Run("short ID prefixed with hash", func(t *testing.T) {
		firstLine := strings.Split(input, "\n")[0]
		if !strings.HasPrefix(firstLine, "#") {
			t.Error("expected line to start with # for short ID")
		}
	})
}

func TestParseTaskFzfOutput(t *testing.T) {
	taskMap := map[int]*models.Task{
		1: {ShortID: 1, Title: "First task"},
		2: {ShortID: 2, Title: "Second task"},
	}

	t.Run("parses valid selection", func(t *testing.T) {
		selected := "#1\t📋\t⬜\tFirst task"
		parts := strings.Split(selected, "\t")
		if len(parts) == 0 {
			t.Fatal("expected parts from split")
		}
		idStr := strings.TrimPrefix(parts[0], "#")
		var shortID int
		_, err := fmt.Sscanf(idStr, "%d", &shortID)
		if err != nil {
			t.Fatalf("failed to parse short ID: %v", err)
		}
		task, ok := taskMap[shortID]
		if !ok {
			t.Fatal("expected to find task in map")
		}
		if task.Title != "First task" {
			t.Errorf("expected 'First task', got %q", task.Title)
		}
	})

	t.Run("handles whitespace in selection", func(t *testing.T) {
		selected := strings.TrimSpace("  #2\t📋\t⬜\tSecond task  \n")
		parts := strings.Split(selected, "\t")
		idStr := strings.TrimPrefix(parts[0], "#")
		var shortID int
		_, err := fmt.Sscanf(idStr, "%d", &shortID)
		if err != nil {
			t.Fatalf("failed to parse short ID: %v", err)
		}
		if shortID != 2 {
			t.Errorf("expected short ID 2, got %d", shortID)
		}
	})
}

func TestParseMultiSelectOutput(t *testing.T) {
	taskMap := map[int]*models.Task{
		1: {ShortID: 1, Title: "First task"},
		2: {ShortID: 2, Title: "Second task"},
		3: {ShortID: 3, Title: "Third task"},
	}

	output := "#1\t📋\t⬜\tFirst task\n#3\t📋\t⬜\tThird task"

	var selectedTasks []*models.Task
	for line := range strings.SplitSeq(output, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) > 0 {
			idStr := strings.TrimPrefix(parts[0], "#")
			var shortID int
			if _, err := fmt.Sscanf(idStr, "%d", &shortID); err == nil {
				if task, ok := taskMap[shortID]; ok {
					selectedTasks = append(selectedTasks, task)
				}
			}
		}
	}

	if len(selectedTasks) != 2 {
		t.Errorf("expected 2 selected tasks, got %d", len(selectedTasks))
	}
	if selectedTasks[0].ShortID != 1 {
		t.Errorf("expected first selected task to have ShortID 1, got %d", selectedTasks[0].ShortID)
	}
	if selectedTasks[1].ShortID != 3 {
		t.Errorf("expected second selected task to have ShortID 3, got %d", selectedTasks[1].ShortID)
	}
}
