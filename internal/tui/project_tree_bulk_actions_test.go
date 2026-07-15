package tui

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	appsvc "github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

// newProjectTreeBulkActionTestApp seeds the test project on disk under a shared
// commit service + workspace ledger and wires the secondary coordinators, so the
// migrated shared picker/palette paths (e.g. bulk tags) commit through the
// application layer. The returned ledger backs undo assertions.
func newProjectTreeBulkActionTestApp(t *testing.T) (*AppModel, *models.Project) {
	t.Helper()

	app, project := newTestAppModelWithProject()

	dataDir := t.TempDir()
	project.Path = filepath.Join(dataDir, "projects", models.Slugify(project.Name))
	project.BuildTaskTree()

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newScenarioRepo(commitSvc, project)
	repo.seed(t)
	app.secondaryFeatures = newSecondaryCoordinators(appsvc.NewService(repo, nil, nil), app.configManager)

	app.currentProject = project
	setTestAppProjects(app, project)
	setTestCurrentProject(app, project.ID)
	app.projectTree = NewProjectTreeModel(project, app.configManager, app.runtime)
	app.wireSecondaryCoordinators(app.projectTree)
	app.viewStack = []ViewType{ViewProjectTree}
	app.views.Register(ViewProjectTree, app.projectTree)
	return app, project
}

func findProjectTreeBindingAction(t *testing.T, km *keymap.KeyMap, keys ...string) keymap.Action {
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
		t.Fatalf("project tree binding %v not found", keys)
	nextKey:
	}

	t.Fatalf("project tree binding %v did not resolve to an action", keys)
	return nil
}

func selectProjectTreeVisibleTasks(t *testing.T, tree *ProjectTreeModel, cursor int, selectedIndexes ...int) []*models.Task {
	t.Helper()

	visible := tree.visibleNodes()
	if len(visible) <= cursor {
		t.Fatalf("visible nodes = %d, need cursor %d", len(visible), cursor)
	}

	tree.cursor = cursor
	tree.multiSelectMode = true
	tree.selectedTasks = make(map[string]bool)

	selected := make([]*models.Task, 0, len(selectedIndexes))
	for _, idx := range selectedIndexes {
		if len(visible) <= idx {
			t.Fatalf("visible nodes = %d, need selected index %d", len(visible), idx)
		}
		task := visible[idx].Task
		tree.selectedTasks[task.ID] = true
		selected = append(selected, task)
	}

	return selected
}

func taskIDs(tasks []*models.Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func taskRefIDs(refs []TaskProjectRef) []string {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.Task != nil {
			ids = append(ids, ref.Task.ID)
		}
	}
	return ids
}

func runProjectTreeAction(t *testing.T, action keymap.Action) tea.Msg {
	t.Helper()

	_, cmd := action()
	if cmd == nil {
		t.Fatal("expected action to return a command")
	}
	return cmd()
}

func failOnErrorMsg(t *testing.T, msg tea.Msg) {
	t.Helper()

	if errMsg, ok := msg.(ErrorMsg); ok {
		t.Fatalf("unexpected ErrorMsg: %v", errMsg.Err)
	}
}

