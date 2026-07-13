package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// TemplateChildSpec describes one child task a template materializes under the
// root task. The adapter resolves the effective child type (templates default a
// child to a plain task) before building the request.
type TemplateChildSpec struct {
	// Title is the child task's title. Required.
	Title string
	// Type is the resolved child task type.
	Type models.TaskType
}

// ApplyTemplateRequest describes a `tt template use` invocation: the root task
// (the embedded AddTaskRequest, carrying the template's type, priority, tags,
// section, description, checklist, parent placement, and the config artifact
// with the bumped use-count) plus the child tasks an epic/story template
// creates alongside it.
type ApplyTemplateRequest struct {
	AddTaskRequest
	// TemplateName is the template's display name, recorded in the undo context.
	TemplateName string
	// Children are the child tasks to create under the root task. They inherit
	// the root's priority, tags, and section, matching the legacy template
	// semantics. Ignored when the root type cannot have children.
	Children []TemplateChildSpec
}

// ApplyTemplateResult is the presentation-neutral outcome of materializing a
// template.
type ApplyTemplateResult struct {
	// Task is the root task's standard mutation result.
	Task TaskResult
	// SubtasksCreated is the number of child tasks created under the root.
	SubtasksCreated int
}

// ApplyTemplate materializes a template into a root task plus its child tasks
// and commits them — together with any auxiliary durable artifacts the request
// carries (the config store with the incremented template use-count) — as ONE
// ChangeSet. The legacy path saved the project and then bumped the use-count in
// a separate config write; this use case makes the pair atomic. Children are
// silently skipped when the root type cannot have children, matching the legacy
// template semantics; every child title is validated before any task is
// constructed, so a rejected request never persists a partial template.
func (s *Service) ApplyTemplate(req ApplyTemplateRequest) (ApplyTemplateResult, error) {
	for _, child := range req.Children {
		if err := models.ValidateTaskTitle(child.Title); err != nil {
			return ApplyTemplateResult{}, invalidInput("child", fmt.Sprintf("invalid child task %q: %s", child.Title, err))
		}
	}

	ref, revision, err := s.buildAddTask(req.AddTaskRequest)
	if err != nil {
		return ApplyTemplateResult{}, err
	}
	task, project := ref.Task, ref.Project

	created := 0
	if task.Type.CanHaveChildren() {
		for _, child := range req.Children {
			childType := child.Type
			if childType == "" {
				childType = models.TaskTypeTask
			}
			childTask, childErr := models.NewTask(s.ids.NewID(), project.GetNextShortID(), childType, child.Title)
			if childErr != nil {
				return ApplyTemplateResult{}, invalidInput("child", fmt.Sprintf("invalid child task %q: %s", child.Title, childErr))
			}
			parentID := task.ID
			childTask.ParentID = &parentID
			childTask.Priority = task.Priority
			for _, tag := range task.Tags {
				childTask.AddTag(tag)
			}
			// SetSection (not a raw Section assignment) keeps the stored
			// WorkflowState coherent with the legacy fields.
			if req.Section != "" {
				childTask.SetSection(req.Section)
			}
			project.Tasks = append(project.Tasks, childTask)
			created++
		}
	}

	undoContext := fmt.Sprintf("%s -> #%d %s", req.TemplateName, task.ShortID, task.Title)
	res, err := s.commitProjectWithArtifacts(ref, revision, req.Artifacts, "Use Template", undoContext)
	if err != nil {
		return ApplyTemplateResult{}, err
	}
	return ApplyTemplateResult{Task: res, SubtasksCreated: created}, nil
}
