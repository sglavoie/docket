package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// auxiliaryTaskTestApp builds a registered app over one project holding a story
// that can legally hold a child and a second task the cursor can move to.
func auxiliaryTaskTestApp(t *testing.T) (*AppModel, *models.Project, *models.Task) {
	t.Helper()
	model := newUITestApp()
	target := &models.Task{
		ID: "target", ShortID: 1, Title: "Target",
		Type: models.TaskTypeStory, WorkflowState: models.StateReady,
	}
	other := &models.Task{
		ID: "other", ShortID: 2, Title: "Other",
		Type: models.TaskTypeTask, WorkflowState: models.StateReady,
	}
	project := &models.Project{
		ID: "alpha", Name: "Alpha", Slug: "alpha",
		Tasks: []*models.Task{target, other},
	}
	project.BuildTaskTree()
	setTestAppProjects(model, project)
	return model, project, target
}

func executeTaskOperationForTest(
	t *testing.T,
	model *AppModel,
	id OperationID,
	context CommandContext,
) OperationExecutionResult {
	t.Helper()
	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:      id,
		Context: context,
		Source:  OperationSourceActionSheet,
	})
	if result.Err != nil {
		t.Fatalf("execute %s: %v", id, result.Err)
	}
	return result
}

func TestTaskOperationAuxiliaryCatalogRoutesToProductionMessages(t *testing.T) {
	model, project, task := auxiliaryTaskTestApp(t)
	context := taskOperationTestContext(task, project)

	checks := map[OperationID]func(*testing.T, any){
		operationTaskChecklist: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(ShowTaskChecklistMsg).Task, task)
		},
		operationTaskComments: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(ShowTaskCommentsMsg).Task, task)
		},
		operationTaskDecisions: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(ShowTaskDecisionsMsg).Task, task)
		},
		operationTaskAttachments: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(ShowTaskAttachmentsMsg).Task, task)
		},
		operationTaskActivity: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(OpenTaskActivityMsg).Task, task)
		},
		operationTaskQuickNote: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(OpenQuickNoteMsg).Task, task)
		},
		operationTaskCopyTitle: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(CopyTaskTitleMsg).Task, task)
		},
		operationTaskCopyDetails: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(CopyTaskDetailsMsg).Task, task)
		},
		operationTaskCopyTitleDescription: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(CopyTaskTitleAndDescMsg).Task, task)
		},
		operationTaskCopyReference: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(CopyTaskReferenceMsg).Task, task)
		},
		operationTaskScore: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(ShowScoreEditorMsg).Task, task)
		},
		operationTaskPark: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(ShowParkInputMsg).Task, task)
		},
		operationTaskLinkedTasks: func(t *testing.T, msg any) {
			assertTaskMessageTarget(t, msg.(ShowLinkedTaskSelectorMsg).Task, task)
		},
		operationTaskNewChild: func(t *testing.T, msg any) {
			create := msg.(CreateTaskMsg)
			assertTaskMessageTarget(t, create.ParentTask, task)
			if !create.SkipTemplatePicker {
				t.Error("new child task offered the template picker")
			}
		},
		operationTaskNewChildFromTemplate: func(t *testing.T, msg any) {
			create := msg.(CreateTaskMsg)
			assertTaskMessageTarget(t, create.ParentTask, task)
			if create.SkipTemplatePicker {
				t.Error("new child from template skipped the template picker")
			}
		},
		operationTaskNewLinkedSubtask: func(t *testing.T, msg any) {
			linked := msg.(CreateLinkedTaskMsg)
			assertTaskMessageTarget(t, linked.SourceTask, task)
			if linked.LinkType != LinkTypeSubtask {
				t.Errorf("link type = %v, want subtask", linked.LinkType)
			}
		},
		operationTaskPin: func(t *testing.T, msg any) {
			if msg.(OperationCommandResultMsg).Cmd == nil {
				t.Error("pin produced no coordinator command")
			}
		},
		operationTaskDim: func(t *testing.T, msg any) {
			if msg.(OperationCommandResultMsg).Cmd == nil {
				t.Error("dim produced no coordinator command")
			}
		},
	}

	for id, check := range checks {
		result := executeTaskOperationForTest(t, model, id, context)
		if result.Message == nil {
			t.Fatalf("%s produced no message", id)
		}
		check(t, result.Message)
	}
}

func assertTaskMessageTarget(t *testing.T, got, want *models.Task) {
	t.Helper()
	if got != want {
		t.Errorf("message target = %v, want %v", got, want)
	}
}

