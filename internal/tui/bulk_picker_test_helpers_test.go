package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func newBulkPickerTestTask(t *testing.T, shortID int, title string, tags ...string) *models.Task {
	t.Helper()
	task, err := models.NewTask(uuid.New().String(), shortID, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Tags = append([]string{}, tags...)
	return task
}

func newBulkPickerTestProject(t *testing.T, name string, tasks ...*models.Task) *models.Project {
	t.Helper()
	project, err := models.NewProject(uuid.New().String(), name, filepath.Join(t.TempDir(), models.Slugify(name)))
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", project.Path, err)
	}
	project.Tasks = tasks
	project.NextShortID = len(tasks) + 1
	for _, task := range tasks {
		task.Project = project
	}
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	return project
}

func loadBulkPickerTestProject(t *testing.T, project *models.Project) *models.Project {
	t.Helper()
	loaded, err := models.NewProject(project.ID, project.Name, project.Path)
	if err != nil {
		t.Fatalf("NewProject reload: %v", err)
	}
	if err := storage.LoadProject(loaded); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	return loaded
}

func runBulkPickerBatchCmd(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}

	msgs := make([]tea.Msg, 0, len(batch))
	for _, batchCmd := range batch {
		if batchCmd == nil {
			continue
		}
		msgs = append(msgs, batchCmd())
	}
	return msgs
}

func requireBulkPickerSaveComplete(t *testing.T, msgs []tea.Msg) {
	t.Helper()
	for _, msg := range msgs {
		if _, ok := msg.(SaveCompleteMsg); ok {
			return
		}
	}
	t.Fatalf("expected SaveCompleteMsg, got %#v", msgs)
}

func assertBulkPickerUndoTaskIDs(t *testing.T, project *models.Project, want []string) {
	t.Helper()

	metaPath := filepath.Join(project.Path, ".undo", "meta.json")
	content, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", metaPath, err)
	}

	var meta struct {
		Entries []struct {
			Action  string   `json:"action"`
			TaskIDs []string `json:"task_ids"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(content, &meta); err != nil {
		t.Fatalf("Unmarshal undo meta: %v", err)
	}
	if len(meta.Entries) == 0 {
		t.Fatal("expected at least one undo entry")
	}
	got := meta.Entries[len(meta.Entries)-1].TaskIDs
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("undo task IDs = %v, want %v", got, want)
	}
}
