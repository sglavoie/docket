package app

import (
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// preparedCapture contains every normalized value needed to construct a task.
// Preparing a request never consumes an ID or mutates the target project.
type preparedCapture struct {
	req          CaptureRequest
	title        string
	priority     int
	tags         []string
	context      string
	removeTags   []string
	parent       *models.Task
	children     []*models.Task
	destination  captureDestinationEffect
	attachments  []ResolvedAttachmentInput
	comments     []string
	dependencies []models.TaskReference
}

func (s *Service) prepareCapturedTask(snap Snapshot, project *models.Project, req CaptureRequest) (preparedCapture, error) {
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

	attachments, err := resolveCapturedAttachments(req.Attachments)
	if err != nil {
		return preparedCapture{}, err
	}

	comments, err := resolveCapturedComments(req.Comments)
	if err != nil {
		return preparedCapture{}, err
	}

	// A captured task has no identity yet, so no locator can name it and it stores
	// nothing yet: the resolver is built with an empty self over an empty existing
	// set, and every dependency is an ordinary addition.
	dependencies, err := newReferenceResolver(snap, TaskLocator{}).resolveDependencyAdditions(nil, req.Dependencies)
	if err != nil {
		return preparedCapture{}, err
	}

	return preparedCapture{
		req:          req,
		title:        title,
		priority:     priority,
		tags:         normalizedTags,
		context:      context,
		removeTags:   removeTags,
		parent:       parent,
		children:     children,
		destination:  destination,
		attachments:  attachments,
		comments:     comments,
		dependencies: dependencies,
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
	task.Dependencies = models.CloneTaskReferences(prepared.dependencies)
	for _, tag := range prepared.tags {
		task.AddTag(tag)
	}
	for _, tag := range prepared.removeTags {
		task.RemoveTag(tag)
	}
	if req.Due != nil {
		task.DueDate = cloneDueDatePtr(req.Due)
	}
	if req.Pinned {
		task.ApplyPinned(true, now)
	}
	if req.Review {
		task.ApplyReview(true, now)
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
		if err := task.ApplyDeferAt(*req.Defer, now); err != nil {
			return nil, invalidInput("defer", err.Error())
		}
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