func TestProjectTreeManageTagsUsesSelectedTasksWhenCursorElsewhere(t *testing.T) {
	app, project := newProjectTreeBulkActionTestApp(t)
	selected := selectProjectTreeVisibleTasks(t, app.projectTree, 0, 1, 2)
	cursorTaskID := app.projectTree.visibleNodes()[0].Task.ID

	action := findProjectTreeBindingAction(t, app.projectTree.buildKeyMap(), "t", "t")
	msg, ok := runProjectTreeAction(t, action).(ShowBulkTagSelectorMsg)
	if !ok {
		t.Fatalf("expected ShowBulkTagSelectorMsg, got %T", msg)
	}
	if got, want := taskRefIDs(msg.TaskRefs), taskIDs(selected); !reflect.DeepEqual(got, want) {
		t.Fatalf("bulk tag refs = %v, want selected task IDs %v", got, want)
	}

	handled, cmd := app.handleTagMessages(BulkTagSelectorDoneMsg{
		TaskRefs:     msg.TaskRefs,
		SelectedTags: []string{"bulk-tag"},
	})
	if !handled {
		t.Fatal("BulkTagSelectorDoneMsg was not handled")
	}

	// The migrated tag palette commits asynchronously through the BulkCoordinator;
	// the shell clears the selection on the committed batch result.
	batch := runBatchResult(t, cmd)
	app.handleBatchTransactionResult(batch)

	reloaded := loadBulkPickerTestProject(t, project)
	for _, selectedTask := range selected {
		task := reloaded.GetTaskByID(selectedTask.ID)
		if task == nil {
			t.Fatalf("selected task %q not found", selectedTask.ID)
		}
		if !reflect.DeepEqual(task.Tags, []string{"bulk-tag"}) {
			t.Fatalf("selected task %q tags = %v, want [bulk-tag]", selectedTask.ID, task.Tags)
		}
	}
	cursorTask := reloaded.GetTaskByID(cursorTaskID)
	if cursorTask == nil {
		t.Fatal("cursor task not found")
	}
	if reflect.DeepEqual(cursorTask.Tags, []string{"bulk-tag"}) {
		t.Fatalf("cursor task tags = %v, should not receive selected-task tag mutation", cursorTask.Tags)
	}
	if app.projectTree.multiSelectMode || len(app.projectTree.selectedTasks) != 0 {
		t.Fatal("bulk tag completion should clear Project Tree multi-select")
	}
}

func TestProjectTreeDueDatePickerUsesSelectedTasksWhenCursorElsewhere(t *testing.T) {
	app, project := newProjectTreeBulkActionTestApp(t)
	selected := selectProjectTreeVisibleTasks(t, app.projectTree, 0, 1, 2)
	cursorTaskID := app.projectTree.visibleNodes()[0].Task.ID

	action := findProjectTreeBindingAction(t, app.projectTree.buildKeyMap(), "d", "d")
	msg, ok := runProjectTreeAction(t, action).(SetDueDateMsg)
	if !ok {
		t.Fatalf("expected SetDueDateMsg, got %T", msg)
	}
	if got, want := taskRefIDs(msg.TaskRefs), taskIDs(selected); !reflect.DeepEqual(got, want) {
		t.Fatalf("bulk due-date refs = %v, want selected task IDs %v", got, want)
	}

	handled, cmd := app.handleSetDueDate(msg)
	if !handled {
		t.Fatal("SetDueDateMsg was not handled")
	}
	if cmd != nil {
		failOnErrorMsg(t, cmd())
	}

	dueDate := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)
	firstRef := msg.TaskRefs[0]
	firstRef.Project.MutateTask(firstRef.Task.ID, func(task *models.Task) {
		task.DueDate = &dueDate
		task.HasTime = false
	})

	handled, cmd = app.handleDueDateChanged(DueDateChangedMsg{Task: firstRef.Task, Project: firstRef.Project})
	if !handled {
		t.Fatal("DueDateChangedMsg was not handled")
	}

	// The migrated multiselect due-date fallback commits asynchronously through the
	// BulkCoordinator; drain the batch result and let the shell clear the selection.
	batch := runBatchResult(t, cmd)
	app.handleBatchTransactionResult(batch)

	reloaded := loadBulkPickerTestProject(t, project)
	for _, selectedTask := range selected {
		task := reloaded.GetTaskByID(selectedTask.ID)
		if task == nil {
			t.Fatalf("selected task %q not found", selectedTask.ID)
		}
		assertTaskDueDate(t, task, dueDate, false)
	}
	cursorTask := reloaded.GetTaskByID(cursorTaskID)
	if cursorTask == nil {
		t.Fatal("cursor task not found")
	}
	if cursorTask.DueDate != nil {
		t.Fatalf("cursor task due date = %v, should remain nil", cursorTask.DueDate)
	}
	if app.projectTree.multiSelectMode || len(app.projectTree.selectedTasks) != 0 {
		t.Fatal("bulk due-date completion should clear Project Tree multi-select")
	}
}