func TestTaskOperationPairedActionsFollowLiveTaskState(t *testing.T) {
	model, project, task := auxiliaryTaskTestApp(t)

	pairs := []struct {
		enabled  OperationID
		disabled OperationID
		flip     func()
	}{
		{operationTaskPin, operationTaskUnpin, func() { task.Pinned = true }},
		{operationTaskDim, operationTaskUndim, func() { task.Dimmed = true }},
		{operationTaskPark, operationTaskUnpark, func() {
			parked := time.Now()
			task.ParkedAt = &parked
		}},
	}
	for _, pair := range pairs {
		if !taskOperationApplicable(t, model, pair.enabled, task, project) {
			t.Errorf("%s was unavailable on a task that has not been acted on", pair.enabled)
		}
		if taskOperationApplicable(t, model, pair.disabled, task, project) {
			t.Errorf("%s was offered alongside %s", pair.disabled, pair.enabled)
		}
		pair.flip()
		if taskOperationApplicable(t, model, pair.enabled, task, project) {
			t.Errorf("%s stayed available after the task entered that state", pair.enabled)
		}
		if !taskOperationApplicable(t, model, pair.disabled, task, project) {
			t.Errorf("%s was unavailable on a task already in that state", pair.disabled)
		}
	}
}

func TestTaskOperationParkIsUnavailableOnTerminalTasks(t *testing.T) {
	model, project, task := auxiliaryTaskTestApp(t)
	task.WorkflowState = models.StateDone
	if taskOperationApplicable(t, model, operationTaskPark, task, project) {
		t.Error("park was offered on a completed task")
	}
}

func TestTaskOperationSheetReusesCoreEditAndDetailExactlyOnce(t *testing.T) {
	model, project, task := auxiliaryTaskTestApp(t)
	descriptors := TaskActionSheetDescriptors(
		model.operationRegistry,
		taskOperationTestContext(task, project),
	)

	labels := make(map[string][]OperationID)
	for _, descriptor := range descriptors {
		key := strings.ToLower(descriptor.Label)
		labels[key] = append(labels[key], descriptor.ID)
	}
	for _, expected := range []struct {
		label string
		id    OperationID
	}{
		{"edit task", operationCoreEdit},
		{"open task detail", operationCoreDetail},
	} {
		ids := labels[expected.label]
		if len(ids) != 1 {
			t.Fatalf("%q appears %d times in the task sheet: %v", expected.label, len(ids), ids)
		}
		if ids[0] != expected.id {
			t.Errorf("%q is served by %s, want the core operation %s", expected.label, ids[0], expected.id)
		}
	}
}

// TestTaskOperationCatalogIsFullyManifested guards against an auxiliary
// operation reaching the sheet without a manifest rank, which would silently
// sort it after every ranked action.
func TestTaskOperationCatalogIsFullyManifested(t *testing.T) {
	model, project, task := auxiliaryTaskTestApp(t)
	descriptors := TaskActionSheetDescriptors(
		model.operationRegistry,
		taskOperationTestContext(task, project),
	)
	if len(descriptors) == 0 {
		t.Fatal("the task sheet resolved no actions")
	}
	sentinel := len(taskOperationManifest())
	for _, descriptor := range descriptors {
		if taskOperationManifestRank(descriptor.ID) == sentinel {
			t.Errorf("%s reaches the task sheet without a manifest entry", descriptor.ID)
		}
	}
}

func TestTaskOperationAliasesCoverUserTerminology(t *testing.T) {
	model, _, _ := auxiliaryTaskTestApp(t)
	wanted := map[OperationID]string{
		operationTaskChecklist:        "checklist",
		operationTaskComments:         "add comment",
		operationTaskDecisions:        "design decisions",
		operationTaskAttachments:      "add attachment",
		operationTaskActivity:         "task history",
		operationTaskQuickNote:        "jot note",
		operationTaskCopyTitle:        "yank title",
		operationTaskCopyReference:    "copy task id",
		operationTaskScore:            "set score",
		operationTaskPark:             "note where you left off",
		operationTaskUnpark:           "resume task",
		operationTaskPin:              "keep on top",
		operationTaskDim:              "mute task",
		operationTaskLinkedTasks:      "linked tasks",
		operationTaskNewChild:         "new subtask",
		operationTaskNewLinkedSubtask: "linked subtask",
		operationCoreEdit:             "rename task",
		operationCoreDetail:           "task overview",
	}
	for id, alias := range wanted {
		descriptor, ok := model.operationRegistry.Descriptor(id)
		if !ok {
			t.Fatalf("operation %s is not registered", id)
		}
		if !descriptorHasAlias(descriptor, alias) {
			t.Errorf("%s aliases %v are missing %q", id, descriptor.Aliases, alias)
		}
	}
}

func descriptorHasAlias(descriptor OperationDescriptor, alias string) bool {
	for _, candidate := range descriptor.Aliases {
		if strings.EqualFold(candidate, alias) {
			return true
		}
	}
	return false
}

// TestTaskOperationExecutesAgainstFrozenTargetAfterCursorMove is the core
// session guarantee: a sheet opened over one row keeps acting on that row even
// after the Outliner cursor lands somewhere else.
func TestTaskOperationExecutesAgainstFrozenTargetAfterCursorMove(t *testing.T) {
	model, project, task := auxiliaryTaskTestApp(t)
	context := taskOperationTestContext(task, project)

	other := project.GetTaskByID("other")
	out := model.crossOutliner
	out.loadedProjects = []*models.Project{project}
	out.rows = []outlinerRow{{task: other, ProjectID: project.ID}}
	out.cursor = 0

	detail := executeTaskOperationForTest(t, model, operationCoreDetail, context)
	if got := detail.Message.(ShowTaskDetailMsg).Task; got != task {
		t.Errorf("detail target = %v, want the frozen task %v", got, task)
	}
	edit := executeTaskOperationForTest(t, model, operationCoreEdit, context)
	if got := edit.Message.(EditTaskMsg).Task; got != task {
		t.Errorf("edit target = %v, want the frozen task %v", got, task)
	}
	copied := executeTaskOperationForTest(t, model, operationTaskCopyTitle, context)
	if got := copied.Message.(CopyTaskTitleMsg).Task; got != task {
		t.Errorf("copy target = %v, want the frozen task %v", got, task)
	}
}

