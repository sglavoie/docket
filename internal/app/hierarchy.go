package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Reparent changes a task's parent within its owning project. A nil newParentID
// promotes the task to a root. The new parent must exist in the same project and
// accept the task's type as a child, and the change must not create a circular
// parent reference; violating any of these rejects the request before any
// mutation. The task tree — levels and the max-nesting-depth cap — is rebuilt by
// the post-mutation hooks.
func (s *Service) Reparent(taskID string, newParentID *string) (TaskResult, error) {
	return s.reparent(taskID, TaskLocator{}, newParentID)
}

func (s *Service) ReparentByLocator(locator TaskLocator, newParentID *string) (TaskResult, error) {
	return s.reparent("", locator, newParentID)
}

func (s *Service) reparent(taskID string, locator TaskLocator, newParentID *string) (TaskResult, error) {
	return s.mutateTaskSelection(taskID, locator, "Reparent Task", "", func(ref TaskRef) error {
		if newParentID == nil {
			ref.Task.ParentID = nil
			ref.Task.UpdatedAt = s.clock.Now()
			return nil
		}
		parentID := strings.TrimSpace(*newParentID)
		if parentID == "" {
			return invalidInput("parent", "parent id is required")
		}
		if parentID == ref.Task.ID {
			return invalidInput("parent", "a task cannot be its own parent")
		}
		parent := ref.Project.GetTaskByID(parentID)
		if parent == nil {
			return fmt.Errorf("%w: parent %s", ErrTaskNotFound, parentID)
		}
		if !ref.Task.Type.CanBeChildOf(parent.Type) {
			return invalidInput("parent", fmt.Sprintf("%s cannot be a child of %s", ref.Task.Type, parent.Type))
		}
		taskMap := make(map[string]*models.Task, len(ref.Project.Tasks))
		for _, t := range ref.Project.Tasks {
			taskMap[t.ID] = t
		}
		if models.WouldCreateCycle(taskMap, ref.Task.ID, parentID) {
			return invalidInput("parent", "would create a circular parent reference")
		}
		ref.Task.ParentID = &parentID
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

// MoveTaskUp swaps a task with its previous sibling. It enforces the same
// user-visible guards the legacy CLI applied: a task already at the top of its
// siblings, or a completed root task that would cross above an uncompleted root
// sibling, is rejected with ErrInvalidInput before any mutation. The model swap is
// a silent no-op on those boundaries, so the guard is what produces the error the
// user sees.
func (s *Service) MoveTaskUp(taskID string) (TaskResult, error) {
	return s.moveTaskUp(taskID, TaskLocator{})
}

func (s *Service) MoveTaskUpByLocator(locator TaskLocator) (TaskResult, error) {
	return s.moveTaskUp("", locator)
}

func (s *Service) moveTaskUp(taskID string, locator TaskLocator) (TaskResult, error) {
	return s.mutateTaskSelection(taskID, locator, "Move Task Up", "", func(ref TaskRef) error {
		t := ref.Task
		siblings := ref.Project.GetSiblings(t)
		idx := ref.Project.FindSiblingIndex(t)
		if idx <= 0 {
			return invalidInput("order", "task is already at the top of its siblings")
		}
		if t.IsRoot() && t.IsComplete() && !siblings[idx-1].IsComplete() {
			return invalidInput("order", "cannot move completed task above uncompleted task")
		}
		return ref.Project.MoveTaskUp(t)
	})
}

// MoveTaskDown swaps a task with its next sibling, enforcing the mirror guards of
// MoveTaskUp: a task already at the bottom of its siblings, or an uncompleted root
// task that would cross below a completed root sibling, is rejected with
// ErrInvalidInput before any mutation.
func (s *Service) MoveTaskDown(taskID string) (TaskResult, error) {
	return s.moveTaskDown(taskID, TaskLocator{})
}

func (s *Service) MoveTaskDownByLocator(locator TaskLocator) (TaskResult, error) {
	return s.moveTaskDown("", locator)
}

func (s *Service) moveTaskDown(taskID string, locator TaskLocator) (TaskResult, error) {
	return s.mutateTaskSelection(taskID, locator, "Move Task Down", "", func(ref TaskRef) error {
		t := ref.Task
		siblings := ref.Project.GetSiblings(t)
		idx := ref.Project.FindSiblingIndex(t)
		if idx == -1 || idx >= len(siblings)-1 {
			return invalidInput("order", "task is already at the bottom of its siblings")
		}
		if t.IsRoot() && !t.IsComplete() && siblings[idx+1].IsComplete() {
			return invalidInput("order", "cannot move uncompleted task below completed task")
		}
		return ref.Project.MoveTaskDown(t)
	})
}

// MoveTaskToTop moves a task to the top of its siblings, one swap at a time,
// stopping at the completion boundary a completed root task cannot cross above an
// uncompleted sibling. Unlike MoveTaskUp it does NOT reject a task already at the
// top: reaching the edge is the operation's goal, so an already-top (or
// boundary-pinned) task commits unchanged, matching the idempotent attribute
// seams. Ordering is computed over MODEL siblings, not the outliner's VISIBLE
// (filtered/collapsed) siblings the legacy crossMoveTaskToPosition used; a caller
// that needs visible-order semantics must reconcile that before migrating.
func (s *Service) MoveTaskToTop(taskID string) (TaskResult, error) {
	return s.moveTaskToTop(taskID, TaskLocator{})
}

func (s *Service) MoveTaskToTopByLocator(locator TaskLocator) (TaskResult, error) {
	return s.moveTaskToTop("", locator)
}

func (s *Service) moveTaskToTop(taskID string, locator TaskLocator) (TaskResult, error) {
	return s.mutateTaskSelection(taskID, locator, "Move to Top", "", func(ref TaskRef) error {
		moveTaskToSiblingEdge(ref.Project, ref.Task, true)
		return nil
	})
}

// MoveTaskToBottom moves a task to the bottom of its siblings, mirroring
// MoveTaskToTop and honoring the mirror completion boundary (an uncompleted root
// cannot cross below a completed sibling).
func (s *Service) MoveTaskToBottom(taskID string) (TaskResult, error) {
	return s.moveTaskToBottom(taskID, TaskLocator{})
}

func (s *Service) MoveTaskToBottomByLocator(locator TaskLocator) (TaskResult, error) {
	return s.moveTaskToBottom("", locator)
}

func (s *Service) moveTaskToBottom(taskID string, locator TaskLocator) (TaskResult, error) {
	return s.mutateTaskSelection(taskID, locator, "Move to Bottom", "", func(ref TaskRef) error {
		moveTaskToSiblingEdge(ref.Project, ref.Task, false)
		return nil
	})
}

// moveTaskToSiblingEdge bubbles task toward the top (toTop) or bottom of its
// model-order siblings via repeated single-step model swaps, delegating the root
// completion boundary entirely to Project.MoveTaskUp/MoveTaskDown (each is a
// silent no-op at a boundary or edge). The loop terminates on the first swap that
// does not change the task's sibling index — the edge or the boundary.
func moveTaskToSiblingEdge(project *models.Project, task *models.Task, toTop bool) {
	for {
		before := project.FindSiblingIndex(task)
		if before < 0 {
			return
		}
		if toTop {
			if before == 0 {
				return
			}
			_ = project.MoveTaskUp(task)
		} else {
			if before >= len(project.GetSiblings(task))-1 {
				return
			}
			_ = project.MoveTaskDown(task)
		}
		if after := project.FindSiblingIndex(task); after == before {
			return
		}
	}
}

// IndentTask nests a task under the task visually above it, making that task its
// new parent. The target must be able to have children, the task's type must be
// valid under it, and the resulting subtree must not exceed the nesting-depth cap;
// any violation is rejected with ErrInvalidInput before any mutation. The
// visual-above semantics cannot be expressed by Reparent (which takes an explicit
// parent id), so this is a distinct ordering-aware use case.
func (s *Service) IndentTask(taskID string) (TaskResult, error) {
	return s.indentTask(taskID, TaskLocator{})
}

func (s *Service) IndentTaskByLocator(locator TaskLocator) (TaskResult, error) {
	return s.indentTask("", locator)
}

func (s *Service) indentTask(taskID string, locator TaskLocator) (TaskResult, error) {
	return s.mutateTaskSelection(taskID, locator, "Indent Task", "", func(ref TaskRef) error {
		t := ref.Task
		targetParent := ref.Project.GetTaskAbove(t)
		if targetParent == nil {
			return invalidInput("indent", "no task above to nest under")
		}
		if ok, reason := t.CanIndent(targetParent); !ok {
			return invalidInput("indent", reason)
		}
		if err := ref.Project.IndentTask(t); err != nil {
			return invalidInput("indent", err.Error())
		}
		return nil
	})
}

// OutdentTask moves a task to become a sibling of its current parent. The task must
// have a parent, and its type must remain valid at the new (grandparent) level;
// otherwise the request is rejected with ErrInvalidInput before any mutation.
func (s *Service) OutdentTask(taskID string) (TaskResult, error) {
	return s.outdentTask(taskID, TaskLocator{})
}

func (s *Service) OutdentTaskByLocator(locator TaskLocator) (TaskResult, error) {
	return s.outdentTask("", locator)
}

func (s *Service) outdentTask(taskID string, locator TaskLocator) (TaskResult, error) {
	return s.mutateTaskSelection(taskID, locator, "Outdent Task", "", func(ref TaskRef) error {
		t := ref.Task
		if ok, reason := t.CanOutdent(); !ok {
			return invalidInput("outdent", reason)
		}
		if err := ref.Project.OutdentTask(t); err != nil {
			return invalidInput("outdent", err.Error())
		}
		return nil
	})
}

// TaskTypeChange selects one of the type-hierarchy mutations. These change a task's
// type in place — they are not parent reparenting — so they are a distinct typed
// use case rather than a variant of Reparent.
type TaskTypeChange int

const (
	// PromoteType raises a task to the next higher type (Task/Bug/Idea → Story → Epic).
	PromoteType TaskTypeChange = iota
	// DemoteType lowers a task to the next lower type (Epic → Story → Task).
	DemoteType
	// CycleTypeRight cycles a leaf type right (Task → Bug → Idea → Task).
	CycleTypeRight
	// CycleTypeLeft cycles a leaf type left (Task → Idea → Bug → Task).
	CycleTypeLeft
)

// ChangeTaskType applies a type-hierarchy mutation (promote, demote, or leaf-type
// cycle) to a task. The model methods enforce the type-compatibility rules against
// the current parent and children and return the reason a change is rejected; that
// error is surfaced verbatim (matching the legacy CLI message) and aborts before
// any commit, so a rejected type change never persists. Type attributes are cleared
// on a successful change by the model method.
func (s *Service) ChangeTaskType(taskID string, change TaskTypeChange) (TaskResult, error) {
	return s.changeTaskType(taskID, TaskLocator{}, change)
}

func (s *Service) ChangeTaskTypeByLocator(locator TaskLocator, change TaskTypeChange) (TaskResult, error) {
	return s.changeTaskType("", locator, change)
}

func (s *Service) changeTaskType(taskID string, locator TaskLocator, change TaskTypeChange) (TaskResult, error) {
	undoAction, ok := typeChangeUndoAction(change)
	if !ok {
		return TaskResult{}, invalidInput("type change", "unknown type change operation")
	}
	return s.mutateTaskSelection(taskID, locator, undoAction, "", func(ref TaskRef) error {
		return applyTaskTypeChange(ref.Task, change)
	})
}

// ChangeTaskTypeWithWorkflow applies the same structural type mutation as
// ChangeTaskType and evaluates workflow rules contextually against the true
// pre-change type. This is required for type_changed event rules.
func (s *Service) ChangeTaskTypeWithWorkflow(taskID string, change TaskTypeChange, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.changeTaskTypeWithWorkflow(taskID, TaskLocator{}, change, opts)
}

func (s *Service) ChangeTaskTypeWithWorkflowByLocator(locator TaskLocator, change TaskTypeChange, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.changeTaskTypeWithWorkflow("", locator, change, opts)
}

func (s *Service) changeTaskTypeWithWorkflow(taskID string, locator TaskLocator, change TaskTypeChange, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	undoAction, ok := typeChangeUndoAction(change)
	if !ok {
		return BulkResult{}, invalidInput("type change", "unknown type change operation")
	}
	if opts.UndoAction == "" {
		opts.UndoAction = undoAction
	}
	return s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, _ *WorkspaceResolver) error {
		return applyTaskTypeChange(ref.Task, change)
	})
}

func applyTaskTypeChange(task *models.Task, change TaskTypeChange) error {
	switch change {
	case PromoteType:
		return task.Promote()
	case DemoteType:
		return task.Demote()
	case CycleTypeRight:
		return task.CycleLeafType(false)
	case CycleTypeLeft:
		return task.CycleLeafType(true)
	default:
		return invalidInput("type change", "unknown type change operation")
	}
}

// typeChangeUndoAction maps a type change to its undo-ledger label.
func typeChangeUndoAction(change TaskTypeChange) (string, bool) {
	switch change {
	case PromoteType:
		return "Promote Task", true
	case DemoteType:
		return "Demote Task", true
	case CycleTypeRight, CycleTypeLeft:
		return "Cycle Task Type", true
	default:
		return "", false
	}
}

// MoveResult reports the outcome of a cross-project subtree move.
type MoveResult struct {
	// Task is the presentation-neutral result for the moved root task in its new
	// project, carrying the destination-qualified reference and new short ID.
	Task TaskResult
	// MovedCount is the number of tasks relocated (the root plus its descendants).
	MovedCount int
	// ClearedDeps lists same-project dependency IDs dropped because they pointed
	// outside the moved subtree.
	ClearedDeps []string
	// SourceProjectID and DestProjectID identify the two projects the single
	// transaction touched.
	SourceProjectID string
	DestProjectID   string
	// SourceRevision and DestRevision are the new canonical revisions of the two
	// committed projects.
	SourceRevision string
	DestRevision   string
}

// MoveToProject moves a task and its whole subtree to another project, committing
// the source removal and the destination insertion as ONE transaction (two
// ChangeSets). Same-project dependencies that point outside the moved subtree are
// cleared; cross-project qualified references are preserved. Short IDs are
// regenerated in the destination while the stable UUID is kept, so each moved
// task's attachment directory is relocated (via an AttachmentMove op) in the same
// transaction as the metadata that references it. Any artifacts the caller supplies
// — the durable planning refs (Focus Queue, Daily Plan) reassigned to the
// destination project — ride the same transaction so a partial move can never leave
// planning refs pointing at the old project. Machine-local UI state (recent visits)
// is deliberately NOT part of this transaction; the adapter updates it separately
// after a successful commit.
func (s *Service) MoveToProject(taskID, destProjectID string, artifacts ...storage.ArtifactChange) (MoveResult, error) {
	return s.moveToProject(taskID, TaskLocator{}, destProjectID, artifacts)
}

func (s *Service) MoveToProjectByLocator(locator TaskLocator, destProjectID string, artifacts ...storage.ArtifactChange) (MoveResult, error) {
	return s.moveToProject("", locator, destProjectID, artifacts)
}

func (s *Service) moveToProject(taskID string, locator TaskLocator, destProjectID string, artifacts []storage.ArtifactChange) (MoveResult, error) {
	if strings.TrimSpace(destProjectID) == "" {
		return MoveResult{}, invalidInput("destination", "project id is required")
	}
	ref, snap, srcRev, err := s.loadTaskSelection(taskID, locator)
	if err != nil {
		return MoveResult{}, err
	}
	src := ref.Project
	if src.ID == destProjectID {
		return MoveResult{}, invalidInput("destination", "task is already in that project")
	}
	dest := snap.ProjectByID(destProjectID)
	if dest == nil {
		return MoveResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, destProjectID)
	}
	destRev := snap.Revision(dest.ID)

	moveRes, err := src.MoveTaskToProject(ref.Task, dest)
	if err != nil {
		return MoveResult{}, invalidInput("move", err.Error())
	}

	// Relocate every moved task's attachment directory in the same transaction. The
	// stable UUID is preserved across the move, so the destination metadata keeps
	// referencing the same filenames; without the move op those files would be
	// orphaned in the source project's tree. A move is a no-op when the task has no
	// attachments directory (resolveAttachmentMove handles that).
	attachmentOps := make([]storage.AttachmentOp, 0, len(moveRes.MovedTasks))
	for _, mt := range moveRes.MovedTasks {
		attachmentOps = append(attachmentOps, storage.AttachmentOp{
			Kind:          storage.AttachmentMove,
			Task:          mt,
			SourceProject: src,
			DestProject:   dest,
		})
	}

	results, err := s.commitProjects([]projectCommit{
		{project: src, expectedRevision: srcRev, base: ref.Base, artifacts: artifacts, attachmentOps: attachmentOps},
		{project: dest, expectedRevision: destRev, base: snap.Document(dest.ID)},
	}, "Move Task to Project", fmt.Sprintf("#%d → %s", ref.Task.ShortID, dest.Slug))
	if err != nil {
		return MoveResult{}, err
	}

	srcNewRev, destNewRev := revisionAt(results.results, 0), revisionAt(results.results, 1)
	return MoveResult{
		Task:            newTaskResult(TaskRef{Task: ref.Task, Project: dest}, destNewRev),
		MovedCount:      len(moveRes.MovedTasks),
		ClearedDeps:     moveRes.ClearedDeps,
		SourceProjectID: src.ID,
		DestProjectID:   dest.ID,
		SourceRevision:  srcNewRev,
		DestRevision:    destNewRev,
	}, nil
}
