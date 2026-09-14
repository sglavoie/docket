package tui

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
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

func taskOperationApplicable(
	t *testing.T,
	model *AppModel,
	id operation.OperationID,
	task *models.Task,
	project *models.Project,
) bool {
	t.Helper()
	descriptor, ok := model.operations.Registry.Descriptor(id)
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
		model.operations.Registry,
		taskOperationTestContext(loose, project),
	)
	ordered := make([]operation.OperationID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if _, manifested := taskOperationManifestEntryFor(descriptor.ID); manifested {
			ordered = append(ordered, descriptor.ID)
		}
	}
	// Every direct and retired operation is absent. Check-in is the reusable
	// scheduling form and Delete remains the destructive action.
	want := []operation.OperationID{
		operation.OperationTaskSetCheckIn,
		operation.OperationTaskDelete,
	}
	if !reflect.DeepEqual(ordered, want) {
		t.Fatalf("manifest order = %v, want %v", ordered, want)
	}
	for _, id := range []operation.OperationID{operation.OperationTaskIndent, operation.OperationTaskOutdent} {
		descriptor, ok := model.operations.Registry.Descriptor(id)
		if !ok {
			t.Fatalf("operation %s is not registered", id)
		}
		if got := taskActionCategory(descriptor); got != taskActionCategoryRelationships {
			t.Errorf("%s category = %s, want %s", id, got, taskActionCategoryRelationships)
		}
	}
}
