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
// state, description, checklist, parent placement, and the config artifact
// with the bumped use-count) plus the child tasks an epic/story template
// creates alongside it.
type ApplyTemplateRequest struct {
	AddTaskRequest
	// TemplateName is the template's display name, recorded in the undo context.
	TemplateName string
	// Children are the child tasks to create under the root task. They inherit
	// the root's priority, tags, and state, matching the legacy template
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

// ApplyTemplateWorkflowResult preserves the template-specific result alongside
// the common workflow cascade result. Template carries the root task and child
// count existing callers expect; Cascade carries touched projects, conflicts,
// focus/catalog effects, and trigger counts.
type ApplyTemplateWorkflowResult struct {
	Template ApplyTemplateResult
	Cascade  BulkResult
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
	normalized, err := validateApplyTemplateRequest(req)
	if err != nil {
		return ApplyTemplateResult{}, err
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return ApplyTemplateResult{}, err
	}
	ref, revision, created, err := s.buildApplyTemplate(snap, req, normalized)
	if err != nil {
		return ApplyTemplateResult{}, err
	}

	undoContext := fmt.Sprintf("%s -> #%d %s", req.TemplateName, ref.Task.ShortID, ref.Task.Title)
	res, err := s.commitProjectWithArtifacts(ref, revision, req.Artifacts, "Use Template", undoContext)
	if err != nil {
		return ApplyTemplateResult{}, err
	}
	return ApplyTemplateResult{Task: res, SubtasksCreated: created}, nil
}

// ApplyTemplateWithWorkflow materializes a template and runs a stateless
// workflow cascade over the resulting project. The template use-count artifact,
// root and child tasks, workflow follow-ups, focus/catalog changes, and any
// deferred cross-project moves all ride one grouped transaction.
func (s *Service) ApplyTemplateWithWorkflow(req ApplyTemplateRequest, opts SingleTaskWorkflowOptions) (ApplyTemplateWorkflowResult, error) {
	normalized, err := validateApplyTemplateRequest(req)
	if err != nil {
		return ApplyTemplateWorkflowResult{}, err
	}
	undoAction := opts.UndoAction
	if undoAction == "" {
		undoAction = "Use Template"
	}

	var ref TaskRef
	created := 0
	orchestrated, err := s.runWorkflowOrchestration(workflowOrchestrationRequest{
		rules:                  opts.Rules,
		projectStatusArtifacts: opts.ProjectStatusArtifacts,
		undoAction:             undoAction,
		undoContext:            opts.UndoContext,
		forceCommit:            true,
		prepare: func(snap Snapshot) (workflowMutationPlan, error) {
			built, revision, childCount, buildErr := s.buildApplyTemplate(snap, req, normalized)
			if buildErr != nil {
				return workflowMutationPlan{}, buildErr
			}
			ref = built
			created = childCount
			undoContext := opts.UndoContext
			if undoContext == "" {
				undoContext = fmt.Sprintf("%s -> #%d %s", req.TemplateName, ref.Task.ShortID, ref.Task.Title)
			}
			return workflowMutationPlan{
				commits: []projectCommit{{
					project:          ref.Project,
					expectedRevision: revision,
					base:             ref.Base,
					artifacts:        req.Artifacts,
				}},
				sources: []*models.Project{ref.Project},
				mutate: func(*WorkspaceResolver) error {
					// Materialized children joined the project after the snapshot was
					// indexed; rebuild the tree before hierarchical rule evaluation.
					ref.Project.BuildTaskTree()
					return nil
				},
				undoContext: undoContext,
			}, nil
		},
	})
	if err != nil {
		return ApplyTemplateWorkflowResult{}, err
	}

	cascade := bulkResultFromWorkflow(orchestrated)
	root := taskResultFromWorkflow(orchestrated, ref, ref.Project.ID)
	cascade.Tasks = []TaskResult{root}
	return ApplyTemplateWorkflowResult{
		Template: ApplyTemplateResult{Task: root, SubtasksCreated: created},
		Cascade:  cascade,
	}, nil
}

func validateApplyTemplateRequest(req ApplyTemplateRequest) (normalizedAddTaskRequest, error) {
	for _, child := range req.Children {
		if err := models.ValidateTaskTitle(child.Title); err != nil {
			return normalizedAddTaskRequest{}, invalidInput("child", fmt.Sprintf("invalid child task %q: %s", child.Title, err))
		}
	}
	return normalizeAddTaskRequest(req.AddTaskRequest)
}

// buildApplyTemplate applies the template mutation inside a caller-owned
// snapshot without committing, so bare and workflow-aware paths share child
// validation, ID allocation, hierarchy, and inheritance semantics exactly.
func (s *Service) buildApplyTemplate(snap Snapshot, req ApplyTemplateRequest, normalized normalizedAddTaskRequest) (TaskRef, string, int, error) {
	ref, revision, err := s.buildNormalizedAddTask(snap, req.AddTaskRequest, normalized)
	if err != nil {
		return TaskRef{}, "", 0, err
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
				return TaskRef{}, "", 0, invalidInput("child", fmt.Sprintf("invalid child task %q: %s", child.Title, childErr))
			}
			parentID := task.ID
			childTask.ParentID = &parentID
			childTask.Priority = task.Priority
			for _, tag := range task.Tags {
				childTask.AddTag(tag)
			}
			if stateErr := s.initializeCreatedTask(childTask, task.State()); stateErr != nil {
				return TaskRef{}, "", 0, stateErr
			}
			project.Tasks = append(project.Tasks, childTask)
			created++
		}
	}
	return ref, revision, created, nil
}
