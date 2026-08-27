package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// resolveChildParent resolves a requested parent for a task that is about to be
// created and validates the relationship the domain accepts: the parent exists
// in the same project and the child would not land past the nesting cap.
func resolveChildParent(project *models.Project, parentID string) (*models.Task, error) {
	parent := project.GetTaskByID(parentID)
	if parent == nil {
		return nil, fmt.Errorf("%w: parent %s", ErrTaskNotFound, parentID)
	}
	if parent.Level+1 > models.MaxNestingDepth {
		return nil, invalidInput("parent", fmt.Sprintf("nesting under #%d would exceed maximum depth of %d levels", parent.ShortID, models.MaxNestingDepth))
	}
	return parent, nil
}

// resolveAdoptedChildren resolves every requested child to the canonical task
// object the project owns. Repeated IDs collapse to their first occurrence, so
// a caller that already deduplicated aliases and one that did not produce the
// same edge set. Depth and cycles are deliberately not checked here: an adopted
// child is only legal in combination with the new task's own parent edge, which
// the single ApplyParentAssignments call validates as one graph.
func resolveAdoptedChildren(project *models.Project, childIDs []string) ([]*models.Task, error) {
	if len(childIDs) == 0 {
		return nil, nil
	}
	children := make([]*models.Task, 0, len(childIDs))
	seen := make(map[string]bool, len(childIDs))
	for _, raw := range childIDs {
		childID := strings.TrimSpace(raw)
		if childID == "" {
			return nil, invalidInput("child", "child id is required")
		}
		if seen[childID] {
			continue
		}
		child := project.GetTaskByID(childID)
		if child == nil {
			return nil, fmt.Errorf("%w: child %s", ErrTaskNotFound, childID)
		}
		seen[childID] = true
		children = append(children, child)
	}
	return children, nil
}

