package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func TestSharedTaskKeymapBulkPickerMessagesUseSelectedRefs(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) (*keymap.KeyMap, []TaskProjectRef)
	}{
		{
			name:  "all tasks",
			setup: setupAllTasksSharedBulkKeymap,
		},
		{
			name:  "focus view",
			setup: setupFocusSharedBulkKeymap,
		},
		{
			name:  "priority view",
			setup: setupPrioritySharedBulkKeymap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			km, want := tt.setup(t)
			assertSharedPrefixGateAllows(t, km, "t")
			assertSharedPrefixGateAllows(t, km, "d")

			tagAction := findSharedTaskKeymapAction(t, km, "t", "t")
			tagMsg, ok := runSharedTaskKeymapAction(t, tagAction).(ShowBulkTagSelectorMsg)
			if !ok {
				t.Fatalf("tt: expected ShowBulkTagSelectorMsg, got %T", tagMsg)
			}
			assertTaskProjectRefs(t, tagMsg.TaskRefs, want)

			dueAction := findSharedTaskKeymapAction(t, km, "d", "d")
			dueMsg, ok := runSharedTaskKeymapAction(t, dueAction).(SetDueDateMsg)
			if !ok {
				t.Fatalf("dd: expected SetDueDateMsg, got %T", dueMsg)
			}
			assertTaskProjectRefs(t, dueMsg.TaskRefs, want)
		})
	}
}

func TestSharedTaskKeymapBulkSectionPickerRejectsCrossProjectSelection(t *testing.T) {
	km, _ := setupAllTasksSharedBulkKeymap(t)

	sectionAction := findSharedTaskKeymapAction(t, km, "t", "s")
	model, cmd := sectionAction()
	if cmd != nil {
		t.Fatalf("ts: expected nil command for cross-project bulk section picker, got %T", cmd())
	}
	allTasks, ok := model.(*AllTasksViewModel)
	if !ok {
		t.Fatalf("ts: expected AllTasksViewModel, got %T", model)
	}
	if got, want := allTasks.statusMessage, "bulk section move not supported across projects"; got != want {
		t.Fatalf("ts status = %q, want %q", got, want)
	}
}

func setupAllTasksSharedBulkKeymap(t *testing.T) (*keymap.KeyMap, []TaskProjectRef) {
	t.Helper()

	projectA, tasksA := newSharedBulkProject("Alpha", "cursor", "selected-alpha")
	projectB, tasksB := newSharedBulkProject("Beta", "other", "selected-beta")
	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{projectA, projectB})

	model := newMinimalAllTasksVM()
	model.PrimaryViewBundle = NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg))
	model.MultiSelectState = MultiSelectState{
		multiSelectMode: true,
		selectedTasks: map[string]bool{
			tasksA[1].ID: true,
			tasksB[1].ID: true,
		},
	}
	model.groups = []AllTasksGroup{
		makeGroup("Alpha", projectA, tasksA...),
		makeGroup("Beta", projectB, tasksB...),
	}
	model.cursor = 0
	model.calculateNavigation()

	want := []TaskProjectRef{
		{Task: tasksA[1], Project: projectA},
		{Task: tasksB[1], Project: projectB},
	}
	return model.buildKeyMap(), want
}

func setupFocusSharedBulkKeymap(t *testing.T) (*keymap.KeyMap, []TaskProjectRef) {
	t.Helper()

	projectA, tasksA := newSharedBulkProject("Alpha", "cursor", "selected-alpha")
	projectB, tasksB := newSharedBulkProject("Beta", "selected-beta")
	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{projectA, projectB})

	model := newTestFocusViewModel(cfg)
	model.MultiSelectState = MultiSelectState{
		multiSelectMode: true,
		selectedTasks: map[string]bool{
			tasksA[1].ID: true,
			tasksB[0].ID: true,
		},
	}
	model.filtered = []FocusTask{
		{Task: tasksA[0], Project: projectA},
		{Task: tasksA[1], Project: projectA},
		{Task: tasksB[0], Project: projectB},
	}
	model.tasks = model.filtered
	model.cursor = 0

	want := []TaskProjectRef{
		{Task: tasksA[1], Project: projectA},
		{Task: tasksB[0], Project: projectB},
	}
	return model.buildKeyMap(), want
}

