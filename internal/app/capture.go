package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// CaptureRequest describes a new task to create in a project. Only ProjectID and
// Title are required; every other field is optional and defaults to the domain
// default (task type, medium priority, Inbox state, no due date).
type CaptureRequest struct {
	// ProjectID is the owning project's stable ID. Required.
	ProjectID string
	// Title is the task title. Required; validated with models.ValidateTaskTitle.
	Title string
	// Type is the task type; empty defaults to a leaf "task".
	Type models.TaskType
	// Priority is 1..5; zero (or unset) defaults to the medium priority.
	Priority int
	// Tags are normalized and validated; a malformed tag aborts the capture.
	Tags []string
	// RemoveTags are normalized tags to strip from the new task after Tags are
	// applied (the focus-board quick-add supports "-tag" removal directives).
	RemoveTags []string
	// Description is optional free-form body text.
	Description string
	// Due, when non-nil, sets the task's due date. HasTime marks a meaningful
	// clock component.
	Due     *time.Time
	HasTime bool
	// ParentID, when non-nil, nests the new task under an existing task in the
	// same project (validated to exist).
	ParentID *string
	// State is the task's initial canonical workflow state. Empty defaults to
	// Inbox.
	State models.WorkflowState
	// Recurrence, when non-nil, is the parsed recurrence pattern applied via
	// task.SetRecurrence (which populates derived fields such as OriginalDay).
	Recurrence *models.RecurrencePattern
	// URL and Source are optional metadata captured by the focus-board quick-add.
	URL    string
	Source string
	// Prepend inserts the new task at the front of the project's task slice so it
	// is prominently visible (Quick/Bulk Capture), rather than appended at the end.
	Prepend bool
	// PlanningArtifacts, when set, builds a checked planning-domain artifact for
	// the freshly created task and commits it in the same ChangeSet. Planner uses
	// this to make Today/Next creation and horizon membership one undoable write.
	PlanningArtifacts PlanningArtifactBuilder
}

// Capture creates a new task in the target project and commits it as one
// ChangeSet. It validates the title, type, priority, tags, state, and parent
// before mutating, so a rejected request never persists a partial task.
func (s *Service) Capture(req CaptureRequest) (TaskResult, error) {
	snap, err := s.repo.Snapshot()
	if err != nil {
		return TaskResult{}, err
	}
	project := snap.ProjectByID(req.ProjectID)
	if project == nil {
		return TaskResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, req.ProjectID)
	}
	revision := snap.Revision(project.ID)

	task, err := s.buildCapturedTask(project, req)
	if err != nil {
		return TaskResult{}, err
	}
	insertCapturedTask(project, task, req.Prepend)

	ref := TaskRef{Task: task, Project: project, Base: snap.Document(project.ID)}
	artifacts, err := s.transitionPlanningArtifacts(ref, s.defaultedPlanningArtifacts(req.PlanningArtifacts))
	if err != nil {
		return TaskResult{}, err
	}
	return s.commitProjectWithArtifacts(ref, revision, artifacts, "Add Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
}

// CaptureMany creates every requested task in a single project and commits them
// as ONE ChangeSet, preserving Bulk Capture's atomic multi-task creation and its
// single undo entry. Every request must target the same project; a mismatch, an
// empty batch, or any rejected request aborts before mutating, so a partial batch
// never persists. Prepended tasks land at the front in request order (newest
// first), matching the legacy bulk-capture ordering.
func (s *Service) CaptureMany(reqs []CaptureRequest) ([]TaskResult, error) {
	if len(reqs) == 0 {
		return nil, invalidInput("capture", "no tasks to create")
	}
	projectID := reqs[0].ProjectID
	for _, req := range reqs {
		if req.ProjectID != projectID {
			return nil, invalidInput("capture", "all bulk-capture tasks must target the same project")
		}
	}

	snap, err := s.repo.Snapshot()
	if err != nil {
		return nil, err
	}
	project := snap.ProjectByID(projectID)
	if project == nil {
		return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}
	revision := snap.Revision(project.ID)

	tasks := make([]*models.Task, 0, len(reqs))
	var planningBuilder PlanningArtifactBuilder
	for _, req := range reqs {
		task, buildErr := s.buildCapturedTask(project, req)
		if buildErr != nil {
			return nil, buildErr
		}
		tasks = append(tasks, task)
		if planningBuilder == nil && req.PlanningArtifacts != nil {
			planningBuilder = req.PlanningArtifacts
		}
	}
	planningBuilder = s.defaultedPlanningArtifacts(planningBuilder)
	// Prepend the whole batch at once (newest first), then append the rest.
	prepend := make([]*models.Task, 0, len(tasks))
	for _, task := range tasks {
		if reqs[0].Prepend {
			prepend = append(prepend, task)
		} else {
			project.Tasks = append(project.Tasks, task)
		}
	}
	if len(prepend) > 0 {
		project.Tasks = append(prepend, project.Tasks...)
	}

	ctxTitle := fmt.Sprintf("%d tasks", len(tasks))
	if len(tasks) == 1 {
		ctxTitle = fmt.Sprintf("#%d %s", tasks[0].ShortID, tasks[0].Title)
	}
	ref := TaskRef{Task: tasks[0], Project: project, Base: snap.Document(project.ID)}
	targets := make([]PlanningReconcileTarget, len(tasks))
	for i, task := range tasks {
		targets[i] = PlanningReconcileTarget{TaskID: task.ID, ProjectID: project.ID, State: task.State()}
	}
	artifacts, err := planningArtifactsForTargets(targets, planningBuilder)
	if err != nil {
		return nil, err
	}
	committed, err := s.commitProjectWithArtifacts(ref, revision, artifacts, "Add Tasks", ctxTitle)
	if err != nil {
		return nil, err
	}

	results := make([]TaskResult, len(tasks))
	for i, task := range tasks {
		results[i] = newTaskResult(TaskRef{Task: task, Project: project}, committed.Revision)
	}
	return results, nil
}