// newChildHierarchyTestProject builds a leaf task, an epic, and a story chain
// that reaches the nesting cap, so the child creators can be probed against
// every relationship the domain accepts or rejects.
func newChildHierarchyTestProject() *models.Project {
	leaf := &models.Task{
		ID: "leaf", ShortID: 1, Title: "Leaf",
		Type: models.TaskTypeTask, WorkflowState: models.StateReady,
	}
	epic := &models.Task{
		ID: "epic", ShortID: 2, Title: "Epic",
		Type: models.TaskTypeEpic, WorkflowState: models.StateReady,
	}
	tasks := []*models.Task{leaf, epic}
	parentID := epic.ID
	for level := 1; level <= models.MaxNestingDepth; level++ {
		id := fmt.Sprintf("story-%d", level)
		owner := parentID
		tasks = append(tasks, &models.Task{
			ID: id, ShortID: level + 2, Title: id, ParentID: &owner,
			Type: models.TaskTypeStory, WorkflowState: models.StateReady,
		})
		parentID = id
	}
	project := &models.Project{
		ID: "alpha", Name: "Alpha", Slug: "alpha",
		Tasks: tasks,
	}
	project.BuildTaskTree()
	return project
}

func TestTaskChildActionsFollowHierarchyLeafAndDepthLimits(t *testing.T) {
	model := newUITestApp()
	project := newChildHierarchyTestProject()
	setTestAppProjects(model, project)

	for _, testCase := range []struct {
		taskID string
		id     OperationID
		want   bool
	}{
		{taskID: "epic", id: operationTaskNewChild, want: true},
		{taskID: "epic", id: operationTaskNewChildFromTemplate, want: true},
		{taskID: "story-5", id: operationTaskNewChild, want: true},
		{taskID: "leaf", id: operationTaskNewChild, want: false},
		{taskID: "leaf", id: operationTaskNewChildFromTemplate, want: false},
		{taskID: "story-6", id: operationTaskNewChild, want: false},
		{taskID: "story-6", id: operationTaskNewChildFromTemplate, want: false},
		// Promotion makes a leaf source able to parent, so only depth rejects a
		// linked subtask.
		{taskID: "leaf", id: operationTaskNewLinkedSubtask, want: true},
		{taskID: "story-5", id: operationTaskNewLinkedSubtask, want: true},
		{taskID: "story-6", id: operationTaskNewLinkedSubtask, want: false},
	} {
		task := project.GetTaskByID(testCase.taskID)
		if task == nil {
			t.Fatalf("fixture is missing task %s", testCase.taskID)
		}
		if got := taskOperationApplicable(t, model, testCase.id, task, project); got != testCase.want {
			t.Errorf("%s on %s (%s at level %d) = %v, want %v",
				testCase.id, testCase.taskID, task.Type, task.Level, got, testCase.want)
		}
	}
}

func TestTaskChildActionOnDeletedParentLeavesHierarchyUnchanged(t *testing.T) {
	model := newUITestApp()
	project := newChildHierarchyTestProject()
	setTestAppProjects(model, project)
	epic := project.GetTaskByID("epic")
	context := taskOperationTestContext(epic, project)

	remaining := make([]*models.Task, 0, len(project.Tasks))
	for _, task := range project.Tasks {
		if task.ID != epic.ID {
			remaining = append(remaining, task)
		}
	}
	project.Tasks = remaining
	project.BuildTaskTree()
	before := len(project.Tasks)

	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationTaskNewChild,
		Context: context,
		Source:  OperationSourceActionSheet,
	})
	if result.Err == nil {
		t.Fatal("a child action ran against a deleted parent")
	}
	if !strings.Contains(result.Err.Error(), "no longer available") {
		t.Errorf("error = %v, want the authoritative unavailable message", result.Err)
	}
	if len(project.Tasks) != before {
		t.Errorf("task count = %d, want %d", len(project.Tasks), before)
	}
}

func TestTaskOperationStaleFrozenTargetReportsUnavailable(t *testing.T) {
	model, project, task := auxiliaryTaskTestApp(t)
	context := taskOperationTestContext(task, project)
	project.Tasks = []*models.Task{project.GetTaskByID("other")}
	project.BuildTaskTree()

	result := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationTaskQuickNote,
		Context: context,
		Source:  OperationSourceActionSheet,
	})
	if result.Err == nil {
		t.Fatal("a deleted frozen target executed anyway")
	}
	if !strings.Contains(result.Err.Error(), "no longer available") {
		t.Errorf("error = %v, want the authoritative unavailable message", result.Err)
	}
}
