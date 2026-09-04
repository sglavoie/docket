package tui

import (
	"errors"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// newTaskStructureTestProject lays out one epic with a story child followed by a
// loose root task, which is the smallest shape covering every legal and illegal
// indent/outdent/reparent case.
func newTaskStructureTestProject() *models.Project {
	epic := &models.Task{
		ID: "epic", ShortID: 1, Title: "Epic",
		WorkflowState: models.StateBacklog,
	}
	story := &models.Task{
		ID: "story", ShortID: 2, Title: "Story", ParentID: strPtr("epic"),
		WorkflowState: models.StateBacklog,
	}
	loose := &models.Task{
		ID: "loose", ShortID: 3, Title: "Loose",
		WorkflowState: models.StateBacklog,
	}
	project := &models.Project{
		ID: "alpha", Name: "Alpha", Slug: "alpha",
		Tasks: []*models.Task{epic, story, loose},
	}
	project.BuildTaskTree()
	return project
}

func taskOperationTestContext(task *models.Task, project *models.Project) operation.CommandContext {
	context := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	context.Capabilities.Add(operation.CapabilitySimplifiedInput)
	context.Entity = taskOperationEntity(task, project)
	return context
}

func taskOperationApplicable(
	t *testing.T,
	model *AppModel,
	id operation.OperationID,
	task *models.Task,
	project *models.Project,
) bool {
	t.Helper()
	descriptor, ok := model.operationRegistry.Descriptor(id)
	if !ok {
		t.Fatalf("operation %s is not registered", id)
	}
	return descriptor.Applicable(taskOperationTestContext(task, project))
}

func TestDirectIndentAndOutdentRemainResolvableAcrossHierarchyPositions(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project)

	for _, testCase := range []struct {
		taskID string
		id     operation.OperationID
		want   bool
	}{
		{taskID: "loose", id: operation.OperationTaskIndent, want: true},
		{taskID: "epic", id: operation.OperationTaskIndent, want: true},
		{taskID: "story", id: operation.OperationTaskOutdent, want: true},
		{taskID: "loose", id: operation.OperationTaskOutdent, want: true},
	} {
		task := project.GetTaskByID(testCase.taskID)
		if task == nil {
			t.Fatalf("fixture is missing task %s", testCase.taskID)
		}
		if got := taskOperationApplicable(t, model, testCase.id, task, project); got != testCase.want {
			t.Errorf("%s applicable for %s = %v, want %v", testCase.id, testCase.taskID, got, testCase.want)
		}
	}
}

func TestTaskOperationManifestOrdersTheSingleTaskActionSheet(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project, &models.Project{ID: "beta", Name: "Beta", Slug: "beta"})
	loose := project.GetTaskByID("loose")

	descriptors := TaskActionSheetDescriptors(
		model.operationRegistry,
		taskOperationTestContext(loose, project),
	)
	ordered := make([]operation.OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if _, manifested := taskOperationManifestEntryFor(descriptor.ID); manifested {
			ordered = append(ordered, descriptor.ID)
		}
	}
	// Every direct and retired operation is absent. The remaining projection is
	// the single-row task menu, which Delete alone now occupies.
	want := []operation.OperationID{
		operation.OperationTaskDelete,
	}
	if !reflect.DeepEqual(ordered, want) {
		t.Fatalf("manifest order = %v, want %v", ordered, want)
	}
	for _, id := range []operation.OperationID{operation.OperationTaskIndent, operation.OperationTaskOutdent} {
		descriptor, ok := model.operationRegistry.Descriptor(id)
		if !ok {
			t.Fatalf("operation %s is not registered", id)
		}
		if got := taskActionCategory(descriptor); got != taskActionCategoryRelationships {
			t.Errorf("%s category = %s, want %s", id, got, taskActionCategoryRelationships)
		}
	}
}

func waitingCheckBackInvocation(id operation.OperationID, value any) operation.OperationInvocation {
	return operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: id},
		Context:    taskActionTestContext(),
		Value:      value,
		HasValue:   true,
	}
}

