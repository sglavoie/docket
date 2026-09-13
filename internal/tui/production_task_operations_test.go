package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
)

func TestOperationDiscoveryExcludesRetiredProductConcepts(t *testing.T) {
	model := newUITestApp()
	retired := []string{"dashboard", "planner", "roadmap", "analytics", "actionable", "pinned-tag", "pinned tag"}
	for _, descriptor := range model.operations.Registry.All() {
		text := strings.ToLower(string(descriptor.ID) + " " + descriptor.Label + " " + strings.Join(descriptor.Aliases, " "))
		for _, term := range retired {
			if strings.Contains(text, term) {
				t.Errorf("operation %s exposes retired concept %q through discovery metadata", descriptor.ID, term)
			}
		}
	}
}

// productionTaskShape is one live task arrangement the manifest is exercised
// against. setup runs before every probe so paired operations get a context
// where they are legally offered.
type productionTaskShape struct {
	name  string
	setup func()
	task  func() *models.Task
}

// productionTaskOperationShapes builds the real registry over a hierarchy that
// covers every structural precondition in the manifest: a nested task that can
// outdent and reparent, a root task that can indent, a second project for
// move-project and the pinned variant.
func productionTaskOperationShapes(
	t *testing.T,
) (*AppModel, *models.Project, []productionTaskShape) {
	t.Helper()
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project, &models.Project{ID: "beta", Name: "Beta", Slug: "beta"})
	story := project.GetTaskByID("story")
	loose := project.GetTaskByID("loose")

	reset := func() {
		loose.Pinned = false
	}
	shape := func(name string, task *models.Task, mutate func()) productionTaskShape {
		return productionTaskShape{
			name: name,
			setup: func() {
				reset()
				if mutate != nil {
					mutate()
				}
			},
			task: func() *models.Task { return task },
		}
	}
	return model, project, []productionTaskShape{
		shape("root task", loose, nil),
		shape("nested task", story, nil),
		shape("pinned task", loose, func() { loose.Pinned = true }),
	}
}

// TestProductionTaskOperationManifestIsRegisteredApplicableAndExecutable is the
// catalog completeness gate: an entry that reaches the manifest but is never
// registered, never applicable, or never reaches a handler would advertise a
// task action the Space sheet cannot actually perform.
func TestProductionTaskOperationManifestIsRegisteredApplicableAndExecutable(t *testing.T) {
	model, project, shapes := productionTaskOperationShapes(t)

	for _, entry := range taskOperationManifest() {
		descriptor, registered := model.operations.Registry.Descriptor(entry.ID)
		if !registered {
			t.Errorf("manifest operation %s is not registered in the production registry", entry.ID)
			continue
		}
		applicable := false
		for _, shape := range shapes {
			shape.setup()
			context := taskOperationTestContext(shape.task(), project)
			if !descriptor.Applicable(context) {
				continue
			}
			applicable = true
			result := model.operations.Executor.Execute(operation.OperationExecutionRequest{
				ID:      entry.ID,
				Context: context,
			})
			for _, sentinel := range []error{
				operation.ErrOperationNotRegistered,
				operation.ErrOperationUnavailable,
				operation.ErrOperationHandler,
			} {
				if errors.Is(result.Err, sentinel) {
					t.Errorf("%s on a %s: %v", entry.ID, shape.name, result.Err)
				}
			}
			if result.Err != nil {
				t.Errorf("%s on a %s: %v", entry.ID, shape.name, result.Err)
			}
			// A handler either produced work or the operation's value provider
			// asked for the input it declares. Neither means nothing ran.
			if result.Message == nil && result.ValueRequired == nil {
				t.Errorf("%s on a %s produced no message and requested no value", entry.ID, shape.name)
			}
			break
		}
		if !applicable {
			t.Errorf("manifest operation %s is applicable in no production task shape", entry.ID)
		}
	}
}