func TestProjectTreeCommandPaletteUsesSelectedTasksForTagsDueDateAndCopy(t *testing.T) {
	app, _ := newProjectTreeBulkActionTestApp(t)
	selected := selectProjectTreeVisibleTasks(t, app.projectTree, 0, 1, 2)

	commands := app.taskActionCommands()

	tagCmd := findCommand(commands, "manage tags")
	if tagCmd == nil {
		t.Fatal("manage tags command not found")
	}
	tagMsg, ok := tagCmd.Handler().(ShowBulkTagSelectorMsg)
	if !ok {
		t.Fatalf("expected ShowBulkTagSelectorMsg, got %T", tagMsg)
	}
	if got, want := taskRefIDs(tagMsg.TaskRefs), taskIDs(selected); !reflect.DeepEqual(got, want) {
		t.Fatalf("palette bulk tag refs = %v, want %v", got, want)
	}

	dueCmd := findCommand(commands, "set due date")
	if dueCmd == nil {
		t.Fatal("set due date command not found")
	}
	dueMsg, ok := dueCmd.Handler().(SetDueDateMsg)
	if !ok {
		t.Fatalf("expected SetDueDateMsg, got %T", dueMsg)
	}
	if got, want := taskRefIDs(dueMsg.TaskRefs), taskIDs(selected); !reflect.DeepEqual(got, want) {
		t.Fatalf("palette bulk due-date refs = %v, want %v", got, want)
	}

	copyCases := []struct {
		name string
		want any
	}{
		{name: "copy title", want: CopyTasksTitleMsg{}},
		{name: "copy details", want: CopyTasksDetailsMsg{}},
		{name: "copy with description", want: CopyTasksTitleAndDescMsg{}},
	}
	for _, tc := range copyCases {
		selected = selectProjectTreeVisibleTasks(t, app.projectTree, 0, 1, 2)
		cmd := findCommand(commands, tc.name)
		if cmd == nil {
			t.Fatalf("%s command not found", tc.name)
		}
		msg := cmd.Handler()
		switch typed := msg.(type) {
		case CopyTasksTitleMsg:
			if _, ok := tc.want.(CopyTasksTitleMsg); !ok {
				t.Fatalf("%s returned %T", tc.name, msg)
			}
			if got, want := taskIDs(typed.Tasks), taskIDs(selected); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s task IDs = %v, want %v", tc.name, got, want)
			}
		case CopyTasksDetailsMsg:
			if _, ok := tc.want.(CopyTasksDetailsMsg); !ok {
				t.Fatalf("%s returned %T", tc.name, msg)
			}
			if got, want := taskIDs(typed.Tasks), taskIDs(selected); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s task IDs = %v, want %v", tc.name, got, want)
			}
		case CopyTasksTitleAndDescMsg:
			if _, ok := tc.want.(CopyTasksTitleAndDescMsg); !ok {
				t.Fatalf("%s returned %T", tc.name, msg)
			}
			if got, want := taskIDs(typed.Tasks), taskIDs(selected); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s task IDs = %v, want %v", tc.name, got, want)
			}
		default:
			t.Fatalf("%s returned %T, want multi-task copy message", tc.name, msg)
		}
	}
}

func TestProjectTreeCommandPaletteBulkMutationsUseSelectedTasks(t *testing.T) {
	app, project := newProjectTreeBulkActionTestApp(t)
	for _, task := range project.Tasks {
		task.DueDate = nil
	}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	cursorTaskID := app.projectTree.visibleNodes()[0].Task.ID

	selected := selectProjectTreeVisibleTasks(t, app.projectTree, 0, 1, 2)
	selectedIDs := taskIDs(selected)

	dueTodayCmd := findCommand(app.dueDateCommands(), "due today")
	if dueTodayCmd == nil {
		t.Fatal("due today command not found")
	}
	failOnErrorMsg(t, dueTodayCmd.Handler())

	today := util.StartOfDay(time.Now())
	for _, id := range selectedIDs {
		task := project.GetTaskByID(id)
		if task == nil {
			t.Fatalf("selected task %q not found", id)
		}
		if task.DueDate == nil || !task.DueDate.Equal(today) {
			t.Fatalf("selected task %q due date = %v, want %v", id, task.DueDate, today)
		}
	}
	cursorTask := project.GetTaskByID(cursorTaskID)
	if cursorTask == nil {
		t.Fatal("cursor task not found")
	}
	if cursorTask.DueDate != nil {
		t.Fatalf("cursor task due date = %v, should remain nil", cursorTask.DueDate)
	}
	if app.projectTree.multiSelectMode || len(app.projectTree.selectedTasks) != 0 {
		t.Fatal("bulk quick due-date command should clear Project Tree multi-select")
	}
}
