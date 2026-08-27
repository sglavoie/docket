package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

const operationTaskDelete OperationID = "tui.task.delete"

// executeTaskDeleteOperation turns the frozen action-sheet targets into the one
// canonical delete request. The registry never mutates task data: confirmation
// and the atomic transaction stay in handleTaskDeleteRequested/confirmTaskDelete,
// which is why the binding delegates confirmation instead of adding a second,
// generic prompt in front of the canonical one.
func (m *AppModel) executeTaskDeleteOperation(
	invocation OperationInvocation,
) OperationHandlerResult {
	refs := make([]TaskProjectRef, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		task, project := m.taskAndProjectForIdentity(target.Task)
		if task == nil || project == nil {
			return OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
		}
		refs = append(refs, TaskProjectRef{Task: task, Project: project})
	}
	if len(refs) == 0 {
		return OperationHandlerResult{
			Err: fmt.Errorf("operation %s requires at least one task", invocation.Descriptor.ID),
		}
	}
	return OperationHandlerResult{Message: TaskDeleteRequestedMsg{
		Refs:      refs,
		Selection: deleteSelectionCounts(invocation.Context, len(refs)),
	}}
}

func deleteSelectionCounts(context CommandContext, targetCount int) SelectionCounts {
	counts := SelectionCounts{Total: targetCount, Visible: targetCount}
	if context.Selection.VisibleCount+context.Selection.HiddenCount == targetCount {
		counts.Visible = context.Selection.VisibleCount
		counts.Hidden = context.Selection.HiddenCount
	}
	return counts
}

// TaskDeleteRequestedMsg is the canonical, shell-owned request to permanently
// delete one or more task subtrees. Any task surface emits it instead of
// running its own delete flow: the shell
// canonicalizes the roots, prompts for confirmation, and — only after the user
// confirms — executes ONE atomic BulkCoordinator.DeleteLocators transaction that folds
// the durable planning-ref cleanup into the same commit. Refs carry each root
// task with its owning project so heterogeneous cross-project selections delete
// together.
type TaskDeleteRequestedMsg struct {
	Refs      []TaskProjectRef
	Selection SelectionCounts
}

// taskDeletePending holds the canonicalized roots and origin for an in-flight
// shell-owned task deletion awaiting confirmation.
type taskDeletePending struct {
	roots    []TaskProjectRef
	selected []TaskProjectRef
}

type taskDeleteImpact struct {
	selectedRoots     int
	descendants       int
	totalTasks        int
	selection         SelectionCounts
	storedAttachments int
	linkedAttachments int
}

const taskDeleteConfirmAction = "task-delete"

// handleTaskDeleteRequested canonicalizes the requested roots, resolves the
// confirmation prompt, and arms the shell-owned confirm dialog. It never mutates
// task data — the delete transaction runs only after the user confirms in
// confirmTaskDelete. An empty or fully-invalid request is a no-op.
func (m *AppModel) handleTaskDeleteRequested(msg TaskDeleteRequestedMsg) {
	requested := canonicalizeTaskDeleteRoots(msg.Refs)
	if len(requested) == 0 {
		return
	}
	roots := collapseNestedTaskDeleteRoots(requested)
	selection := msg.Selection
	if selection.Total != len(requested) || selection.Visible+selection.Hidden != selection.Total {
		selection = SelectionCounts{Total: len(requested), Visible: len(requested)}
	}
	impact := calculateTaskDeleteImpact(roots, selection)
	m.pendingTaskDelete = taskDeletePending{
		roots: roots, selected: requested,
	}
	m.taskDeleteConfirm.ShowWithDetail(
		taskDeleteConfirmAction,
		taskDeletePrompt(roots),
		taskDeleteConfirmDetail(roots, impact),
	)
}

