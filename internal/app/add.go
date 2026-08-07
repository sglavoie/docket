package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// AddTaskRequest is the full representation of a task-add request: metadata,
// hierarchy placement, checklist content, and
// any auxiliary durable artifacts that must commit atomically with the new task.
// The adapter resolves presentation concerns and hands this request fully-resolved
// primitives; the use case owns the snapshot, validation,
// ID/short-ID minting, and the single ChangeSet commit. It is the richer sibling
// of CaptureRequest, which covers only the minimal quick-capture fields.
type AddTaskRequest struct {
	// ProjectID is the owning project's stable ID. Required.
	ProjectID string
	// Title is the task title. Required; validated with models.ValidateTaskTitle.
	Title string
	// Priority is schema-6 0..3 (Unset, High, Medium, Low).
	Priority int
	// Tags are normalized and validated; a malformed tag aborts the add.
	Tags []string
	// Description is optional free-form body text.
	Description string
	// State is the task's initial canonical workflow state. Empty defaults to
	// Inbox. Capture-originated form requests use Destination instead; supplying
	// both is rejected.
	State models.WorkflowState
	// Destination carries the unified capture destination through expanded form
	// creation. Today and Next include their planning membership in the same
	// transaction as the project document.
	Destination CaptureDestination
	// ParentID, when non-nil, nests the new task under an existing task in the
	// same project; the parent must exist and preserve the depth limit.
	ParentID *string
	// Checklist holds checklist item texts. Each becomes an unchecked
	// models.ChecklistItem.
	Checklist []string
	// ChecklistItems holds full checklist items (text + done) set by the task
	// form, which can seed items already marked done. Blank-text items are
	// skipped. Applied after Checklist.
	ChecklistItems []models.ChecklistItem
	// Artifacts are auxiliary durable artifact writes committed in the SAME
	// ChangeSet as the new task, so the add and its side effects are atomic.
	Artifacts []storage.ArtifactChange
	// PlanningArtifacts carries route-local composer presets (notably planning
	// Today/Next) through the full expanded form path.
	PlanningArtifacts PlanningArtifactBuilder

	// The following fields are populated only by the task form (via
	// AddTaskWithChildren); the CLI `tt task add` path leaves them zero/nil so its
	// behavior is unchanged.

	// DueDate carries the form's calendar-date planning metadata.
	DueDate *time.Time
	// Pinned carries the form's flag state and records PinnedAt.
	// Focus Queue membership is independent planning state handled at the
	// config-aware layer, not a task field set here.
	Pinned bool
	// Subtasks are child tasks created under the new task in the SAME ChangeSet
	// (task-form Epic/Story create). Each inherits the parent's priority, state,
	// and tags; a blank-title subtask is skipped.
	Subtasks []AddSubtaskRequest
}

// AddSubtaskRequest is one child task the task form creates under a new
// parent.
type AddSubtaskRequest struct {
	Title string
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
// ChangeSet. It validates the title, priority, tags, state, parent placement,
// before mutating, so a rejected
// request never persists a partial task or a dangling config write.
func (s *Service) AddTask(req AddTaskRequest) (TaskResult, error) {
	built, err := s.buildAddTask(req)
	if err != nil {
		return TaskResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(built.snap, built.before)
		}
	}()
	ref := built.ref
	planning, err := planningArtifactsForTargets(
		[]PlanningReconcileTarget{addTaskPlanningTarget(ref, built.destination)},
		s.defaultedPlanningArtifacts(req.PlanningArtifacts),
	)
	if err != nil {
		return TaskResult{}, err
	}
	artifacts := append(append([]storage.ArtifactChange(nil), req.Artifacts...), planning...)
	result, err := s.commitProjectWithArtifacts(ref, built.revision, artifacts, "Add Task", fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title))
	if err != nil {
		return TaskResult{}, err
	}
	committed = true
	return result, nil
}

type normalizedAddTaskRequest struct {
	title       string
	priority    int
	tags        []string
	state       models.WorkflowState
	destination CaptureDestination
}

func normalizeAddTaskRequest(req AddTaskRequest) (normalizedAddTaskRequest, error) {
	title := strings.TrimSpace(req.Title)
	if err := models.ValidateTaskTitle(title); err != nil {
		return normalizedAddTaskRequest{}, invalidInput("title", err.Error())
	}

	priority := req.Priority
	if err := models.ValidatePriority(priority, true); err != nil {
		return normalizedAddTaskRequest{}, invalidInput("priority", err.Error())
	}

	state := req.State
	destination := req.Destination
	if destination != "" {
		if state != "" {
			return normalizedAddTaskRequest{}, invalidInput("destination", "provide destination or state, not both")
		}
		effect, err := normalizeCaptureDestination(destination, req.ParentID != nil)
		if err != nil {
			return normalizedAddTaskRequest{}, err
		}
		destination = effect.destination
		state = effect.state
	} else {
		var err error
		state, err = normalizeCreationState(state)
		if err != nil {
			return normalizedAddTaskRequest{}, err
		}
	}

	tags, err := normalizeTags(req.Tags)
	if err != nil {
		return normalizedAddTaskRequest{}, err
	}
	return normalizedAddTaskRequest{title: title, priority: priority, tags: tags, state: state, destination: destination}, nil
}

