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
// default (medium priority, Inbox/child Ready state, no due date).
type CaptureRequest struct {
	// SourceLine is the one-based composer line that produced this request.
	// Zero means the request did not originate from a multiline composer.
	SourceLine int
	// ProjectID is the owning project's stable ID. Required.
	ProjectID string
	// Title is the task title. Required; validated with models.ValidateTaskTitle.
	Title string
	// Priority is 1..5; zero (or unset) defaults to the medium priority.
	Priority int
	// Tags are normalized and validated; a malformed tag aborts the capture.
	Tags []string
	// RemoveTags are normalized tags to strip from the new task after Tags are
	// applied (the focus-board quick-add supports "-tag" removal directives).
	RemoveTags []string
	// Description is optional free-form body text.
	Description string
	// Due, when non-nil, sets the task's calendar due date.
	Due *time.Time
	// ParentID, when non-nil, nests the new task under an existing task in the
	// same project. The parent must exist and leave the child within the nesting cap.
	ParentID *string
	// State is the task's initial canonical workflow state. Empty defaults to
	// Ready when ParentID is set and Inbox otherwise.
	State models.WorkflowState
	// Recurrence, when non-nil, is the parsed recurrence pattern applied via
	// task.SetRecurrence.
	Recurrence *models.RecurrencePattern
	// Prepend inserts the new task at the front of the project's task slice so it
	// is prominently visible (Quick/Bulk Capture), rather than appended at the end.
	Prepend bool
	// PlanningArtifacts, when set, builds a checked planning-domain artifact for
	// the freshly created task and commits it in the same ChangeSet. Planner uses
	// this to make Today/Next creation and horizon membership one undoable write.
	PlanningArtifacts PlanningArtifactBuilder
}

// Capture creates a new task in the target project and commits it as one
// ChangeSet. It validates the title, priority, tags, state, and parent
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