// canonicalizeTaskDeleteRoots drops refs with a nil task/project or blank ID and
// removes duplicate roots by project-qualified identity, preserving first-seen
// order. Descendant resolution and nested-root collapsing happen later (in
// confirmTaskDelete and the application service); this only shapes the
// confirmation summary.
func canonicalizeTaskDeleteRoots(refs []TaskProjectRef) []TaskProjectRef {
	seen := make(map[TaskIdentity]bool, len(refs))
	roots := make([]TaskProjectRef, 0, len(refs))
	for _, ref := range refs {
		if ref.Task == nil || ref.Project == nil || ref.Task.ID == "" {
			continue
		}
		identity := ref.Identity()
		if seen[identity] {
			continue
		}
		seen[identity] = true
		roots = append(roots, ref)
	}
	return roots
}

// collapseNestedTaskDeleteRoots removes a selected task when an ancestor in the
// same project is already selected. The ancestor carries that entire subtree,
// so these are the exact effective roots used by both impact accounting and the
// application transaction.
func collapseNestedTaskDeleteRoots(refs []TaskProjectRef) []TaskProjectRef {
	selected := make(map[TaskIdentity]bool, len(refs))
	for _, ref := range refs {
		selected[ref.Identity()] = true
	}
	roots := make([]TaskProjectRef, 0, len(refs))
	for _, ref := range refs {
		ancestorSelected := false
		seen := make(map[string]bool)
		parentID := ref.Task.ParentID
		for parentID != nil && *parentID != "" && !seen[*parentID] {
			seen[*parentID] = true
			if selected[TaskIdentity{ProjectID: ref.Project.ID, TaskID: *parentID}] {
				ancestorSelected = true
				break
			}
			parent := ref.Project.GetTaskByID(*parentID)
			if parent == nil {
				break
			}
			parentID = parent.ParentID
		}
		if !ancestorSelected {
			roots = append(roots, ref)
		}
	}
	return roots
}

func calculateTaskDeleteImpact(roots []TaskProjectRef, selection SelectionCounts) taskDeleteImpact {
	rootSet := make(map[TaskIdentity]bool, len(roots))
	projects := make(map[string]*models.Project)
	for _, root := range roots {
		rootSet[root.Identity()] = true
		projects[root.Project.ID] = root.Project
	}
	removed := make(map[TaskIdentity]*models.Task)
	for projectID, project := range projects {
		for _, task := range project.Tasks {
			identity := TaskIdentity{ProjectID: projectID, TaskID: task.ID}
			if taskDeleteRootAncestor(identity, task, project, rootSet) {
				removed[identity] = task
			}
		}
	}
	impact := taskDeleteImpact{
		selectedRoots: len(roots),
		totalTasks:    len(removed),
		selection:     selection,
	}
	impact.descendants = max(impact.totalTasks-impact.selectedRoots, 0)
	tasks := make([]*models.Task, 0, len(removed))
	for _, task := range removed {
		tasks = append(tasks, task)
	}
	impact.storedAttachments, impact.linkedAttachments = countAttachmentImpact(tasks)
	return impact
}

// countAttachmentImpact splits the attachments a deletion destroys from the ones
// it merely forgets. Every surface that warns about a delete reads this, so a
// stored file and a link can never be counted differently in two places.
func countAttachmentImpact(tasks []*models.Task) (stored, linked int) {
	for _, task := range tasks {
		for _, attachment := range task.Attachments {
			if attachment.IsURL || attachment.IsCodeRef {
				linked++
			} else {
				stored++
			}
		}
	}
	return stored, linked
}

func attachmentImpactNote(stored, linked int) string {
	return fmt.Sprintf(
		"attachments: %d stored file(s) deleted · %d URL/code reference(s) removed",
		stored,
		linked,
	)
}

func taskDeleteRootAncestor(
	identity TaskIdentity,
	task *models.Task,
	project *models.Project,
	roots map[TaskIdentity]bool,
) bool {
	seen := make(map[string]bool)
	for task != nil && !seen[task.ID] {
		seen[task.ID] = true
		identity.TaskID = task.ID
		if roots[identity] {
			return true
		}
		if task.ParentID == nil || *task.ParentID == "" {
			return false
		}
		task = project.GetTaskByID(*task.ParentID)
	}
	return false
}

