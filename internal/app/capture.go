package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// CaptureDestination is the unified creation destination accepted by every
// capture adapter. Today and Next are planning memberships; the remaining
// values are canonical lifecycle states.
type CaptureDestination string

const (
	CaptureDestinationInbox    CaptureDestination = "inbox"
	CaptureDestinationBacklog  CaptureDestination = "backlog"
	CaptureDestinationToday    CaptureDestination = "today"
	CaptureDestinationNext     CaptureDestination = "next"
	CaptureDestinationDoing    CaptureDestination = "doing"
	CaptureDestinationWaiting  CaptureDestination = "waiting"
	CaptureDestinationPaused   CaptureDestination = "paused"
	CaptureDestinationSomeday  CaptureDestination = "someday"
	CaptureDestinationDone     CaptureDestination = "done"
	CaptureDestinationCanceled CaptureDestination = "canceled"
)

var captureDestinations = []CaptureDestination{
	CaptureDestinationInbox,
	CaptureDestinationBacklog,
	CaptureDestinationToday,
	CaptureDestinationNext,
	CaptureDestinationDoing,
	CaptureDestinationWaiting,
	CaptureDestinationPaused,
	CaptureDestinationSomeday,
	CaptureDestinationDone,
	CaptureDestinationCanceled,
}

// ParseCaptureDestination resolves one exact case-insensitive destination.
func ParseCaptureDestination(raw string) (CaptureDestination, bool) {
	destination := CaptureDestination(strings.ToLower(strings.TrimSpace(raw)))
	for _, candidate := range captureDestinations {
		if destination == candidate {
			return candidate, true
		}
	}
	return "", false
}

// InvalidCaptureDestinationError gives capture adapters one actionable error
// vocabulary, including explicit replacements for retired state names users
// are most likely to enter.
func InvalidCaptureDestinationError(raw string) error {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "ready":
		return fmt.Errorf("destination $ready was removed (use $backlog)")
	case "in_progress":
		return fmt.Errorf("destination $in_progress was removed (use $doing)")
	case "todo":
		return fmt.Errorf("destination $todo was removed (use $inbox or $backlog)")
	case "active":
		return fmt.Errorf("destination $active was removed (use $backlog)")
	case "start", "started", "progress":
		return fmt.Errorf("destination $%s was removed (use $doing)", value)
	case "blocked":
		return fmt.Errorf("destination $blocked was removed (use $waiting)")
	case "standby", "hold":
		return fmt.Errorf("destination $%s was removed (use $paused)", value)
	case "review":
		return fmt.Errorf("destination $review is not a state (use #review)")
	case "maybe":
		return fmt.Errorf("destination $maybe was removed (use $someday)")
	case "complete", "completed":
		return fmt.Errorf("destination $%s was removed (use $done)", value)
	case "cancel", "cancelled": //nolint:misspell // Intentional rejected-input alias.
		return fmt.Errorf("destination $%s was removed (use $canceled)", value)
	default:
		values := make([]string, len(captureDestinations))
		for i, destination := range captureDestinations {
			values[i] = "$" + string(destination)
		}
		return fmt.Errorf("invalid destination: $%s (use %s)", raw, strings.Join(values, ", "))
	}
}

// WorkflowState returns the lifecycle state created for the destination. Today
// and Next both create Backlog tasks and add their explicit planning directive.
func (d CaptureDestination) WorkflowState() models.WorkflowState {
	switch d {
	case CaptureDestinationToday, CaptureDestinationNext:
		return models.StateBacklog
	default:
		state, _ := models.ParseWorkflowState(string(d))
		return state
	}
}

// IsPlanning reports whether creation must also persist an ordered planning
// membership.
func (d CaptureDestination) IsPlanning() bool {
	return d == CaptureDestinationToday || d == CaptureDestinationNext
}

