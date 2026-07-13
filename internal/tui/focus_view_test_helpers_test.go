package tui

import (
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// newTestFocusConfig creates a test config with the given projects.
func newTestFocusConfig(projects []*models.Project) *config.Config {
	cfg := &config.Config{
		Projects: projects,
	}
	return cfg
}

// newTestFocusProject creates a test project for focus view tests.
func newTestFocusProject(t *testing.T, name string) *models.Project {
	t.Helper()
	project, err := models.NewProject(uuid.New().String(), name, "")
	if err != nil {
		t.Fatal(err)
	}
	return project
}

// newTestFocusTask creates a task. The active flag is retired (membership is now
// derived from the Focus Queue), so it is accepted for call-site compatibility
// but no longer applied to the task.
func newTestFocusTask(t *testing.T, title string, priority int, _ bool) *models.Task {
	t.Helper()
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	if err != nil {
		t.Fatal(err)
	}
	task.Priority = priority
	return task
}

// newTestFocusViewModel creates a FocusViewModel with session state for testing.
func newTestFocusViewModel(cfg *config.Config) *FocusViewModel {
	queue := []FocusTask{}
	queueIDs := make(map[string]bool)
	dismissedIDs := make(map[string]bool)
	return &FocusViewModel{
		ViewBundle:   ViewBundle{config: cfg},
		queue:        &queue,
		queueIDs:     queueIDs,
		dismissedIDs: dismissedIDs,
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
