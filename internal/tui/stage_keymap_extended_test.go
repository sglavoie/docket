package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func findStageBindingAction(t *testing.T, km *keymap.KeyMap, key string) keymap.Action {
	t.Helper()

	for _, entry := range km.Entries {
		if entry.PrefixNode == nil || entry.PrefixNode.Key != "s" {
			continue
		}
		for _, child := range entry.PrefixNode.Children {
			if child.Binding != nil && child.Binding.Key == key {
				return child.Binding.Action
			}
		}
	}

	t.Fatalf("stage binding %q not found", key)
	return nil
}

func saveAndReloadProject(t *testing.T, project *models.Project) *models.Project {
	t.Helper()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	return project
}

func TestOutlinerStageKeymapHandlesExtendedStageKeys(t *testing.T) {
	projectDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project := &models.Project{
		ID:          "proj-1",
		Name:        "Project",
		Path:        projectDir,
		NextShortID: 2,
		Tasks:       []*models.Task{task},
	}
	project.BuildTaskTree()
	project = saveAndReloadProject(t, project)

	cfg := &config.Config{Stages: []string{
		"Stage 1", "Stage 2", "Stage 3", "Stage 4", "Stage 5",
		"Stage 6", "Stage 7", "Stage 8", "Stage 9", "Stage 10",
	}}

	loaded := project.GetTaskByID("task-1")
	if loaded == nil {
		t.Fatal("task not found after reload")
	}

	m := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(cfg, nil),
		project:           project,
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		rows:              []outlinerRow{{task: loaded}},
	}

	action := findStageBindingAction(t, m.buildKeyMap(), "a")
	model, cmd := action()
	if model != m {
		t.Fatalf("action returned model %T, want original outliner", model)
	}
	if cmd != nil {
		_ = cmd()
	}

	project = saveAndReloadProject(t, project)
	if got := project.GetTaskByID("task-1").Stage; got != "Stage 10" {
		t.Fatalf("stage = %q, want %q", got, "Stage 10")
	}
}

func TestProjectTreeStageKeymapHandlesExtendedStageKeys(t *testing.T) {
	projectDir, cleanup := testutil.TempProjectDir(t)
	defer cleanup()

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project := &models.Project{
		ID:          "proj-1",
		Name:        "Project",
		Path:        projectDir,
		NextShortID: 2,
		Tasks:       []*models.Task{task},
	}
	project.BuildTaskTree()
	project = saveAndReloadProject(t, project)

	cfg := testutil.NewTestConfig()
	cfg.Stages = []string{
		"Stage 1", "Stage 2", "Stage 3", "Stage 4", "Stage 5",
		"Stage 6", "Stage 7", "Stage 8", "Stage 9", "Stage 10",
	}

	m := NewProjectTreeModel(project, cfg, nil)
	m.cursor = 0

	action := findStageBindingAction(t, m.buildKeyMap(), "a")
	model, cmd := action()
	if model != m {
		t.Fatalf("action returned model %T, want original project tree", model)
	}
	if cmd != nil {
		_ = cmd()
	}

	project = saveAndReloadProject(t, project)
	if got := project.GetTaskByID("task-1").Stage; got != "Stage 10" {
		t.Fatalf("stage = %q, want %q", got, "Stage 10")
	}
}
