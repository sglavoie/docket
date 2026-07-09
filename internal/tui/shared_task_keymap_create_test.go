package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

func TestSharedTaskPrefixNewTaskDefaultMessages(t *testing.T) {
	project, tasks := newSharedBulkProject("Default", "parent")
	task := tasks[0]
	view := &sharedTaskPrefixCreateView{task: task, project: project}
	km := sharedTaskPrefixKeyMap(TaskKeymapDeps{
		View:    view,
		HasItem: func() bool { return true },
	})

	tests := []struct {
		name     string
		key      string
		wantSkip bool
	}{
		{name: "new task", key: "n", wantSkip: true},
		{name: "new task with template", key: "N", wantSkip: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := findSharedTaskKeymapAction(t, km, "t", tt.key)
			create, ok := runSharedTaskKeymapAction(t, action).(CreateTaskMsg)
			if !ok {
				t.Fatalf("t%s: expected CreateTaskMsg", tt.key)
			}
			if create.ParentTask != task {
				t.Fatalf("t%s ParentTask = %v, want %v", tt.key, create.ParentTask, task)
			}
			if create.Project != nil {
				t.Fatalf("t%s Project = %v, want nil to preserve default routing", tt.key, create.Project)
			}
			if create.SkipTemplatePicker != tt.wantSkip {
				t.Fatalf("t%s SkipTemplatePicker = %v, want %v", tt.key, create.SkipTemplatePicker, tt.wantSkip)
			}
		})
	}
}

func TestSharedTaskPrefixNewTaskOverrideMessagesUseSelectedProject(t *testing.T) {
	project, tasks := newSharedBulkProject("Override", "parent")
	task := tasks[0]
	view := &sharedTaskPrefixCreateView{task: task, project: project}
	km := sharedTaskPrefixKeyMap(TaskKeymapDeps{
		View:                view,
		HasItem:             func() bool { return true },
		NewTask:             createTaskOverrideAction(view, true),
		NewTaskWithTemplate: createTaskOverrideAction(view, false),
	})

	tests := []struct {
		name     string
		key      string
		wantSkip bool
	}{
		{name: "new task", key: "n", wantSkip: true},
		{name: "new task with template", key: "N", wantSkip: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := findSharedTaskKeymapAction(t, km, "t", tt.key)
			create, ok := runSharedTaskKeymapAction(t, action).(CreateTaskMsg)
			if !ok {
				t.Fatalf("t%s: expected CreateTaskMsg", tt.key)
			}
			if create.ParentTask != task {
				t.Fatalf("t%s ParentTask = %v, want %v", tt.key, create.ParentTask, task)
			}
			if create.Project != project {
				t.Fatalf("t%s Project = %v, want selected project %v", tt.key, create.Project, project)
			}
			if create.SkipTemplatePicker != tt.wantSkip {
				t.Fatalf("t%s SkipTemplatePicker = %v, want %v", tt.key, create.SkipTemplatePicker, tt.wantSkip)
			}
		})
	}
}

func sharedTaskPrefixKeyMap(deps TaskKeymapDeps) *keymap.KeyMap {
	return &keymap.KeyMap{Entries: buildStandardTaskPrefixEntries(deps)}
}

func createTaskOverrideAction(view *sharedTaskPrefixCreateView, skipTemplatePicker bool) keymap.Action {
	return func() (tea.Model, tea.Cmd) {
		task, project := view.getTaskRef()
		return view, func() tea.Msg {
			return CreateTaskMsg{
				ParentTask:         task,
				Project:            project,
				SkipTemplatePicker: skipTemplatePicker,
			}
		}
	}
}

type sharedTaskPrefixCreateView struct {
	task    *models.Task
	project *models.Project
}

func (v *sharedTaskPrefixCreateView) Init() tea.Cmd { return nil }

func (v *sharedTaskPrefixCreateView) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return v, nil
}

func (v *sharedTaskPrefixCreateView) View() tea.View { return tea.NewView("") }

func (v *sharedTaskPrefixCreateView) getTaskRef() (*models.Task, *models.Project) {
	return v.task, v.project
}

func (v *sharedTaskPrefixCreateView) getPersister() Persister { return nil }

func (v *sharedTaskPrefixCreateView) getConfig() *config.Config { return nil }

func (v *sharedTaskPrefixCreateView) getConfigManager() *config.Manager { return nil }

func (v *sharedTaskPrefixCreateView) reloadData() {}

func (v *sharedTaskPrefixCreateView) clampCursor() {}

func (v *sharedTaskPrefixCreateView) setErr(error) {}

func (v *sharedTaskPrefixCreateView) setStatus(string) {}
