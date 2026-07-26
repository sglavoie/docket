package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// AddTaskRequest is the full typed representation of a `tt task add` invocation:
// every supported input — metadata (type, priority, tags, description, state),
// hierarchy placement, recurrence, task-type attributes, and a
// template-materialized checklist — plus any auxiliary durable artifacts that must
// commit atomically with the new task (the config store carrying an incremented
// template use-count). The adapter resolves presentation concerns (flag
// precedence, template lookup, reference parsing) and hands this request
// fully-resolved primitives; the use case owns the snapshot, validation,
// ID/short-ID minting, and the single ChangeSet commit. It is the richer sibling
// of CaptureRequest, which covers only the minimal quick-capture fields.
type AddTaskRequest struct {
	// ProjectID is the owning project's stable ID. Required.
	ProjectID string
	// Title is the task title. Required; validated with models.ValidateTaskTitle.
	Title string
	// Type is the task type; empty defaults to a leaf "task".
	Type models.TaskType
	// Priority is 1..5; zero (or unset) defaults to the medium priority.
	Priority int
	// Tags are normalized and validated; a malformed tag aborts the add.
	Tags []string
	// Description is optional free-form body text.
	Description string
	// State is the task's initial canonical workflow state. Empty defaults to
	// Inbox; compatibility adapters must explicitly request StateReady while
	// their visible legacy creation surfaces still default to the active bucket.
	State models.WorkflowState
	// Recurrence, when non-nil, is the parsed recurrence pattern applied via
	// task.SetRecurrence (which populates derived fields such as OriginalDay).
	Recurrence *models.RecurrencePattern
	// ParentID, when non-nil, nests the new task under an existing task in the
	// same project; the parent must exist and accept a child of Type.
	ParentID *string
	// Checklist holds the materialized checklist item texts (typically from a
	// template). Each becomes an unchecked models.ChecklistItem.
	Checklist []string
	// ChecklistItems holds full checklist items (text + done) set by the task
	// form, which can seed items already marked done. Blank-text items are
	// skipped. Applied after Checklist.
	ChecklistItems []models.ChecklistItem
	// Attrs carries type-specific attributes applied through
	// models.ApplyTypeAttrValues; the zero value applies nothing.
	Attrs models.TypeAttrValues
	// Artifacts are auxiliary durable artifact writes committed in the SAME
	// ChangeSet as the new task (e.g. the config store with an incremented
	// template use-count), so the add and its side effects are atomic.
	Artifacts []storage.ArtifactChange
	// PlanningArtifacts carries route-local composer presets (notably Planner
	// Today/Next) through the full expanded form path.
	PlanningArtifacts PlanningArtifactBuilder

	// The following fields are populated only by the task form (via
	// AddTaskWithChildren); the CLI `tt task add` path leaves them zero/nil so its
	// behavior is unchanged.

	// DueDate/HasTime, StartDate, EstimatedDays, URL, and Source carry the form's
	// scheduling and reference metadata.
	DueDate       *time.Time
	HasTime       bool
	StartDate     *time.Time
	EstimatedDays float64
	URL           string
	Source        string
	// Pinned and Dimmed carry the form's flag state. Pinned also records PinnedAt.
	// Focus Queue membership is independent planning state handled at the
	// config-aware layer, not a task field set here.
	Pinned bool
	Dimmed bool
	// InsertBeforeDimmed positions a new ROOT task before the first dimmed root
	// task sharing its display section (the task form's create positioning),
	// rather than appending at the end.
	InsertBeforeDimmed bool
	// Type-specific attribute pointers set by the form (only the active type's
	// pointer is non-nil). The CLI uses Attrs above instead; a set pointer here is
	// assigned after ApplyTypeAttrValues.
	BugAttrs   *models.BugAttrs
	IdeaAttrs  *models.IdeaAttrs
	TaskAttrs  *models.TaskAttrs
	StoryAttrs *models.StoryAttrs
	EpicAttrs  *models.EpicAttrs
	// Subtasks are child tasks created under the new task in the SAME ChangeSet
	// (task-form Epic/Story create). Each inherits the parent's priority, state,
	// and tags; a blank-title subtask is skipped.
	Subtasks []AddSubtaskRequest
}

// AddSubtaskRequest is one child task the task form creates under a new
// Epic/Story parent.
type AddSubtaskRequest struct {
	Title string
	Type  models.TaskType
}