func setupPrioritySharedBulkKeymap(t *testing.T) (*keymap.KeyMap, []TaskProjectRef) {
	t.Helper()

	project, tasks := newSharedBulkProject("Priority", "cursor", "selected-one", "selected-two")
	cfg := newTestConfig()
	setTestProjects(cfg, []*models.Project{project})

	model := &PriorityViewModel{
		PrimaryViewBundle: NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg)),
		project:           project,
		FilterMixin:       FilterMixin{filter: newFilterState()},
		MultiSelectState:  MultiSelectState{multiSelectMode: true, selectedTasks: map[string]bool{}},
		tabs:              TabState{Names: priorityTabNames},
	}
	model.selectedTasks[tasks[1].ID] = true
	model.selectedTasks[tasks[2].ID] = true
	model.backlog = []*PriorityTask{
		{Task: tasks[0]},
		{Task: tasks[1]},
		{Task: tasks[2]},
	}
	model.totalTasks = len(model.backlog)
	model.cursor = 0

	want := []TaskProjectRef{
		{Task: tasks[1], Project: project},
		{Task: tasks[2], Project: project},
	}
	return model.buildKeyMap(), want
}

func newSharedBulkProject(name string, titles ...string) (*models.Project, []*models.Task) {
	project := testutil.NewTestProject(name)
	tasks := make([]*models.Task, 0, len(titles))
	for i, title := range titles {
		task := testutil.NewTestTask(title, models.TaskTypeTask)
		task.ShortID = i + 1
		task.Project = project
		project.Tasks = append(project.Tasks, task)
		tasks = append(tasks, task)
	}
	project.NextShortID = len(project.Tasks) + 1
	project.BuildTaskTree()
	return project, tasks
}

func findSharedTaskKeymapAction(t *testing.T, km *keymap.KeyMap, keys ...string) keymap.Action {
	t.Helper()

	entries := km.Entries
	for i, key := range keys {
		for _, entry := range entries {
			if i == len(keys)-1 && entry.Binding != nil && entry.Binding.Key == key {
				return entry.Binding.Action
			}
			if entry.PrefixNode != nil && entry.PrefixNode.Key == key {
				entries = entry.PrefixNode.Children
				goto nextKey
			}
		}
		t.Fatalf("binding %v not found", keys)
	nextKey:
	}

	t.Fatalf("binding %v did not resolve to an action", keys)
	return nil
}

func assertSharedPrefixGateAllows(t *testing.T, km *keymap.KeyMap, key string) {
	t.Helper()

	for _, entry := range km.Entries {
		if entry.PrefixNode == nil || entry.PrefixNode.Key != key {
			continue
		}
		if entry.PrefixNode.Gate == nil {
			t.Fatalf("prefix %q has no gate", key)
		}
		if !entry.PrefixNode.Gate() {
			t.Fatalf("prefix %q gate rejected selected task refs", key)
		}
		return
	}
	t.Fatalf("prefix %q not found", key)
}

func runSharedTaskKeymapAction(t *testing.T, action keymap.Action) tea.Msg {
	t.Helper()

	_, cmd := action()
	if cmd == nil {
		t.Fatal("expected action to return a command")
	}
	return cmd()
}

func assertTaskProjectRefs(t *testing.T, got, want []TaskProjectRef) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("refs length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Task != want[i].Task {
			t.Fatalf("ref %d task = %v, want %v", i, got[i].Task, want[i].Task)
		}
		if got[i].Project != want[i].Project {
			t.Fatalf("ref %d project = %v, want %v", i, got[i].Project, want[i].Project)
		}
	}
}
