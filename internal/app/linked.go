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
	results, err := s.CreateLinkedTasks([]LinkedTaskRequest{req})
	if err != nil {
		return LinkedTaskResult{}, err
	}
	return results[0], nil
}

// CreateLinkedTasks creates every requested task and relationship in one
// ChangeSet. It is the multiline Task Composer counterpart to CreateLinkedTask:
// validation or relationship failure aborts the whole batch before persistence.
func (s *Service) CreateLinkedTasks(reqs []LinkedTaskRequest) ([]LinkedTaskResult, error) {
	if len(reqs) == 0 {
		return nil, invalidInput("linked tasks", "no tasks to create")
	}
	projectID := reqs[0].Task.ProjectID
	for _, req := range reqs {
		if req.Task.ProjectID != projectID {
			return nil, invalidInput("linked tasks", "all linked tasks must target the same project")
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
	sources := make([]*models.Task, 0, len(reqs))
	for _, req := range reqs {
		source := project.GetTaskByID(req.SourceTaskID)
		if source == nil {
			return nil, fmt.Errorf("%w: source %s", ErrTaskNotFound, req.SourceTaskID)
		}

		// A subtask's parent is the source, so ignore any parsed parent on that
		// path (matching the single linked-task creator).
		buildReq := req.Task
		if req.Link == LinkSubtask {
			buildReq.ParentID = nil
		}
		task, buildErr := s.buildCapturedTask(project, buildReq)
		if buildErr != nil {
			return nil, buildErr
		}

		switch req.Link {
		case LinkBlocker:
			if linkErr := source.AddDependency(task.ID); linkErr != nil {
				return nil, invalidInput("blocker", linkErr.Error())
			}
		case LinkDependent:
			if linkErr := task.AddDependency(source.ID); linkErr != nil {
				return nil, invalidInput("dependent", linkErr.Error())
			}
		case LinkSubtask:
			// Promotion is what makes a leaf source able to parent at all, so the
			// relationship is validated against the promoted type before it is
			// applied and before the source is mutated.
			promoted := source.Type
			if promoted.IsLeafType() {
				promoted = models.TaskTypeStory
			}
			if !task.Type.CanBeChildOf(promoted) {
				return nil, invalidInput("subtask", fmt.Sprintf("%s cannot be a child of %s", task.Type.HeadingPrefix(), promoted.HeadingPrefix()))
			}
			if source.Level+1 > models.MaxNestingDepth {
				return nil, invalidInput("subtask", fmt.Sprintf("nesting under #%d would exceed maximum depth of %d levels", source.ShortID, models.MaxNestingDepth))
			}
			source.Type = promoted
			parentID := source.ID
			task.ParentID = &parentID
		case LinkFollowUp:
			if linkErr := source.AddRelatedTo(task.ID); linkErr != nil {
				return nil, invalidInput("follow-up", linkErr.Error())
			}
			if linkErr := task.AddRelatedTo(source.ID); linkErr != nil {
				return nil, invalidInput("follow-up", linkErr.Error())
			}
			for _, tag := range source.Tags {
				task.AddTag(tag)
			}
		default:
			return nil, invalidInput("link", fmt.Sprintf("%q is not a valid link kind", req.Link))
		}
		tasks = append(tasks, task)
		sources = append(sources, source)
	}

	// Preserve compact-line order for roots while children stay at the end of
	// their sibling collection.
	rootTasks := make([]*models.Task, 0, len(tasks))
	for _, task := range tasks {
		if task.ParentID == nil {
			rootTasks = append(rootTasks, task)
		} else {
			project.Tasks = append(project.Tasks, task)
		}
	}
	if len(rootTasks) > 0 {
		project.Tasks = append(rootTasks, project.Tasks...)
	}

	context := fmt.Sprintf("%d linked tasks", len(tasks))
	undoAction := "Add Linked Tasks"
	if len(tasks) == 1 {
		context = fmt.Sprintf("#%d %s", tasks[0].ShortID, tasks[0].Title)
		undoAction = "Add Linked Task"
	}
	ref := TaskRef{Task: tasks[0], Project: project, Base: snap.Document(project.ID)}
	result, err := s.commitProject(ref, revision, undoAction, context)
	if err != nil {
		return nil, err
	}

	results := make([]LinkedTaskResult, len(tasks))
	for i, task := range tasks {
		newResult := newTaskResult(TaskRef{Task: task, Project: project}, result.Revision)
		sourceResult := newTaskResult(TaskRef{Task: sources[i], Project: project}, result.Revision)
		results[i] = LinkedTaskResult{New: newResult, Source: sourceResult}
	}
	return results, nil
}