type addTaskBuild struct {
	ref         TaskRef
	revision    string
	destination CaptureDestination
	snap        Snapshot
	before      Snapshot
}

// buildAddTask validates an AddTaskRequest against a fresh snapshot and appends
// the fully-constructed task to its project WITHOUT committing. The returned
// snapshots let callers restore every in-memory mutation if artifact creation or
// the atomic commit fails.
func (s *Service) buildAddTask(req AddTaskRequest) (addTaskBuild, error) {
	normalized, err := normalizeAddTaskRequest(req)
	if err != nil {
		return addTaskBuild{}, err
	}

	snap, err := s.repo.Snapshot()
	if err != nil {
		return addTaskBuild{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return addTaskBuild{}, fmt.Errorf("add task rollback state: %w", err)
	}
	useRollbackDocuments(before)
	ref, revision, err := s.buildNormalizedAddTask(snap, req, normalized)
	if err != nil {
		restoreSnapshotProjects(snap, before)
		return addTaskBuild{}, err
	}
	return addTaskBuild{ref: ref, revision: revision, destination: normalized.destination, snap: snap, before: before}, nil
}

// buildNormalizedAddTask constructs a validated task inside a caller-owned
// snapshot without committing it. Bare creation obtains its own snapshot before
// calling this helper. Grouped creation paths can supply their own snapshot so
// validation, mutation, and commit share one revision-consistent workspace view.
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
		resolved, parentErr := resolveChildParent(project, *req.ParentID)
		if parentErr != nil {
			return TaskRef{}, "", parentErr
		}
		parent = resolved
	}

	task, err := newTaskForCreation(s.ids.NewID(), project.GetNextShortID(), normalized.title)
	if err != nil {
		return TaskRef{}, "", invalidInput("title", err.Error())
	}
	task.Priority = normalized.priority
	task.Description = req.Description
	for _, tag := range normalized.tags {
		task.AddTag(tag)
	}
	if err := s.initializeCreatedTask(task, normalized.state); err != nil {
		return TaskRef{}, "", err
	}
	if parent != nil {
		parentID := parent.ID
		task.ParentID = &parentID
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
		task.DueDate = cloneCalendarDatePtr(req.DueDate)
	}
	if req.Pinned {
		now := s.clock.Now()
		task.Pinned = true
		task.PinnedAt = &now
	}

	project.Tasks = append(project.Tasks, task)

	return TaskRef{Task: task, Project: project, Base: snap.Document(project.ID)}, revision, nil
}

// AddTaskWithChildren creates a fully-specified parent task and its subtasks and
// commits them — with any auxiliary artifacts —
// as ONE ChangeSet, preserving the task form's atomic Epic/Story creation and
// single undo entry. Each subtask inherits the parent's priority, state, and
// tags; a blank-title subtask is skipped. It returns the parent result plus a result per created
// subtask in creation order.
func (s *Service) AddTaskWithChildren(req AddTaskRequest) (FormCreateResult, error) {
	built, err := s.buildAddTask(req)
	if err != nil {
		return FormCreateResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(built.snap, built.before)
		}
	}()
	ref := built.ref

	childRefs := make([]TaskRef, 0, len(req.Subtasks))
	for _, sub := range req.Subtasks {
		title := strings.TrimSpace(sub.Title)
		if title == "" {
			continue
		}
		child, buildErr := newTaskForCreation(s.ids.NewID(), ref.Project.GetNextShortID(), title)
		if buildErr != nil {
			return FormCreateResult{}, invalidInput("title", buildErr.Error())
		}
		child.Priority = ref.Task.Priority
		if stateErr := s.initializeCreatedTask(child, ref.Task.State()); stateErr != nil {
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

	planning, err := planningArtifactsForTargets(
		[]PlanningReconcileTarget{addTaskPlanningTarget(ref, built.destination)},
		s.defaultedPlanningArtifacts(req.PlanningArtifacts),
	)
	if err != nil {
		return FormCreateResult{}, err
	}
	artifacts := append(append([]storage.ArtifactChange(nil), req.Artifacts...), planning...)
	result, err := s.commitProjectWithArtifacts(ref, built.revision, artifacts, "Add Task", fmt.Sprintf("#%d %s", ref.Task.ShortID, ref.Task.Title))
	if err != nil {
		return FormCreateResult{}, err
	}
	committed = true

	out := FormCreateResult{Task: result}
	for _, cref := range childRefs {
		out.Subtasks = append(out.Subtasks, newTaskResult(cref, result.Revision, result.TransactionID))
	}
	return out, nil
}

func addTaskPlanningTarget(ref TaskRef, destination CaptureDestination) PlanningReconcileTarget {
	if destination != "" {
		if effect, err := normalizeCaptureDestination(destination, ref.Task.ParentID != nil); err == nil {
			return effect.planningTarget(ref.Task.ID, ref.Project.ID)
		}
	}
	return PlanningReconcileTarget{TaskID: ref.Task.ID, ProjectID: ref.Project.ID, State: ref.Task.State()}
}