// TestProductionTaskOperationBulkEntriesExecuteAcrossEverySelectedTask proves
// each Bulk manifest flag is backed by a coordinator that carries the whole
// frozen selection, so a bulk sheet can never advertise unsupported behavior.
func TestProductionTaskOperationBulkEntriesExecuteAcrossEverySelectedTask(t *testing.T) {
	model := newUITestApp()
	project := newTaskStructureTestProject()
	setTestAppProjects(model, project, &models.Project{ID: "beta", Name: "Beta", Slug: "beta"})
	context := bulkTaskActionTestContext(
		project,
		project.GetTaskByID("story"),
		project.GetTaskByID("loose"),
	)

	values := map[operation.OperationID]any{
		operation.OperationTaskSetState:    models.StateBacklog,
		operation.OperationTaskSetPriority: 2,
		operation.OperationTaskSetDueDate:  operation.OperationDateValue{},
		operation.OperationTaskSetCheckIn: operation.OperationCheckInValue{
			Clear: true,
		},
		operation.OperationTaskSetTags: []string{"release"},
	}
	for _, entry := range taskOperationManifest() {
		if !entry.Bulk {
			continue
		}
		request := operation.OperationExecutionRequest{
			ID:      entry.ID,
			Context: context,
		}
		if value, ok := values[entry.ID]; ok {
			request.Value, request.HasValue = value, true
		} else if entry.Provider != "" {
			t.Errorf("bulk manifest entry %s has no multi-target value in this test", entry.ID)
			continue
		}
		result := model.operations.Executor.Execute(request)
		if result.Err != nil {
			t.Errorf("bulk %s across %d tasks: %v", entry.ID, len(context.Selection.Tasks), result.Err)
			continue
		}
		if result.Message == nil {
			t.Errorf("bulk %s produced no coordinator work", entry.ID)
		}
	}
}

// TestTaskDeleteOperationRoutesThroughCanonicalConfirmation walks the whole
// production path: the registry emits the canonical request instead of mutating,
// the executor raises no second generic prompt, and only the shell-owned confirm
// commits the atomic subtree transaction.
func TestTaskDeleteOperationRoutesThroughCanonicalConfirmation(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m := e.appModel()
	m.operations.Registry = operation.NewOperationRegistry()
	m.operations.Executor = operation.NewOperationExecutor(m.operations.Registry)
	if err := m.registerTaskActionOperations(); err != nil {
		t.Fatalf("registerTaskActionOperations: %v", err)
	}

	context := route.PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	context.Capabilities.Add(operation.CapabilitySimplifiedInput)
	context.Entity = outliner.TaskOperationEntity(e.parent, e.alpha)

	result := m.operations.Executor.Execute(operation.OperationExecutionRequest{
		ID:      operation.OperationTaskDelete,
		Context: context,
	})
	if result.Err != nil {
		t.Fatalf("execute delete: %v", result.Err)
	}
	if result.ConfirmationRequired != nil {
		t.Fatal("delete raised a generic confirmation in front of the canonical one")
	}
	request, ok := result.Message.(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("delete produced %T, want TaskDeleteRequestedMsg", result.Message)
	}
	if len(request.Refs) != 1 || request.Refs[0].Task != e.parent {
		t.Fatalf("delete refs = %v, want only the frozen parent", request.Refs)
	}
	if reloaded := e.reload(t, e.alpha); reloaded.GetTaskByID(e.parent.ID) == nil {
		t.Fatal("the registry mutated task data before confirmation")
	}

	m.handleTaskDeleteRequested(request)
	if got := m.taskMutation.deleteConfirm.Prompt(); got != "Delete task #1?" {
		t.Fatalf("prompt = %q, want the canonical single-root grammar", got)
	}
	handled, cmd := m.handleGlobalKeyPress(keyPress("y"))
	if !handled || cmd == nil {
		t.Fatal("y should confirm and dispatch the canonical transaction")
	}
	m.handleBatchTransactionResult(runBatchResult(t, cmd))

	reloaded := e.reload(t, e.alpha)
	if reloaded.GetTaskByID(e.parent.ID) != nil || reloaded.GetTaskByID(e.child.ID) != nil {
		t.Error("the confirmed delete left the subtree on disk")
	}
	if reloaded.GetTaskByID(e.sibling.ID) == nil {
		t.Error("the confirmed delete removed an unselected sibling")
	}
}