// CaptureRequest describes a new task to create in a project. Only ProjectID and
// Title are required; every other field is optional and defaults to the domain
// default (unset priority, root Inbox/child Backlog destination, no due date).
type CaptureRequest struct {
	// SourceLine is the one-based composer line that produced this request.
	// Zero means the request did not originate from a multiline composer.
	SourceLine int
	// ProjectID is the owning project's stable ID. Required.
	ProjectID string
	// Title is the task title. Required; validated with models.ValidateTaskTitle.
	Title string
	// Priority is 0..3, where zero is the durable Unset value.
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
	// Destination is the typed lifecycle/planning destination. Empty defaults to
	// Backlog when ParentID is set and Inbox otherwise.
	Destination CaptureDestination
	// Prepend inserts the new task at the front of the project's task slice so it
	// is prominently visible (Quick/Bulk Capture), rather than appended at the end.
	Prepend bool
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
	before, err := cloneSnapshot(snap)
	if err != nil {
		return TaskResult{}, fmt.Errorf("capture rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()

	prepared, err := s.prepareCapturedTask(project, req)
	if err != nil {
		return TaskResult{}, err
	}
	task, err := s.buildPreparedCapturedTask(project, prepared)
	if err != nil {
		return TaskResult{}, err
	}
	insertCapturedTask(project, task, req.Prepend)

	ref := TaskRef{Task: task, Project: project, Base: snap.Document(project.ID)}
	artifacts, err := planningArtifactsForTargets(
		[]PlanningReconcileTarget{prepared.destination.planningTarget(task.ID, project.ID)},
		nil,
		s.defaultedPlanningArtifacts(nil),
	)
	if err != nil {
		return TaskResult{}, err
	}
	result, err := s.commitProjectWithArtifacts(ref, revision, artifacts, "Add Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
	if err != nil {
		return TaskResult{}, err
	}
	committed = true
	return result, nil
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

	planningBuilder := s.defaultedPlanningArtifacts(nil)

	ctxTitle := fmt.Sprintf("%d tasks across %d projects", len(items), len(projectOrder))
	if len(items) == 1 {
		ctxTitle = fmt.Sprintf("#%d %s", items[0].task.ShortID, items[0].task.Title)
	}
	targets := make([]PlanningReconcileTarget, len(items))
	for i, item := range items {
		targets[i] = item.prepared.destination.planningTarget(item.task.ID, item.project.ID)
	}
	artifacts, err := planningArtifactsForTargets(targets, nil, planningBuilder)
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
			transactionIDAt(outcome.results, projectIndex[item.project.ID]),
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
// capture batch, dropping the memberships of any task the same transaction
// removes. Calling the builder once is important: every created task must be
// added to Today/Next in the same planning snapshot and the same ChangeSet, and
// a removal has to leave from that same snapshot.
func planningArtifactsForTargets(targets []PlanningReconcileTarget, remove []TaskLocator, builder PlanningArtifactBuilder) ([]storage.ArtifactChange, error) {
	if len(targets) == 0 && len(remove) == 0 {
		return nil, nil
	}
	if builder == nil {
		// Only an add refuses here. A removal without a builder falls through the
		// way Delete and BulkDelete already do: the membership is left to read-time
		// filtering rather than failing a deletion the user confirmed.
		for _, target := range targets {
			if target.Today == PlanningTodayAdd || target.Next == PlanningNextAdd {
				return nil, invalidInput("destination", "planning persistence is unavailable")
			}
		}
		return nil, nil
	}
	artifacts, err := builder(PlanningReconcileRequest{Targets: targets, Remove: remove})
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
	req         CaptureRequest
	title       string
	priority    int
	tags        []string
	removeTags  []string
	destination captureDestinationEffect
}

func (s *Service) prepareCapturedTask(project *models.Project, req CaptureRequest) (preparedCapture, error) {
	title := strings.TrimSpace(req.Title)
	if err := models.ValidateTaskTitle(title); err != nil {
		return preparedCapture{}, invalidInput("title", err.Error())
	}

	priority := req.Priority
	if err := models.ValidatePriority(priority, true); err != nil {
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

	destination, err := normalizeCaptureDestination(req.Destination, req.ParentID != nil)
	if err != nil {
		return preparedCapture{}, err
	}

	return preparedCapture{
		req:         req,
		title:       title,
		priority:    priority,
		tags:        normalizedTags,
		removeTags:  removeTags,
		destination: destination,
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
	if req.Due != nil {
		task.DueDate = cloneCalendarDatePtr(req.Due)
	}
	if req.ParentID != nil {
		parentID := *req.ParentID
		task.ParentID = &parentID
	}
	if err := s.initializeCreatedTask(task, prepared.destination.state); err != nil {
		return nil, err
	}
	return task, nil
}

type captureDestinationEffect struct {
	destination CaptureDestination
	state       models.WorkflowState
	today       PlanningTodayDirective
	next        PlanningNextDirective
}

func normalizeCaptureDestination(destination CaptureDestination, child bool) (captureDestinationEffect, error) {
	if destination == "" {
		if child {
			destination = CaptureDestinationBacklog
		} else {
			destination = CaptureDestinationInbox
		}
	}
	normalized, ok := ParseCaptureDestination(string(destination))
	if !ok {
		return captureDestinationEffect{}, invalidInput("destination", InvalidCaptureDestinationError(string(destination)).Error())
	}
	effect := captureDestinationEffect{destination: normalized, state: normalized.WorkflowState()}
	switch normalized {
	case CaptureDestinationToday:
		effect.today = PlanningTodayAdd
		effect.next = PlanningNextRemove
	case CaptureDestinationNext:
		effect.today = PlanningTodayRemove
		effect.next = PlanningNextAdd
	default:
		// Lifecycle-only destinations require no independent planning effect.
	}
	return effect, nil
}

func (e captureDestinationEffect) planningTarget(taskID, projectID string) PlanningReconcileTarget {
	return PlanningReconcileTarget{
		TaskID: taskID, ProjectID: projectID, State: e.state,
		Today: e.today, Next: e.next,
	}
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

// normalizeTagList validates and de-duplicates a tag operation list.
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