// planningArtifactsForTargets renders one checked planning artifact set for a
// capture batch. Calling the builder once is important: every created task must
// be added to Today/Next in the same planning snapshot and the same ChangeSet.
func planningArtifactsForTargets(targets []PlanningReconcileTarget, builder PlanningArtifactBuilder) ([]storage.ArtifactChange, error) {
	if builder == nil || len(targets) == 0 {
		return nil, nil
	}
	artifacts, err := builder(PlanningReconcileRequest{Targets: targets})
	if err != nil {
		return nil, err
	}
	for _, artifact := range artifacts {
		if artifact.ExpectedRevision == nil {
			return nil, fmt.Errorf("app: planning artifact %q has no expected revision", artifact.Path)
		}
	}
	return artifacts, nil
}

// buildCapturedTask validates a CaptureRequest against an already-resolved
// project and returns the fully-constructed task WITHOUT inserting it, so both
// Capture and CreateLinkedTask share the exact field-application order (metadata,
// recurrence, due, parent, then canonical state initialization). It does not
// commit; the caller positions the task and commits the ChangeSet.
func (s *Service) buildCapturedTask(project *models.Project, req CaptureRequest) (*models.Task, error) {
	title := strings.TrimSpace(req.Title)
	if err := models.ValidateTaskTitle(title); err != nil {
		return nil, invalidInput("title", err.Error())
	}

	taskType := req.Type
	if taskType == "" {
		taskType = models.TaskTypeTask
	} else if !taskType.IsValid() {
		return nil, invalidInput("type", fmt.Sprintf("%q is not a valid task type", taskType))
	}

	priority := req.Priority
	if priority == 0 {
		priority = models.PriorityDefault
	}
	if err := models.ValidatePriority(priority, false); err != nil {
		return nil, invalidInput("priority", err.Error())
	}

	normalizedTags, err := normalizeTags(req.Tags)
	if err != nil {
		return nil, err
	}
	removeTags, err := normalizeTags(req.RemoveTags)
	if err != nil {
		return nil, err
	}

	if req.ParentID != nil && project.GetTaskByID(*req.ParentID) == nil {
		return nil, fmt.Errorf("%w: parent %s", ErrTaskNotFound, *req.ParentID)
	}

	state, err := normalizeCreationState(req.State)
	if err != nil {
		return nil, err
	}

	task, err := models.NewTask(s.ids.NewID(), project.GetNextShortID(), taskType, title)
	if err != nil {
		return nil, invalidInput("title", err.Error())
	}
	task.Priority = priority
	task.Description = req.Description
	task.URL = req.URL
	task.Source = req.Source
	for _, tag := range normalizedTags {
		task.AddTag(tag)
	}
	for _, tag := range removeTags {
		task.RemoveTag(tag)
	}
	if req.Recurrence != nil {
		if err := task.SetRecurrence(req.Recurrence); err != nil {
			return nil, invalidInput("recurrence", err.Error())
		}
	}
	if req.Due != nil {
		due := *req.Due
		task.DueDate = &due
		task.HasTime = req.HasTime
	}
	if req.ParentID != nil {
		parentID := *req.ParentID
		task.ParentID = &parentID
	}
	if err := s.initializeCreatedTask(task, state); err != nil {
		return nil, err
	}
	return task, nil
}

// insertCapturedTask positions a freshly built task in its project: prepended for
// prominence (Quick/Bulk Capture) or appended (default).
func insertCapturedTask(project *models.Project, task *models.Task, prepend bool) {
	if prepend {
		project.Tasks = append([]*models.Task{task}, project.Tasks...)
		return
	}
	project.Tasks = append(project.Tasks, task)
}

// normalizeTags normalizes and validates a set of tags, returning the cleaned
// set. A malformed tag is a hard error so the caller learns the capture was
// rejected rather than silently dropping the tag (models.AddTag ignores invalid
// tags on its own).
func normalizeTags(tags []string) ([]string, error) {
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := models.NormalizeTag(raw)
		if err := models.ValidateTag(tag); err != nil {
			return nil, invalidInput("tag", err.Error())
		}
		out = append(out, tag)
	}
	return out, nil
}
