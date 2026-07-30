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

// Reparent changes a task's parent within its owning project. A nil newParentID
// promotes the task to a root. The new parent must exist in the same project and
// and the change must not create a circular
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
	return s.mutateHierarchyTaskSelection(taskID, locator, "Reparent Task", "", func(ref TaskRef) error {
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
	return s.mutateHierarchyTaskSelection(taskID, locator, "Move Task Up", "", func(ref TaskRef) error {
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
	return s.mutateHierarchyTaskSelection(taskID, locator, "Move Task Down", "", func(ref TaskRef) error {
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
	return s.mutateHierarchyTaskSelection(taskID, locator, "Move to Top", "", func(ref TaskRef) error {
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
	return s.mutateHierarchyTaskSelection(taskID, locator, "Move to Bottom", "", func(ref TaskRef) error {
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
	return s.mutateHierarchyTaskSelection(taskID, locator, "Indent Task", "", func(ref TaskRef) error {
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
	return s.mutateHierarchyTaskSelection(taskID, locator, "Outdent Task", "", func(ref TaskRef) error {
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

// ReorderSiblingTasks swaps two explicitly selected siblings as one checked
// project transaction. A filtered or collapsed presentation can pass the
// adjacent visible row directly; hidden siblings keep their own relative order.
func (s *Service) ReorderSiblingTasks(taskID, targetID string) (TaskResult, error) {
	return s.reorderSiblingTasks(taskID, TaskLocator{}, targetID, TaskLocator{})
}

func (s *Service) ReorderSiblingTasksByLocator(task, target TaskLocator) (TaskResult, error) {
	return s.reorderSiblingTasks("", task, "", target)
}

func (s *Service) reorderSiblingTasks(taskID string, taskLocator TaskLocator, targetID string, targetLocator TaskLocator) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection(taskID, taskLocator)
	if err != nil {
		return TaskResult{}, err
	}
	target, err := resolveTaskSelection(snap, targetID, targetLocator)
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
			value := *task
			// Dependency cleanup filters slices in place. Preserve its
			// backing values so a failed multi-project move can restore refs even
			// when a removed entry preceded a retained entry.
			value.DependsOn = append([]string(nil), task.DependsOn...)
			backup.values[i] = value
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

func (s *Service) mutateHierarchyTaskSelection(taskID string, locator TaskLocator, undoAction, undoContext string, mutate func(TaskRef) error) (TaskResult, error) {
	ref, _, revision, err := s.loadTaskSelection(taskID, locator)
	if err != nil {
		return TaskResult{}, err
	}
	backup := captureHierarchyProject(ref.Project)
	if mutateErr := mutate(ref); mutateErr != nil {
		backup.restore()
		return TaskResult{}, mutateErr
	}
	result, err := s.commitProject(ref, revision, undoAction, undoContext)
	if err != nil {
		backup.restore()
		return TaskResult{}, err
	}
	return result, nil
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
	// CrossProjectRefsCleared counts dependency references dropped
	// because they still qualified a moved task through its former project.
	CrossProjectRefsCleared int
	// TouchedProjectIDs lists every project committed in the transaction: source
	// and destination first, followed by cross-project-reference repairs in
	// snapshot order.
	TouchedProjectIDs []string
	// SourceProjectID and DestProjectID identify the move endpoints.
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
// planning refs pointing at the old project. Qualified dependency references
// from any loaded project that still point at the moved tasks through the source
// project are cleared in the same transaction. Machine-local UI state (recent visits)
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
	srcBackup := captureHierarchyProject(src)
	destBackup := captureHierarchyProject(dest)
	backups := []hierarchyProjectBackup{srcBackup, destBackup}
	restore := func() {
		for i := len(backups) - 1; i >= 0; i-- {
			backups[i].restore()
		}
	}

	moveRes, err := src.MoveTaskToProject(ref.Task, dest)
	if err != nil {
		restore()
		return MoveResult{}, invalidInput("move", err.Error())
	}
	movedIDs := make(map[string]bool, len(moveRes.MovedTasks))
	for _, moved := range moveRes.MovedTasks {
		if moved != nil {
			movedIDs[moved.ID] = true
		}
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

	commits := []projectCommit{
		{project: src, expectedRevision: srcRev, base: ref.Base, artifacts: artifacts, attachmentOps: attachmentOps},
		{project: dest, expectedRevision: destRev, base: snap.Document(dest.ID)},
	}
	touched := []string{src.ID, dest.ID}
	crossCleared := 0
	for _, project := range snap.Projects() {
		if project == nil {
			continue
		}
		if project.ID == src.ID || project.ID == dest.ID {
			crossCleared += clearCrossProjectRefs(project, src, movedIDs)
			continue
		}
		backup := captureHierarchyProject(project)
		cleared := clearCrossProjectRefs(project, src, movedIDs)
		if cleared == 0 {
			continue
		}
		backups = append(backups, backup)
		crossCleared += cleared
		commits = append(commits, projectCommit{
			project:          project,
			expectedRevision: snap.Revision(project.ID),
			base:             snap.Document(project.ID),
		})
		touched = append(touched, project.ID)
	}

	results, err := s.commitProjects(commits, "Move Task to Project", fmt.Sprintf("#%d → %s", ref.Task.ShortID, dest.Slug))
	if err != nil {
		restore()
		return MoveResult{}, err
	}

	srcNewRev, destNewRev := revisionAt(results.results, 0), revisionAt(results.results, 1)
	return MoveResult{
		Task:                    newTaskResult(TaskRef{Task: ref.Task, Project: dest}, destNewRev),
		MovedCount:              len(moveRes.MovedTasks),
		ClearedDeps:             moveRes.ClearedDeps,
		CrossProjectRefsCleared: crossCleared,
		TouchedProjectIDs:       touched,
		SourceProjectID:         src.ID,
		DestProjectID:           dest.ID,
		SourceRevision:          srcNewRev,
		DestRevision:            destNewRev,
	}, nil
}
