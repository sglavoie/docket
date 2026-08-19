package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// CaptureDestination is the unified creation destination accepted by every
// capture adapter. Today and Next are planning memberships; the remaining
// values are canonical lifecycle states.
type CaptureDestination = textgrammar.Destination

const (
	CaptureDestinationInbox   = textgrammar.DestinationInbox
	CaptureDestinationBacklog = textgrammar.DestinationBacklog
	CaptureDestinationToday   = textgrammar.DestinationToday
	CaptureDestinationNext    = textgrammar.DestinationNext
)

// ParseCaptureDestination resolves one exact case-insensitive destination.
func ParseCaptureDestination(raw string) (CaptureDestination, bool) {
	return textgrammar.ParseDestination(raw)
}

// InvalidCaptureDestinationError gives capture adapters one actionable error
// vocabulary, including explicit replacements for retired state names users
// are most likely to enter.
func InvalidCaptureDestinationError(raw string) error {
	return textgrammar.InvalidDestinationError(raw)
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
	// Context is the optional single-valued task context. Empty leaves it unset.
	Context string
	// RemoveTags are normalized tags to strip from the new task after Tags are
	// applied (the focus-board quick-add supports "-tag" removal directives).
	RemoveTags []string
	// Description is optional free-form body text.
	Description string
	// Due, when non-nil, sets the task's calendar due date.
	Due *time.Time
	// DeferSet marks that the request expressed a deferral at all, so a caller
	// reads the same pair here as on TaskLineEditRequest. Defer holds the instant
	// when one was named; a set flag with no instant captures without deferring.
	DeferSet bool
	Defer    *time.Time
	// ParentID, when non-nil, nests the new task under an existing task in the
	// same project. The parent must exist and leave the child within the nesting cap.
	ParentID *string
	// ChildIDs names existing tasks in the same project the new task adopts.
	// Each keeps its own subtree and the project's task order is untouched.
	// Repeated IDs collapse to their first occurrence.
	ChildIDs []string
	// Destination is the typed lifecycle/planning destination. Empty defaults to
	// Backlog when ParentID is set and Inbox otherwise. Adopting children alone
	// does not select the child default.
	Destination CaptureDestination
	// Prepend inserts the new task at the front of the project's task slice so it
	// is prominently visible (Quick/Bulk Capture), rather than appended at the end.
	Prepend bool
	// Pinned pins the new task; the pinned section still filters terminal tasks,
	// so a terminal destination pins without rendering a row.
	Pinned bool
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
	task, err := s.buildPreparedCapturedTask(project, prepared, s.clock.Now())
	if err != nil {
		return TaskResult{}, err
	}
	insertCapturedTask(project, task, req.Prepend)
	if edgeErr := applyCapturedTaskEdges(project, task, prepared); edgeErr != nil {
		return TaskResult{}, edgeErr
	}
	if guardErr := captureDestinationChildrenGuard(project, task, prepared); guardErr != nil {
		return TaskResult{}, guardErr
	}

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
	claimedChildren := make(map[string]int)
	for i, req := range reqs {
		project := snap.ProjectByID(req.ProjectID)
		if project == nil {
			return nil, captureRequestError(req, i, fmt.Errorf("%w: %s", ErrProjectNotFound, req.ProjectID))
		}
		prepared, prepareErr := s.prepareCapturedTask(project, req)
		if prepareErr != nil {
			return nil, captureRequestError(req, i, prepareErr)
		}
		// A child has exactly one parent, so two lines claiming it describe an
		// unresolvable batch. Rejecting here keeps the whole batch unmutated
		// instead of letting the later assignment silently win.
		for _, child := range prepared.children {
			key := project.ID + "\x00" + child.ID
			first, claimed := claimedChildren[key]
			if claimed {
				return nil, captureRequestError(req, i, invalidInput("child", fmt.Sprintf(
					"child #%d is already adopted on line %d", child.ShortID, captureRequestLine(reqs[first], first))))
			}
			claimedChildren[key] = i
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

	// One clock read for the whole batch: PinnedAt is the pinned section's sort
	// key, so per-line reads would order the lines newest-first and reverse the
	// order they were written in.
	now := s.clock.Now()
	for i := range items {
		task, buildErr := s.buildPreparedCapturedTask(items[i].project, items[i].prepared, now)
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
	// Every new task is in its project before any edge is applied, so an adopted
	// child is validated against the batch's final task set.
	for i, item := range items {
		if edgeErr := applyCapturedTaskEdges(item.project, item.task, item.prepared); edgeErr != nil {
			return nil, captureRequestError(reqs[i], i, edgeErr)
		}
		if guardErr := captureDestinationChildrenGuard(item.project, item.task, item.prepared); guardErr != nil {
			return nil, captureRequestError(reqs[i], i, guardErr)
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
	return fmt.Errorf("line %d: %w", captureRequestLine(req, index), err)
}

// captureRequestLine is the line number a batch failure is attributed to,
// falling back to the request's position when the request did not come from a
// multiline composer.
func captureRequestLine(req CaptureRequest, index int) int {
	if req.SourceLine > 0 {
		return req.SourceLine
	}
	return index + 1
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
	context     string
	removeTags  []string
	parent      *models.Task
	children    []*models.Task
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
	context := ""
	if req.Context != "" {
		context, err = models.NormalizeTaskContext(req.Context)
		if err != nil {
			return preparedCapture{}, invalidInput("context", err.Error())
		}
	}
	removeTags, err := normalizeTagList(req.RemoveTags)
	if err != nil {
		return preparedCapture{}, err
	}

	var parent *models.Task
	if req.ParentID != nil {
		resolvedParent, parentErr := resolveChildParent(project, *req.ParentID)
		if parentErr != nil {
			return preparedCapture{}, parentErr
		}
		parent = resolvedParent
	}

	children, err := resolveAdoptedChildren(project, req.ChildIDs)
	if err != nil {
		return preparedCapture{}, err
	}

	destination, err := normalizeCaptureDestination(req.Destination, req.ParentID != nil)
	if err != nil {
		return preparedCapture{}, err
	}
	destination, err = deferredCaptureDestination(destination, req)
	if err != nil {
		return preparedCapture{}, err
	}

	return preparedCapture{
		req:         req,
		title:       title,
		priority:    priority,
		tags:        normalizedTags,
		context:     context,
		removeTags:  removeTags,
		parent:      parent,
		children:    children,
		destination: destination,
	}, nil
}

// applyCapturedTaskEdges applies the new task's own parent edge together with
// every child it adopts, so cycles and nesting depth are validated against the
// hierarchy the capture actually produces rather than an intermediate one. With
// no adopted children the parent edge is already set and already validated, so
// the capture keeps the error vocabulary resolveChildParent produced.
func applyCapturedTaskEdges(project *models.Project, task *models.Task, prepared preparedCapture) error {
	if len(prepared.children) == 0 {
		return nil
	}
	assignments := make([]models.ParentAssignment, 0, len(prepared.children)+1)
	if prepared.parent != nil {
		assignments = append(assignments, models.ParentAssignment{Task: task, Parent: prepared.parent})
	}
	for _, child := range prepared.children {
		assignments = append(assignments, models.ParentAssignment{Task: child, Parent: task})
	}
	if _, err := project.ApplyParentAssignments(assignments); err != nil {
		return invalidInput("child", err.Error())
	}
	return nil
}

// captureDestinationChildrenGuard is capture's half of the one terminal-adoption
// policy, run after the hierarchy patch so the destination is judged against the
// children the capture produced. Only an adoption can reach it, because a task
// created without children has none, and it is the same rule and the same error
// taskLineDestinationChildrenGuard applies on the inline-edit surface. Without it
// a $done or $canceled capture would land illegally and be silently reopened by
// the post-mutation reconciliation.
func captureDestinationChildrenGuard(project *models.Project, task *models.Task, prepared preparedCapture) error {
	return childrenCompletionGuard(task, project, prepared.destination.state)
}

func (s *Service) buildPreparedCapturedTask(project *models.Project, prepared preparedCapture, now time.Time) (*models.Task, error) {
	req := prepared.req
	task, err := newTaskForCreation(s.ids.NewID(), project.GetNextShortID(), prepared.title)
	if err != nil {
		return nil, invalidInput("title", err.Error())
	}
	task.Priority = prepared.priority
	task.Context = prepared.context
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
	if req.Pinned {
		task.ApplyPinned(true, now)
	}
	if req.ParentID != nil {
		parentID := *req.ParentID
		task.ParentID = &parentID
	}
	if err := s.initializeCreatedTask(task, prepared.destination.state); err != nil {
		return nil, err
	}
	// The deferral is written after the state transition because entering a state
	// clears it: a captured deferral is the destination's companion, not a value
	// the transition should wipe.
	if req.DeferSet && req.Defer != nil {
		task.DeferredUntil = cloneInstantPtr(req.Defer)
	}
	return task, nil
}

// deferredCaptureDestination routes a captured deferral through deferTask's
// policy: a terminal destination refuses, and every other one lands in Backlog
// with neither planning horizon populated. A capture that names no instant keeps
// the destination it asked for.
func deferredCaptureDestination(destination captureDestinationEffect, req CaptureRequest) (captureDestinationEffect, error) {
	if !req.DeferSet || req.Defer == nil {
		return destination, nil
	}
	if err := deferralRefusesState(destination.state); err != nil {
		return captureDestinationEffect{}, err
	}
	return normalizeCaptureDestination(CaptureDestinationBacklog, req.ParentID != nil)
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
