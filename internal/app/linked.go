package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
)

// LinkKind names the relationship a linked-task capture establishes between the
// new task and its source task.
type LinkKind string

const (
	// LinkBlocker makes the new task block the source (source depends on new).
	LinkBlocker LinkKind = "blocker"
	// LinkDependent makes the new task depend on the source.
	LinkDependent LinkKind = "dependent"
	// LinkSubtask nests the new task under the source, promoting a leaf source to
	// a story so it can hold children.
	LinkSubtask LinkKind = "subtask"
	// LinkFollowUp relates the new task to the source bidirectionally and copies
	// the source's tags onto the new task.
	LinkFollowUp LinkKind = "followup"
)

// LinkedTaskRequest describes a new task created in relation to an existing
// source task in the same project. Task carries the new task's capture fields
// (its ParentID is honored only for non-subtask links); SourceTaskID identifies
// the existing task; Link selects the relationship.
type LinkedTaskRequest struct {
	Task         CaptureRequest
	SourceTaskID string
	Link         LinkKind
}

// LinkedTaskResult reports both sides of a linked-task creation: the new task and
// the (possibly mutated — dependency added, type promoted, related-to updated)
// source task.
type LinkedTaskResult struct {
	New    TaskResult
	Source TaskResult
}

// CreateLinkedTask creates a new task related to an existing source task and
// commits both — the new task and the mutated source — as ONE ChangeSet, so the
// task and its relationship land atomically. The source must live in the same
// project as the request's ProjectID. The new task is prepended for prominence
// unless it becomes a child (subtask, or a parsed parent on a non-subtask link),
// mirroring the legacy linked-task-creator ordering.
func (s *Service) CreateLinkedTask(req LinkedTaskRequest) (LinkedTaskResult, error) {
	snap, err := s.repo.Snapshot()
	if err != nil {
		return LinkedTaskResult{}, err
	}
	project := snap.ProjectByID(req.Task.ProjectID)
	if project == nil {
		return LinkedTaskResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, req.Task.ProjectID)
	}
	revision := snap.Revision(project.ID)

	source := project.GetTaskByID(req.SourceTaskID)
	if source == nil {
		return LinkedTaskResult{}, fmt.Errorf("%w: source %s", ErrTaskNotFound, req.SourceTaskID)
	}

	// A subtask's parent is the source, so ignore any parsed parent on that path
	// (matching the legacy creator, which applied a parsed >parent only for
	// non-subtask links).
	buildReq := req.Task
	if req.Link == LinkSubtask {
		buildReq.ParentID = nil
	}
	task, err := s.buildCapturedTask(project, buildReq)
	if err != nil {
		return LinkedTaskResult{}, err
	}

	switch req.Link {
	case LinkBlocker:
		if err := source.AddDependency(task.ID); err != nil {
			return LinkedTaskResult{}, invalidInput("blocker", err.Error())
		}
	case LinkDependent:
		if err := task.AddDependency(source.ID); err != nil {
			return LinkedTaskResult{}, invalidInput("dependent", err.Error())
		}
	case LinkSubtask:
		if source.Type.IsLeafType() {
			source.Type = models.TaskTypeStory
		}
		parentID := source.ID
		task.ParentID = &parentID
	case LinkFollowUp:
		if err := source.AddRelatedTo(task.ID); err != nil {
			return LinkedTaskResult{}, invalidInput("follow-up", err.Error())
		}
		if err := task.AddRelatedTo(source.ID); err != nil {
			return LinkedTaskResult{}, invalidInput("follow-up", err.Error())
		}
		for _, tag := range source.Tags {
			task.AddTag(tag)
		}
	default:
		return LinkedTaskResult{}, invalidInput("link", fmt.Sprintf("%q is not a valid link kind", req.Link))
	}

	// A child stays at the end of its siblings; a root task is prepended for
	// prominence.
	insertCapturedTask(project, task, task.ParentID == nil)

	ref := TaskRef{Task: task, Project: project}
	result, err := s.commitProject(ref, revision, "Add Linked Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))
	if err != nil {
		return LinkedTaskResult{}, err
	}

	// Re-resolve the source from the committed project so its result reflects any
	// promotion/relationship the mutation applied.
	sourceResult := newTaskResult(TaskRef{Task: source, Project: project}, result.Revision)
	return LinkedTaskResult{New: result, Source: sourceResult}, nil
}