// FormCreateResult is the outcome of a task-form create: the parent task plus a
// result per created subtask, in creation order, so the form can navigate to a
// specific subtask (open-after) after the reload.
type FormCreateResult struct {
	Task     TaskResult
	Subtasks []TaskResult
}

// AddTask creates a fully-specified task in the target project and commits it —
// together with any auxiliary durable artifacts the request carries — as ONE
// ChangeSet. It validates the title, type, priority, tags, state, parent
// placement, recurrence, and type attributes before mutating, so a rejected
// request never persists a partial task or a dangling config write. It is the
// task-add parity path routed through the application layer; the pre-application
// CLI committed the task and the template use-count in two separate writes, which
// this use case collapses into one atomic transaction.
func (s *Service) AddTask(req AddTaskRequest) (TaskResult, error) {
	ref, revision, err := s.buildAddTask(req)
	if err != nil {
		return TaskResult{}, err
	}
	planning, err := s.transitionPlanningArtifacts(ref, s.defaultedPlanningArtifacts(req.PlanningArtifacts))
	if err != nil {
		return TaskResult{}, err
	}
	artifacts := append(append([]storage.ArtifactChange(nil), req.Artifacts...), planning...)
	return s.commitProjectWithArtifacts(ref, revision, artifacts, "Add Task", fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title))
}

// AddTaskWithWorkflow creates a fully-specified task and runs a stateless
// workflow cascade over the resulting project, committing the new task, any
// caller-supplied artifacts, follow-ups, catalog effects, and deferred
// cross-project moves as one grouped transaction. Creation deliberately has no
// previous-state map: event-only rules must not treat a newly created task as a
// transition from a fabricated prior state.
func (s *Service) AddTaskWithWorkflow(req AddTaskRequest, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	normalized, err := normalizeAddTaskRequest(req)
	if err != nil {
		return BulkResult{}, err
	}

	undoAction := opts.UndoAction
	if undoAction == "" {
		undoAction = "Add Task"
	}
	var ref TaskRef
	orchestrated, err := s.runWorkflowOrchestration(workflowOrchestrationRequest{
		rules:                  opts.Rules,
		projectStatusArtifacts: opts.ProjectStatusArtifacts,
		undoAction:             undoAction,
		undoContext:            opts.UndoContext,
		forceCommit:            true,
		prepare: func(snap Snapshot) (workflowMutationPlan, error) {
			built, revision, buildErr := s.buildNormalizedAddTask(snap, req, normalized)
			if buildErr != nil {
				return workflowMutationPlan{}, buildErr
			}
			ref = built
			undoContext := opts.UndoContext
			if undoContext == "" {
				undoContext = fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title)
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
					// Snapshot indexes predate this new task; rebuild the model tree so
					// hierarchical rules see its real parent and level.
					ref.Project.BuildTaskTree()
					return nil
				},
				undoContext: undoContext,
			}, nil
		},
	})
	if err != nil {
		return BulkResult{}, err
	}

	out := bulkResultFromWorkflow(orchestrated)
	out.Tasks = []TaskResult{taskResultFromWorkflow(orchestrated, ref, ref.Project.ID)}
	return out, nil
}

type normalizedAddTaskRequest struct {
	title    string
	taskType models.TaskType
	priority int
	tags     []string
	state    models.WorkflowState
}

func normalizeAddTaskRequest(req AddTaskRequest) (normalizedAddTaskRequest, error) {
	title := strings.TrimSpace(req.Title)
	if err := models.ValidateTaskTitle(title); err != nil {
		return normalizedAddTaskRequest{}, invalidInput("title", err.Error())
	}

	taskType := req.Type
	if taskType == "" {
		taskType = models.TaskTypeTask
	} else if !taskType.IsValid() {
		return normalizedAddTaskRequest{}, invalidInput("type", fmt.Sprintf("%q is not a valid task type", taskType))
	}

	priority := req.Priority
	if priority == 0 {
		priority = models.PriorityDefault
	}
	if err := models.ValidatePriority(priority, false); err != nil {
		return normalizedAddTaskRequest{}, invalidInput("priority", err.Error())
	}

	state, err := normalizeCreationState(req.State)
	if err != nil {
		return normalizedAddTaskRequest{}, err
	}

	tags, err := normalizeTags(req.Tags)
	if err != nil {
		return normalizedAddTaskRequest{}, err
	}
	return normalizedAddTaskRequest{title: title, taskType: taskType, priority: priority, tags: tags, state: state}, nil
}