// CaptureMany creates every requested task across one or more projects and
// commits all touched projects as ONE transaction with one undo entry. The full
// batch is resolved and validated before any project is mutated. Prepended tasks
// land at the front of each owning project in request order.
func (s *Service) CaptureMany(reqs []CaptureRequest) ([]TaskResult, error) {
	if len(reqs) == 0 {
		return nil, invalidInput("capture", "no tasks to create")
	}

	snap, err := s.repo.Snapshot()
	if err != nil {
		return nil, err
	}

	type batchItem struct {
		project  *models.Project
		prepared preparedCapture
		task     *models.Task
	}
	items := make([]batchItem, len(reqs))
	projectOrder := make([]*models.Project, 0)
	seenProjects := make(map[string]bool)
	var planningBuilder PlanningArtifactBuilder
	for i, req := range reqs {
		project := snap.ProjectByID(req.ProjectID)
		if project == nil {
			return nil, captureRequestError(req, i, fmt.Errorf("%w: %s", ErrProjectNotFound, req.ProjectID))
		}
		prepared, prepareErr := s.prepareCapturedTask(project, req)
		if prepareErr != nil {
			return nil, captureRequestError(req, i, prepareErr)
		}
		items[i] = batchItem{project: project, prepared: prepared}
		if !seenProjects[project.ID] {
			seenProjects[project.ID] = true
			projectOrder = append(projectOrder, project)
		}
		if planningBuilder == nil && req.PlanningArtifacts != nil {
			planningBuilder = req.PlanningArtifacts
		}
	}

	before, err := cloneSnapshot(snap)
	if err != nil {
		return nil, fmt.Errorf("capture batch rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	for i := range items {
		task, buildErr := s.buildPreparedCapturedTask(items[i].project, items[i].prepared)
		if buildErr != nil {
			return nil, captureRequestError(reqs[i], i, buildErr)
		}
		items[i].task = task
	}

	prependedByProject := make(map[string][]*models.Task, len(projectOrder))
	for i, item := range items {
		if reqs[i].Prepend {
			prependedByProject[item.project.ID] = append(prependedByProject[item.project.ID], item.task)
			continue
		}
		item.project.Tasks = append(item.project.Tasks, item.task)
	}
	for _, project := range projectOrder {
		if prepended := prependedByProject[project.ID]; len(prepended) > 0 {
			project.Tasks = append(prepended, project.Tasks...)
		}
	}

	planningBuilder = s.defaultedPlanningArtifacts(planningBuilder)

	ctxTitle := fmt.Sprintf("%d tasks across %d projects", len(items), len(projectOrder))
	if len(items) == 1 {
		ctxTitle = fmt.Sprintf("#%d %s", items[0].task.ShortID, items[0].task.Title)
	}
	targets := make([]PlanningReconcileTarget, len(items))
	for i, item := range items {
		targets[i] = PlanningReconcileTarget{TaskID: item.task.ID, ProjectID: item.project.ID, State: item.task.State()}
	}
	artifacts, err := planningArtifactsForTargets(targets, planningBuilder)
	if err != nil {
		return nil, err
	}

	commits := make([]projectCommit, len(projectOrder))
	projectIndex := make(map[string]int, len(projectOrder))
	for i, project := range projectOrder {
		projectIndex[project.ID] = i
		commits[i] = projectCommit{
			project:          project,
			expectedRevision: snap.Revision(project.ID),
			base:             snap.Document(project.ID),
		}
	}
	if len(commits) > 0 {
		commits[0].artifacts = artifacts
	}
	outcome, err := s.commitProjects(commits, "Add Tasks", ctxTitle)
	if err != nil {
		return nil, err
	}
	committed = true

	results := make([]TaskResult, len(items))
	for i, item := range items {
		results[i] = newTaskResult(
			TaskRef{Task: item.task, Project: item.project},
			revisionAt(outcome.results, projectIndex[item.project.ID]),
		)
	}
	return results, nil
}

func captureRequestError(req CaptureRequest, index int, err error) error {
	line := req.SourceLine
	if line <= 0 {
		line = index + 1
	}
	return fmt.Errorf("line %d: %w", line, err)
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

// preparedCapture contains every normalized value needed to construct a task.
// Preparing a request never consumes an ID or mutates the target project.
type preparedCapture struct {
	req        CaptureRequest
	title      string
	priority   int
	tags       []string
	removeTags []string
	state      models.WorkflowState
}

func (s *Service) prepareCapturedTask(project *models.Project, req CaptureRequest) (preparedCapture, error) {
	title := strings.TrimSpace(req.Title)
	if err := models.ValidateTaskTitle(title); err != nil {
		return preparedCapture{}, invalidInput("title", err.Error())
	}

	priority := req.Priority
	if priority == 0 {
		priority = models.PriorityDefault
	}
	if err := models.ValidatePriority(priority, false); err != nil {
		return preparedCapture{}, invalidInput("priority", err.Error())
	}

	normalizedTags, err := normalizeTags(req.Tags)
	if err != nil {
		return preparedCapture{}, err
	}
	removeTags, err := normalizeTagList(req.RemoveTags)
	if err != nil {
		return preparedCapture{}, err
	}

	if req.ParentID != nil {
		if _, parentErr := resolveChildParent(project, *req.ParentID); parentErr != nil {
			return preparedCapture{}, parentErr
		}
	}

	state := req.State
	if state == "" && req.ParentID != nil {
		state = models.StateReady
	}
	state, err = normalizeCreationState(state)
	if err != nil {
		return preparedCapture{}, err
	}

	if req.Recurrence != nil {
		if err := req.Recurrence.Validate(); err != nil {
			return preparedCapture{}, invalidInput("recurrence", err.Error())
		}
	}

	return preparedCapture{
		req:        req,
		title:      title,
		priority:   priority,
		tags:       normalizedTags,
		removeTags: removeTags,
		state:      state,
	}, nil
}

func (s *Service) buildPreparedCapturedTask(project *models.Project, prepared preparedCapture) (*models.Task, error) {
	req := prepared.req
	task, err := newTaskForCreation(s.ids.NewID(), project.GetNextShortID(), prepared.title)
	if err != nil {
		return nil, invalidInput("title", err.Error())
	}
	task.Priority = prepared.priority
	task.Description = req.Description
	for _, tag := range prepared.tags {
		task.AddTag(tag)
	}
	for _, tag := range prepared.removeTags {
		task.RemoveTag(tag)
	}
	if req.Recurrence != nil {
		if err := task.SetRecurrence(req.Recurrence); err != nil {
			return nil, invalidInput("recurrence", err.Error())
		}
	}
	if req.Due != nil {
		task.DueDate = cloneCalendarDatePtr(req.Due)
	}
	if req.ParentID != nil {
		parentID := *req.ParentID
		task.ParentID = &parentID
	}
	if err := s.initializeCreatedTask(task, prepared.state); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Service) buildCapturedTask(project *models.Project, req CaptureRequest) (*models.Task, error) {
	prepared, err := s.prepareCapturedTask(project, req)
	if err != nil {
		return nil, err
	}
	return s.buildPreparedCapturedTask(project, prepared)
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
	out, err := models.NormalizeTaskTags(tags)
	if err != nil {
		return nil, invalidInput("tag", err.Error())
	}
	return out, nil
}

// normalizeTagList validates and canonicalizes a tag operation list without
// treating multiple marker names as a conflict. Removal lists may legitimately
// clear different markers from different tasks in one bulk operation.
func normalizeTagList(tags []string) ([]string, error) {
	out := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, raw := range tags {
		tag, err := models.NormalizeTaskTag(raw)
		if err != nil {
			return nil, invalidInput("tag", err.Error())
		}
		if !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	return out, nil
}