func TestWaitingCheckBackStepChainsOnlyTheWaitingMove(t *testing.T) {
	committed := func() tea.Msg { return coordinator.BatchTransactionResultMsg{Action: "Set State"} }

	chained := withWaitingCheckBackStep(
		waitingCheckBackInvocation(operation.OperationTaskSetState, models.StateWaiting),
		committed,
	)
	cmds, ok := commandGroupMessage(chained())
	if !ok {
		t.Fatalf("moving to Waiting did not produce a command sequence")
	}
	if len(cmds) != 2 {
		t.Fatalf("sequence length = %d, want 2 (state move, then check-back)", len(cmds))
	}
	if _, isResult := cmds[0]().(coordinator.BatchTransactionResultMsg); !isResult {
		t.Fatalf("the state move is not the first step of the sequence")
	}
	request, isRequest := cmds[1]().(operation.OperationValueRequestMsg)
	if !isRequest {
		t.Fatalf("the follow-on step is not a value request: %T", cmds[1]())
	}
	if request.Request.ID != operation.OperationTaskSetDueDate {
		t.Errorf("follow-on operation = %s, want %s", request.Request.ID, operation.OperationTaskSetDueDate)
	}
	if request.Provider != operation.ValueProviderDueDate {
		t.Errorf("follow-on provider = %v, want %v", request.Provider, operation.ValueProviderDueDate)
	}

	unchained := []operation.OperationInvocation{
		waitingCheckBackInvocation(operation.OperationTaskSetState, models.StatePaused),
		waitingCheckBackInvocation(operation.OperationTaskSetState, models.StateDone),
		waitingCheckBackInvocation(operation.OperationTaskSetPriority, models.StateWaiting),
	}
	for _, invocation := range unchained {
		msg := withWaitingCheckBackStep(invocation, committed)()
		if _, isResult := msg.(coordinator.BatchTransactionResultMsg); !isResult {
			t.Errorf("%s with value %v was wrapped in a check-back chain", invocation.Descriptor.ID, invocation.Value)
		}
	}
}

// The picker is scheduled only after the transition commits, and it targets the
// identities frozen at invocation time rather than whatever the cursor names
// once the committed move has relocated it.
func TestWaitingCheckBackStepSuccessKeepsFrozenTargets(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	first := &models.Task{ID: "first"}
	second := &models.Task{ID: "second"}
	invocation := operation.OperationInvocation{
		Descriptor: operation.OperationDescriptor{ID: operation.OperationTaskSetState},
		Context:    bulkTaskActionTestContext(project, first, second),
		Value:      models.StateWaiting,
		HasValue:   true,
	}
	frozen := append([]taskref.TaskIdentity(nil), invocation.Context.Selection.Tasks...)

	chained := withWaitingCheckBackStep(invocation, func() tea.Msg {
		return coordinator.BatchTransactionResultMsg{Action: "Set State"}
	})
	// The commit relocates the cursor, which is what the live selection would
	// follow; the already-built chain must not.
	invocation.Context.Selection.Tasks[0] = taskref.TaskIdentity{ProjectID: "project", TaskID: "relocated"}

	cmds, ok := commandGroupMessage(chained())
	if !ok {
		t.Fatalf("a committed Waiting move did not schedule the check-back")
	}
	if len(cmds) != 2 {
		t.Fatalf("sequence length = %d, want 2 (state move, then check-back)", len(cmds))
	}
	request, isRequest := cmds[1]().(operation.OperationValueRequestMsg)
	if !isRequest {
		t.Fatalf("the follow-on step is not a value request: %T", cmds[1]())
	}
	if !reflect.DeepEqual(request.Request.Context.Selection.Tasks, frozen) {
		t.Errorf("check-back targets = %v, want the frozen %v", request.Request.Context.Selection.Tasks, frozen)
	}
}

// A rejected transition leaves the task outside Waiting, so no picker opens and
// no due date can be committed against the refusal.
func TestWaitingCheckBackStepRejectedMoveOpensNoPicker(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	first := &models.Task{ID: "first"}
	second := &models.Task{ID: "second"}
	rejected := func() tea.Msg { return coordinator.TransactionErrorMsg{Err: errors.New("cannot move to waiting")} }

	contexts := map[string]operation.CommandContext{
		"single":   taskActionTestContext(),
		"multiple": bulkTaskActionTestContext(project, first, second),
	}
	for name, context := range contexts {
		t.Run(name, func(t *testing.T) {
			chained := withWaitingCheckBackStep(operation.OperationInvocation{
				Descriptor: operation.OperationDescriptor{ID: operation.OperationTaskSetState},
				Context:    context,
				Value:      models.StateWaiting,
				HasValue:   true,
			}, rejected)

			msg := chained()
			if cmds, isGroup := commandGroupMessage(msg); isGroup {
				t.Fatalf("a rejected Waiting move scheduled %d follow-on command(s)", len(cmds))
			}
			failure, isFailure := msg.(coordinator.TransactionErrorMsg)
			if !isFailure {
				t.Fatalf("rejected move resolved to %T, want TransactionErrorMsg", msg)
			}
			if failure.Err == nil || failure.Err.Error() != "cannot move to waiting" {
				t.Errorf("rejection error = %v, want the underlying refusal", failure.Err)
			}
		})
	}
}