// buildAddTask validates an AddTaskRequest against a fresh snapshot and appends
// the fully-constructed task to its project WITHOUT committing, returning the
// task reference and the revision the project was loaded at. AddTask commits it
// directly; ApplyTemplate appends the template's child tasks first so the whole
// template materializes in one transaction.
func (s *Service) buildAddTask(req AddTaskRequest) (TaskRef, string, error) {
	normalized, err := normalizeAddTaskRequest(req)
	if err != nil {
		return TaskRef{}, "", err
	}

	snap, err := s.repo.Snapshot()
	if err != nil {
		return TaskRef{}, "", err
	}
	return s.buildNormalizedAddTask(snap, req, normalized)
}

// buildNormalizedAddTask constructs a validated task inside a caller-owned
// snapshot without committing it. Bare creation obtains its own snapshot before
// calling this helper; workflow-aware creation supplies the orchestration
// runner's snapshot so validation, mutation, rule evaluation, and commit all
// share one revision-consistent workspace view.
func (s *Service) buildNormalizedAddTask(snap Snapshot, req AddTaskRequest, normalized normalizedAddTaskRequest) (TaskRef, string, error) {
	project := snap.ProjectByID(req.ProjectID)
	if project == nil {
		return TaskRef{}, "", fmt.Errorf("%w: %s", ErrProjectNotFound, req.ProjectID)
	}
	revision := snap.Revision(project.ID)

	// Resolve and validate parent placement against the freshly snapshotted
	// project so hierarchy and depth rules see current state, not the adapter's
	// possibly-stale load.
	var parent *models.Task
	if req.ParentID != nil {
		parent = project.GetTaskByID(*req.ParentID)
		if parent == nil {
			return TaskRef{}, "", fmt.Errorf("%w: parent %s", ErrTaskNotFound, *req.ParentID)
		}
		if !normalized.taskType.CanBeChildOf(parent.Type) {
			return TaskRef{}, "", invalidInput("parent", fmt.Sprintf("%s cannot be a child of %s", normalized.taskType.HeadingPrefix(), parent.Type.HeadingPrefix()))
		}
		if parent.Level+1 > models.MaxNestingDepth {
			return TaskRef{}, "", invalidInput("parent", fmt.Sprintf("nesting under #%d would exceed maximum depth of %d levels", parent.ShortID, models.MaxNestingDepth))
		}
	}

	task, err := models.NewTask(s.ids.NewID(), project.GetNextShortID(), normalized.taskType, normalized.title)
	if err != nil {
		return TaskRef{}, "", invalidInput("title", err.Error())
	}
	task.Priority = normalized.priority
	task.Description = req.Description
	for _, tag := range normalized.tags {
		task.AddTag(tag)
	}
	if req.Recurrence != nil {
		if err := task.SetRecurrence(req.Recurrence); err != nil {
			return TaskRef{}, "", invalidInput("recurrence", err.Error())
		}
	}
	if err := s.initializeCreatedTask(project, task, normalized.state); err != nil {
		return TaskRef{}, "", err
	}
	if parent != nil {
		parentID := parent.ID
		task.ParentID = &parentID
	}
	if err := models.ApplyTypeAttrValues(task, req.Attrs); err != nil {
		return TaskRef{}, "", invalidInput("attributes", err.Error())
	}
	// Form path: assign the concrete attr struct for the active type directly
	// (only one pointer is non-nil), mirroring the form's legacy field assignment.
	if req.BugAttrs != nil {
		task.BugAttrs = req.BugAttrs
	}
	if req.IdeaAttrs != nil {
		task.IdeaAttrs = req.IdeaAttrs
	}
	if req.TaskAttrs != nil {
		task.TaskAttrs = req.TaskAttrs
	}
	if req.StoryAttrs != nil {
		task.StoryAttrs = req.StoryAttrs
	}
	if req.EpicAttrs != nil {
		task.EpicAttrs = req.EpicAttrs
	}
	for _, item := range req.Checklist {
		task.Checklist = append(task.Checklist, models.ChecklistItem{Text: item, Done: false})
	}
	for _, item := range req.ChecklistItems {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		task.Checklist = append(task.Checklist, models.ChecklistItem{Text: text, Done: item.Done})
	}

	// Form-originated scheduling/reference/flag metadata. All zero/nil for the CLI.
	if req.DueDate != nil {
		task.DueDate = cloneTimePtr(req.DueDate)
		task.HasTime = req.HasTime
	}
	if req.StartDate != nil {
		task.StartDate = cloneTimePtr(req.StartDate)
	}
	task.EstimatedDays = req.EstimatedDays
	task.URL = req.URL
	task.Source = req.Source
	task.Dimmed = req.Dimmed
	if req.Pinned {
		now := s.clock.Now()
		task.Pinned = true
		task.PinnedAt = &now
	}

	if req.InsertBeforeDimmed {
		project.Tasks = insertBeforeDimmedInSection(project.Tasks, task)
	} else {
		project.Tasks = append(project.Tasks, task)
	}

	return TaskRef{Task: task, Project: project, Base: snap.Document(project.ID)}, revision, nil
}