// Reparent changes a task's parent within its owning project. A nil
// newParentID promotes the task to a root. The new parent must exist in the same
// project and the change must not create a circular
// parent reference; violating any of these rejects the request before any
// mutation. The task tree — levels and the max-nesting-depth cap — is rebuilt by
// the post-mutation hooks.
func (s *Service) Reparent(locator TaskLocator, newParentID *string) (TaskResult, error) {
	return s.mutateHierarchyTaskSelection(locator, "Reparent Task", func(ref TaskRef) error {
		if newParentID == nil {
			if err := ref.Project.ReparentTask(ref.Task, nil); err != nil {
				return invalidInput("parent", err.Error())
			}
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
		if err := ref.Project.ReparentTask(ref.Task, parent); err != nil {
			return invalidInput("parent", err.Error())
		}
		return nil
	})
}

// MoveTaskUp swaps a task with its previous sibling. It enforces the
// same user-visible guards the legacy CLI applied: a task already at the top of
// its siblings, or a completed root task that would cross above an uncompleted
// root sibling, is rejected with ErrInvalidInput before any mutation. The model
// swap is a silent no-op on those boundaries, so the guard is what produces the
// error the user sees.
func (s *Service) MoveTaskUp(locator TaskLocator) (TaskResult, error) {
	return s.mutateHierarchyTaskSelection(locator, "Move Task Up", func(ref TaskRef) error {
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

// MoveTaskDown swaps a task with its next sibling, enforcing the mirror
// guards of MoveTaskUp: a task already at the bottom of its siblings, or
// an uncompleted root task that would cross below a completed root sibling, is
// rejected with ErrInvalidInput before any mutation.
func (s *Service) MoveTaskDown(locator TaskLocator) (TaskResult, error) {
	return s.mutateHierarchyTaskSelection(locator, "Move Task Down", func(ref TaskRef) error {
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

// MoveTaskToTop moves a task to the top of its siblings, one swap at a
// time, stopping at the completion boundary a completed root task cannot cross
// above an uncompleted sibling. Unlike MoveTaskUp it does NOT reject a task
// already at the top: reaching the edge is the operation's goal, so an
// already-top (or boundary-pinned) task commits unchanged, matching the
// idempotent attribute seams. Ordering is computed over MODEL siblings, not the
// outliner's VISIBLE (filtered/collapsed) siblings the legacy
// crossMoveTaskToPosition used; a caller that needs visible-order semantics must
// reconcile that before migrating.
func (s *Service) MoveTaskToTop(locator TaskLocator) (TaskResult, error) {
	return s.mutateHierarchyTaskSelection(locator, "Move to Top", func(ref TaskRef) error {
		moveTaskToSiblingEdge(ref.Project, ref.Task, true)
		return nil
	})
}

// MoveTaskToBottom moves a task to the bottom of its siblings,
// mirroring MoveTaskToTop and honoring the mirror completion boundary
// (an uncompleted root cannot cross below a completed sibling).
func (s *Service) MoveTaskToBottom(locator TaskLocator) (TaskResult, error) {
	return s.mutateHierarchyTaskSelection(locator, "Move to Bottom", func(ref TaskRef) error {
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

// IndentTask nests a task under the task visually above it, making that
// task its new parent. The target must be able to have children, the task's type
// must be valid under it, and the resulting subtree must not exceed the
// nesting-depth cap; any violation is rejected with ErrInvalidInput before any
// mutation. The visual-above semantics cannot be expressed by Reparent
// (which takes an explicit parent id), so this is a distinct ordering-aware use
// case.
func (s *Service) IndentTask(locator TaskLocator) (TaskResult, error) {
	return s.mutateHierarchyTaskSelection(locator, "Indent Task", func(ref TaskRef) error {
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

// OutdentTask moves a task to become a sibling of its current parent.
// The task must have a parent, and its type must remain valid at the new
// (grandparent) level; otherwise the request is rejected with ErrInvalidInput
// before any mutation.
func (s *Service) OutdentTask(locator TaskLocator) (TaskResult, error) {
	return s.mutateHierarchyTaskSelection(locator, "Outdent Task", func(ref TaskRef) error {
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

// ReorderSiblingTasks swaps two explicitly selected siblings as one
// checked project transaction. A filtered or collapsed presentation can pass the
// adjacent visible row directly; hidden siblings keep their own relative order.
func (s *Service) ReorderSiblingTasks(taskLocator, targetLocator TaskLocator) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection("", taskLocator)
	if err != nil {
		return TaskResult{}, err
	}
	target, err := resolveTaskSelection(snap, "", targetLocator)
	if err != nil {
		return TaskResult{}, err
	}
	if ref.Project.ID != target.Project.ID {
		return TaskResult{}, invalidInput("order", "tasks must belong to the same project")
	}
	if ref.Task.ID == target.Task.ID {
		return TaskResult{}, invalidInput("order", "task and reorder target must be different")
	}

	backup := captureHierarchyProject(ref.Project)
	if swapErr := ref.Project.SwapSiblingTasks(ref.Task, target.Task); swapErr != nil {
		backup.restore()
		return TaskResult{}, invalidInput("order", swapErr.Error())
	}
	result, err := s.commitProject(ref, revision, "Reorder Siblings", fmt.Sprintf("#%d <-> #%d", ref.Task.ShortID, target.Task.ShortID))
	if err != nil {
		backup.restore()
		return TaskResult{}, err
	}
	return result, nil
}

// ReorderSiblingOrder permutes an explicitly named set of siblings into
// the given top-to-bottom order as one checked project transaction. It is what a
// multi-step move collapses to: the presentation layer names the visible peers in
// their final order and only the slots those peers already hold are rewritten, so
// siblings a filter or a collapsed parent hides keep their positions. moved names
// the task the request is about, which the order alone cannot say, and the
// transaction is recorded against it.
func (s *Service) ReorderSiblingOrder(moved TaskLocator, order []TaskLocator) (TaskResult, error) {
	if len(order) < 2 {
		return TaskResult{}, invalidInput("order", "reordering needs at least two tasks")
	}
	ref, snap, revision, err := s.loadTaskSelection("", moved)
	if err != nil {
		return TaskResult{}, err
	}
	tasks := make([]*models.Task, 0, len(order))
	seen := make(map[string]bool, len(order))
	inOrder := false
	for _, locator := range order {
		peer, resolveErr := resolveTaskSelection(snap, "", locator)
		if resolveErr != nil {
			return TaskResult{}, resolveErr
		}
		if peer.Project.ID != ref.Project.ID {
			return TaskResult{}, invalidInput("order", "tasks must belong to the same project")
		}
		if seen[peer.Task.ID] {
			return TaskResult{}, invalidInput("order", "a task may appear only once in the reorder order")
		}
		seen[peer.Task.ID] = true
		inOrder = inOrder || peer.Task.ID == ref.Task.ID
		tasks = append(tasks, peer.Task)
	}
	if !inOrder {
		return TaskResult{}, invalidInput("order", "the moved task must appear in the reorder order")
	}

	backup := captureHierarchyProject(ref.Project)
	if reorderErr := ref.Project.ReorderSiblingTasks(tasks); reorderErr != nil {
		backup.restore()
		return TaskResult{}, invalidInput("order", reorderErr.Error())
	}
	result, err := s.commitProject(ref, revision, "Reorder Siblings", fmt.Sprintf("#%d", ref.Task.ShortID))
	if err != nil {
		backup.restore()
		return TaskResult{}, err
	}
	return result, nil
}

// hierarchyProjectBackup preserves repository-owned pointers while recording
// pre-mutation values. It makes hierarchy atomicity observable even for
// in-memory repositories whose snapshots share objects with their callers.
type hierarchyProjectBackup struct {
	project *models.Project
	value   models.Project
	tasks   []*models.Task
	values  []models.Task
}

func captureHierarchyProject(project *models.Project) hierarchyProjectBackup {
	backup := hierarchyProjectBackup{project: project}
	if project == nil {
		return backup
	}
	backup.value = *project
	backup.tasks = append([]*models.Task(nil), project.Tasks...)
	backup.values = make([]models.Task, len(backup.tasks))
	for i, task := range backup.tasks {
		if task != nil {
			backup.values[i] = *task
		}
	}
	return backup
}

func (backup hierarchyProjectBackup) restore() {
	if backup.project == nil {
		return
	}
	for i, task := range backup.tasks {
		if task != nil {
			*task = backup.values[i]
		}
	}
	value := backup.value
	value.Tasks = append([]*models.Task(nil), backup.tasks...)
	*backup.project = value
	backup.project.BuildTaskTree()
}

func (s *Service) mutateHierarchyTaskSelection(locator TaskLocator, undoAction string, mutate func(TaskRef) error) (TaskResult, error) {
	ref, _, revision, err := s.loadTaskSelection("", locator)
	if err != nil {
		return TaskResult{}, err
	}
	backup := captureHierarchyProject(ref.Project)
	if mutateErr := mutate(ref); mutateErr != nil {
		backup.restore()
		return TaskResult{}, mutateErr
	}
	result, err := s.commitProject(ref, revision, undoAction, "")
	if err != nil {
		backup.restore()
		return TaskResult{}, err
	}
	return result, nil
}

// crossProjectSubtreeMove is the application-owned delta shared by every
// cross-project task move. The model performs the structural transfer and this
// value carries the qualified planning rewrites and attachment relocations that
// must commit beside the two project documents.
type crossProjectSubtreeMove struct {
	MovedTasks    []*models.Task
	PlanningMoves []PlanningProjectMove
	AttachmentOps []storage.AttachmentOp
}

// moveTaskSubtreeBetweenProjects performs the in-memory portion of a
// cross-project move. Callers own rollback until their grouped commit succeeds.
// An optional destination parent is applied only after the complete subtree is
// present in the destination, so ReparentTask can validate the final depth and
// canonical ownership before either project document is committed.
func moveTaskSubtreeBetweenProjects(source *models.Project, task *models.Task, destination *models.Project, parent *models.Task) (crossProjectSubtreeMove, error) {
	moveResult, err := source.MoveTaskToProject(task, destination)
	if err != nil {
		return crossProjectSubtreeMove{}, err
	}
	if parent != nil {
		if err := destination.ReparentTask(task, parent); err != nil {
			return crossProjectSubtreeMove{}, err
		}
	}

	result := crossProjectSubtreeMove{
		MovedTasks:    moveResult.MovedTasks,
		PlanningMoves: make([]PlanningProjectMove, 0, len(moveResult.MovedTasks)),
		AttachmentOps: make([]storage.AttachmentOp, 0, len(moveResult.MovedTasks)),
	}
	for _, moved := range moveResult.MovedTasks {
		if moved == nil {
			continue
		}
		result.PlanningMoves = append(result.PlanningMoves, PlanningProjectMove{
			TaskID: moved.ID, FromProjectID: source.ID, ToProjectID: destination.ID,
		})
		result.AttachmentOps = append(result.AttachmentOps, storage.AttachmentOp{
			Kind:          storage.AttachmentMove,
			Task:          moved,
			SourceProject: source,
			DestProject:   destination,
		})
	}
	return result, nil
}