func taskDeleteConfirmDetail(roots []TaskProjectRef, impact taskDeleteImpact) *ConfirmDetail {
	items := make([]string, 0, min(len(roots), 5))
	for _, root := range roots {
		if len(items) == 5 {
			break
		}
		items = append(items, fmt.Sprintf("%s:#%d %s", root.Project.Slug, root.Task.ShortID, root.Task.Title))
	}
	attachmentImpact := attachmentImpactNote(impact.storedAttachments, impact.linkedAttachments)
	return &ConfirmDetail{
		Items:      items,
		TotalCount: len(roots),
		CascadeNote: fmt.Sprintf(
			"%d selected root(s) · %d deduplicated descendant(s) · %d task(s) total · %d selected (%d visible · %d hidden) · %s",
			impact.selectedRoots,
			impact.descendants,
			impact.totalTasks,
			impact.selection.Total,
			impact.selection.Visible,
			impact.selection.Hidden,
			attachmentImpact,
		),
		UndoHint: true,
	}
}

// taskDeletePrompt renders "Delete task #N?" for a single root or
// "Delete N tasks?" for several, matching the canonical deletion grammar.
func taskDeletePrompt(roots []TaskProjectRef) string {
	if len(roots) == 1 {
		return fmt.Sprintf("Delete task #%d?", roots[0].Task.ShortID)
	}
	return fmt.Sprintf("Delete %d tasks?", len(roots))
}

// handleTaskDeleteConfirmKey gives the shell-owned task-delete confirmation the
// same modal key priority as project archive: while it is active it consumes
// every key (confirm on y/Y, cancel on n/N/Esc, swallow the rest) so nothing
// leaks into the underlying view. It returns handled=false only when no delete
// confirmation is pending.
func (m *AppModel) handleTaskDeleteConfirmKey(key string) (bool, tea.Cmd) {
	if !m.taskDeleteConfirm.IsActive() {
		return false, nil
	}
	confirmed, handled := m.taskDeleteConfirm.HandleKey(key)
	if !handled {
		return false, nil
	}
	if confirmed {
		return true, m.confirmTaskDelete()
	}
	switch key {
	case "n", "N", "esc":
		m.clearPendingTaskDelete()
	}
	return true, nil
}

// confirmTaskDelete executes the canonical deletion transaction for the pending
// roots. It runs ONE BulkCoordinator.DeleteLocators for both single- and multi-root
// deletion; the application service expands each subtree and folds canonical
// Today/Next cleanup into the checked project transaction. The
// returned command is decorated so the committed batch clears the requested
// selection IDs after the commit (finalizeLocatedDeleteBatchResult passes a TransactionError
// through untouched, leaving state intact on failure). An artifact-build error
// aborts before any mutation and preserves the pending selection's task data.
func (m *AppModel) confirmTaskDelete() tea.Cmd {
	roots := m.pendingTaskDelete.roots
	selected := m.pendingTaskDelete.selected
	m.clearPendingTaskDelete()
	if len(roots) == 0 {
		return nil
	}
	m.appService.WithPlanningArtifacts(shellStructuralPlanningArtifactBuilder(m.runtime, m.configManager))

	locators := taskLocatorsFromTaskRefs(roots)
	cmd := m.secondaryFeatures.Bulk.DeleteLocators(locators)
	if len(selected) == 0 {
		selected = roots
	}
	return finalizeLocatedDeleteBatchResult(cmd, taskLocatorsFromTaskRefs(selected))
}

// clearPendingTaskDelete discards the pending roots and dismisses the confirm
// dialog. It runs on cancellation and after the transaction is dispatched so the
// shell never re-prompts for a stale selection.
func (m *AppModel) clearPendingTaskDelete() {
	m.pendingTaskDelete = taskDeletePending{}
	m.taskDeleteConfirm.Cancel()
}