// AddTaskWithChildren creates a fully-specified parent task and its subtasks and
// commits them — with any auxiliary artifacts (e.g. a template use-count bump) —
// as ONE ChangeSet, preserving the task form's atomic Epic/Story creation and
// single undo entry. Each subtask inherits the parent's priority, state, and
// tags; a blank-title subtask is skipped. The parent is positioned per
// InsertBeforeDimmed. It returns the parent result plus a result per created
// subtask in creation order.
func (s *Service) AddTaskWithChildren(req AddTaskRequest) (FormCreateResult, error) {
	ref, revision, err := s.buildAddTask(req)
	if err != nil {
		return FormCreateResult{}, err
	}

	childRefs := make([]TaskRef, 0, len(req.Subtasks))
	for _, sub := range req.Subtasks {
		title := strings.TrimSpace(sub.Title)
		if title == "" {
			continue
		}
		subType := sub.Type
		if subType == "" {
			subType = models.TaskTypeTask
		} else if !subType.IsValid() {
			return FormCreateResult{}, invalidInput("type", fmt.Sprintf("%q is not a valid task type", subType))
		}
		child, buildErr := models.NewTask(s.ids.NewID(), ref.Project.GetNextShortID(), subType, title)
		if buildErr != nil {
			return FormCreateResult{}, invalidInput("title", buildErr.Error())
		}
		child.Priority = ref.Task.Priority
		if stateErr := s.initializeCreatedTask(ref.Project, child, ref.Task.State()); stateErr != nil {
			return FormCreateResult{}, stateErr
		}
		for _, tag := range ref.Task.Tags {
			child.AddTag(tag)
		}
		parentID := ref.Task.ID
		child.ParentID = &parentID
		ref.Project.Tasks = append(ref.Project.Tasks, child)
		childRefs = append(childRefs, TaskRef{Task: child, Project: ref.Project})
	}

	planning, err := s.transitionPlanningArtifacts(ref, s.defaultedPlanningArtifacts(req.PlanningArtifacts))
	if err != nil {
		return FormCreateResult{}, err
	}
	artifacts := append(append([]storage.ArtifactChange(nil), req.Artifacts...), planning...)
	result, err := s.commitProjectWithArtifacts(ref, revision, artifacts, "Add Task", fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title))
	if err != nil {
		return FormCreateResult{}, err
	}

	out := FormCreateResult{Task: result}
	for _, cref := range childRefs {
		out.Subtasks = append(out.Subtasks, newTaskResult(cref, result.Revision))
	}
	return out, nil
}

// insertBeforeDimmedInSection inserts newTask before the first dimmed ROOT task
// that shares newTask's display section (derived from workflow state), falling
// back to append when none is found. Child tasks (non-nil ParentID) are always
// appended — their position is managed by the tree, not section order. This
// preserves the task form's dimmed-aware create positioning inside the
// application layer.
func insertBeforeDimmedInSection(tasks []*models.Task, newTask *models.Task) []*models.Task {
	if newTask.ParentID != nil {
		return append(tasks, newTask)
	}
	newSection := models.SectionForState(newTask.State())
	for i, t := range tasks {
		if t.ParentID != nil {
			continue
		}
		if t.Dimmed && models.SectionForState(t.State()) == newSection {
			result := make([]*models.Task, 0, len(tasks)+1)
			result = append(result, tasks[:i]...)
			result = append(result, newTask)
			result = append(result, tasks[i:]...)
			return result
		}
	}
	return append(tasks, newTask)
}
